package acr

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDockerConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	err := WriteDockerConfig("myregistry.azurecr.io", NullGUID, "refresh-token-abc")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	auths, ok := cfg["auths"].(map[string]any)
	if !ok {
		t.Fatal("Missing auths key in config")
	}

	entry, ok := auths["myregistry.azurecr.io"].(map[string]any)
	if !ok {
		t.Fatal("Missing registry entry in auths")
	}

	authStr, ok := entry["auth"].(string)
	if !ok {
		t.Fatal("Missing auth field in registry entry")
	}

	decoded, err := base64.StdEncoding.DecodeString(authStr)
	if err != nil {
		t.Fatalf("Failed to decode auth: %v", err)
	}

	expected := NullGUID + ":refresh-token-abc"
	if string(decoded) != expected {
		t.Errorf("Expected decoded auth %q, got %q", expected, string(decoded))
	}
}

func TestWriteDockerConfig_MergeExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	existing := `{
	"auths": {
		"docker.io": {
			"auth": "ZXhpc3Rpbmc6Y3JlZHM="
		}
	},
	"credsStore": "desktop"
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(existing), 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	err := WriteDockerConfig("myregistry.azurecr.io", NullGUID, "new-token")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify existing entry preserved
	auths := cfg["auths"].(map[string]any)
	if _, ok := auths["docker.io"]; !ok {
		t.Error("Existing docker.io entry was lost during merge")
	}

	// Verify new entry added
	if _, ok := auths["myregistry.azurecr.io"]; !ok {
		t.Error("New ACR entry was not added")
	}

	// Verify credsStore preserved
	if cfg["credsStore"] == nil {
		t.Error("credsStore was lost during merge")
	}
}

func TestWriteDockerConfig_OverwriteExistingACR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	existing := `{
	"auths": {
		"myregistry.azurecr.io": {
			"auth": "b2xkOnRva2Vu"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(existing), 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	err := WriteDockerConfig("myregistry.azurecr.io", NullGUID, "brand-new-token")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	entry, ok := cfg.Auths["myregistry.azurecr.io"]
	if !ok {
		t.Fatal("ACR entry missing after overwrite")
	}

	decoded, _ := base64.StdEncoding.DecodeString(entry.Auth)
	expected := NullGUID + ":brand-new-token"
	if string(decoded) != expected {
		t.Errorf("Expected updated auth %q, got %q", expected, string(decoded))
	}
}

func TestWriteDockerConfig_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	err := WriteDockerConfig("myregistry.azurecr.io", NullGUID, "token")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Failed to stat config: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected file permissions 0600, got %04o", perm)
	}
}

func TestWriteDockerConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested", "docker")
	t.Setenv("DOCKER_CONFIG", nested)

	err := WriteDockerConfig("myregistry.azurecr.io", NullGUID, "token")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(nested, "config.json")); err != nil {
		t.Errorf("Config file should exist in created directory: %v", err)
	}
}

func TestDockerConfigRoundTrip_PreservesExtraFields(t *testing.T) {
	input := `{
	"auths": {
		"docker.io": {"auth": "dGVzdDp0ZXN0"}
	},
	"credsStore": "osxkeychain",
	"credHelpers": {
		"gcr.io": "gcr"
	}
}`

	var cfg dockerConfig
	if err := json.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(cfg.Auths) != 1 {
		t.Errorf("Expected 1 auth entry, got %d", len(cfg.Auths))
	}

	if len(cfg.Extra) != 2 {
		t.Errorf("Expected 2 extra fields (credsStore, credHelpers), got %d", len(cfg.Extra))
	}

	output, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var roundTripped map[string]any
	if err := json.Unmarshal(output, &roundTripped); err != nil {
		t.Fatalf("Failed to parse round-tripped JSON: %v", err)
	}

	if roundTripped["credsStore"] == nil {
		t.Error("credsStore lost during round-trip")
	}
	if roundTripped["credHelpers"] == nil {
		t.Error("credHelpers lost during round-trip")
	}
	if roundTripped["auths"] == nil {
		t.Error("auths lost during round-trip")
	}
}
