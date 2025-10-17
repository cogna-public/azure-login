package commands

import (
	"fmt"
	"os"
	"regexp"

	"github.com/cogna-public/azure-login/internal/auth"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

var (
	clientID            string
	tenantID            string
	subscriptionID      string
	allowNoSubscription bool

	// uuidPattern matches Azure UUID/GUID format (8-4-4-4-12 hex digits)
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate to Azure using OIDC",
	Long: `Authenticate to Azure using OpenID Connect (OIDC) workload identity federation.
This command is designed for use in GitHub Actions with federated credentials.`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&clientID, "client-id", "", "Azure Application (Client) ID")
	loginCmd.Flags().StringVar(&tenantID, "tenant-id", "", "Azure Active Directory Tenant ID")
	loginCmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure Subscription ID (optional)")
	loginCmd.Flags().BoolVar(&allowNoSubscription, "allow-no-subscriptions", false, "Allow authentication without subscription")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Apply environment variable defaults if flags not provided
	// CLI flags take precedence over environment variables
	if clientID == "" {
		clientID = os.Getenv("AZURE_CLIENT_ID")
	}
	if tenantID == "" {
		tenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if subscriptionID == "" {
		subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	}

	// Validate required parameters
	if clientID == "" {
		return fmt.Errorf("client-id is required")
	}
	if !isValidUUID(clientID) {
		return fmt.Errorf("client-id must be a valid UUID/GUID format (e.g., 12345678-1234-1234-1234-123456789abc)")
	}

	if tenantID == "" {
		return fmt.Errorf("tenant-id is required")
	}
	if !isValidUUID(tenantID) {
		return fmt.Errorf("tenant-id must be a valid UUID/GUID format (e.g., 12345678-1234-1234-1234-123456789abc)")
	}

	if subscriptionID == "" && !allowNoSubscription {
		return fmt.Errorf("subscription-id is required (or use --allow-no-subscriptions)")
	}
	if subscriptionID != "" && !isValidUUID(subscriptionID) {
		return fmt.Errorf("subscription-id must be a valid UUID/GUID format (e.g., 12345678-1234-1234-1234-123456789abc)")
	}

	// Get OIDC token from GitHub Actions environment
	oidcToken, err := auth.GetGitHubOIDCToken(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get OIDC token: %w", err)
	}

	// Exchange OIDC token for Azure access token
	authClient := auth.NewClient(tenantID, clientID, subscriptionID)
	tokenResponse, err := authClient.ExchangeOIDCToken(cmd.Context(), oidcToken)
	if err != nil {
		return fmt.Errorf("failed to exchange OIDC token: %w", err)
	}

	// Save token to cache
	cfg := config.NewConfig()
	if err := cfg.SaveToken(tokenResponse); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	// Explicitly ignore errors from stderr writes (nowhere to report if stderr fails)
	_, _ = fmt.Fprintf(os.Stderr, "Successfully authenticated to Azure\n")
	_, _ = fmt.Fprintf(os.Stderr, "Tenant: %s\n", tenantID)
	_, _ = fmt.Fprintf(os.Stderr, "Client: %s\n", clientID)
	if subscriptionID != "" {
		_, _ = fmt.Fprintf(os.Stderr, "Subscription: %s\n", subscriptionID)
	}

	return nil
}

// isValidUUID checks if a string is a valid UUID/GUID format
func isValidUUID(id string) bool {
	return uuidPattern.MatchString(id)
}
