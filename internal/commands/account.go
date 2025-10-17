package commands

import (
	"fmt"
	"time"

	"github.com/cogna-public/azure-login/internal/output"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	queryString  string
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage Azure account and authentication",
	Long:  `Commands for managing Azure account information and access tokens.`,
}

var accountShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current account information",
	RunE:  runAccountShow,
}

var accountGetAccessTokenCmd = &cobra.Command{
	Use:   "get-access-token",
	Short: "Get an access token for Azure resource access",
	Long: `Get an Azure access token that can be used to authenticate to Azure resources.
The token is automatically refreshed if it has expired.`,
	RunE: runGetAccessToken,
}

func init() {
	accountCmd.AddCommand(accountShowCmd)
	accountCmd.AddCommand(accountGetAccessTokenCmd)

	// Add flags for output formatting
	accountShowCmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, tsv, table")

	accountGetAccessTokenCmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, tsv, table")
	accountGetAccessTokenCmd.Flags().StringVar(&queryString, "query", "", "JMESPath query string")
}

func runAccountShow(cmd *cobra.Command, args []string) error {
	cfg := config.NewConfig()
	token, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	accountInfo := map[string]any{
		"environmentName": "AzureCloud",
		"id":              token.SubscriptionID,
		"name":            "Azure Subscription",
		"tenantId":        token.TenantID,
		"user": map[string]string{
			"name": token.ClientID,
			"type": "servicePrincipal",
		},
	}

	return output.Print(accountInfo, outputFormat, queryString)
}

func runGetAccessToken(cmd *cobra.Command, args []string) error {
	cfg := config.NewConfig()
	token, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	// Check if token is expired or expiring soon (5 minute buffer for clock skew and API latency)
	// Use UTC to avoid timezone-related issues
	const tokenExpirationBuffer = 5 * time.Minute
	if time.Now().UTC().Add(tokenExpirationBuffer).After(token.ExpiresOn) {
		return fmt.Errorf("token expired or expiring soon. Please re-authenticate with 'azure-login login'")
	}

	// Create response matching Azure CLI format
	tokenInfo := map[string]any{
		"accessToken":  token.AccessToken,
		"expiresOn":    token.ExpiresOn.Format("2006-01-02 15:04:05.000000"),
		"subscription": token.SubscriptionID,
		"tenant":       token.TenantID,
		"tokenType":    "Bearer",
	}

	return output.Print(tokenInfo, outputFormat, queryString)
}
