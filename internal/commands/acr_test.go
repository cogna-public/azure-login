package commands

import (
	"strings"
	"testing"
)

func TestACRLogin_NotAuthenticated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AZURE_CONFIG_DIR", dir)

	registryName = "myregistry"

	err := runACRLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error when not authenticated, got none")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("Expected 'not authenticated' error, got: %v", err)
	}
}

func TestACRLogin_RequiresRegistryName(t *testing.T) {
	cmd := acrLoginCmd

	// The --name flag should be marked required
	flag := cmd.Flags().Lookup("name")
	if flag == nil {
		t.Fatal("Expected --name flag to be defined")
	}

	annotations := flag.Annotations
	if annotations == nil {
		t.Fatal("Expected --name flag to have required annotation")
	}

	required, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]
	if !ok || len(required) == 0 {
		t.Error("Expected --name flag to be marked as required")
	}
}

func TestACRCommandStructure(t *testing.T) {
	if acrCmd.Use != "acr" {
		t.Errorf("Expected acrCmd.Use to be 'acr', got %q", acrCmd.Use)
	}

	found := false
	for _, sub := range acrCmd.Commands() {
		if sub.Use == "login" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'login' subcommand under 'acr'")
	}
}

func TestACRCommandRegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "acr" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'acr' command to be registered in root")
	}
}

func TestACRLogin_FlagShorthand(t *testing.T) {
	flag := acrLoginCmd.Flags().ShorthandLookup("n")
	if flag == nil {
		t.Fatal("Expected -n shorthand for --name flag")
	}
	if flag.Name != "name" {
		t.Errorf("Expected -n to map to 'name', got %q", flag.Name)
	}
}

func TestACRLogin_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AZURE_CONFIG_DIR", dir)

	registryName = "myregistry"
	err := runACRLogin(nil, []string{})
	if err == nil {
		t.Fatal("Expected error, got none")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("Expected 'not authenticated' error (using temp dir), got: %v", err)
	}
}
