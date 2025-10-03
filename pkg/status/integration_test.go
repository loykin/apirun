package status

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/internal/migration"
	"github.com/loykin/apirun/pkg/env"
)

// TestIntegrationFullMigrationCycle tests a complete migration lifecycle
func TestIntegrationFullMigrationCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temporary directory with migration files
	tempDir := t.TempDir()

	// Create test migration files
	createTestMigrationFile(t, tempDir, "001_initial.yaml", `
up:
  - url: https://httpbin.org/status/200
    method: GET
down:
  - url: https://httpbin.org/status/200
    method: GET
`)

	createTestMigrationFile(t, tempDir, "002_second.yaml", `
up:
  - url: https://httpbin.org/status/201
    method: GET
    store:
      result_id: "{{.Response.Headers.Get \"X-Request-Id\"}}"
down:
  - url: https://httpbin.org/status/200
    method: GET
`)

	// Set up store
	cfg := &apirun.StoreConfig{}
	cfg.Config.Driver = apirun.DriverSqlite
	cfg.Config.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(tempDir, apirun.StoreDBFileName)}

	store, err := apirun.OpenStoreFromOptions(tempDir, cfg)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Test initial status (no migrations applied)
	initialInfo, err := FromStore(store)
	if err != nil {
		t.Fatalf("Failed to get initial status: %v", err)
	}

	if initialInfo.Version != 0 {
		t.Errorf("Expected initial version 0, got %d", initialInfo.Version)
	}

	if len(initialInfo.Applied) != 0 {
		t.Errorf("Expected no applied migrations, got %v", initialInfo.Applied)
	}

	if len(initialInfo.History) != 0 {
		t.Errorf("Expected no history, got %d entries", len(initialInfo.History))
	}

	// Run up migrations
	migrator := &migration.Migrator{
		Dir:   tempDir,
		Store: (*store), // Dereference to get the store.Store interface
		Env:   env.New(),
	}

	ctx := context.Background()
	results, err := migrator.MigrateUp(ctx, 0) // Migrate all
	if err != nil {
		t.Fatalf("Migration up failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 migration results, got %d", len(results))
	}

	// Test status after up migrations
	afterUpInfo, err := FromStore(store)
	if err != nil {
		t.Fatalf("Failed to get status after up: %v", err)
	}

	if afterUpInfo.Version != 2 {
		t.Errorf("Expected version 2 after up, got %d", afterUpInfo.Version)
	}

	expectedApplied := []int{1, 2}
	if len(afterUpInfo.Applied) != len(expectedApplied) {
		t.Errorf("Expected applied %v, got %v", expectedApplied, afterUpInfo.Applied)
	}

	if len(afterUpInfo.History) != 2 {
		t.Errorf("Expected 2 history entries after up, got %d", len(afterUpInfo.History))
	}

	// Verify history entries are correct
	for i, entry := range afterUpInfo.History {
		if entry.Version != i+1 {
			t.Errorf("History entry %d: expected version %d, got %d", i, i+1, entry.Version)
		}
		if entry.Direction != "up" {
			t.Errorf("History entry %d: expected direction 'up', got %s", i, entry.Direction)
		}
		if entry.Failed {
			t.Errorf("History entry %d: expected successful migration, got failed", i)
		}
		if entry.RanAt == "" {
			t.Errorf("History entry %d: missing RanAt timestamp", i)
		}
	}

	// Test colorized formatting
	colorized := afterUpInfo.FormatColorized(true, true)
	if colorized == "" {
		t.Error("Colorized format should not be empty")
	}

	// Test human-readable formatting with history
	human := afterUpInfo.FormatHuman(true)
	if human == "" {
		t.Error("Human format with history should not be empty")
	}

	// Test history limit functionality
	limited := afterUpInfo.FormatHumanWithLimit(true, 1, false)
	if limited == "" {
		t.Error("Limited history format should not be empty")
	}

	// Run down migration
	downResults, err := migrator.MigrateDown(ctx, 1) // Down to version 1
	if err != nil {
		t.Fatalf("Migration down failed: %v", err)
	}

	if len(downResults) != 1 {
		t.Errorf("Expected 1 down migration result, got %d", len(downResults))
	}

	// Test status after down migration
	afterDownInfo, err := FromStore(store)
	if err != nil {
		t.Fatalf("Failed to get status after down: %v", err)
	}

	if afterDownInfo.Version != 1 {
		t.Errorf("Expected version 1 after down, got %d", afterDownInfo.Version)
	}

	expectedAppliedAfterDown := []int{1}
	if len(afterDownInfo.Applied) != len(expectedAppliedAfterDown) {
		t.Errorf("Expected applied %v after down, got %v", expectedAppliedAfterDown, afterDownInfo.Applied)
	}

	// Should have 3 history entries now (2 up + 1 down)
	if len(afterDownInfo.History) != 3 {
		t.Errorf("Expected 3 history entries after down, got %d", len(afterDownInfo.History))
	}

	// Verify the down migration entry
	downEntry := afterDownInfo.History[2] // Last entry should be the down migration
	if downEntry.Version != 2 {
		t.Errorf("Down entry: expected version 2, got %d", downEntry.Version)
	}
	if downEntry.Direction != "down" {
		t.Errorf("Down entry: expected direction 'down', got %s", downEntry.Direction)
	}
}

// TestIntegrationErrorHandling tests status reporting with failed migrations
func TestIntegrationErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	// Create a migration that will fail
	createTestMigrationFile(t, tempDir, "001_failing.yaml", `
up:
  - url: https://httpbin.org/status/500
    method: GET
    expected: 200  # This will fail
down:
  - url: https://httpbin.org/status/200
    method: GET
`)

	cfg := &apirun.StoreConfig{}
	cfg.Config.Driver = apirun.DriverSqlite
	cfg.Config.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(tempDir, apirun.StoreDBFileName)}

	store, err := apirun.OpenStoreFromOptions(tempDir, cfg)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	migrator := &migration.Migrator{
		Dir:   tempDir,
		Store: (*store), // Dereference to get the store.Store interface
		Env:   env.New(),
	}

	ctx := context.Background()
	_, err = migrator.MigrateUp(ctx, 0)
	if err == nil {
		t.Fatal("Expected migration to fail, but it succeeded")
	}

	// Check status after failed migration
	info, err := FromStore(store)
	if err != nil {
		t.Fatalf("Failed to get status after failed migration: %v", err)
	}

	// Version should still be 0 since migration failed
	if info.Version != 0 {
		t.Errorf("Expected version 0 after failed migration, got %d", info.Version)
	}

	// Should have history entry marked as failed
	if len(info.History) == 0 {
		t.Fatal("Expected history entry for failed migration")
	}

	failedEntry := info.History[0]
	if !failedEntry.Failed {
		t.Error("Expected history entry to be marked as failed")
	}
	if failedEntry.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code 500, got %d", failedEntry.StatusCode)
	}
}

// createTestMigrationFile creates a migration file for testing
func createTestMigrationFile(t *testing.T, dir, filename, content string) {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test migration file %s: %v", filename, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("Failed to create test migration file %s: %v", filename, err)
	}
}
