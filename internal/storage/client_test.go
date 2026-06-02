package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContainerCreate(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"created", http.StatusCreated, false},
		{"already exists", http.StatusConflict, false},
		{"forbidden", http.StatusForbidden, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertRequest(t, r, http.MethodPut, "restype=container", "test-token")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			client := newTestClient(srv, "test-token")
			err := client.ContainerCreate(context.Background(), "ignored", "test-container")

			if (err != nil) != tt.wantErr {
				t.Errorf("ContainerCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainerDelete(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"accepted", http.StatusAccepted, false},
		{"not found", http.StatusNotFound, false},
		{"forbidden", http.StatusForbidden, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertRequest(t, r, http.MethodDelete, "restype=container", "test-token")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			client := newTestClient(srv, "test-token")
			err := client.ContainerDelete(context.Background(), "ignored", "test-container")

			if (err != nil) != tt.wantErr {
				t.Errorf("ContainerDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueueCreate(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"created", http.StatusCreated, false},
		{"already exists same metadata", http.StatusNoContent, false},
		{"already exists different metadata", http.StatusConflict, false},
		{"forbidden", http.StatusForbidden, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertRequest(t, r, http.MethodPut, "", "test-token")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			client := newTestClient(srv, "test-token")
			err := client.QueueCreate(context.Background(), "ignored", "test-queue")

			if (err != nil) != tt.wantErr {
				t.Errorf("QueueCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueueDelete(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"deleted", http.StatusNoContent, false},
		{"not found", http.StatusNotFound, false},
		{"forbidden", http.StatusForbidden, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertRequest(t, r, http.MethodDelete, "", "test-token")
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			client := newTestClient(srv, "test-token")
			err := client.QueueDelete(context.Background(), "ignored", "test-queue")

			if (err != nil) != tt.wantErr {
				t.Errorf("QueueDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer secret-token")
		}
		version := r.Header.Get("x-ms-version")
		if version != apiVersion {
			t.Errorf("x-ms-version = %q, want %q", version, apiVersion)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClient(srv, "secret-token")
	_ = client.ContainerCreate(context.Background(), "ignored", "c")
}

func TestErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("AuthorizationFailure"))
	}))
	defer srv.Close()

	client := newTestClient(srv, "test-token")
	err := client.ContainerCreate(context.Background(), "ignored", "c")
	if err == nil || !strings.Contains(err.Error(), "AuthorizationFailure") {
		t.Errorf("expected error containing 'AuthorizationFailure', got %v", err)
	}
}

// newTestClient creates a client that routes all requests to the test server.
func newTestClient(srv *httptest.Server, token string) *Client {
	c := NewClient(token)
	c.httpClient = srv.Client()
	// Override the do method's URL construction by replacing the httpClient's
	// transport to redirect all requests to the test server.
	c.httpClient.Transport = &rewriteTransport{base: srv.Client().Transport, target: srv.URL}
	return c
}

// rewriteTransport redirects requests to the test server while preserving
// the path and query string for assertion.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.target, "http://")
	return t.base.RoundTrip(req)
}

func assertRequest(t *testing.T, r *http.Request, wantMethod, wantQuery, wantToken string) {
	t.Helper()
	if r.Method != wantMethod {
		t.Errorf("method = %s, want %s", r.Method, wantMethod)
	}
	if wantQuery != "" && !strings.Contains(r.URL.RawQuery, wantQuery) {
		t.Errorf("query = %q, want to contain %q", r.URL.RawQuery, wantQuery)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+wantToken)
	}
}
