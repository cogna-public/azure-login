package acr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExchangeAADToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		if got := r.FormValue("grant_type"); got != "access_token" {
			t.Errorf("Expected grant_type=access_token, got %s", got)
		}
		if got := r.FormValue("service"); got != "myregistry.azurecr.io" {
			t.Errorf("Expected service=myregistry.azurecr.io, got %s", got)
		}
		if got := r.FormValue("tenant"); got != "test-tenant" {
			t.Errorf("Expected tenant=test-tenant, got %s", got)
		}
		if got := r.FormValue("access_token"); got != "aad-token-value" {
			t.Errorf("Expected access_token=aad-token-value, got %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"refresh_token":"acr-refresh-token-value"}`)
	}))
	defer server.Close()

	client := &Client{
		registry:    "myregistry.azurecr.io",
		tenantID:    "test-tenant",
		aadToken:    "aad-token-value",
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		endpointURL: server.URL,
	}

	token, err := client.ExchangeAADToken(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if token != "acr-refresh-token-value" {
		t.Errorf("Expected refresh token 'acr-refresh-token-value', got '%s'", token)
	}
}

func TestExchangeAADToken_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer server.Close()

	client := &Client{
		registry:    "myregistry.azurecr.io",
		tenantID:    "test-tenant",
		aadToken:    "bad-token",
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		endpointURL: server.URL,
	}

	_, err := client.ExchangeAADToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for 401 response, got none")
	}
}

func TestExchangeAADToken_EmptyRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"refresh_token":""}`)
	}))
	defer server.Close()

	client := &Client{
		registry:    "myregistry.azurecr.io",
		tenantID:    "test-tenant",
		aadToken:    "aad-token",
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		endpointURL: server.URL,
	}

	_, err := client.ExchangeAADToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for empty refresh token, got none")
	}
}

func TestExchangeAADToken_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{not valid json}`)
	}))
	defer server.Close()

	client := &Client{
		registry:    "myregistry.azurecr.io",
		tenantID:    "test-tenant",
		aadToken:    "aad-token",
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		endpointURL: server.URL,
	}

	_, err := client.ExchangeAADToken(context.Background())
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got none")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("myregistry.azurecr.io", "tenant-id", "access-token")

	if client.registry != "myregistry.azurecr.io" {
		t.Errorf("Expected registry myregistry.azurecr.io, got %s", client.registry)
	}
	if client.tenantID != "tenant-id" {
		t.Errorf("Expected tenantID tenant-id, got %s", client.tenantID)
	}
	if client.aadToken != "access-token" {
		t.Errorf("Expected aadToken access-token, got %s", client.aadToken)
	}
	if client.httpClient == nil {
		t.Fatal("Expected httpClient to be initialized")
	}
	if client.httpClient.Timeout != ExchangeTimeout {
		t.Errorf("Expected timeout %v, got %v", ExchangeTimeout, client.httpClient.Timeout)
	}
	if client.endpointURL != "" {
		t.Errorf("Expected empty endpointURL for production client, got %s", client.endpointURL)
	}
}

func TestNormalizeRegistry(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Short name", "myregistry", "myregistry.azurecr.io"},
		{"Already FQDN", "myregistry.azurecr.io", "myregistry.azurecr.io"},
		{"Custom domain", "registry.contoso.com", "registry.contoso.com"},
		{"With subdomain", "my.custom.registry.io", "my.custom.registry.io"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRegistry(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeRegistry(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
