package commands

import (
	"encoding/json"
	"fmt"
	"os"

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
	// Load authentication token
	cfg := config.NewConfig()
	token, err := cfg.LoadToken()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'azure-login login' first")
	}

	// Create ExecCredential response
	credential := ExecCredential{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Kind:       "ExecCredential",
		Status: ExecCredentialStatus{
			Token:               token.AccessToken,
			ExpirationTimestamp: token.ExpiresOn.Format("2006-01-02T15:04:05Z"),
		},
	}

	// Output as JSON to stdout
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(credential); err != nil {
		return fmt.Errorf("failed to encode credential: %w", err)
	}

	return nil
}
