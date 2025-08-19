package store

import (
	"os"
	"path/filepath"
	"testing"
)

// newTempDB returns a temp path for sqlite file under t.TempDir().
func newTempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, StoreDBFileName)
}

func TestStore_OpenEnsureSchema_Idempotent(t *testing.T) {
	path := newTempDB(t)
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer func() { _ = st.Close() }()
	// EnsureSchema should be idempotent
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema idempotent call failed: %v", err)
	}
	// File should exist on disk (driver uses file: DSN; ensure file created)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file to exist, stat err: %v", err)
	}
}

func TestStore_Apply_IsApplied_List_Current(t *testing.T) {
	path := newTempDB(t)
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Initially nothing applied
	if cur, err := st.CurrentVersion(); err != nil || cur != 0 {
		t.Fatalf("expected current=0, got cur=%d err=%v", cur, err)
	}
	if ok, err := st.IsApplied(1); err != nil || ok {
		t.Fatalf("expected version 1 not applied, ok=%v err=%v", ok, err)
	}

	// Apply versions 1 and 2 (with duplicate 2)
	if err := st.Apply(1); err != nil {
		t.Fatalf("apply 1: %v", err)
	}
	if err := st.Apply(2); err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if err := st.Apply(2); err != nil {
		t.Fatalf("apply duplicate 2 should be ignored, err=%v", err)
	}

	// Checks
	if ok, err := st.IsApplied(1); err != nil || !ok {
		t.Fatalf("expected version 1 applied, ok=%v err=%v", ok, err)
	}
	if ok, err := st.IsApplied(2); err != nil || !ok {
		t.Fatalf("expected version 2 applied, ok=%v err=%v", ok, err)
	}
	applied, err := st.ListApplied()
	if err != nil {
		t.Fatalf("list applied: %v", err)
	}
	if len(applied) != 2 || applied[0] != 1 || applied[1] != 2 {
		t.Fatalf("expected [1 2], got %v", applied)
	}
	cur, err := st.CurrentVersion()
	if err != nil || cur != 2 {
		t.Fatalf("expected current=2, got cur=%d err=%v", cur, err)
	}

	// Remove version 2 and verify
	if err := st.Remove(2); err != nil {
		t.Fatalf("remove 2: %v", err)
	}
	if ok, err := st.IsApplied(2); err != nil || ok {
		t.Fatalf("expected version 2 removed, ok=%v err=%v", ok, err)
	}
	cur, err = st.CurrentVersion()
	if err != nil || cur != 1 {
		t.Fatalf("expected current=1 after remove, got cur=%d err=%v", cur, err)
	}
}

func TestStore_SetVersion_DownOnly(t *testing.T) {
	path := newTempDB(t)
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Apply 1,2,3
	for _, v := range []int{1, 2, 3} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("apply %d: %v", v, err)
		}
	}
	if cur, _ := st.CurrentVersion(); cur != 3 {
		t.Fatalf("expected current=3, got %d", cur)
	}
	// SetVersion down to 1
	if err := st.SetVersion(1); err != nil {
		t.Fatalf("SetVersion down to 1: %v", err)
	}
	if cur, _ := st.CurrentVersion(); cur != 1 {
		t.Fatalf("expected current=1 after SetVersion, got %d", cur)
	}
	// No-op when equal
	if err := st.SetVersion(1); err != nil {
		t.Fatalf("SetVersion equal should be no-op, err=%v", err)
	}
	// Error when trying to move up
	if err := st.SetVersion(2); err == nil {
		t.Fatalf("expected error when moving up with SetVersion, got nil")
	}
}
