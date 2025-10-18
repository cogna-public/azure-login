// Package retry provides configurable retry logic for transient errors in CI environments.
//
// This package is designed to handle network-related transient failures that can occur
// in CI/CD environments, such as connection resets, timeouts, and temporary service unavailability.
// Configuration is done exclusively through environment variables to avoid breaking the CLI interface.
package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"time"
)

// Config holds retry configuration loaded from environment variables
type Config struct {
	// MaxAttempts is the maximum number of retry attempts (including the initial attempt)
	// Default: 3 (2 retries), configurable via AZURE_LOGIN_RETRY_MAX_ATTEMPTS
	MaxAttempts int

	// InitialDelay is the initial delay between retries
	// Default: 1s, configurable via AZURE_LOGIN_RETRY_INITIAL_DELAY (in seconds)
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	// Default: 30s, configurable via AZURE_LOGIN_RETRY_MAX_DELAY (in seconds)
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	// Default: 2.0, configurable via AZURE_LOGIN_RETRY_BACKOFF_MULTIPLIER
	BackoffMultiplier float64
}

// DefaultConfig returns the default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:       3, // 3 attempts (2 retries) by default for CI/CD resilience
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// LoadConfig loads retry configuration from environment variables
func LoadConfig() *Config {
	cfg := DefaultConfig()

	// Load MaxAttempts
	if maxAttemptsStr := os.Getenv("AZURE_LOGIN_RETRY_MAX_ATTEMPTS"); maxAttemptsStr != "" {
		if maxAttempts, err := strconv.Atoi(maxAttemptsStr); err == nil && maxAttempts > 0 && maxAttempts <= 10 {
			cfg.MaxAttempts = maxAttempts
		}
	}

	// Load InitialDelay
	if initialDelayStr := os.Getenv("AZURE_LOGIN_RETRY_INITIAL_DELAY"); initialDelayStr != "" {
		if initialDelay, err := strconv.Atoi(initialDelayStr); err == nil && initialDelay > 0 && initialDelay <= 60 {
			cfg.InitialDelay = time.Duration(initialDelay) * time.Second
		}
	}

	// Load MaxDelay
	if maxDelayStr := os.Getenv("AZURE_LOGIN_RETRY_MAX_DELAY"); maxDelayStr != "" {
		if maxDelay, err := strconv.Atoi(maxDelayStr); err == nil && maxDelay > 0 && maxDelay <= 300 {
			cfg.MaxDelay = time.Duration(maxDelay) * time.Second
		}
	}

	// Load BackoffMultiplier
	if backoffStr := os.Getenv("AZURE_LOGIN_RETRY_BACKOFF_MULTIPLIER"); backoffStr != "" {
		if backoff, err := strconv.ParseFloat(backoffStr, 64); err == nil && backoff >= 1.0 && backoff <= 5.0 {
			cfg.BackoffMultiplier = backoff
		}
	}

	return cfg
}

// IsRetryable determines if an error is retryable based on its type
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for URL errors first (they often wrap other errors)
	// This must come before the context.DeadlineExceeded check because
	// http.Client timeouts wrap context.DeadlineExceeded in a url.Error,
	// and we want to retry HTTP client timeouts (transient) but not
	// user-initiated context deadlines (intentional).
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// If this is a timeout error from an HTTP client, it's retryable
		// even though the underlying error may be context.DeadlineExceeded
		if urlErr.Timeout() {
			return true
		}
		// For non-timeout URL errors, recursively check the wrapped error
		return IsRetryable(urlErr.Err)
	}

	// Check for context errors (don't retry user cancellations or expired contexts)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for operation errors (connection refused, reset, etc.)
	// This must come before the generic net.Error check since OpError implements net.Error
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check for specific syscall errors that are retryable
		if opErr.Err != nil {
			// Check using errors.Is for each retryable syscall error
			if errors.Is(opErr.Err, syscall.ECONNRESET) ||
				errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
				errors.Is(opErr.Err, syscall.ENETUNREACH) ||
				errors.Is(opErr.Err, syscall.EHOSTUNREACH) ||
				errors.Is(opErr.Err, syscall.ECONNABORTED) ||
				errors.Is(opErr.Err, syscall.ETIMEDOUT) {
				return true
			}
		}
		// Also check Temporary() method if available
		if opErr.Temporary() {
			return true
		}
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		// Retry temporary DNS failures, but not "no such host" errors
		return dnsErr.Temporary()
	}

	// Check for generic network errors (should be last since many specific types implement this)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Retry on timeouts and temporary errors
		return netErr.Timeout() || netErr.Temporary()
	}

	// Don't retry by default
	return false
}

// Do executes the given operation with retries according to the configuration
func (c *Config) Do(ctx context.Context, operation func() error) error {
	var lastErr error
	delay := c.InitialDelay

	for attempt := 1; attempt <= c.MaxAttempts; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if the error is not retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't retry if this was the last attempt
		if attempt >= c.MaxAttempts {
			break
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			// Context was cancelled, return the context error
			return ctx.Err()
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * c.BackoffMultiplier)
			if delay > c.MaxDelay {
				delay = c.MaxDelay
			}
		}
	}

	// All retries exhausted
	if c.MaxAttempts > 1 {
		return fmt.Errorf("operation failed after %d attempts: %w", c.MaxAttempts, lastErr)
	}
	return lastErr
}
