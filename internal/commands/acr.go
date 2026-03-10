package commands

import (
	"fmt"
	"os"

	"github.com/cogna-public/azure-login/internal/acr"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

var registryName string

var acrCmd = &cobra.Command{
	Use:   "acr",
	Short: "Manage Azure Container Registries",
	Long:  `Commands for managing Azure Container Registry authentication.`,
}

var acrLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to an Azure Container Registry",
	Long: `Log in to an Azure Container Registry by exchanging your Azure AD token
for ACR credentials and writing them to the Docker config.

You must run 'azure-login login' first to authenticate with Azure.
The registry identity (service principal / managed identity) must have
an appropriate ACR role assignment (acrPull, acrPush, etc.).`,
	RunE: runACRLogin,
}

func init() {
	acrCmd.AddCommand(acrLoginCmd)

	acrLoginCmd.Flags().StringVarP(&registryName, "name", "n", "", "Container registry name (e.g. myregistry or myregistry.azurecr.io)")
	_ = acrLoginCmd.MarkFlagRequired("name")
}

func runACRLogin(cmd *cobra.Command, args []string) error {
	cfg := config.NewConfig()
	token, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	registry := acr.NormalizeRegistry(registryName)

	_, _ = fmt.Fprintf(os.Stderr, "Logging in to %s...\n", registry)

	client := acr.NewClient(registry, token.TenantID, token.AccessToken)
	refreshToken, err := client.ExchangeAADToken(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get ACR credentials: %w", err)
	}

	if err := acr.WriteDockerConfig(registry, acr.NullGUID, refreshToken); err != nil {
		return fmt.Errorf("failed to update Docker config: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Login succeeded for %s\n", registry)

	return nil
}
