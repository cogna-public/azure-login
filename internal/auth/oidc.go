package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cogna-public/azure-login/internal/retry"
)

const (
	// OIDCRequestTimeout is the maximum time to wait for OIDC token request.
	// This is set relatively short to fail fast on transient issues, since
	// the retry logic will handle retries with exponential backoff.
	// With 3 retries and default backoff (1s, 2s), total worst case: ~18 seconds
	OIDCRequestTimeout = 5 * time.Second
)

// GetGitHubOIDCToken retrieves the OIDC token from GitHub Actions environment
func GetGitHubOIDCToken(ctx context.Context) (string, error) {
	// Get environment variables
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")

	if requestToken == "" {
		return "", fmt.Errorf("ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable not set. Are you running in GitHub Actions?")
	}
	if requestURL == "" {
		return "", fmt.Errorf("ACTIONS_ID_TOKEN_REQUEST_URL environment variable not set. Are you running in GitHub Actions?")
	}

	// Parse the URL and add audience parameter
	tokenURL, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("invalid ACTIONS_ID_TOKEN_REQUEST_URL: %w", err)
	}

	// Add audience query parameter
	query := tokenURL.Query()
	query.Set("audience", "api://AzureADTokenExchange")
	tokenURL.RawQuery = query.Encode()

	// Load retry configuration
	retryConfig := retry.LoadConfig()

	var token string
	err = retryConfig.Do(ctx, func() error {
		// Create HTTP client with timeout and disabled redirects for security
		client := &http.Client{
			Timeout: OIDCRequestTimeout,
			// Disable redirects for security (prevents redirect-based attacks)
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		// Create request with context for cancellation support
		req, err := http.NewRequestWithContext(ctx, "GET", tokenURL.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Add authorization header
		req.Header.Add("Authorization", "Bearer "+requestToken)
		req.Header.Add("Accept", "application/json")

		// Execute request
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to request OIDC token: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// Limit response body to 1MB to prevent memory exhaustion
		limitedBody := io.LimitReader(resp.Body, 1024*1024)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get OIDC token: status %d (check ACTIONS_ID_TOKEN_REQUEST_TOKEN and workflow permissions)", resp.StatusCode)
		}

		// Parse response
		var tokenResponse struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(limitedBody).Decode(&tokenResponse); err != nil {
			return fmt.Errorf("failed to parse OIDC token response: %w", err)
		}

		if tokenResponse.Value == "" {
			return fmt.Errorf("empty OIDC token received")
		}

		token = tokenResponse.Value
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to get OIDC token: %w", err)
	}

	return token, nil
}
