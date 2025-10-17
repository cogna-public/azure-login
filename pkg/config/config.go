// Package config provides configuration and token storage management for azure-login.
//
// This package handles secure token persistence with atomic writes, file permissions,
// and token lifecycle management for Azure access tokens.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cogna-public/azure-login/internal/auth"
)

const (
	defaultConfigDir = ".azure"
	tokenFile        = "azure-login-token.json"
)

// Config manages configuration and token storage
type Config struct {
	configDir string
}

// SavedToken represents the cached token with metadata
type SavedToken struct {
	AccessToken    string    `json:"access_token"`
	TokenType      string    `json:"token_type"`
	ExpiresOn      time.Time `json:"expires_on"`
	TenantID       string    `json:"tenant_id"`
	ClientID       string    `json:"client_id"`
	SubscriptionID string    `json:"subscription_id"`
}

// NewConfig creates a new configuration manager
func NewConfig() *Config {
	configDir := os.Getenv("AZURE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory
			configDir = defaultConfigDir
		} else {
			configDir = filepath.Join(home, defaultConfigDir)
		}
	}

	return &Config{
		configDir: configDir,
	}
}

// SaveToken saves the authentication token to disk using atomic writes
func (c *Config) SaveToken(token *auth.TokenResponse) error {
	// Ensure config directory exists
	if err := os.MkdirAll(c.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Prepare token for storage
	savedToken := SavedToken{
		AccessToken:    token.AccessToken,
		TokenType:      token.TokenType,
		ExpiresOn:      token.ExpiresOn,
		TenantID:       token.TenantID,
		ClientID:       token.ClientID,
		SubscriptionID: token.SubscriptionID,
	}

	// Marshal to JSON
	data, err := json.Marshal(savedToken)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Write to temp file, then rename
	tokenPath := filepath.Join(c.configDir, tokenFile)
	tmpPath := tokenPath + ".tmp"

	// Write to temp file with restricted permissions
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	// Atomically replace the token file
	if err := os.Rename(tmpPath, tokenPath); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("failed to save token file: %w", err)
	}

	return nil
}

// LoadToken loads the authentication token from disk
func (c *Config) LoadToken() (*SavedToken, error) {
	tokenPath := filepath.Join(c.configDir, tokenFile)

	// Read token file
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not authenticated")
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// Parse token
	var token SavedToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// DeleteToken removes the stored authentication token
func (c *Config) DeleteToken() error {
	tokenPath := filepath.Join(c.configDir, tokenFile)
	if err := os.Remove(tokenPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete token file: %w", err)
	}
	return nil
}
