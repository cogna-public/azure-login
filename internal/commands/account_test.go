package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cogna-public/azure-login/internal/auth"
	"github.com/cogna-public/azure-login/pkg/config"
)

func setupTestConfig(t *testing.T) string {
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	return tmpDir
}

func cleanupTestConfig() {
	_ = os.Unsetenv("AZURE_CONFIG_DIR")
}

func TestRunAccountShow_NotAuthenticated(t *testing.T) {
	tmpDir := setupTestConfig(t)
	defer cleanupTestConfig()

	// Ensure no token file exists
	tokenPath := filepath.Join(tmpDir, "azure-login-token.json")
	_ = os.Remove(tokenPath)

	// Running account show should fail
	cmd := accountShowCmd
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("Expected error for not authenticated, got none")
	}
	if err.Error() != "not authenticated. Run 'azure-login login' first" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunAccountShow_Success(t *testing.T) {
	_ = setupTestConfig(t)
	defer cleanupTestConfig()

	// Save a test token
	cfg := config.NewConfig()
	testToken := &auth.TokenResponse{
		AccessToken:    "test-token",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}
	err := cfg.SaveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Running account show should succeed
	cmd := accountShowCmd
	outputFormat = "json" // Set output format
	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("account show failed: %v", err)
	}
}

func TestRunGetAccessToken_NotAuthenticated(t *testing.T) {
	tmpDir := setupTestConfig(t)
	defer cleanupTestConfig()

	// Ensure no token file exists
	tokenPath := filepath.Join(tmpDir, "azure-login-token.json")
	_ = os.Remove(tokenPath)

	// Running get-access-token should fail
	cmd := accountGetAccessTokenCmd
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("Expected error for not authenticated, got none")
	}
}

func TestRunGetAccessToken_Success(t *testing.T) {
	_ = setupTestConfig(t)
	defer cleanupTestConfig()

	// Save a test token
	cfg := config.NewConfig()
	testToken := &auth.TokenResponse{
		AccessToken:    "test-access-token-123",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}
	err := cfg.SaveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Running get-access-token should succeed
	cmd := accountGetAccessTokenCmd
	outputFormat = "json"
	queryString = ""
	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("get-access-token failed: %v", err)
	}
}

func TestRunGetAccessToken_ExpiredToken(t *testing.T) {
	_ = setupTestConfig(t)
	defer cleanupTestConfig()

	// Save an expired token (expired more than the 5 minute buffer)
	cfg := config.NewConfig()
	testToken := &auth.TokenResponse{
		AccessToken:    "expired-token",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(-10 * time.Minute), // Expired 10 minutes ago (beyond 5 min buffer)
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}
	err := cfg.SaveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Running get-access-token should fail with expiration error
	cmd := accountGetAccessTokenCmd
	err = cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("Expected error for expired token, got none")
	}
	if err.Error() != "token expired or expiring soon. Please re-authenticate with 'azure-login login'" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunGetAccessToken_ExpiringSoonToken(t *testing.T) {
	_ = setupTestConfig(t)
	defer cleanupTestConfig()

	// Save a token expiring in 3 minutes (within the 5 minute buffer)
	cfg := config.NewConfig()
	testToken := &auth.TokenResponse{
		AccessToken:    "expiring-soon-token",
		TokenType:      "Bearer",
		ExpiresIn:      180, // 3 minutes
		ExpiresOn:      time.Now().Add(3 * time.Minute),
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}
	err := cfg.SaveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Running get-access-token should fail due to expiration buffer
	cmd := accountGetAccessTokenCmd
	err = cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("Expected error for expiring-soon token, got none")
	}
	if err.Error() != "token expired or expiring soon. Please re-authenticate with 'azure-login login'" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunGetAccessToken_WithQuery(t *testing.T) {
	_ = setupTestConfig(t)
	defer cleanupTestConfig()

	// Save a test token
	cfg := config.NewConfig()
	testToken := &auth.TokenResponse{
		AccessToken:    "test-token-xyz",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}
	err := cfg.SaveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Test with query string
	cmd := accountGetAccessTokenCmd
	outputFormat = "tsv"
	queryString = "accessToken"
	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("get-access-token with query failed: %v", err)
	}
}

func TestRunGetAccessToken_DifferentFormats(t *testing.T) {
	formats := []string{"json", "tsv"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			_ = setupTestConfig(t)
			defer cleanupTestConfig()

			// Save a test token
			cfg := config.NewConfig()
			testToken := &auth.TokenResponse{
				AccessToken:    "test-token",
				TokenType:      "Bearer",
				ExpiresIn:      3600,
				ExpiresOn:      time.Now().Add(1 * time.Hour),
				TenantID:       "test-tenant",
				ClientID:       "test-client",
				SubscriptionID: "test-subscription",
			}
			err := cfg.SaveToken(testToken)
			if err != nil {
				t.Fatalf("Failed to save test token: %v", err)
			}

			// Test with different formats
			cmd := accountGetAccessTokenCmd
			outputFormat = format
			queryString = ""
			err = cmd.RunE(cmd, []string{})
			if err != nil {
				t.Errorf("get-access-token with format %s failed: %v", format, err)
			}
		})
	}
}
