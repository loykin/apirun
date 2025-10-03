package retry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/loykin/apirun/internal/common"
)

// Config holds configuration for database operation retries
type Config struct {
	MaxRetries      int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Multiplier for exponential backoff
	RetryableErrors []string      // Error strings that trigger retries
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *Config {
	return &Config{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection refused",
			"connection reset",
			"timeout",
			"temporary failure",
			"deadlock",
			"lock wait timeout",
			"database is locked",
			"connection lost",
			"broken pipe",
		},
	}
}

// isRetryableError checks if an error should trigger a retry
func (rc *Config) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - don't retry these
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, retryableErr := range rc.RetryableErrors {
		if strings.Contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay for a given retry attempt using exponential backoff
func (rc *Config) calculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialDelay
	}

	delay := time.Duration(float64(rc.InitialDelay) * math.Pow(rc.BackoffFactor, float64(attempt-1)))
	if delay > rc.MaxDelay {
		delay = rc.MaxDelay
	}
	return delay
}

// RetryableOperation represents a database operation that can be retried
type RetryableOperation func() error

// WithRetry executes a database operation with retry logic
func WithRetry(ctx context.Context, config *Config, operation RetryableOperation) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	logger := common.GetLogger().WithComponent("store-retry")

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			// Success!
			if attempt > 0 {
				logger.Info("database operation succeeded after retry",
					"attempt", attempt+1,
					"total_attempts", config.MaxRetries+1)
			}
			return nil
		}

		lastErr = err

		// Don't retry on the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Check if this error is retryable
		if !config.isRetryableError(err) {
			logger.Debug("database operation failed with non-retryable error",
				"error", err,
				"attempt", attempt+1)
			return err
		}

		// Calculate delay and wait
		delay := config.calculateDelay(attempt)
		logger.Warn("database operation failed, retrying",
			"error", err,
			"attempt", attempt+1,
			"max_attempts", config.MaxRetries+1,
			"retry_delay", delay)

		// Wait before retry, but respect context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	logger.Error("database operation failed after all retry attempts",
		"error", lastErr,
		"attempts", config.MaxRetries+1)

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// RetryableQuery represents a database query that can be retried
type RetryableQuery func() (*sql.Rows, error)

// WithRetryQuery executes a database query with retry logic
func WithRetryQuery(ctx context.Context, config *Config, query RetryableQuery) (*sql.Rows, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var rows *sql.Rows
	err := WithRetry(ctx, config, func() error {
		var err error
		rows, err = query()
		return err
	})

	return rows, err
}

// RetryableQueryRow represents a database query row operation that can be retried
type RetryableQueryRow func() *sql.Row

// RetryableExec represents a database exec operation that can be retried
type RetryableExec func() (sql.Result, error)

// WithRetryExec executes a database exec with retry logic
func WithRetryExec(ctx context.Context, config *Config, exec RetryableExec) (sql.Result, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var result sql.Result
	err := WithRetry(ctx, config, func() error {
		var err error
		result, err = exec()
		return err
	})

	return result, err
}
