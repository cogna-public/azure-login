package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/cogna-public/azure-login/internal/aks"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

var (
	resourceGroup string
	clusterName   string
)

var aksCmd = &cobra.Command{
	Use:   "aks",
	Short: "Manage Azure Kubernetes Service",
	Long:  `Commands for managing Azure Kubernetes Service clusters.`,
}

var aksGetCredentialsCmd = &cobra.Command{
	Use:   "get-credentials",
	Short: "Get AKS cluster credentials and update kubeconfig",
	Long: `Get access credentials for a managed Kubernetes cluster.

This command retrieves the cluster credentials from Azure and merges them into
your kubeconfig file. The cluster will be configured to use Azure CLI authentication
via kubelogin.`,
	RunE: runGetCredentials,
}

func init() {
	aksCmd.AddCommand(aksGetCredentialsCmd)

	// Add flags for get-credentials
	aksGetCredentialsCmd.Flags().StringVarP(&resourceGroup, "resource-group", "g", "", "Resource group name (required)")
	aksGetCredentialsCmd.Flags().StringVarP(&clusterName, "name", "n", "", "Cluster name (required)")
	_ = aksGetCredentialsCmd.MarkFlagRequired("resource-group")
	_ = aksGetCredentialsCmd.MarkFlagRequired("name")
}

func runGetCredentials(cmd *cobra.Command, args []string) error {
	// Load authentication token
	cfg := config.NewConfig()
	token, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	// Check if subscription ID is available
	if token.SubscriptionID == "" {
		return fmt.Errorf("no subscription configured. Run 'azure-login login' with --subscription-id")
	}

	// Create AKS client
	aksClient := aks.NewClient(token.SubscriptionID, token.AccessToken)

	// Get cluster credentials
	_, _ = fmt.Fprintf(os.Stderr, "Retrieving credentials for cluster %s in resource group %s...\n", clusterName, resourceGroup)

	ctx := context.Background()
	credentials, err := aksClient.GetClusterCredentials(ctx, resourceGroup, clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster credentials: %w", err)
	}

	// Load kubeconfig
	kubeconfigPath := aks.GetKubeconfigPath()
	kubeconfig, err := aks.LoadKubeconfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Merge credentials into kubeconfig
	kubeconfig.MergeClusterCredentials(credentials)

	// Save kubeconfig
	if err := aks.SaveKubeconfig(kubeconfigPath, kubeconfig); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Merged \"%s\" as current context in %s\n", clusterName, kubeconfigPath)
	_, _ = fmt.Fprintf(os.Stderr, "\nNote: This kubeconfig uses Azure CLI authentication. Run 'kubelogin convert-kubeconfig -l azurecli' if needed.\n")

	return nil
}
