// Package main provides the azure-login CLI binary for Azure authentication via OIDC.
//
// This tool authenticates to Azure using GitHub Actions OIDC tokens, providing
// a lightweight alternative to the Azure CLI for CI/CD environments.
package main

import (
	"os"

	"github.com/cogna-public/azure-login/internal/commands"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := commands.Execute(version, commit, date); err != nil {
		_, _ = os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
}
