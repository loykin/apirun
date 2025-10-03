package postgresql

import (
	"reflect"
	"testing"
	"time"
)

func TestNewDialect(t *testing.T) {
	dialect := NewDialect()
	if dialect == nil {
		t.Fatal("NewDialect() returned nil")
	}
}

func TestDialect_GetPlaceholder(t *testing.T) {
	dialect := NewDialect()

	tests := []struct {
		name  string
		index int
		want  string
	}{
		{
			name:  "first placeholder",
			index: 1,
			want:  "$1",
		},
		{
			name:  "tenth placeholder",
			index: 10,
			want:  "$10",
		},
		{
			name:  "hundredth placeholder",
			index: 100,
			want:  "$100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialect.GetPlaceholder(tt.index)
			if got != tt.want {
				t.Errorf("GetPlaceholder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDialect_GetUpsertClause(t *testing.T) {
	dialect := NewDialect()
	got := dialect.GetUpsertClause()
	want := ""
	if got != want {
		t.Errorf("GetUpsertClause() = %v, want %v", got, want)
	}
}

func TestDialect_ConvertBoolToStorage(t *testing.T) {
	dialect := NewDialect()

	tests := []struct {
		name  string
		input bool
		want  interface{}
	}{
		{
			name:  "true value",
			input: true,
			want:  true,
		},
		{
			name:  "false value",
			input: false,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialect.ConvertBoolToStorage(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertBoolToStorage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDialect_ConvertTimeToStorage(t *testing.T) {
	dialect := NewDialect()

	testTime := time.Date(2023, 12, 25, 10, 30, 45, 123456789, time.UTC)
	got := dialect.ConvertTimeToStorage(testTime)
	if !reflect.DeepEqual(got, testTime) {
		t.Errorf("ConvertTimeToStorage() = %v, want %v", got, testTime)
	}
}

func TestDialect_ConvertBoolFromStorage(t *testing.T) {
	dialect := NewDialect()

	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{
			name:  "true bool",
			input: true,
			want:  true,
		},
		{
			name:  "false bool",
			input: false,
			want:  false,
		},
		{
			name:  "string value",
			input: "true",
			want:  false,
		},
		{
			name:  "int value",
			input: 1,
			want:  false,
		},
		{
			name:  "nil value",
			input: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialect.ConvertBoolFromStorage(tt.input)
			if got != tt.want {
				t.Errorf("ConvertBoolFromStorage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDialect_ConvertTimeFromStorage(t *testing.T) {
	dialect := NewDialect()

	testTime := time.Date(2023, 12, 25, 10, 30, 45, 123456789, time.UTC)
	expectedString := testTime.Format(time.RFC3339Nano)

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "time pointer",
			input: &testTime,
			want:  expectedString,
		},
		{
			name:  "time value",
			input: testTime,
			want:  expectedString,
		},
		{
			name:  "nil pointer",
			input: (*time.Time)(nil),
			want:  "",
		},
		{
			name:  "string value",
			input: "not-a-time",
			want:  "",
		},
		{
			name:  "int value",
			input: 123,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialect.ConvertTimeFromStorage(tt.input)
			if got != tt.want {
				t.Errorf("ConvertTimeFromStorage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDialect_Connect(t *testing.T) {
	dialect := NewDialect()

	// Test with invalid DSN - should fail
	_, err := dialect.Connect("invalid-dsn")
	if err == nil {
		t.Error("Connect() should fail with invalid DSN")
	}

	// We can't test successful connection without a real PostgreSQL instance
	// but we can test that the method exists and handles invalid DSNs properly
}

func TestDialect_GetEnsureStatements(t *testing.T) {
	dialect := NewDialect()

	schemaMigrations := "schema_migrations"
	migrationRuns := "migration_runs"
	storedEnv := "stored_env"

	statements := dialect.GetEnsureStatements(schemaMigrations, migrationRuns, storedEnv)

	if len(statements) != 3 {
		t.Errorf("GetEnsureStatements() returned %d statements, want 3", len(statements))
	}

	expectedStatements := []string{
		"CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)",
		"CREATE TABLE IF NOT EXISTS migration_runs (id SERIAL PRIMARY KEY, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed BOOLEAN NOT NULL DEFAULT FALSE, ran_at TIMESTAMPTZ NOT NULL)",
		"CREATE TABLE IF NOT EXISTS stored_env (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))",
	}

	for i, expected := range expectedStatements {
		if statements[i] != expected {
			t.Errorf("GetEnsureStatements()[%d] = %v, want %v", i, statements[i], expected)
		}
	}
}

func TestDialect_GetDriverName(t *testing.T) {
	dialect := NewDialect()
	got := dialect.GetDriverName()
	want := "postgresql"
	if got != want {
		t.Errorf("GetDriverName() = %v, want %v", got, want)
	}
}
