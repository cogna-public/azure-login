package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cogna-public/azure-login/internal/auth"
)

func TestNewConfig(t *testing.T) {
	// Test with default (no AZURE_CONFIG_DIR set)
	_ = os.Unsetenv("AZURE_CONFIG_DIR")
	config := NewConfig()
	if config.configDir == "" {
		t.Error("Expected configDir to be set")
	}
	if !filepath.IsAbs(config.configDir) {
		t.Errorf("Expected absolute path, got %s", config.configDir)
	}

	// Test with custom AZURE_CONFIG_DIR
	customDir := "/tmp/test-azure-config"
	_ = os.Setenv("AZURE_CONFIG_DIR", customDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config = NewConfig()
	if config.configDir != customDir {
		t.Errorf("Expected configDir %s, got %s", customDir, config.configDir)
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Create test token
	expiresOn := time.Now().Add(1 * time.Hour)
	testToken := &auth.TokenResponse{
		AccessToken:    "test-access-token-12345",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      expiresOn,
		TenantID:       "test-tenant-id",
		ClientID:       "test-client-id",
		SubscriptionID: "test-subscription-id",
	}

	// Test SaveToken
	testScope := "https://management.azure.com/.default"
	err := config.SaveToken(testToken, testScope)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Verify file exists
	tokenPath := filepath.Join(tmpDir, tokenFile)
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Fatal("Token file was not created")
	}

	// Verify file permissions
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Failed to stat token file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Test LoadToken
	loadedToken, err := config.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}

	// Verify loaded token matches saved token
	if loadedToken.AccessToken != testToken.AccessToken {
		t.Errorf("AccessToken mismatch: expected %s, got %s", testToken.AccessToken, loadedToken.AccessToken)
	}
	if loadedToken.TokenType != testToken.TokenType {
		t.Errorf("TokenType mismatch: expected %s, got %s", testToken.TokenType, loadedToken.TokenType)
	}
	if loadedToken.TenantID != testToken.TenantID {
		t.Errorf("TenantID mismatch: expected %s, got %s", testToken.TenantID, loadedToken.TenantID)
	}
	if loadedToken.ClientID != testToken.ClientID {
		t.Errorf("ClientID mismatch: expected %s, got %s", testToken.ClientID, loadedToken.ClientID)
	}
	if loadedToken.SubscriptionID != testToken.SubscriptionID {
		t.Errorf("SubscriptionID mismatch: expected %s, got %s", testToken.SubscriptionID, loadedToken.SubscriptionID)
	}
	if loadedToken.Scope != testScope {
		t.Errorf("Scope mismatch: expected %s, got %s", testScope, loadedToken.Scope)
	}
	// Time comparison with small delta for rounding
	if loadedToken.ExpiresOn.Sub(testToken.ExpiresOn).Abs() > time.Second {
		t.Errorf("ExpiresOn mismatch: expected %v, got %v", testToken.ExpiresOn, loadedToken.ExpiresOn)
	}
}

func TestLoadToken_NotFound(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Try to load non-existent token
	token, err := config.LoadToken()
	if err == nil {
		t.Fatal("Expected error for non-existent token, got none")
	}
	if token != nil {
		t.Errorf("Expected nil token, got %v", token)
	}
	if err.Error() != "not authenticated" {
		t.Errorf("Expected 'not authenticated' error, got: %v", err)
	}
}

func TestDeleteToken(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Create and save a token
	testToken := &auth.TokenResponse{
		AccessToken:    "test-token",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "tenant",
		ClientID:       "client",
		SubscriptionID: "subscription",
	}

	err := config.SaveToken(testToken, "https://management.azure.com/.default")
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Verify token exists
	tokenPath := filepath.Join(tmpDir, tokenFile)
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Fatal("Token file was not created")
	}

	// Delete token
	err = config.DeleteToken()
	if err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}

	// Verify token is deleted
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("Token file still exists after deletion")
	}

	// Delete again should not error
	err = config.DeleteToken()
	if err != nil {
		t.Errorf("DeleteToken on non-existent file should not error, got: %v", err)
	}
}

func TestSaveToken_AtomicWrite(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	testToken := &auth.TokenResponse{
		AccessToken:    "test-token",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "tenant",
		ClientID:       "client",
		SubscriptionID: "subscription",
	}

	// Save token
	err := config.SaveToken(testToken, "https://management.azure.com/.default")
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Verify temp file is cleaned up
	tmpPath := filepath.Join(tmpDir, tokenFile+".tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after atomic write")
	}

	// Verify actual token file exists
	tokenPath := filepath.Join(tmpDir, tokenFile)
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Error("Token file should exist after save")
	}
}

func TestSaveToken_ConcurrentWrites(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	// Simulate concurrent writes with slight delays to be more realistic
	// In real usage, concurrent authentications would have some time separation
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			// Each goroutine creates its own config instance (realistic)
			cfg := NewConfig()
			// Small stagger to simulate realistic timing
			time.Sleep(time.Duration(n) * 10 * time.Millisecond)
			testToken := &auth.TokenResponse{
				AccessToken:    "test-token",
				TokenType:      "Bearer",
				ExpiresIn:      3600,
				ExpiresOn:      time.Now().Add(1 * time.Hour),
				TenantID:       "tenant",
				ClientID:       "client",
				SubscriptionID: "subscription",
			}
			err := cfg.SaveToken(testToken, "https://management.azure.com/.default")
			done <- err
		}(i)
	}

	// Wait for all goroutines and check for errors
	var errors []error
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	// All writes should succeed (atomic rename handles concurrency)
	if len(errors) > 0 {
		t.Errorf("Some concurrent writes failed: %v", errors)
	}

	// Verify token file is valid (not corrupted)
	cfg := NewConfig()
	loadedToken, err := cfg.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken after concurrent writes failed: %v", err)
	}
	if loadedToken.AccessToken != "test-token" {
		t.Error("Token file was corrupted by concurrent writes")
	}
}

func TestSaveToken_DirectoryCreation(t *testing.T) {
	// Use a non-existent directory
	tmpDir := filepath.Join(os.TempDir(), "azure-login-test-"+time.Now().Format("20060102150405"))
	defer func() { _ = os.RemoveAll(tmpDir) }()

	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	testToken := &auth.TokenResponse{
		AccessToken:    "test-token",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "tenant",
		ClientID:       "client",
		SubscriptionID: "subscription",
	}

	// Should create directory if it doesn't exist
	err := config.SaveToken(testToken, "https://management.azure.com/.default")
	if err != nil {
		t.Fatalf("SaveToken failed to create directory: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Config directory was not created")
	}

	// Verify directory permissions
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("Expected directory permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestLoadToken_CorruptedFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Write corrupted JSON to token file
	tokenPath := filepath.Join(tmpDir, tokenFile)
	err := os.WriteFile(tokenPath, []byte("{corrupted json"), 0600)
	if err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	// Try to load corrupted token
	token, err := config.LoadToken()
	if err == nil {
		t.Fatal("Expected error for corrupted JSON, got none")
	}
	if token != nil {
		t.Errorf("Expected nil token, got %v", token)
	}
}

func TestSavedTokenFields(t *testing.T) {
	now := time.Now()
	token := SavedToken{
		AccessToken:    "test-token",
		TokenType:      "Bearer",
		ExpiresOn:      now,
		TenantID:       "tenant",
		ClientID:       "client",
		SubscriptionID: "subscription",
		Scope:          "https://management.azure.com/.default",
	}

	if token.AccessToken != "test-token" {
		t.Errorf("Expected AccessToken test-token, got %s", token.AccessToken)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("Expected TokenType Bearer, got %s", token.TokenType)
	}
	if !token.ExpiresOn.Equal(now) {
		t.Errorf("Expected ExpiresOn %v, got %v", now, token.ExpiresOn)
	}
	if token.Scope != "https://management.azure.com/.default" {
		t.Errorf("Expected Scope https://management.azure.com/.default, got %s", token.Scope)
	}
}

func TestSaveAndLoadToken_WithCustomScope(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Create test token
	expiresOn := time.Now().Add(1 * time.Hour)
	testToken := &auth.TokenResponse{
		AccessToken:    "test-access-token-devops",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      expiresOn,
		TenantID:       "test-tenant-id",
		ClientID:       "test-client-id",
		SubscriptionID: "test-subscription-id",
	}

	// Test SaveToken with Azure DevOps scope
	devOpsScope := "https://app.vssps.visualstudio.com/.default"
	err := config.SaveToken(testToken, devOpsScope)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Test LoadToken
	loadedToken, err := config.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}

	// Verify scope matches
	if loadedToken.Scope != devOpsScope {
		t.Errorf("Scope mismatch: expected %s, got %s", devOpsScope, loadedToken.Scope)
	}
}

func TestSaveToken_OverwritesDifferentScope(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	_ = os.Setenv("AZURE_CONFIG_DIR", tmpDir)
	defer func() { _ = os.Unsetenv("AZURE_CONFIG_DIR") }()

	config := NewConfig()

	// Create test token
	testToken := &auth.TokenResponse{
		AccessToken:    "test-token-1",
		TokenType:      "Bearer",
		ExpiresIn:      3600,
		ExpiresOn:      time.Now().Add(1 * time.Hour),
		TenantID:       "test-tenant",
		ClientID:       "test-client",
		SubscriptionID: "test-subscription",
	}

	// Save token with ARM scope
	armScope := "https://management.azure.com/.default"
	err := config.SaveToken(testToken, armScope)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Verify ARM scope is saved
	loadedToken, err := config.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}
	if loadedToken.Scope != armScope {
		t.Errorf("Expected ARM scope, got %s", loadedToken.Scope)
	}

	// Save token with DevOps scope (overwrites previous)
	devOpsScope := "https://app.vssps.visualstudio.com/.default"
	testToken.AccessToken = "test-token-2"
	err = config.SaveToken(testToken, devOpsScope)
	if err != nil {
		t.Fatalf("SaveToken with new scope failed: %v", err)
	}

	// Verify DevOps scope is now saved
	loadedToken, err = config.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}
	if loadedToken.Scope != devOpsScope {
		t.Errorf("Expected DevOps scope, got %s", loadedToken.Scope)
	}
	if loadedToken.AccessToken != "test-token-2" {
		t.Errorf("Expected new token, got %s", loadedToken.AccessToken)
	}
}
