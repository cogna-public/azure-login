package aks

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Kubeconfig represents a Kubernetes configuration file
type Kubeconfig struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	CurrentContext string         `yaml:"current-context"`
	Clusters       []NamedCluster `yaml:"clusters"`
	Contexts       []NamedContext `yaml:"contexts"`
	Users          []NamedUser    `yaml:"users"`
	Preferences    map[string]any `yaml:"preferences,omitempty"`
}

// NamedCluster represents a cluster entry in kubeconfig
type NamedCluster struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

// Cluster represents cluster connection details
type Cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

// NamedContext represents a context entry in kubeconfig
type NamedContext struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Context represents a context (cluster + user + namespace)
type Context struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

// NamedUser represents a user entry in kubeconfig
type NamedUser struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

// User represents user authentication configuration
type User struct {
	Exec *ExecConfig `yaml:"exec,omitempty"`
}

// ExecConfig represents exec-based authentication
type ExecConfig struct {
	APIVersion         string       `yaml:"apiVersion"`
	Command            string       `yaml:"command"`
	Args               []string     `yaml:"args,omitempty"`
	Env                []ExecEnvVar `yaml:"env,omitempty"`
	InteractiveMode    string       `yaml:"interactiveMode,omitempty"`
	ProvideClusterInfo bool         `yaml:"provideClusterInfo,omitempty"`
}

// ExecEnvVar represents an environment variable for exec auth
type ExecEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// GetKubeconfigPath returns the path to the kubeconfig file
func GetKubeconfigPath() string {
	// Check KUBECONFIG environment variable
	if path := os.Getenv("KUBECONFIG"); path != "" {
		return path
	}

	// Default to ~/.kube/config
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".kube", "config")
	}
	return filepath.Join(home, ".kube", "config")
}

// LoadKubeconfig loads an existing kubeconfig or creates a new one
func LoadKubeconfig(path string) (*Kubeconfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty kubeconfig
			return &Kubeconfig{
				APIVersion:  "v1",
				Kind:        "Config",
				Clusters:    []NamedCluster{},
				Contexts:    []NamedContext{},
				Users:       []NamedUser{},
				Preferences: map[string]any{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	var config Kubeconfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Initialize slices if nil
	if config.Clusters == nil {
		config.Clusters = []NamedCluster{}
	}
	if config.Contexts == nil {
		config.Contexts = []NamedContext{}
	}
	if config.Users == nil {
		config.Users = []NamedUser{}
	}

	return &config, nil
}

// SaveKubeconfig saves the kubeconfig to disk atomically
func SaveKubeconfig(path string, config *Kubeconfig) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	// Write to temp file, then rename (atomic)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	return nil
}

// MergeClusterCredentials merges AKS cluster credentials into kubeconfig
func (k *Kubeconfig) MergeClusterCredentials(creds *ClusterCredentials) {
	clusterName := creds.ClusterName
	contextName := clusterName
	userName := fmt.Sprintf("clusterUser_%s_%s", creds.ResourceGroup, creds.ClusterName)

	// Encode CA certificate to base64
	caCertBase64 := base64.StdEncoding.EncodeToString(creds.CACertificate)

	// Add or update cluster
	k.upsertCluster(clusterName, creds.ServerURL, caCertBase64)

	// Add or update user with Azure CLI authentication
	k.upsertUser(userName)

	// Add or update context
	k.upsertContext(contextName, clusterName, userName)

	// Set as current context
	k.CurrentContext = contextName
}

func (k *Kubeconfig) upsertCluster(name, server, caCert string) {
	for i, cluster := range k.Clusters {
		if cluster.Name == name {
			k.Clusters[i].Cluster.Server = server
			k.Clusters[i].Cluster.CertificateAuthorityData = caCert
			return
		}
	}

	// Add new cluster
	k.Clusters = append(k.Clusters, NamedCluster{
		Name: name,
		Cluster: Cluster{
			Server:                   server,
			CertificateAuthorityData: caCert,
		},
	})
}

func (k *Kubeconfig) upsertUser(name string) {
	for i, user := range k.Users {
		if user.Name == name {
			// Update existing user with Azure CLI auth
			k.Users[i].User = User{
				Exec: &ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    "kubelogin",
					Args: []string{
						"get-token",
						"--login",
						"azurecli",
					},
				},
			}
			return
		}
	}

	// Add new user with Azure CLI auth
	k.Users = append(k.Users, NamedUser{
		Name: name,
		User: User{
			Exec: &ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1beta1",
				Command:    "kubelogin",
				Args: []string{
					"get-token",
					"--login",
					"azurecli",
				},
			},
		},
	})
}

func (k *Kubeconfig) upsertContext(name, cluster, user string) {
	for i, ctx := range k.Contexts {
		if ctx.Name == name {
			k.Contexts[i].Context.Cluster = cluster
			k.Contexts[i].Context.User = user
			return
		}
	}

	// Add new context
	k.Contexts = append(k.Contexts, NamedContext{
		Name: name,
		Context: Context{
			Cluster: cluster,
			User:    user,
		},
	})
}
