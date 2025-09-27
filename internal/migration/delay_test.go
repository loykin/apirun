package migration

import (
	"testing"
	"time"
)

func TestMigrator_getDelayBetweenMigrations(t *testing.T) {
	tests := []struct {
		name          string
		delayConfig   time.Duration
		expectedDelay time.Duration
	}{
		{
			name:          "default delay when not configured",
			delayConfig:   0,
			expectedDelay: 1 * time.Second,
		},
		{
			name:          "custom delay 500ms",
			delayConfig:   500 * time.Millisecond,
			expectedDelay: 500 * time.Millisecond,
		},
		{
			name:          "custom delay 2 seconds",
			delayConfig:   2 * time.Second,
			expectedDelay: 2 * time.Second,
		},
		{
			name:          "very short delay 10ms",
			delayConfig:   10 * time.Millisecond,
			expectedDelay: 10 * time.Millisecond,
		},
		{
			name:          "long delay 30 seconds",
			delayConfig:   30 * time.Second,
			expectedDelay: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Migrator{
				DelayBetweenMigrations: tt.delayConfig,
			}

			actualDelay := m.getDelayBetweenMigrations()
			if actualDelay != tt.expectedDelay {
				t.Errorf("expected delay %v, got %v", tt.expectedDelay, actualDelay)
			}
		})
	}
}

func TestMigrator_getDelayBetweenMigrations_ZeroHandling(t *testing.T) {
	// Test that zero value correctly defaults to 1 second
	m := &Migrator{} // DelayBetweenMigrations will be zero value

	delay := m.getDelayBetweenMigrations()
	expected := 1 * time.Second

	if delay != expected {
		t.Errorf("zero delay should default to %v, got %v", expected, delay)
	}
}

func BenchmarkMigrator_getDelayBetweenMigrations(b *testing.B) {
	m := &Migrator{
		DelayBetweenMigrations: 500 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.getDelayBetweenMigrations()
	}
}
