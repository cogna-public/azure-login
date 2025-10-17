package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cogna-public/azure-login/internal/auth"
	"github.com/cogna-public/azure-login/pkg/config"
	"github.com/spf13/cobra"
)

var kubectlCredentialCmd = &cobra.Command{
	Use:    "kubectl-credential",
	Hidden: true, // Hidden from help output
	Short:  "Output credentials in kubectl ExecCredential format",
	Long:   `Output Azure credentials in kubectl ExecCredential format for use as an exec credential plugin.`,
	RunE:   runKubectlCredential,
}

func init() {
	// This command is for internal use by kubectl
}

// ExecCredential is the credential format expected by kubectl
type ExecCredential struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Status     ExecCredentialStatus `json:"status"`
}

// ExecCredentialStatus contains the token and expiration
type ExecCredentialStatus struct {
	Token               string `json:"token"`
	ExpirationTimestamp string `json:"expirationTimestamp"`
}

func runKubectlCredential(cmd *cobra.Command, args []string) error {
	// Load saved authentication details
	cfg := config.NewConfig()
	savedToken, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	// Get OIDC token from GitHub Actions
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	oidcToken, err := auth.GetGitHubOIDCToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OIDC token: %w", err)
	}

	// Exchange OIDC token for Kubernetes-scoped access token
	// Azure Kubernetes Service AAD Server application ID
	client := auth.NewClientWithScope(
		savedToken.TenantID,
		savedToken.ClientID,
		savedToken.SubscriptionID,
		"6dae42f8-4368-4678-94ff-3960e28e3630/.default", // AKS server scope
	)

	kubeToken, err := client.ExchangeOIDCToken(ctx, oidcToken)
	if err != nil {
		return fmt.Errorf("failed to exchange token for Kubernetes scope: %w", err)
	}

	// Create ExecCredential response
	credential := ExecCredential{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Kind:       "ExecCredential",
		Status: ExecCredentialStatus{
			Token:               kubeToken.AccessToken,
			ExpirationTimestamp: kubeToken.ExpiresOn.Format("2006-01-02T15:04:05Z"),
		},
	}

	// Output as JSON to stdout
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(credential); err != nil {
		return fmt.Errorf("failed to encode credential: %w", err)
	}

	return nil
}
