package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOIDCGetToken_Success(t *testing.T) {
	// Create mock OIDC token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer mock-request-token" {
			t.Errorf("Expected Authorization header with request token")
		}

		// Return mock OIDC token
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.mock-oidc-token"}`))
	}))
	defer server.Close()

	// Set up environment variables
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "mock-request-token")
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Test JSON output (default)
	t.Run("JSON output", func(t *testing.T) {
		// Reset flags
		oidcOutputFormat = "json"
		oidcQueryString = ""

		// Execute command - it will print to stdout
		err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Note: In actual usage, output goes to stdout
		// For this test, we're just verifying the command executes without error
		// The JSON output format is tested by the output package
	})

	// Test TSV output
	t.Run("TSV output", func(t *testing.T) {
		// Reset flags
		oidcOutputFormat = "tsv"
		oidcQueryString = ""

		// Execute command
		err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Note: Output format testing is handled by the output package
	})

	// Test query string
	t.Run("Query string", func(t *testing.T) {
		// Reset flags
		oidcOutputFormat = "tsv"
		oidcQueryString = "value"

		// Execute command
		err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Note: Query string functionality is handled by the output package
	})
}

func TestOIDCGetToken_MissingEnvironmentVariables(t *testing.T) {
	// Ensure environment variables are not set
	os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")

	// Execute command
	err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
	if err == nil {
		t.Fatal("Expected error when ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set")
	}

	// Verify error message
	expectedMsg := "ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable not set"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestOIDCGetToken_InvalidRequestURL(t *testing.T) {
	// Set up environment variables with invalid URL
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "mock-request-token")
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "://invalid-url")
	defer func() {
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Execute command
	err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
	if err == nil {
		t.Fatal("Expected error when ACTIONS_ID_TOKEN_REQUEST_URL is invalid")
	}

	// Verify error indicates URL parsing issue
	expectedMsg := "invalid ACTIONS_ID_TOKEN_REQUEST_URL"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestOIDCGetToken_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	// Set up environment variables
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "mock-request-token")
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Execute command
	err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
	if err == nil {
		t.Fatal("Expected error when server returns 401")
	}

	// Verify error message indicates failure
	if !strings.Contains(err.Error(), "failed to get OIDC token") {
		t.Errorf("Expected error message about failed token retrieval, got: %v", err)
	}
}

func TestOIDCGetToken_EmptyToken(t *testing.T) {
	// Create mock server that returns empty token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value": ""}`))
	}))
	defer server.Close()

	// Set up environment variables
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "mock-request-token")
	os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL)
	defer func() {
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	}()

	// Execute command
	err := oidcGetTokenCmd.RunE(oidcGetTokenCmd, []string{})
	if err == nil {
		t.Fatal("Expected error when token value is empty")
	}

	// Verify error message
	expectedMsg := "empty OIDC token received"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}
}
