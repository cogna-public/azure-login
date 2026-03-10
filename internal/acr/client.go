// Package acr provides Azure Container Registry authentication via OAuth2 token exchange.
//
// This package handles exchanging an Azure AD access token for an ACR refresh token,
// which can then be used with Docker CLI for registry operations.
package acr

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
	// ExchangeTimeout is the maximum time for the ACR token exchange request.
	ExchangeTimeout = 10 * time.Second

	// NullGUID is used as the Docker username when authenticating with an ACR refresh token.
	// This signals to the registry that the password is an OAuth refresh token.
	NullGUID = "00000000-0000-0000-0000-000000000000"
)

// Client handles ACR OAuth2 token exchange
type Client struct {
	registry    string // FQDN, e.g. "myregistry.azurecr.io"
	tenantID    string
	aadToken    string
	httpClient  *http.Client
	endpointURL string // override for testing; empty uses https://{registry}
}

// exchangeResponse represents the response from the ACR /oauth2/exchange endpoint
type exchangeResponse struct {
	RefreshToken string `json:"refresh_token"`
}

// NewClient creates a new ACR client for the given registry
func NewClient(registry, tenantID, aadToken string) *Client {
	return &Client{
		registry: registry,
		tenantID: tenantID,
		aadToken: aadToken,
		httpClient: &http.Client{
			Timeout: ExchangeTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ExchangeAADToken exchanges an Azure AD access token for an ACR refresh token
// via POST https://{registry}/oauth2/exchange
func (c *Client) ExchangeAADToken(ctx context.Context) (string, error) {
	base := c.endpointURL
	if base == "" {
		base = fmt.Sprintf("https://%s", c.registry)
	}
	endpoint := base + "/oauth2/exchange"

	data := url.Values{}
	data.Set("grant_type", "access_token")
	data.Set("service", c.registry)
	data.Set("tenant", c.tenantID)
	data.Set("access_token", c.aadToken)

	retryConfig := retry.LoadConfig()

	var refreshToken string
	err := retryConfig.Do(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create exchange request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to exchange token with ACR: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		if err != nil {
			return fmt.Errorf("failed to read exchange response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("ACR token exchange failed (status %d): check that your identity has ACR role assignments (acrPull/acrPush)", resp.StatusCode)
		}

		var result exchangeResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse exchange response: %w", err)
		}

		if result.RefreshToken == "" {
			return fmt.Errorf("ACR returned empty refresh token")
		}

		refreshToken = result.RefreshToken
		return nil
	})

	if err != nil {
		return "", err
	}

	return refreshToken, nil
}

// NormalizeRegistry ensures the registry name is a fully qualified domain name.
// If the input doesn't contain a dot, ".azurecr.io" is appended.
func NormalizeRegistry(name string) string {
	if !strings.Contains(name, ".") {
		return name + ".azurecr.io"
	}
	return name
}
