package aks

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetClusterCredentials_Success(t *testing.T) {
	// Create mock kubeconfig YAML
	mockKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJZVlLQ3RWUU1ZMHM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://test-cluster.hcp.eastus.azmk8s.io:443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: clusterUser_test-rg_test-cluster
  name: test-cluster
current-context: test-cluster
users:
- name: clusterUser_test-rg_test-cluster
  user:
    token: mock-token
`
	base64Kubeconfig := base64.StdEncoding.EncodeToString([]byte(mockKubeconfig))

	// Create mock Azure API server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Verify authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Errorf("Expected Bearer token authorization")
		}
		if authHeader != "Bearer mock-access-token" {
			t.Errorf("Expected Bearer mock-access-token, got %s", authHeader)
		}

		// First call: Get cluster info
		if callCount == 1 {
			if r.Method != "GET" {
				t.Errorf("Expected GET request for cluster info, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/managedClusters/test-cluster") {
				t.Errorf("Expected path to contain /managedClusters/test-cluster, got %s", r.URL.Path)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"id": "/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster",
				"name": "test-cluster",
				"location": "eastus",
				"properties": {
					"fqdn": "test-cluster.hcp.eastus.azmk8s.io"
				}
			}`)
			return
		}

		// Second call: Get cluster credentials
		if callCount == 2 {
			if r.Method != "POST" {
				t.Errorf("Expected POST request for credentials, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/listClusterUserCredential") {
				t.Errorf("Expected path to contain /listClusterUserCredential, got %s", r.URL.Path)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"kubeconfigs": [
					{
						"name": "clusterUser",
						"value": "%s"
					}
				]
			}`, base64Kubeconfig)
			return
		}
	}))
	defer server.Close()

	// Create AKS client with mock server URL
	client := &Client{
		subscriptionID: "test-subscription",
		accessToken:    "mock-access-token",
		httpClient:     &http.Client{},
	}

	// Override the base URL for testing
	originalURL := AzureManagementURL
	defer func() {
		// Can't actually change the const, but this shows the test design
		_ = originalURL
	}()

	// Get cluster credentials (using mock server URLs)
	ctx := context.Background()

	// Manually construct the URLs to use the test server
	clusterURL := server.URL + "/subscriptions/test-subscription/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster?api-version=2023-01-01"
	credentialsURL := server.URL + "/subscriptions/test-subscription/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster/listClusterUserCredential?api-version=2023-01-01"

	// Test individual methods
	_, err := client.getClusterInfo(ctx, clusterURL)
	if err != nil {
		t.Fatalf("Failed to get cluster info: %v", err)
	}

	credentials, err := client.getClusterUserCredentials(ctx, credentialsURL)
	if err != nil {
		t.Fatalf("Failed to get cluster credentials: %v", err)
	}

	if len(credentials.Kubeconfigs) == 0 {
		t.Fatal("Expected kubeconfigs, got none")
	}

	// Verify we can decode and parse the kubeconfig
	kubeconfigData, err := base64.StdEncoding.DecodeString(credentials.Kubeconfigs[0].Value)
	if err != nil {
		t.Fatalf("Failed to decode kubeconfig: %v", err)
	}

	if !strings.Contains(string(kubeconfigData), "test-cluster") {
		t.Error("Expected kubeconfig to contain cluster name")
	}
}

func TestGetClusterCredentials_ClusterNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{
			"error": {
				"code": "ResourceNotFound",
				"message": "The Resource 'Microsoft.ContainerService/managedClusters/nonexistent' was not found."
			}
		}`)
	}))
	defer server.Close()

	client := &Client{
		subscriptionID: "test-subscription",
		accessToken:    "mock-access-token",
		httpClient:     &http.Client{},
	}

	ctx := context.Background()
	clusterURL := server.URL + "/test"

	_, err := client.getClusterInfo(ctx, clusterURL)
	if err == nil {
		t.Error("Expected error for non-existent cluster, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected 404 error, got: %v", err)
	}
}

func TestGetClusterCredentials_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, `{
			"error": {
				"code": "AuthenticationFailed",
				"message": "Authentication failed."
			}
		}`)
	}))
	defer server.Close()

	client := &Client{
		subscriptionID: "test-subscription",
		accessToken:    "invalid-token",
		httpClient:     &http.Client{},
	}

	ctx := context.Background()
	clusterURL := server.URL + "/test"

	_, err := client.getClusterInfo(ctx, clusterURL)
	if err == nil {
		t.Error("Expected error for unauthorized request, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}
}

func TestExtractClusterInfo_Success(t *testing.T) {
	kubeconfigMap := map[string]any{
		"clusters": []any{
			map[string]any{
				"name": "test-cluster",
				"cluster": map[string]any{
					"server":                     "https://test-cluster.hcp.eastus.azmk8s.io:443",
					"certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJZVlLQ3RWUU1ZMHM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",
				},
			},
		},
	}

	serverURL, caCert, err := extractClusterInfo(kubeconfigMap)
	if err != nil {
		t.Fatalf("Failed to extract cluster info: %v", err)
	}

	if serverURL != "https://test-cluster.hcp.eastus.azmk8s.io:443" {
		t.Errorf("Expected server URL https://test-cluster.hcp.eastus.azmk8s.io:443, got %s", serverURL)
	}

	if len(caCert) == 0 {
		t.Error("Expected CA certificate data, got empty")
	}
}

func TestExtractClusterInfo_MissingClusters(t *testing.T) {
	kubeconfigMap := map[string]any{
		"users": []any{},
	}

	_, _, err := extractClusterInfo(kubeconfigMap)
	if err == nil {
		t.Error("Expected error for missing clusters, got nil")
	}
	if !strings.Contains(err.Error(), "no clusters") {
		t.Errorf("Expected 'no clusters' error, got: %v", err)
	}
}

func TestExtractClusterInfo_MissingServerURL(t *testing.T) {
	kubeconfigMap := map[string]any{
		"clusters": []any{
			map[string]any{
				"name": "test-cluster",
				"cluster": map[string]any{
					"certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==",
				},
			},
		},
	}

	_, _, err := extractClusterInfo(kubeconfigMap)
	if err == nil {
		t.Error("Expected error for missing server URL, got nil")
	}
	if !strings.Contains(err.Error(), "no server URL") {
		t.Errorf("Expected 'no server URL' error, got: %v", err)
	}
}

func TestExtractClusterInfo_MissingCACert(t *testing.T) {
	kubeconfigMap := map[string]any{
		"clusters": []any{
			map[string]any{
				"name": "test-cluster",
				"cluster": map[string]any{
					"server": "https://test-cluster.hcp.eastus.azmk8s.io:443",
				},
			},
		},
	}

	_, _, err := extractClusterInfo(kubeconfigMap)
	if err == nil {
		t.Error("Expected error for missing CA certificate, got nil")
	}
	if !strings.Contains(err.Error(), "no CA certificate") {
		t.Errorf("Expected 'no CA certificate' error, got: %v", err)
	}
}

func TestExtractClusterInfo_InvalidBase64(t *testing.T) {
	kubeconfigMap := map[string]any{
		"clusters": []any{
			map[string]any{
				"name": "test-cluster",
				"cluster": map[string]any{
					"server":                     "https://test-cluster.hcp.eastus.azmk8s.io:443",
					"certificate-authority-data": "not-valid-base64!!!",
				},
			},
		},
	}

	_, _, err := extractClusterInfo(kubeconfigMap)
	if err == nil {
		t.Error("Expected error for invalid base64, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode CA certificate") {
		t.Errorf("Expected 'failed to decode' error, got: %v", err)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("test-sub", "test-token")

	if client.subscriptionID != "test-sub" {
		t.Errorf("Expected subscriptionID test-sub, got %s", client.subscriptionID)
	}
	if client.accessToken != "test-token" {
		t.Errorf("Expected accessToken test-token, got %s", client.accessToken)
	}
	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
}
