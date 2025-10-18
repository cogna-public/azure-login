// Package auth provides Azure authentication functionality using OIDC token exchange.
//
// This package handles the authentication flow from GitHub Actions OIDC tokens
// to Azure AD access tokens using OAuth 2.0 client credentials flow with
// federated identity.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cogna-public/azure-login/internal/retry"
)

const (
	// AzureTokenExchangeTimeout is the maximum time to wait for Azure AD token exchange.
	// This is set to fail fast on transient issues, since the retry logic will handle
	// retries with exponential backoff.
	// With 3 retries and default backoff (1s, 2s), total worst case: ~33 seconds
	AzureTokenExchangeTimeout = 10 * time.Second
)

// TokenResponse represents the response from Azure AD token endpoint
type TokenResponse struct {
	AccessToken    string    `json:"access_token"`
	TokenType      string    `json:"token_type"`
	ExpiresIn      int       `json:"expires_in"`
	ExtExpiresIn   int       `json:"ext_expires_in,omitempty"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	ExpiresOn      time.Time `json:"-"`
	TenantID       string    `json:"-"`
	ClientID       string    `json:"-"`
	SubscriptionID string    `json:"-"`
}

// Client handles Azure AD authentication
type Client struct {
	tenantID       string
	clientID       string
	subscriptionID string
	scope          string
	httpClient     *http.Client
}

// NewClient creates a new authentication client with default scope for Azure Resource Management
func NewClient(tenantID, clientID, subscriptionID string) *Client {
	return NewClientWithScope(tenantID, clientID, subscriptionID, "https://management.azure.com/.default")
}

// NewClientWithScope creates a new authentication client with a custom OAuth2 scope
func NewClientWithScope(tenantID, clientID, subscriptionID, scope string) *Client {
	return &Client{
		tenantID:       tenantID,
		clientID:       clientID,
		subscriptionID: subscriptionID,
		scope:          scope,
		httpClient: &http.Client{
			Timeout: AzureTokenExchangeTimeout,
			// Disable redirects for security (prevents redirect-based attacks)
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ExchangeOIDCToken exchanges a GitHub OIDC token for an Azure access token
func (c *Client) ExchangeOIDCToken(ctx context.Context, oidcToken string) (*TokenResponse, error) {
	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)

	// Prepare form data for token exchange
	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", oidcToken)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", c.scope)

	// Load retry configuration
	retryConfig := retry.LoadConfig()

	var tokenResp *TokenResponse
	err := retryConfig.Do(ctx, func() error {
		// Create request
		req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create token request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to exchange token: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// Limit response body to 1MB to prevent memory exhaustion
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			// Try to parse error response
			var errorResp struct {
				Error            string `json:"error"`
				ErrorDescription string `json:"error_description"`
			}
			if err := json.Unmarshal(body, &errorResp); err == nil {
				// Sanitize error description to avoid leaking sensitive data
				return fmt.Errorf("authentication failed: %s (check credentials and federated identity configuration)", errorResp.Error)
			}
			return fmt.Errorf("authentication failed with status %d (check credentials and network connectivity)", resp.StatusCode)
		}

		// Parse successful response
		var response TokenResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to parse token response: %w", err)
		}

		// Calculate expiration time (use UTC to avoid timezone issues)
		response.ExpiresOn = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)
		response.TenantID = c.tenantID
		response.ClientID = c.clientID
		response.SubscriptionID = c.subscriptionID

		tokenResp = &response
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tokenResp, nil
}
