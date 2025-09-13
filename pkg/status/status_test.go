package status

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/loykin/apimigrate"
)

// helper to open a temp sqlite store under a temp dir for status tests
func openTempStoreForStatus(t *testing.T) *apimigrate.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := &apimigrate.StoreConfig{}
	cfg.Config.Driver = apimigrate.DriverSqlite
	cfg.Config.DriverConfig = &apimigrate.SqliteConfig{Path: filepath.Join(dir, apimigrate.StoreDBFileName)}
	st, err := apimigrate.OpenStoreFromOptions(dir, cfg)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions: %v", err)
	}
	return st
}

func TestFormatHuman_NoHistory(t *testing.T) {
	i := Info{Version: 3, Applied: []int{1, 3}}
	got := i.FormatHuman(false)
	want := "current: 3\napplied: [1 3]\n"
	if got != want {
		t.Fatalf("FormatHuman(false) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestFormatHumanWithLimit_NewestFirstAndLimit(t *testing.T) {
	i := Info{
		Version: 5,
		Applied: []int{1, 2, 3, 4, 5},
		History: []HistoryItem{
			{ID: 1, Version: 1, Direction: "up", StatusCode: 200, RanAt: "2025-01-01T00:00:00Z"},
			{ID: 2, Version: 2, Direction: "up", StatusCode: 200, RanAt: "2025-01-01T00:01:00Z"},
			{ID: 3, Version: 3, Direction: "up", StatusCode: 200, RanAt: "2025-01-01T00:02:00Z"},
			{ID: 4, Version: 4, Direction: "up", StatusCode: 200, RanAt: "2025-01-01T00:03:00Z"},
			{ID: 5, Version: 5, Direction: "up", StatusCode: 200, RanAt: "2025-01-01T00:04:00Z"},
		},
	}
	got := i.FormatHumanWithLimit(true, 3, false)
	// Expect newest-first (IDs 5,4,3) and exactly 3 lines in history
	re := regexp.MustCompile(`(?s)^current: 5\napplied: \[1 2 3 4 5]\nhistory:\n#5 .*\n#4 .*\n#3 .*\n$`)
	if !re.MatchString(got) {
		t.Fatalf("unexpected output with limit:\n%s", got)
	}
}

func TestFormatHumanWithLimit_AllIgnoresLimit(t *testing.T) {
	i := Info{
		Version: 2,
		Applied: []int{1, 2},
		History: []HistoryItem{{ID: 1, Version: 1, Direction: "up", StatusCode: 200, RanAt: "t1"}, {ID: 2, Version: 2, Direction: "up", StatusCode: 200, RanAt: "t2"}},
	}
	got := i.FormatHumanWithLimit(true, 1, true)
	re := regexp.MustCompile(`(?s)^current: 2\napplied: \[1 2]\nhistory:\n#2 .*\n#1 .*\n$`)
	if !re.MatchString(got) {
		t.Fatalf("unexpected output with all=true:\n%s", got)
	}
}

func TestFromStore_Empty(t *testing.T) {
	st := openTempStoreForStatus(t)
	t.Cleanup(func() { _ = st.Close() })
	i, err := FromStore(st)
	if err != nil {
		t.Fatalf("FromStore: %v", err)
	}
	if i.Version != 0 || len(i.Applied) != 0 || len(i.History) != 0 {
		t.Fatalf("unexpected empty status: %+v", i)
	}
}

func TestFromStore_WithRuns(t *testing.T) {
	st := openTempStoreForStatus(t)
	defer func() { _ = st.Close() }()
	// Apply some versions to affect current/applied
	if err := st.Apply(1); err != nil {
		t.Fatalf("Apply(1): %v", err)
	}
	if err := st.Apply(3); err != nil {
		t.Fatalf("Apply(3): %v", err)
	}
	// Record a couple of runs
	body := "ok"
	if err := st.RecordRun(1, "up", 200, &body, map[string]string{"a": "1"}, false); err != nil {
		t.Fatalf("RecordRun #1: %v", err)
	}
	if err := st.RecordRun(2, "up", 500, nil, nil, true); err != nil {
		t.Fatalf("RecordRun #2: %v", err)
	}
	info, err := FromStore(st)
	if err != nil {
		t.Fatalf("FromStore: %v", err)
	}
	if info.Version != 3 {
		t.Fatalf("Version=%d, want 3", info.Version)
	}
	if len(info.Applied) != 2 || info.Applied[0] != 1 || info.Applied[1] != 3 {
		t.Fatalf("Applied=%v, want [1 3]", info.Applied)
	}
	if len(info.History) != 2 {
		t.Fatalf("History len=%d, want 2", len(info.History))
	}
	// Check fields mapped
	if info.History[0].Version != 1 || info.History[0].Direction != "up" || info.History[0].StatusCode != 200 || info.History[0].Failed {
		t.Fatalf("History[0] unexpected: %#v", info.History[0])
	}
	if info.History[1].Version != 2 || info.History[1].Direction != "up" || info.History[1].StatusCode != 500 || !info.History[1].Failed {
		t.Fatalf("History[1] unexpected: %#v", info.History[1])
	}
	// ran_at should look like a timestamp
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)
	for i, h := range info.History {
		if h.RanAt == "" || !re.MatchString(h.RanAt) {
			t.Fatalf("History[%d].RanAt invalid: %q", i, h.RanAt)
		}
	}
}

func TestFromOptions_DefaultSqlite(t *testing.T) {
	dir := t.TempDir()
	info, err := FromOptions(dir, nil)
	if err != nil {
		t.Fatalf("FromOptions(dir,nil): %v", err)
	}
	if info.Version != 0 || len(info.Applied) != 0 || len(info.History) != 0 {
		t.Fatalf("unexpected initial status: %+v", info)
	}
}
