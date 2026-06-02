package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cogna-public/azure-login/internal/auth"
	"github.com/cogna-public/azure-login/internal/storage"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

const storageScope = "https://storage.azure.com/.default"

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage Azure Storage resources",
	Long:  `Commands for managing Azure Storage containers and queues.`,
}

var storageContainerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage blob containers",
}

var storageContainerCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a blob container",
	RunE:  runStorageContainerCreate,
}

var storageContainerDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a blob container",
	RunE:  runStorageContainerDelete,
}

var storageQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage storage queues",
}

var storageQueueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a storage queue",
	RunE:  runStorageQueueCreate,
}

var storageQueueDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a storage queue",
	RunE:  runStorageQueueDelete,
}

func init() {
	storageContainerCmd.AddCommand(storageContainerCreateCmd, storageContainerDeleteCmd)
	storageQueueCmd.AddCommand(storageQueueCreateCmd, storageQueueDeleteCmd)
	storageCmd.AddCommand(storageContainerCmd, storageQueueCmd)

	for _, cmd := range []*cobra.Command{
		storageContainerCreateCmd, storageContainerDeleteCmd,
		storageQueueCreateCmd, storageQueueDeleteCmd,
	} {
		cmd.Flags().String("account-name", "", "Storage account name (required)")
		cmd.Flags().String("name", "", "Resource name (required)")
		_ = cmd.MarkFlagRequired("account-name")
		_ = cmd.MarkFlagRequired("name")
	}
}

func runStorageContainerCreate(cmd *cobra.Command, _ []string) error {
	accountName, resourceName, client, err := storageClientFromFlags(cmd)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Creating container %s on %s...\n", resourceName, accountName)
	if err := client.ContainerCreate(cmd.Context(), accountName, resourceName); err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	_, _ = fmt.Fprintf(os.Stderr, "Container %s created\n", resourceName)
	return nil
}

func runStorageContainerDelete(cmd *cobra.Command, _ []string) error {
	accountName, resourceName, client, err := storageClientFromFlags(cmd)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Deleting container %s on %s...\n", resourceName, accountName)
	if err := client.ContainerDelete(cmd.Context(), accountName, resourceName); err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}
	_, _ = fmt.Fprintf(os.Stderr, "Container %s deleted\n", resourceName)
	return nil
}

func runStorageQueueCreate(cmd *cobra.Command, _ []string) error {
	accountName, resourceName, client, err := storageClientFromFlags(cmd)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Creating queue %s on %s...\n", resourceName, accountName)
	if err := client.QueueCreate(cmd.Context(), accountName, resourceName); err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}
	_, _ = fmt.Fprintf(os.Stderr, "Queue %s created\n", resourceName)
	return nil
}

func runStorageQueueDelete(cmd *cobra.Command, _ []string) error {
	accountName, resourceName, client, err := storageClientFromFlags(cmd)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Deleting queue %s on %s...\n", resourceName, accountName)
	if err := client.QueueDelete(cmd.Context(), accountName, resourceName); err != nil {
		return fmt.Errorf("failed to delete queue: %w", err)
	}
	_, _ = fmt.Fprintf(os.Stderr, "Queue %s deleted\n", resourceName)
	return nil
}

func storageClientFromFlags(cmd *cobra.Command) (accountName, resourceName string, client *storage.Client, err error) {
	accountName, _ = cmd.Flags().GetString("account-name")
	resourceName, _ = cmd.Flags().GetString("name")

	token, err := getAccessTokenForScope(cmd.Context(), storageScope)
	if err != nil {
		return "", "", nil, err
	}
	return accountName, resourceName, storage.NewClient(token), nil
}

// getAccessTokenForScope returns an access token for the given OAuth2 scope.
// In GitHub Actions: exchanges a fresh OIDC token for a scoped access token.
// Locally: returns the cached token from login.
func getAccessTokenForScope(ctx context.Context, scope string) (string, error) {
	cfg := config.NewConfig()
	savedToken, err := cfg.LoadToken()
	if err != nil {
		return "", fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	if os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL") == "" {
		return savedToken.AccessToken, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	oidcToken, err := auth.GetGitHubOIDCToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OIDC token: %w", err)
	}

	client := auth.NewClientWithScope(
		savedToken.TenantID,
		savedToken.ClientID,
		savedToken.SubscriptionID,
		scope,
	)
	token, err := client.ExchangeOIDCToken(ctx, oidcToken)
	if err != nil {
		return "", fmt.Errorf("failed to exchange token for scope %s: %w", scope, err)
	}
	return token.AccessToken, nil
}
