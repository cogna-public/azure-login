package acr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// dockerConfig represents the structure of ~/.docker/config.json.
// We use json.RawMessage for unknown fields to preserve them during round-trip.
type dockerConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths,omitempty"`

	// Preserve all other top-level keys (credHelpers, credsStore, etc.)
	Extra map[string]json.RawMessage `json:"-"`
}

type dockerAuthEntry struct {
	Auth string `json:"auth"`
}

// MarshalJSON implements custom marshalling to merge Auths back with Extra fields.
func (d dockerConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(d.Extra)+1)
	for k, v := range d.Extra {
		m[k] = v
	}
	if d.Auths != nil {
		m["auths"] = d.Auths
	}
	return json.MarshalIndent(m, "", "\t")
}

// UnmarshalJSON implements custom unmarshalling to capture Auths and preserve Extra fields.
func (d *dockerConfig) UnmarshalJSON(data []byte) error {
	// First pass: grab everything as raw messages
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Extra = make(map[string]json.RawMessage)

	for k, v := range raw {
		if k == "auths" {
			if err := json.Unmarshal(v, &d.Auths); err != nil {
				return fmt.Errorf("failed to parse auths: %w", err)
			}
		} else {
			d.Extra[k] = v
		}
	}

	return nil
}

// WriteDockerConfig merges ACR credentials into the Docker config file.
// It reads the existing config, adds/updates the auth entry for the given registry,
// and writes it back atomically.
func WriteDockerConfig(registry, username, password string) error {
	configPath := dockerConfigPath()

	cfg, err := loadDockerConfig(configPath)
	if err != nil {
		return err
	}

	if cfg.Auths == nil {
		cfg.Auths = make(map[string]dockerAuthEntry)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cfg.Auths[registry] = dockerAuthEntry{Auth: encoded}

	return saveDockerConfig(configPath, cfg)
}

func dockerConfigPath() string {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".docker", "config.json")
	}
	return filepath.Join(home, ".docker", "config.json")
}

func loadDockerConfig(path string) (*dockerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &dockerConfig{
				Auths: make(map[string]dockerAuthEntry),
				Extra: make(map[string]json.RawMessage),
			}, nil
		}
		return nil, fmt.Errorf("failed to read Docker config %s: %w", path, err)
	}

	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Docker config %s: %w", path, err)
	}

	return &cfg, nil
}

func saveDockerConfig(path string, cfg *dockerConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create Docker config directory: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal Docker config: %w", err)
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write Docker config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to save Docker config: %w", err)
	}

	return nil
}
