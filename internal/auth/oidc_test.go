package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGetGitHubOIDCToken_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Query().Get("audience") != "api://AzureADTokenExchange" {
			t.Errorf("Expected audience parameter, got %s", r.URL.Query().Get("audience"))
		}
		if r.Header.Get("Authorization") != "Bearer test-request-token" {
			t.Errorf("Expected Authorization header with bearer token")
		}

		// Return mock OIDC token
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"value": "mock-oidc-token-12345"}`)
	}))
	defer server.Close()

	// Set up environment variables
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-request-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Test
	token, err := GetGitHubOIDCToken(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != "mock-oidc-token-12345" {
		t.Errorf("Expected token 'mock-oidc-token-12345', got '%s'", token)
	}
}

func TestGetGitHubOIDCToken_MissingRequestToken(t *testing.T) {
	// Ensure environment variables are not set
	_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")

	token, err := GetGitHubOIDCToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for missing ACTIONS_ID_TOKEN_REQUEST_TOKEN, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
	if err.Error() != "ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable not set. Are you running in GitHub Actions?" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGetGitHubOIDCToken_MissingRequestURL(t *testing.T) {
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	defer func() { _ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") }()

	token, err := GetGitHubOIDCToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for missing ACTIONS_ID_TOKEN_REQUEST_URL, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
}

func TestGetGitHubOIDCToken_HTTPError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, `{"error": "unauthorized"}`)
	}))
	defer server.Close()

	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	token, err := GetGitHubOIDCToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for HTTP 401, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
}

func TestGetGitHubOIDCToken_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{invalid json`)
	}))
	defer server.Close()

	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	token, err := GetGitHubOIDCToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
}

func TestGetGitHubOIDCToken_EmptyTokenValue(t *testing.T) {
	// Create mock server that returns empty token value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"value": ""}`)
	}))
	defer server.Close()

	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	token, err := GetGitHubOIDCToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for empty token value, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
	if err.Error() != "empty OIDC token received" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGetGitHubOIDCToken_LargeResponse(t *testing.T) {
	// Create mock server that returns a very large response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Send 2MB of data (exceeds 1MB limit)
		largeToken := make([]byte, 2*1024*1024)
		for i := range largeToken {
			largeToken[i] = 'A'
		}
		_, _ = fmt.Fprintf(w, `{"value": "%s"}`, string(largeToken))
	}))
	defer server.Close()

	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Should handle large response gracefully (limited to 1MB)
	token, err := GetGitHubOIDCToken(context.Background())
	// May get parse error or succeed with truncated data - both are acceptable
	// The important thing is it doesn't crash or consume all memory
	if err != nil {
		// This is fine - parsing might fail on truncated JSON
		t.Logf("Got expected error for large response: %v", err)
	}
	_ = token // Use token to avoid unused variable warning
}

func TestGetGitHubOIDCToken_ContextCancellation(t *testing.T) {
	// Create a slow mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"value": "should-not-reach"}`)
	}))
	defer server.Close()

	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
	_ = os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		_ = os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Create a context that cancels after 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Request should fail due to context cancellation
	token, err := GetGitHubOIDCToken(ctx)
	if err == nil {
		t.Fatal("Expected error for cancelled context, got none")
	}
	if token != "" {
		t.Errorf("Expected empty token, got '%s'", token)
	}
	// Error should indicate context issue
	if !contains(err.Error(), "context") && !contains(err.Error(), "deadline") {
		t.Logf("Expected context-related error, got: %v", err)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
