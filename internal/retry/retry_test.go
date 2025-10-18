package retry

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts = 3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay = 1s, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay = 30s, got %v", cfg.MaxDelay)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier = 2.0, got %f", cfg.BackoffMultiplier)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "default values when no env vars set",
			envVars: map[string]string{},
			expected: &Config{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
			},
		},
		{
			name: "custom max attempts",
			envVars: map[string]string{
				"AZURE_LOGIN_RETRY_MAX_ATTEMPTS": "5",
			},
			expected: &Config{
				MaxAttempts:       5,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
			},
		},
		{
			name: "all custom values",
			envVars: map[string]string{
				"AZURE_LOGIN_RETRY_MAX_ATTEMPTS":       "3",
				"AZURE_LOGIN_RETRY_INITIAL_DELAY":      "2",
				"AZURE_LOGIN_RETRY_MAX_DELAY":          "60",
				"AZURE_LOGIN_RETRY_BACKOFF_MULTIPLIER": "1.5",
			},
			expected: &Config{
				MaxAttempts:       3,
				InitialDelay:      2 * time.Second,
				MaxDelay:          60 * time.Second,
				BackoffMultiplier: 1.5,
			},
		},
		{
			name: "invalid values should use defaults",
			envVars: map[string]string{
				"AZURE_LOGIN_RETRY_MAX_ATTEMPTS":       "invalid",
				"AZURE_LOGIN_RETRY_INITIAL_DELAY":      "-1",
				"AZURE_LOGIN_RETRY_MAX_DELAY":          "1000",
				"AZURE_LOGIN_RETRY_BACKOFF_MULTIPLIER": "10.0",
			},
			expected: &Config{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				// Clean up
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			cfg := LoadConfig()

			if cfg.MaxAttempts != tt.expected.MaxAttempts {
				t.Errorf("MaxAttempts: expected %d, got %d", tt.expected.MaxAttempts, cfg.MaxAttempts)
			}
			if cfg.InitialDelay != tt.expected.InitialDelay {
				t.Errorf("InitialDelay: expected %v, got %v", tt.expected.InitialDelay, cfg.InitialDelay)
			}
			if cfg.MaxDelay != tt.expected.MaxDelay {
				t.Errorf("MaxDelay: expected %v, got %v", tt.expected.MaxDelay, cfg.MaxDelay)
			}
			if cfg.BackoffMultiplier != tt.expected.BackoffMultiplier {
				t.Errorf("BackoffMultiplier: expected %f, got %f", tt.expected.BackoffMultiplier, cfg.BackoffMultiplier)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			retryable: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			retryable: false,
		},
		{
			name:      "connection reset by peer",
			err:       &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET},
			retryable: true,
		},
		{
			name:      "connection refused",
			err:       &net.OpError{Err: syscall.ECONNREFUSED},
			retryable: true,
		},
		{
			name:      "network unreachable",
			err:       &net.OpError{Err: syscall.ENETUNREACH},
			retryable: true,
		},
		{
			name:      "host unreachable",
			err:       &net.OpError{Err: syscall.EHOSTUNREACH},
			retryable: true,
		},
		{
			name:      "connection aborted",
			err:       &net.OpError{Err: syscall.ECONNABORTED},
			retryable: true,
		},
		{
			name: "url error wrapping retryable error",
			err: &url.Error{
				Err: &net.OpError{Err: syscall.ECONNRESET},
			},
			retryable: true,
		},
		{
			name: "url error wrapping non-retryable error",
			err: &url.Error{
				Err: errors.New("permanent error"),
			},
			retryable: false,
		},
		{
			name:      "generic error",
			err:       errors.New("some error"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}

func TestDoWithNoRetries(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       1,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	err := cfg.Do(context.Background(), func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoWithRetries(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	err := cfg.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return &net.OpError{Err: syscall.ECONNRESET}
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithNonRetryableError(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	permanentErr := errors.New("permanent error")
	err := cfg.Do(context.Background(), func() error {
		attempts++
		return permanentErr
	})

	if err != permanentErr {
		t.Errorf("expected permanent error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retries), got %d", attempts)
	}
}

func TestDoWithContextCancellation(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       5,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	// Cancel context after first retry
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := cfg.Do(ctx, func() error {
		attempts++
		return &net.OpError{Err: syscall.ECONNRESET}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if attempts > 2 {
		t.Errorf("expected at most 2 attempts, got %d", attempts)
	}
}

func TestDoExhaustsRetries(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	retryableErr := &net.OpError{Err: syscall.ECONNRESET}
	err := cfg.Do(context.Background(), func() error {
		attempts++
		return retryableErr
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	// Error message should mention the number of attempts
	if !errors.Is(err, retryableErr) {
		t.Errorf("expected wrapped retryable error, got %v", err)
	}
}

func TestExponentialBackoff(t *testing.T) {
	cfg := &Config{
		MaxAttempts:       4,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	start := time.Now()
	attempts := 0
	err := cfg.Do(context.Background(), func() error {
		attempts++
		if attempts < 4 {
			return &net.OpError{Err: syscall.ECONNRESET}
		}
		return nil
	})

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}

	// Expected delays: 10ms, 20ms, 40ms (total ~70ms)
	// Allow for some timing variance
	minExpected := 60 * time.Millisecond
	maxExpected := 150 * time.Millisecond
	if elapsed < minExpected || elapsed > maxExpected {
		t.Errorf("expected elapsed time between %v and %v, got %v", minExpected, maxExpected, elapsed)
	}
}
