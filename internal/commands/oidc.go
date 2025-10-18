package commands

import (
	"context"
	"fmt"

	"github.com/cogna-public/azure-login/internal/auth"
	"github.com/cogna-public/azure-login/internal/output"
	"github.com/spf13/cobra"
)

var oidcCmd = &cobra.Command{
	Use:   "oidc",
	Short: "Manage OIDC tokens",
	Long:  `Commands for managing GitHub Actions OIDC tokens.`,
}

var oidcGetTokenCmd = &cobra.Command{
	Use:   "get-token",
	Short: "Get the GitHub Actions OIDC token",
	Long: `Get the GitHub Actions OIDC token for Azure authentication.
This token can be used with WorkloadIdentityCredential in Azure SDKs.

The token is written to stdout in the specified format (json, tsv, or table).
For use with Azure Python SDK, write the token to a file and set AZURE_FEDERATED_TOKEN_FILE.`,
	RunE: runOIDCGetToken,
}

var (
	oidcOutputFormat string
	oidcQueryString  string
)

func init() {
	oidcCmd.AddCommand(oidcGetTokenCmd)

	// Add flags for output formatting
	oidcGetTokenCmd.Flags().StringVarP(&oidcOutputFormat, "output", "o", "json", "Output format: json, tsv, table")
	oidcGetTokenCmd.Flags().StringVar(&oidcQueryString, "query", "", "JMESPath query string")
}

func runOIDCGetToken(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	token, err := auth.GetGitHubOIDCToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OIDC token: %w", err)
	}

	// Create response with token info
	tokenInfo := map[string]any{
		"value": token,
	}

	return output.Print(tokenInfo, oidcOutputFormat, oidcQueryString)
}
