package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
	if store.dialect == nil {
		t.Error("NewStore() should initialize dialect")
	}
}

func TestStore_Load(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{
			name:   "valid dsn",
			config: map[string]interface{}{"dsn": "file:test.db?_busy_timeout=5000"},
			want:   "file:test.db?_busy_timeout=5000",
		},
		{
			name:   "valid path",
			config: map[string]interface{}{"path": "/tmp/test.db"},
			want:   "file:/tmp/test.db?_busy_timeout=5000&_fk=1",
		},
		{
			name:   "empty dsn",
			config: map[string]interface{}{"dsn": ""},
			want:   "",
		},
		{
			name:   "no dsn or path key",
			config: map[string]interface{}{"other": "value"},
			want:   "",
		},
		{
			name:   "dsn wrong type",
			config: map[string]interface{}{"dsn": 123},
			want:   "",
		},
		{
			name:   "path wrong type",
			config: map[string]interface{}{"path": 123},
			want:   "",
		},
		{
			name:   "dsn takes precedence over path",
			config: map[string]interface{}{"dsn": "custom-dsn", "path": "/tmp/test.db"},
			want:   "custom-dsn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore()
			err := store.Load(tt.config)
			if err != nil {
				t.Errorf("Load() error = %v, want nil", err)
			}
			if store.DSN != tt.want {
				t.Errorf("Load() DSN = %v, want %v", store.DSN, tt.want)
			}
		})
	}
}

func TestStore_Validate(t *testing.T) {
	store := NewStore()
	err := store.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestStore_Connect(t *testing.T) {
	store := NewStore()
	store.DSN = "invalid-dsn"

	// Test connection failure - this will fail but we're testing error handling
	_, err := store.Connect()
	if err == nil {
		t.Error("Connect() should fail with invalid DSN")
	}
}

func TestStore_Connect_DefaultMemory(t *testing.T) {
	store := NewStore()
	// Don't set DSN - should default to :memory:

	// We can't test real connection without SQLite driver, but we can test DSN setting
	if store.DSN != "" {
		store.DSN = "" // Reset for test
	}

	_, err := store.Connect()
	// This will fail due to missing driver, but DSN should be set to :memory:
	if store.DSN != ":memory:" {
		t.Errorf("Connect() should set DSN to :memory: when empty, got %v", store.DSN)
	}
	if err == nil {
		t.Error("Connect() should fail without SQLite driver")
	}
}

func TestStore_Close(t *testing.T) {
	tests := []struct {
		name string
		db   *sql.DB
		want error
	}{
		{
			name: "nil db",
			db:   nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &Store{db: tt.db}
			err := store.Close()
			if err != tt.want {
				t.Errorf("Close() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestStore_Ensure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{
		SchemaMigrations: "schema_migrations",
		MigrationRuns:    "migration_runs",
		StoredEnv:        "stored_env",
	}

	// Mock successful table creation
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS migration_runs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS stored_env").WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.Ensure(th)
	if err != nil {
		t.Errorf("Ensure() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Apply(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name    string
		version int
		setup   func()
		wantErr bool
	}{
		{
			name:    "successful apply",
			version: 1,
			setup: func() {
				mock.ExpectExec("INSERT OR IGNORE INTO schema_migrations\\(version\\) VALUES\\(\\?\\)").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:    "database error",
			version: 2,
			setup: func() {
				mock.ExpectExec("INSERT OR IGNORE INTO schema_migrations\\(version\\) VALUES\\(\\?\\)").
					WithArgs(2).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.Apply(th, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_IsApplied(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name    string
		version int
		setup   func()
		want    bool
		wantErr bool
	}{
		{
			name:    "version is applied",
			version: 1,
			setup: func() {
				mock.ExpectQuery("SELECT 1 FROM schema_migrations WHERE version = \\?").
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			},
			want:    true,
			wantErr: false,
		},
		{
			name:    "version not applied",
			version: 2,
			setup: func() {
				mock.ExpectQuery("SELECT 1 FROM schema_migrations WHERE version = \\?").
					WithArgs(2).
					WillReturnError(sql.ErrNoRows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:    "database error",
			version: 3,
			setup: func() {
				mock.ExpectQuery("SELECT 1 FROM schema_migrations WHERE version = \\?").
					WithArgs(3).
					WillReturnError(errors.New("database error"))
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.IsApplied(th, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsApplied() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsApplied() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_CurrentVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name    string
		setup   func()
		want    int
		wantErr bool
	}{
		{
			name: "has current version",
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))
			},
			want:    5,
			wantErr: false,
		},
		{
			name: "no versions applied",
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "database error",
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnError(errors.New("database error"))
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.CurrentVersion(th)
			if (err != nil) != tt.wantErr {
				t.Errorf("CurrentVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CurrentVersion() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListApplied(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name    string
		setup   func()
		want    []int
		wantErr bool
	}{
		{
			name: "multiple versions",
			setup: func() {
				mock.ExpectQuery("SELECT version FROM schema_migrations ORDER BY version").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1).AddRow(3).AddRow(5))
			},
			want:    []int{1, 3, 5},
			wantErr: false,
		},
		{
			name: "no versions",
			setup: func() {
				mock.ExpectQuery("SELECT version FROM schema_migrations ORDER BY version").
					WillReturnRows(sqlmock.NewRows([]string{"version"}))
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "database error",
			setup: func() {
				mock.ExpectQuery("SELECT version FROM schema_migrations ORDER BY version").
					WillReturnError(errors.New("database error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.ListApplied(th)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListApplied() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListApplied() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Remove(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name    string
		version int
		setup   func()
		wantErr bool
	}{
		{
			name:    "successful remove",
			version: 1,
			setup: func() {
				mock.ExpectExec("DELETE FROM schema_migrations WHERE version = \\?").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:    "database error",
			version: 2,
			setup: func() {
				mock.ExpectExec("DELETE FROM schema_migrations WHERE version = \\?").
					WithArgs(2).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.Remove(th, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Remove() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_SetVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{SchemaMigrations: "schema_migrations"}

	tests := []struct {
		name   string
		target int
		setup  func()
		want   bool
	}{
		{
			name:   "target higher than current",
			target: 10,
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))
			},
			want: true, // should error
		},
		{
			name:   "target same as current",
			target: 5,
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))
			},
			want: false, // should not error
		},
		{
			name:   "target lower than current",
			target: 3,
			setup: func() {
				mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations").
					WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))
				mock.ExpectExec("DELETE FROM schema_migrations WHERE version > \\?").
					WithArgs(3).
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
			want: false, // should not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.SetVersion(th, tt.target)
			if (err != nil) != tt.want {
				t.Errorf("SetVersion() error = %v, wantErr %v", err, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_LoadEnv(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{MigrationRuns: "migration_runs"}

	tests := []struct {
		name      string
		version   int
		direction string
		setup     func()
		want      map[string]string
		wantErr   bool
	}{
		{
			name:      "valid env data",
			version:   1,
			direction: "up",
			setup: func() {
				mock.ExpectQuery("SELECT env_json FROM migration_runs WHERE version = \\? AND direction = \\? ORDER BY id DESC LIMIT 1").
					WithArgs(1, "up").
					WillReturnRows(sqlmock.NewRows([]string{"env_json"}).AddRow(`{"key1":"value1","key2":"value2"}`))
			},
			want:    map[string]string{"key1": "value1", "key2": "value2"},
			wantErr: false,
		},
		{
			name:      "no data found",
			version:   2,
			direction: "down",
			setup: func() {
				mock.ExpectQuery("SELECT env_json FROM migration_runs WHERE version = \\? AND direction = \\? ORDER BY id DESC LIMIT 1").
					WithArgs(2, "down").
					WillReturnError(sql.ErrNoRows)
			},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:      "null env_json",
			version:   3,
			direction: "up",
			setup: func() {
				mock.ExpectQuery("SELECT env_json FROM migration_runs WHERE version = \\? AND direction = \\? ORDER BY id DESC LIMIT 1").
					WithArgs(3, "up").
					WillReturnRows(sqlmock.NewRows([]string{"env_json"}).AddRow(nil))
			},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:      "empty env_json",
			version:   4,
			direction: "up",
			setup: func() {
				mock.ExpectQuery("SELECT env_json FROM migration_runs WHERE version = \\? AND direction = \\? ORDER BY id DESC LIMIT 1").
					WithArgs(4, "up").
					WillReturnRows(sqlmock.NewRows([]string{"env_json"}).AddRow(""))
			},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:      "invalid json",
			version:   5,
			direction: "up",
			setup: func() {
				mock.ExpectQuery("SELECT env_json FROM migration_runs WHERE version = \\? AND direction = \\? ORDER BY id DESC LIMIT 1").
					WithArgs(5, "up").
					WillReturnRows(sqlmock.NewRows([]string{"env_json"}).AddRow("invalid-json"))
			},
			want:    map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.LoadEnv(th, tt.version, tt.direction)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadEnv() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_LoadStoredEnv(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{StoredEnv: "stored_env"}

	tests := []struct {
		name    string
		version int
		setup   func()
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "valid stored env",
			version: 1,
			setup: func() {
				mock.ExpectQuery("SELECT name, value FROM stored_env WHERE version = \\?").
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"name", "value"}).
						AddRow("KEY1", "value1").
						AddRow("KEY2", "value2"))
			},
			want:    map[string]string{"KEY1": "value1", "KEY2": "value2"},
			wantErr: false,
		},
		{
			name:    "no stored env",
			version: 2,
			setup: func() {
				mock.ExpectQuery("SELECT name, value FROM stored_env WHERE version = \\?").
					WithArgs(2).
					WillReturnRows(sqlmock.NewRows([]string{"name", "value"}))
			},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "database error",
			version: 3,
			setup: func() {
				mock.ExpectQuery("SELECT name, value FROM stored_env WHERE version = \\?").
					WithArgs(3).
					WillReturnError(errors.New("database error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.LoadStoredEnv(th, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadStoredEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadStoredEnv() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_DeleteStoredEnv(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{StoredEnv: "stored_env"}

	tests := []struct {
		name    string
		version int
		setup   func()
		wantErr bool
	}{
		{
			name:    "successful delete",
			version: 1,
			setup: func() {
				mock.ExpectExec("DELETE FROM stored_env WHERE version = \\?").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
			wantErr: false,
		},
		{
			name:    "database error",
			version: 2,
			setup: func() {
				mock.ExpectExec("DELETE FROM stored_env WHERE version = \\?").
					WithArgs(2).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.DeleteStoredEnv(th, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteStoredEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_RecordRun(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{MigrationRuns: "migration_runs"}

	tests := []struct {
		name      string
		version   int
		direction string
		status    int
		body      *string
		env       map[string]string
		failed    bool
		setup     func()
		wantErr   bool
	}{
		{
			name:      "successful record with env",
			version:   1,
			direction: "up",
			status:    200,
			body:      strPtr("response body"),
			env:       map[string]string{"KEY": "value"},
			failed:    false,
			setup: func() {
				mock.ExpectExec("INSERT INTO migration_runs\\(version, direction, status_code, body, env_json, failed, ran_at\\) VALUES\\(\\?,\\?,\\?,\\?,\\?,\\?,\\?\\)").
					WithArgs(1, "up", 200, strPtr("response body"), strPtr(`{"KEY":"value"}`), 0, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:      "successful record without env",
			version:   2,
			direction: "down",
			status:    404,
			body:      nil,
			env:       map[string]string{},
			failed:    true,
			setup: func() {
				mock.ExpectExec("INSERT INTO migration_runs\\(version, direction, status_code, body, env_json, failed, ran_at\\) VALUES\\(\\?,\\?,\\?,\\?,\\?,\\?,\\?\\)").
					WithArgs(2, "down", 404, nil, nil, 1, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(2, 1))
			},
			wantErr: false,
		},
		{
			name:      "database error",
			version:   3,
			direction: "up",
			status:    500,
			body:      nil,
			env:       map[string]string{},
			failed:    true,
			setup: func() {
				mock.ExpectExec("INSERT INTO migration_runs\\(version, direction, status_code, body, env_json, failed, ran_at\\) VALUES\\(\\?,\\?,\\?,\\?,\\?,\\?,\\?\\)").
					WithArgs(3, "up", 500, nil, nil, 1, sqlmock.AnyArg()).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.RecordRun(th, tt.version, tt.direction, tt.status, tt.body, tt.env, tt.failed)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_InsertStoredEnv(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{StoredEnv: "stored_env"}

	tests := []struct {
		name    string
		version int
		kv      map[string]string
		setup   func()
		wantErr bool
	}{
		{
			name:    "empty map",
			version: 1,
			kv:      map[string]string{},
			setup:   func() {}, // no expectations
			wantErr: false,
		},
		{
			name:    "single entry",
			version: 2,
			kv:      map[string]string{"KEY1": "value1"},
			setup: func() {
				mock.ExpectExec("INSERT OR REPLACE INTO stored_env\\(version, name, value\\) VALUES \\(\\?,\\?,\\?\\)").
					WithArgs(2, "KEY1", "value1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:    "multiple entries",
			version: 3,
			kv:      map[string]string{"KEY1": "value1", "KEY2": "value2"},
			setup: func() {
				// Note: the order of map iteration is not guaranteed, so we match on any args
				mock.ExpectExec("INSERT OR REPLACE INTO stored_env\\(version, name, value\\) VALUES.*").
					WillReturnResult(sqlmock.NewResult(1, 2))
			},
			wantErr: false,
		},
		{
			name:    "database error",
			version: 4,
			kv:      map[string]string{"KEY1": "value1"},
			setup: func() {
				mock.ExpectExec("INSERT OR REPLACE INTO stored_env\\(version, name, value\\) VALUES.*").
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := store.InsertStoredEnv(th, tt.version, tt.kv)
			if (err != nil) != tt.wantErr {
				t.Errorf("InsertStoredEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_InsertStoredEnv_TooLarge(t *testing.T) {
	store := NewStore()
	th := TableNames{StoredEnv: "stored_env"}

	// Create map larger than limit
	largeKV := make(map[string]string)
	for i := 0; i < 10001; i++ {
		largeKV[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	err := store.InsertStoredEnv(th, 1, largeKV)
	if err == nil {
		t.Error("InsertStoredEnv() should fail with too large map")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot store more than") {
		t.Errorf("InsertStoredEnv() error should mention size limit, got: %v", err)
	}
}

func TestStore_ListRuns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := &Store{db: db, dialect: NewDialect()}
	th := TableNames{MigrationRuns: "migration_runs"}

	testTime := "2023-12-25T10:30:45.123456789Z"

	tests := []struct {
		name    string
		setup   func()
		want    []Run
		wantErr bool
	}{
		{
			name: "multiple runs",
			setup: func() {
				mock.ExpectQuery("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM migration_runs ORDER BY id ASC").
					WillReturnRows(sqlmock.NewRows([]string{"id", "version", "direction", "status_code", "body", "env_json", "failed", "ran_at"}).
						AddRow(1, 1, "up", 200, "body1", `{"key":"value"}`, int64(0), testTime).
						AddRow(2, 2, "down", 404, nil, nil, int64(1), testTime))
			},
			want: []Run{
				{
					ID:         1,
					Version:    1,
					Direction:  "up",
					StatusCode: 200,
					Body:       strPtr("body1"),
					Env:        map[string]string{"key": "value"},
					Failed:     false,
					RanAt:      testTime,
				},
				{
					ID:         2,
					Version:    2,
					Direction:  "down",
					StatusCode: 404,
					Body:       nil,
					Env:        map[string]string{},
					Failed:     true,
					RanAt:      testTime,
				},
			},
			wantErr: false,
		},
		{
			name: "no runs",
			setup: func() {
				mock.ExpectQuery("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM migration_runs ORDER BY id ASC").
					WillReturnRows(sqlmock.NewRows([]string{"id", "version", "direction", "status_code", "body", "env_json", "failed", "ran_at"}))
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "database error",
			setup: func() {
				mock.ExpectQuery("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM migration_runs ORDER BY id ASC").
					WillReturnError(errors.New("database error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := store.ListRuns(th)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListRuns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListRuns() = %v, want %v", got, tt.want)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// Helper function for string pointers
func strPtr(s string) *string {
	return &s
}
