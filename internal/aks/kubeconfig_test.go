package aks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadKubeconfig_NewFile(t *testing.T) {
	// Test loading a non-existent file (should create empty config)
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")

	config, err := LoadKubeconfig(kubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to load non-existent kubeconfig: %v", err)
	}

	if config.APIVersion != "v1" {
		t.Errorf("Expected APIVersion v1, got %s", config.APIVersion)
	}
	if config.Kind != "Config" {
		t.Errorf("Expected Kind Config, got %s", config.Kind)
	}
	if len(config.Clusters) != 0 {
		t.Errorf("Expected empty clusters, got %d", len(config.Clusters))
	}
}

func TestLoadKubeconfig_ExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")

	// Create a sample kubeconfig
	existingConfig := `apiVersion: v1
kind: Config
current-context: existing-cluster
clusters:
- name: existing-cluster
  cluster:
    server: https://existing.example.com
    certificate-authority-data: ZXhpc3RpbmctY2VydA==
contexts:
- name: existing-cluster
  context:
    cluster: existing-cluster
    user: existing-user
users:
- name: existing-user
  user:
    token: existing-token
`
	if err := os.WriteFile(kubeconfigPath, []byte(existingConfig), 0600); err != nil {
		t.Fatalf("Failed to write test kubeconfig: %v", err)
	}

	config, err := LoadKubeconfig(kubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to load existing kubeconfig: %v", err)
	}

	if len(config.Clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(config.Clusters))
	}
	if config.Clusters[0].Name != "existing-cluster" {
		t.Errorf("Expected cluster name existing-cluster, got %s", config.Clusters[0].Name)
	}
	if config.CurrentContext != "existing-cluster" {
		t.Errorf("Expected current-context existing-cluster, got %s", config.CurrentContext)
	}
}

func TestSaveKubeconfig(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")

	config := &Kubeconfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "test-cluster",
		Clusters: []NamedCluster{
			{
				Name: "test-cluster",
				Cluster: Cluster{
					Server:                   "https://test.example.com",
					CertificateAuthorityData: "dGVzdC1jZXJ0",
				},
			},
		},
		Contexts: []NamedContext{
			{
				Name: "test-cluster",
				Context: Context{
					Cluster: "test-cluster",
					User:    "test-user",
				},
			},
		},
		Users: []NamedUser{
			{
				Name: "test-user",
				User: User{
					Exec: &ExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    "kubelogin",
						Args:       []string{"get-token", "--login", "azurecli"},
					},
				},
			},
		},
	}

	err := SaveKubeconfig(kubeconfigPath, config)
	if err != nil {
		t.Fatalf("Failed to save kubeconfig: %v", err)
	}

	// Verify file exists and has correct permissions
	info, err := os.Stat(kubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to stat kubeconfig: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Verify content
	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to read kubeconfig: %v", err)
	}

	if !strings.Contains(string(data), "test-cluster") {
		t.Error("Expected kubeconfig to contain test-cluster")
	}
	if !strings.Contains(string(data), "kubelogin") {
		t.Error("Expected kubeconfig to contain kubelogin")
	}
}

func TestMergeClusterCredentials_NewCluster(t *testing.T) {
	config := &Kubeconfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   []NamedCluster{},
		Contexts:   []NamedContext{},
		Users:      []NamedUser{},
	}

	credentials := &ClusterCredentials{
		ClusterName:    "new-cluster",
		ServerURL:      "https://new-cluster.example.com",
		CACertificate:  []byte("test-ca-cert"),
		ResourceGroup:  "test-rg",
		SubscriptionID: "test-sub",
	}

	config.MergeClusterCredentials(credentials)

	// Verify cluster was added
	if len(config.Clusters) != 1 {
		t.Fatalf("Expected 1 cluster, got %d", len(config.Clusters))
	}
	if config.Clusters[0].Name != "new-cluster" {
		t.Errorf("Expected cluster name new-cluster, got %s", config.Clusters[0].Name)
	}
	if config.Clusters[0].Cluster.Server != "https://new-cluster.example.com" {
		t.Errorf("Expected server URL https://new-cluster.example.com, got %s", config.Clusters[0].Cluster.Server)
	}

	// Verify user was added
	if len(config.Users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(config.Users))
	}
	expectedUserName := "clusterUser_test-rg_new-cluster"
	if config.Users[0].Name != expectedUserName {
		t.Errorf("Expected user name %s, got %s", expectedUserName, config.Users[0].Name)
	}
	if config.Users[0].User.Exec == nil {
		t.Fatal("Expected exec config to be set")
	}
	if config.Users[0].User.Exec.Command != "kubelogin" {
		t.Errorf("Expected command kubelogin, got %s", config.Users[0].User.Exec.Command)
	}

	// Verify context was added
	if len(config.Contexts) != 1 {
		t.Fatalf("Expected 1 context, got %d", len(config.Contexts))
	}
	if config.Contexts[0].Name != "new-cluster" {
		t.Errorf("Expected context name new-cluster, got %s", config.Contexts[0].Name)
	}
	if config.Contexts[0].Context.Cluster != "new-cluster" {
		t.Errorf("Expected cluster new-cluster, got %s", config.Contexts[0].Context.Cluster)
	}

	// Verify current context was set
	if config.CurrentContext != "new-cluster" {
		t.Errorf("Expected current-context new-cluster, got %s", config.CurrentContext)
	}
}

func TestMergeClusterCredentials_UpdateExisting(t *testing.T) {
	config := &Kubeconfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []NamedCluster{
			{
				Name: "existing-cluster",
				Cluster: Cluster{
					Server:                   "https://old-url.example.com",
					CertificateAuthorityData: "b2xkLWNlcnQ=",
				},
			},
		},
		Contexts: []NamedContext{
			{
				Name: "existing-cluster",
				Context: Context{
					Cluster: "existing-cluster",
					User:    "old-user",
				},
			},
		},
		Users: []NamedUser{
			{
				Name: "old-user",
				User: User{},
			},
		},
	}

	credentials := &ClusterCredentials{
		ClusterName:    "existing-cluster",
		ServerURL:      "https://new-url.example.com",
		CACertificate:  []byte("new-ca-cert"),
		ResourceGroup:  "test-rg",
		SubscriptionID: "test-sub",
	}

	config.MergeClusterCredentials(credentials)

	// Verify cluster was updated (not duplicated)
	if len(config.Clusters) != 1 {
		t.Fatalf("Expected 1 cluster, got %d", len(config.Clusters))
	}
	if config.Clusters[0].Cluster.Server != "https://new-url.example.com" {
		t.Errorf("Expected updated server URL, got %s", config.Clusters[0].Cluster.Server)
	}

	// Verify user was updated/added
	expectedUserName := "clusterUser_test-rg_existing-cluster"
	found := false
	for _, user := range config.Users {
		if user.Name == expectedUserName {
			found = true
			if user.User.Exec == nil {
				t.Error("Expected exec config to be set")
			}
		}
	}
	if !found {
		t.Errorf("Expected user %s to be present", expectedUserName)
	}

	// Verify context was updated
	if len(config.Contexts) != 1 {
		t.Fatalf("Expected 1 context, got %d", len(config.Contexts))
	}
	if config.Contexts[0].Context.User != expectedUserName {
		t.Errorf("Expected context user %s, got %s", expectedUserName, config.Contexts[0].Context.User)
	}
}

func TestGetKubeconfigPath_EnvVar(t *testing.T) {
	// Set custom KUBECONFIG env var
	customPath := "/custom/path/to/config"
	_ = os.Setenv("KUBECONFIG", customPath)
	defer func() {
		_ = os.Unsetenv("KUBECONFIG")
	}()

	path := GetKubeconfigPath()
	if path != customPath {
		t.Errorf("Expected path %s, got %s", customPath, path)
	}
}

func TestGetKubeconfigPath_Default(t *testing.T) {
	// Unset KUBECONFIG env var
	_ = os.Unsetenv("KUBECONFIG")

	path := GetKubeconfigPath()
	if !strings.Contains(path, ".kube") {
		t.Errorf("Expected path to contain .kube, got %s", path)
	}
	if !strings.Contains(path, "config") {
		t.Errorf("Expected path to contain config, got %s", path)
	}
}

func TestKubeconfigYAMLMarshaling(t *testing.T) {
	config := &Kubeconfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "test-cluster",
		Clusters: []NamedCluster{
			{
				Name: "test-cluster",
				Cluster: Cluster{
					Server:                   "https://test.example.com",
					CertificateAuthorityData: "dGVzdA==",
				},
			},
		},
		Contexts: []NamedContext{
			{
				Name: "test-cluster",
				Context: Context{
					Cluster:   "test-cluster",
					User:      "test-user",
					Namespace: "default",
				},
			},
		},
		Users: []NamedUser{
			{
				Name: "test-user",
				User: User{
					Exec: &ExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    "kubelogin",
						Args:       []string{"get-token", "--login", "azurecli"},
						Env: []ExecEnvVar{
							{Name: "TEST_VAR", Value: "test-value"},
						},
					},
				},
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal kubeconfig: %v", err)
	}

	// Verify YAML structure
	yamlStr := string(data)
	if !strings.Contains(yamlStr, "apiVersion: v1") {
		t.Error("Expected YAML to contain apiVersion: v1")
	}
	if !strings.Contains(yamlStr, "kind: Config") {
		t.Error("Expected YAML to contain kind: Config")
	}
	if !strings.Contains(yamlStr, "command: kubelogin") {
		t.Error("Expected YAML to contain command: kubelogin")
	}

	// Unmarshal back
	var unmarshaled Kubeconfig
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal kubeconfig: %v", err)
	}

	if unmarshaled.CurrentContext != "test-cluster" {
		t.Errorf("Expected current-context test-cluster, got %s", unmarshaled.CurrentContext)
	}
	if len(unmarshaled.Clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(unmarshaled.Clusters))
	}
}

func TestSaveKubeconfig_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")

	config := &Kubeconfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   []NamedCluster{},
	}

	// Save the config
	err := SaveKubeconfig(kubeconfigPath, config)
	if err != nil {
		t.Fatalf("Failed to save kubeconfig: %v", err)
	}

	// Verify temp file was cleaned up
	tmpPath := kubeconfigPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Expected temp file to be cleaned up")
	}

	// Verify final file exists
	if _, err := os.Stat(kubeconfigPath); err != nil {
		t.Errorf("Expected kubeconfig file to exist: %v", err)
	}
}
