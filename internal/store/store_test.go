package store

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to open a store in a temporary file path
func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, DbFileName)
	st, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close(); _ = os.Remove(path) })
	return st
}

func TestOpenAndEmptyState(t *testing.T) {
	st := openTempStore(t)
	// EnsureSchema should be idempotent
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}
	// No versions yet
	v, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected CurrentVersion=0, got %d", v)
	}
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty applied list, got %v", list)
	}
}

func TestApplyListCurrentIsApplied(t *testing.T) {
	st := openTempStore(t)
	// Apply out of order
	for _, v := range []int{1, 3, 2} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("Apply(%d) err: %v", v, err)
		}
	}
	// Idempotent apply should not error
	if err := st.Apply(2); err != nil {
		t.Fatalf("re-Apply err: %v", err)
	}
	// IsApplied checks
	ap, err := st.IsApplied(2)
	if err != nil || !ap {
		t.Fatalf("IsApplied(2) => %v, %v; want true, nil", ap, err)
	}
	ap, err = st.IsApplied(99)
	if err != nil || ap {
		t.Fatalf("IsApplied(99) => %v, %v; want false, nil", ap, err)
	}
	// CurrentVersion should be 3
	cur, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion err: %v", err)
	}
	if cur != 3 {
		t.Fatalf("CurrentVersion=%d, want 3", cur)
	}
	// ListApplied should be sorted ascending
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	want := []int{1, 2, 3}
	if len(list) != len(want) {
		t.Fatalf("ListApplied length=%d, want %d; list=%v", len(list), len(want), list)
	}
	for i := range want {
		if list[i] != want[i] {
			t.Fatalf("ListApplied[%d]=%d, want %d (full: %v)", i, list[i], want[i], list)
		}
	}
}

func TestRemoveAndSetVersion(t *testing.T) {
	st := openTempStore(t)
	for _, v := range []int{1, 2, 3} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("Apply(%d) err: %v", v, err)
		}
	}
	// Remove a specific version
	if err := st.Remove(2); err != nil {
		t.Fatalf("Remove(2) err: %v", err)
	}
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	if len(list) != 2 || list[0] != 1 || list[1] != 3 {
		t.Fatalf("after Remove, list=%v; want [1 3]", list)
	}
	// SetVersion to same current (3) on current 3 should be no-op
	cur, _ := st.CurrentVersion()
	if err := st.SetVersion(cur); err != nil {
		t.Fatalf("SetVersion(same=%d) err: %v", cur, err)
	}
	// Move down to 1 should delete >1
	if err := st.SetVersion(1); err != nil {
		t.Fatalf("SetVersion(1) err: %v", err)
	}
	list, err = st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	if len(list) != 1 || list[0] != 1 {
		t.Fatalf("after SetVersion(1), list=%v; want [1]", list)
	}
	// Attempt to move up should error
	if err := st.SetVersion(2); err == nil {
		t.Fatalf("expected error on SetVersion moving up, got nil")
	}
}

func TestCloseNilSafety(t *testing.T) {
	var s *Store
	if err := s.Close(); err != nil {
		t.Fatalf("nil Close should return nil, got %v", err)
	}
}
