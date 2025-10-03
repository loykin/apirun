package retry

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay to be 100ms, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay to be 5s, got %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor to be 2.0, got %f", config.BackoffFactor)
	}

	expectedErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"deadlock",
		"lock wait timeout",
		"database is locked",
		"connection lost",
		"broken pipe",
	}

	if len(config.RetryableErrors) != len(expectedErrors) {
		t.Errorf("Expected %d retryable errors, got %d", len(expectedErrors), len(config.RetryableErrors))
	}
}

func TestConfig_isRetryableError(t *testing.T) {
	config := DefaultRetryConfig()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset error",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "database is locked error",
			err:      errors.New("database is locked"),
			expected: true,
		},
		{
			name:     "deadlock error",
			err:      errors.New("deadlock detected"),
			expected: true,
		},
		{
			name:     "context canceled error",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded error",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("syntax error"),
			expected: false,
		},
		{
			name:     "case insensitive matching",
			err:      errors.New("CONNECTION REFUSED"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestConfig_calculateDelay(t *testing.T) {
	config := DefaultRetryConfig()

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "attempt 0",
			attempt:  0,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "attempt 1",
			attempt:  1,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "attempt 2",
			attempt:  2,
			expected: 200 * time.Millisecond,
		},
		{
			name:     "attempt 3",
			attempt:  3,
			expected: 400 * time.Millisecond,
		},
		{
			name:     "attempt 4",
			attempt:  4,
			expected: 800 * time.Millisecond,
		},
		{
			name:     "attempt 5",
			attempt:  5,
			expected: 1600 * time.Millisecond,
		},
		{
			name:     "attempt 6",
			attempt:  6,
			expected: 3200 * time.Millisecond,
		},
		{
			name:     "attempt 7 (capped at max)",
			attempt:  7,
			expected: 5 * time.Second,
		},
		{
			name:     "negative attempt",
			attempt:  -1,
			expected: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.calculateDelay(tt.attempt)
			if result != tt.expected {
				t.Errorf("calculateDelay(%d) = %v, expected %v", tt.attempt, result, tt.expected)
			}
		})
	}
}

func TestWithRetry_Success(t *testing.T) {
	config := &Config{
		MaxRetries:    2,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return nil
	}

	err := WithRetry(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}
}

func TestWithRetry_RetryableError(t *testing.T) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("connection refused")
		}
		return nil
	}

	err := WithRetry(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return errors.New("syntax error")
	}

	err := WithRetry(ctx, config, operation)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}

	if err != nil && err.Error() != "syntax error" {
		t.Errorf("Expected original error, got %v", err)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return errors.New("connection refused")
	}

	err := WithRetry(ctx, config, operation)

	if err == nil {
		t.Error("Expected error after max retries, got nil")
	}

	if callCount != 3 { // 1 initial + 2 retries
		t.Errorf("Expected operation to be called 3 times, got %d", callCount)
	}

	if !errors.Is(err, errors.New("connection refused")) {
		expectedMsg := "operation failed after 3 attempts: connection refused"
		if err != nil && err.Error() != expectedMsg {
			t.Errorf("Expected wrapped error message '%s', got '%s'", expectedMsg, err.Error())
		}
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	config := &Config{
		MaxRetries:      5,
		InitialDelay:    50 * time.Millisecond,  // Reduced delay
		MaxDelay:        200 * time.Millisecond, // Reduced max delay
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond) // Use timeout instead
	defer cancel()

	callCount := 0

	operation := func() error {
		callCount++
		return errors.New("connection refused")
	}

	err := WithRetry(ctx, config, operation)

	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}

	// Check for either canceled or deadline exceeded
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// Check the error message contains context cancellation info
		if err != nil && !strings.Contains(err.Error(), "operation cancelled during retry") {
			t.Errorf("Expected context cancellation error, got '%s'", err.Error())
		}
	}
}

func TestWithRetry_NilConfig(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return nil
	}

	err := WithRetry(ctx, nil, operation)

	if err != nil {
		t.Errorf("Expected no error with nil config, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}
}

// Mock sql.Result for testing
type mockResult struct{}

func (m mockResult) LastInsertId() (int64, error) { return 1, nil }
func (m mockResult) RowsAffected() (int64, error) { return 1, nil }

func TestWithRetryExec_Success(t *testing.T) {
	config := &Config{
		MaxRetries:    2,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	ctx := context.Background()
	callCount := 0

	exec := func() (sql.Result, error) {
		callCount++
		return mockResult{}, nil
	}

	result, err := WithRetryExec(ctx, config, exec)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	if callCount != 1 {
		t.Errorf("Expected exec to be called once, got %d", callCount)
	}
}

func TestWithRetryExec_WithRetries(t *testing.T) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx := context.Background()
	callCount := 0

	exec := func() (sql.Result, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("connection refused")
		}
		return mockResult{}, nil
	}

	result, err := WithRetryExec(ctx, config, exec)

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	if callCount != 3 {
		t.Errorf("Expected exec to be called 3 times, got %d", callCount)
	}
}

func TestWithRetryQuery_Success(t *testing.T) {
	config := &Config{
		MaxRetries:    2,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	ctx := context.Background()
	callCount := 0

	query := func() (*sql.Rows, error) {
		callCount++
		// Return nil, nil to test successful case
		return nil, nil
	}

	rows, err := WithRetryQuery(ctx, config, query)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// rows will be nil in our test case, which is fine
	_ = rows

	if callCount != 1 {
		t.Errorf("Expected query to be called once, got %d", callCount)
	}
}

func TestWithRetryQuery_WithRetries(t *testing.T) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}

	ctx := context.Background()
	callCount := 0

	query := func() (*sql.Rows, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("connection refused")
		}
		// Return success on third attempt
		return nil, nil
	}

	rows, err := WithRetryQuery(ctx, config, query)

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	// rows will be nil in our test case, which is fine
	_ = rows

	if callCount != 3 {
		t.Errorf("Expected query to be called 3 times, got %d", callCount)
	}
}

// Benchmark tests
func BenchmarkWithRetry_NoRetries(b *testing.B) {
	config := DefaultRetryConfig()
	ctx := context.Background()

	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithRetry(ctx, config, operation)
	}
}

func BenchmarkWithRetry_WithRetries(b *testing.B) {
	config := &Config{
		MaxRetries:      2,
		InitialDelay:    1 * time.Nanosecond, // Very fast for benchmarking
		MaxDelay:        10 * time.Nanosecond,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"connection refused"},
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 3 {
				return errors.New("connection refused")
			}
			return nil
		}
		_ = WithRetry(ctx, config, operation)
	}
}
