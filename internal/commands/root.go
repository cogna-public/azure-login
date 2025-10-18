// Package commands implements the CLI command structure for azure-login.
//
// This package provides commands for authentication (login), account management
// (show, get-access-token), and version information using the Cobra CLI framework.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "azure-login",
	Short: "Lightweight Azure authentication CLI tool",
	Long: `azure-login is a statically-linked Go tool for Azure authentication.
It provides a drop-in replacement for Azure CLI authentication commands
in CI/CD environments, particularly GitHub Actions.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(v, c, d string) error {
	version = v
	commit = c
	date = d
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(accountCmd)
	rootCmd.AddCommand(aksCmd)
	rootCmd.AddCommand(kubectlCredentialCmd)
	rootCmd.AddCommand(oidcCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("azure-login version %s (commit: %s, built: %s)\n", version, commit, date)
	},
}
