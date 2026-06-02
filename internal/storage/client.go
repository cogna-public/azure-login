// Package storage provides Azure Storage container and queue management.
package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiVersion     = "2023-11-03"
	requestTimeout = 30 * time.Second
)

// Client handles Azure Storage operations.
type Client struct {
	accessToken string
	httpClient  *http.Client
}

// NewClient creates a new Storage client.
func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: requestTimeout},
	}
}

// ContainerCreate creates a blob container. Returns nil if already exists.
func (c *Client) ContainerCreate(ctx context.Context, accountName, containerName string) error {
	url := fmt.Sprintf("https://%s.blob.core.windows.net/%s?restype=container", accountName, containerName)
	return c.do(ctx, http.MethodPut, url, http.StatusCreated, http.StatusConflict)
}

// ContainerDelete deletes a blob container. Returns nil if not found.
func (c *Client) ContainerDelete(ctx context.Context, accountName, containerName string) error {
	url := fmt.Sprintf("https://%s.blob.core.windows.net/%s?restype=container", accountName, containerName)
	return c.do(ctx, http.MethodDelete, url, http.StatusAccepted, http.StatusNotFound)
}

// QueueCreate creates a storage queue. Returns nil if already exists.
func (c *Client) QueueCreate(ctx context.Context, accountName, queueName string) error {
	url := fmt.Sprintf("https://%s.queue.core.windows.net/%s", accountName, queueName)
	return c.do(ctx, http.MethodPut, url, http.StatusCreated, http.StatusNoContent, http.StatusConflict)
}

// QueueDelete deletes a storage queue. Returns nil if not found.
func (c *Client) QueueDelete(ctx context.Context, accountName, queueName string) error {
	url := fmt.Sprintf("https://%s.queue.core.windows.net/%s", accountName, queueName)
	return c.do(ctx, http.MethodDelete, url, http.StatusNoContent, http.StatusNotFound)
}

func (c *Client) do(ctx context.Context, method, url string, acceptCodes ...int) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("x-ms-version", apiVersion)
	req.Header.Set("Content-Length", "0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	for _, code := range acceptCodes {
		if resp.StatusCode == code {
			return nil
		}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("Azure Storage error (status %d): %s", resp.StatusCode, string(body))
}
