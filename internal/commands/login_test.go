package commands

import (
	"os"
	"strings"
	"testing"
)

func TestLoginValidation_MissingClientID(t *testing.T) {
	// Clear environment variables
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	// Set empty values
	clientID = ""
	tenantID = "test-tenant"
	subscriptionID = ""
	allowNoSubscription = false

	// Should fail validation
	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for missing client-id, got none")
	}
	if err.Error() != "client-id is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoginValidation_MissingTenantID(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "12345678-1234-1234-1234-123456789abc"
	tenantID = ""
	subscriptionID = ""
	allowNoSubscription = false

	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for missing tenant-id, got none")
	}
	if err.Error() != "tenant-id is required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoginValidation_MissingSubscriptionID(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "12345678-1234-1234-1234-123456789abc"
	tenantID = "12345678-1234-1234-1234-123456789abc"
	subscriptionID = ""
	allowNoSubscription = false

	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for missing subscription-id, got none")
	}
	if err.Error() != "subscription-id is required (or use --allow-no-subscriptions)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoginValidation_AllowNoSubscriptions(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "test-client"
	tenantID = "test-tenant"
	subscriptionID = ""
	allowNoSubscription = true

	// Should pass validation (but fail later on OIDC token)
	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error (OIDC token missing), got none")
	}
	// Should fail on OIDC token, not validation
	if err.Error() == "subscription-id is required (or use --allow-no-subscriptions)" {
		t.Error("Should not fail on subscription-id validation when allow-no-subscriptions is true")
	}
}

func TestLoginEnvVarPrecedence_FlagsOverrideEnv(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("AZURE_CLIENT_ID", "env-client")
	_ = os.Setenv("AZURE_TENANT_ID", "env-tenant")
	_ = os.Setenv("AZURE_SUBSCRIPTION_ID", "env-subscription")
	defer func() {
		_ = os.Unsetenv("AZURE_CLIENT_ID")
		_ = os.Unsetenv("AZURE_TENANT_ID")
		_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")
	}()

	// Set flag values (simulating CLI flags)
	clientID = "flag-client"
	tenantID = "flag-tenant"
	subscriptionID = "flag-subscription"
	allowNoSubscription = false

	// Run login (will fail on OIDC token, but we can check precedence)
	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error (OIDC token missing), got none")
	}

	// After runLogin, variables should still have flag values
	if clientID != "flag-client" {
		t.Errorf("Expected clientID 'flag-client', got '%s'", clientID)
	}
	if tenantID != "flag-tenant" {
		t.Errorf("Expected tenantID 'flag-tenant', got '%s'", tenantID)
	}
	if subscriptionID != "flag-subscription" {
		t.Errorf("Expected subscriptionID 'flag-subscription', got '%s'", subscriptionID)
	}
}

func TestLoginEnvVarPrecedence_EnvUsedWhenFlagsEmpty(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("AZURE_CLIENT_ID", "env-client")
	_ = os.Setenv("AZURE_TENANT_ID", "env-tenant")
	_ = os.Setenv("AZURE_SUBSCRIPTION_ID", "env-subscription")
	defer func() {
		_ = os.Unsetenv("AZURE_CLIENT_ID")
		_ = os.Unsetenv("AZURE_TENANT_ID")
		_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")
	}()

	// Set empty flag values (simulating no CLI flags)
	clientID = ""
	tenantID = ""
	subscriptionID = ""
	allowNoSubscription = false

	// Run login (will fail on OIDC token, but we can check env var usage)
	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error (OIDC token missing), got none")
	}

	// After runLogin, variables should have env var values
	if clientID != "env-client" {
		t.Errorf("Expected clientID 'env-client', got '%s'", clientID)
	}
	if tenantID != "env-tenant" {
		t.Errorf("Expected tenantID 'env-tenant', got '%s'", tenantID)
	}
	if subscriptionID != "env-subscription" {
		t.Errorf("Expected subscriptionID 'env-subscription', got '%s'", subscriptionID)
	}
}

func TestLoginEnvVarPrecedence_PartialFlags(t *testing.T) {
	// Set environment variables for all
	_ = os.Setenv("AZURE_CLIENT_ID", "env-client")
	_ = os.Setenv("AZURE_TENANT_ID", "env-tenant")
	_ = os.Setenv("AZURE_SUBSCRIPTION_ID", "env-subscription")
	defer func() {
		_ = os.Unsetenv("AZURE_CLIENT_ID")
		_ = os.Unsetenv("AZURE_TENANT_ID")
		_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")
	}()

	// Set only some flag values
	clientID = "flag-client"
	tenantID = "" // Empty - should use env var
	subscriptionID = "flag-subscription"
	allowNoSubscription = false

	// Run login
	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error (OIDC token missing), got none")
	}

	// Check mixed precedence
	if clientID != "flag-client" {
		t.Errorf("Expected clientID 'flag-client' (from flag), got '%s'", clientID)
	}
	if tenantID != "env-tenant" {
		t.Errorf("Expected tenantID 'env-tenant' (from env), got '%s'", tenantID)
	}
	if subscriptionID != "flag-subscription" {
		t.Errorf("Expected subscriptionID 'flag-subscription' (from flag), got '%s'", subscriptionID)
	}
}

func TestLoginValidation_InvalidClientID(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "not-a-valid-uuid"
	tenantID = "12345678-1234-1234-1234-123456789abc"
	subscriptionID = "12345678-1234-1234-1234-123456789abc"
	allowNoSubscription = false
	defer func() {
		clientID = ""
		tenantID = ""
		subscriptionID = ""
	}()

	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for invalid client-id, got none")
	}
	if !strings.Contains(err.Error(), "client-id must be a valid UUID") {
		t.Errorf("Expected 'client-id must be a valid UUID' error, got: %v", err)
	}
}

func TestLoginValidation_InvalidTenantID(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "12345678-1234-1234-1234-123456789abc"
	tenantID = "invalid-tenant"
	subscriptionID = "12345678-1234-1234-1234-123456789abc"
	allowNoSubscription = false
	defer func() {
		clientID = ""
		tenantID = ""
		subscriptionID = ""
	}()

	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for invalid tenant-id, got none")
	}
	if !strings.Contains(err.Error(), "tenant-id must be a valid UUID") {
		t.Errorf("Expected 'tenant-id must be a valid UUID' error, got: %v", err)
	}
}

func TestLoginValidation_InvalidSubscriptionID(t *testing.T) {
	_ = os.Unsetenv("AZURE_CLIENT_ID")
	_ = os.Unsetenv("AZURE_TENANT_ID")
	_ = os.Unsetenv("AZURE_SUBSCRIPTION_ID")

	clientID = "12345678-1234-1234-1234-123456789abc"
	tenantID = "12345678-1234-1234-1234-123456789abc"
	subscriptionID = "not-a-guid"
	allowNoSubscription = false
	defer func() {
		clientID = ""
		tenantID = ""
		subscriptionID = ""
	}()

	err := runLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error for invalid subscription-id, got none")
	}
	if !strings.Contains(err.Error(), "subscription-id must be a valid UUID") {
		t.Errorf("Expected 'subscription-id must be a valid UUID' error, got: %v", err)
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		uuid  string
		valid bool
	}{
		{"Valid UUID lowercase", "12345678-1234-1234-1234-123456789abc", true},
		{"Valid UUID uppercase", "12345678-1234-1234-1234-123456789ABC", true},
		{"Valid UUID mixed case", "12345678-ABCD-1234-abcd-123456789ABC", true},
		{"Invalid - no dashes", "12345678123412341234123456789abc", false},
		{"Invalid - too short", "1234-1234-1234-1234", false},
		{"Invalid - too long", "12345678-1234-1234-1234-123456789abcd", false},
		{"Invalid - wrong format", "not-a-uuid-at-all", false},
		{"Invalid - empty string", "", false},
		{"Invalid - contains spaces", "12345678 1234 1234 1234 123456789abc", false},
		{"Invalid - non-hex characters", "1234567g-1234-1234-1234-123456789abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUUID(tt.uuid)
			if result != tt.valid {
				t.Errorf("isValidUUID(%q) = %v, expected %v", tt.uuid, result, tt.valid)
			}
		})
	}
}
