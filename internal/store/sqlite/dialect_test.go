package sqlite

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

	// SQLite uses single placeholder character
	got := dialect.GetPlaceholder()
	want := "?"
	if got != want {
		t.Errorf("GetPlaceholder() = %v, want %v", got, want)
	}
}

func TestDialect_GetUpsertClause(t *testing.T) {
	dialect := NewDialect()
	got := dialect.GetUpsertClause()
	want := "OR IGNORE"
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
			want:  1,
		},
		{
			name:  "false value",
			input: false,
			want:  0,
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
	expectedString := testTime.Format(time.RFC3339Nano)

	got := dialect.ConvertTimeToStorage(testTime)
	if !reflect.DeepEqual(got, expectedString) {
		t.Errorf("ConvertTimeToStorage() = %v, want %v", got, expectedString)
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
			name:  "int64 non-zero",
			input: int64(1),
			want:  true,
		},
		{
			name:  "int64 zero",
			input: int64(0),
			want:  false,
		},
		{
			name:  "int64 negative",
			input: int64(-1),
			want:  true,
		},
		{
			name:  "int non-zero",
			input: 1,
			want:  true,
		},
		{
			name:  "int zero",
			input: 0,
			want:  false,
		},
		{
			name:  "int negative",
			input: -1,
			want:  true,
		},
		{
			name:  "string value",
			input: "true",
			want:  false,
		},
		{
			name:  "bool value",
			input: true,
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

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "valid string",
			input: "2023-12-25T10:30:45.123456789Z",
			want:  "2023-12-25T10:30:45.123456789Z",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "int value",
			input: 123,
			want:  "",
		},
		{
			name:  "nil value",
			input: nil,
			want:  "",
		},
		{
			name:  "bool value",
			input: true,
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

	// We can't test successful connection without a real SQLite instance
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
		"CREATE TABLE IF NOT EXISTS migration_runs (id INTEGER PRIMARY KEY AUTOINCREMENT, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed INTEGER NOT NULL DEFAULT 0, ran_at TEXT NOT NULL)",
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
	want := "sqlite"
	if got != want {
		t.Errorf("GetDriverName() = %v, want %v", got, want)
	}
}
