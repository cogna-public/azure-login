// Package aks provides Azure Kubernetes Service credential management.
//
// This package handles retrieving AKS cluster credentials from Azure and
// updating kubeconfig files with the appropriate authentication configuration.
package aks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// AzureManagementURL is the base URL for Azure Management API
	AzureManagementURL = "https://management.azure.com"
	// AKSAPIVersion is the API version for AKS operations
	AKSAPIVersion = "2023-01-01"
	// RequestTimeout is the maximum time to wait for Azure API responses
	RequestTimeout = 30 * time.Second
)

// Client handles AKS operations
type Client struct {
	subscriptionID string
	accessToken    string
	httpClient     *http.Client
}

// NewClient creates a new AKS client
func NewClient(subscriptionID, accessToken string) *Client {
	return &Client{
		subscriptionID: subscriptionID,
		accessToken:    accessToken,
		httpClient:     &http.Client{Timeout: RequestTimeout},
	}
}

// ClusterCredentials represents the credentials for an AKS cluster
type ClusterCredentials struct {
	ClusterName    string
	ServerURL      string
	CACertificate  []byte
	ResourceGroup  string
	SubscriptionID string
	TenantID       string
	ClientID       string
}

// managedClusterResponse represents the Azure API response for a managed cluster
type managedClusterResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Properties struct {
		Fqdn              string `json:"fqdn"`
		AzurePortalFQDN   string `json:"azurePortalFQDN"`
		PrivateFQDN       string `json:"privateFQDN"`
		OidcIssuerProfile struct {
			IssuerURL string `json:"issuerURL"`
		} `json:"oidcIssuerProfile"`
		SecurityProfile struct {
			WorkloadIdentity struct {
				Enabled bool `json:"enabled"`
			} `json:"workloadIdentity"`
		} `json:"securityProfile"`
	} `json:"properties"`
}

// clusterUserCredentialResponse represents the credentials response
type clusterUserCredentialResponse struct {
	Kubeconfigs []struct {
		Name  string `json:"name"`
		Value string `json:"value"` // base64 encoded kubeconfig
	} `json:"kubeconfigs"`
}

// GetClusterCredentials retrieves AKS cluster credentials from Azure
func (c *Client) GetClusterCredentials(ctx context.Context, resourceGroup, clusterName string) (*ClusterCredentials, error) {
	// First, get the cluster information
	clusterURL := fmt.Sprintf(
		"%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s?api-version=%s",
		AzureManagementURL,
		c.subscriptionID,
		resourceGroup,
		clusterName,
		AKSAPIVersion,
	)

	_, err := c.getClusterInfo(ctx, clusterURL)
	if err != nil {
		return nil, err
	}

	// Get the user credentials
	credentialsURL := fmt.Sprintf(
		"%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s/listClusterUserCredential?api-version=%s",
		AzureManagementURL,
		c.subscriptionID,
		resourceGroup,
		clusterName,
		AKSAPIVersion,
	)

	credentials, err := c.getClusterUserCredentials(ctx, credentialsURL)
	if err != nil {
		return nil, err
	}

	// Decode the kubeconfig to extract CA certificate and server URL
	if len(credentials.Kubeconfigs) == 0 {
		return nil, fmt.Errorf("no kubeconfig returned from Azure")
	}

	kubeconfigData, err := base64.StdEncoding.DecodeString(credentials.Kubeconfigs[0].Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	var kubeconfigMap map[string]any
	if err := yaml.Unmarshal(kubeconfigData, &kubeconfigMap); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Extract server URL and CA certificate from the kubeconfig
	serverURL, caCert, err := extractClusterInfo(kubeconfigMap)
	if err != nil {
		return nil, err
	}

	return &ClusterCredentials{
		ClusterName:    clusterName,
		ServerURL:      serverURL,
		CACertificate:  caCert,
		ResourceGroup:  resourceGroup,
		SubscriptionID: c.subscriptionID,
	}, nil
}

func (c *Client) getClusterInfo(ctx context.Context, url string) (*managedClusterResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Azure API error (status %d): %s", resp.StatusCode, string(body))
	}

	var clusterInfo managedClusterResponse
	if err := json.Unmarshal(body, &clusterInfo); err != nil {
		return nil, fmt.Errorf("failed to parse cluster info: %w", err)
	}

	return &clusterInfo, nil
}

func (c *Client) getClusterUserCredentials(ctx context.Context, url string) (*clusterUserCredentialResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Azure API error (status %d): %s", resp.StatusCode, string(body))
	}

	var credentials clusterUserCredentialResponse
	if err := json.Unmarshal(body, &credentials); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &credentials, nil
}

func extractClusterInfo(kubeconfigMap map[string]any) (serverURL string, caCert []byte, err error) {
	// Extract clusters array
	clustersInterface, ok := kubeconfigMap["clusters"]
	if !ok {
		return "", nil, fmt.Errorf("no clusters found in kubeconfig")
	}

	clusters, ok := clustersInterface.([]any)
	if !ok || len(clusters) == 0 {
		return "", nil, fmt.Errorf("invalid clusters format in kubeconfig")
	}

	// Get first cluster
	firstCluster, ok := clusters[0].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("invalid cluster format")
	}

	clusterData, ok := firstCluster["cluster"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("invalid cluster data format")
	}

	// Extract server URL
	serverURL, ok = clusterData["server"].(string)
	if !ok {
		return "", nil, fmt.Errorf("no server URL found in cluster data")
	}

	// Extract CA certificate
	caCertBase64, ok := clusterData["certificate-authority-data"].(string)
	if !ok {
		return "", nil, fmt.Errorf("no CA certificate found in cluster data")
	}

	caCert, err = base64.StdEncoding.DecodeString(caCertBase64)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode CA certificate: %w", err)
	}

	return serverURL, caCert, nil
}
