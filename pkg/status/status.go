package status

import (
	"fmt"

	"github.com/loykin/apimigrate"
)

// Status display constants
const (
	defaultHistoryLimit = 10 // Default number of history entries to show
)

// HistoryItem is a single execution record of a migration version.
// RanAt is an RFC3339 timestamp in UTC.
// Body may be nil when response body saving was disabled.
// Env contains stored environment variables snapshot when available.
type HistoryItem struct {
	ID         int
	Version    int
	Direction  string
	StatusCode int
	Failed     bool
	RanAt      string
	Body       *string
	Env        map[string]string
}

// Info aggregates status information: current version, applied list, and run history.
type Info struct {
	Version int
	Applied []int
	History []HistoryItem
}

// FromStore collects status information from an opened store.
func FromStore(st *apimigrate.Store) (Info, error) {
	cur, err := st.CurrentVersion()
	if err != nil {
		return Info{}, err
	}
	applied, err := st.ListApplied()
	if err != nil {
		return Info{}, err
	}
	runs, err := apimigrate.ListRuns(st)
	if err != nil {
		return Info{}, err
	}
	items := make([]HistoryItem, 0, len(runs))
	for _, r := range runs {
		items = append(items, HistoryItem{
			ID:         r.ID,
			Version:    r.Version,
			Direction:  r.Direction,
			StatusCode: r.StatusCode,
			Failed:     r.Failed,
			RanAt:      r.RanAt,
			Body:       r.Body,
			Env:        r.Env,
		})
	}
	return Info{Version: cur, Applied: applied, History: items}, nil
}

// FromOptions opens a store using the provided options, collects status, and closes it.
func FromOptions(dir string, cfg *apimigrate.StoreConfig) (Info, error) {
	st, err := apimigrate.OpenStoreFromOptions(dir, cfg)
	if err != nil {
		return Info{}, err
	}
	defer func() { _ = st.Close() }()
	return FromStore(st)
}

// FormatHuman returns a human-friendly multiline string for CLI output.
// history=false prints only current version and applied list (compatible with existing CLI tests);
// history=true additionally appends a formatted history section.
func (i Info) FormatHuman(history bool) string {
	base := fmt.Sprintf("current: %d\napplied: %v\n", i.Version, i.Applied)
	if !history {
		return base
	}
	// Append history
	if len(i.History) == 0 {
		return base + "history: \n"
	}
	out := base + "history:\n"
	for _, h := range i.History {
		out += fmt.Sprintf("#%d v=%d dir=%s code=%d failed=%t at=%s\n", h.ID, h.Version, h.Direction, h.StatusCode, h.Failed, h.RanAt)
	}
	return out
}

// FormatHumanWithLimit prints status like FormatHuman, but when history=true it prints
// newest-first up to the provided limit. If all=true, the entire history is printed
// newest-first and limit is ignored. Default behavior when limit<=0 is 10.
func (i Info) FormatHumanWithLimit(history bool, limit int, all bool) string {
	base := fmt.Sprintf("current: %d\napplied: %v\n", i.Version, i.Applied)
	if !history {
		return base
	}
	if len(i.History) == 0 {
		return base + "history: \n"
	}
	// reverse copy to make newest-first (assuming underlying history is oldest-first)
	rev := make([]HistoryItem, len(i.History))
	for idx := range i.History {
		rev[len(i.History)-1-idx] = i.History[idx]
	}
	items := rev
	if !all {
		if limit <= 0 {
			limit = defaultHistoryLimit
		}
		if len(items) > limit {
			items = items[:limit]
		}
	}
	out := base + "history:\n"
	for _, h := range items {
		out += fmt.Sprintf("#%d v=%d dir=%s code=%d failed=%t at=%s\n", h.ID, h.Version, h.Direction, h.StatusCode, h.Failed, h.RanAt)
	}
	return out
}
