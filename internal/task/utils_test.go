package task

import (
	"encoding/json"
	"testing"
)

type demoStringer struct{ v string }

func (d demoStringer) String() string { return "STR:" + d.v }

func TestAnyToString_CoversTypesAndJSONFallback(t *testing.T) {
	// string
	if got := anyToString("hello"); got != "hello" {
		t.Fatalf("string path: got %q", got)
	}
	// fmt.Stringer
	if got := anyToString(demoStringer{"x"}); got != "STR:x" {
		t.Fatalf("stringer path: got %q", got)
	}
	// float64 that is an integer value should format without .0
	if got := anyToString(float64(42)); got != "42" {
		t.Fatalf("float64 integer path: got %q", got)
	}
	// float64 fractional retains fraction
	if got := anyToString(3.14); got != "3.14" {
		t.Fatalf("float64 fraction path: got %q", got)
	}
	// bool true/false
	if got := anyToString(true); got != "true" {
		t.Fatalf("bool true: got %q", got)
	}
	if got := anyToString(false); got != "false" {
		t.Fatalf("bool false: got %q", got)
	}
	// nil -> empty string
	if got := anyToString(nil); got != "" {
		t.Fatalf("nil path: got %q", got)
	}
	// JSON fallback: slice and map should marshal to JSON strings
	if got := anyToString([]int{1, 2}); got != "[1,2]" {
		t.Fatalf("slice json: got %q", got)
	}
	if got := anyToString(map[string]any{"a": 1}); got != "{\"a\":1}" {
		t.Fatalf("map json: got %q", got)
	}
	// JSON fallback with basic string should remove surrounding quotes
	b, _ := json.Marshal("text")
	_ = b                                          // just to show equivalence; function should produce plain text
	if got := anyToString("text"); got != "text" { // direct string path
		t.Fatalf("basic string path mismatch: %q", got)
	}
	// For a type that marshals to a quoted string, ensure quotes trimmed
	type quoted string
	q := quoted("hello")
	if got := anyToString(q); got != "hello" {
		t.Fatalf("quoted string via json fallback not trimmed: %q", got)
	}
	// For an unmarshalable value, it should fallback to fmt.Sprintf("%v")
	// Create a channel which json.Marshal cannot encode
	ch := make(chan int)
	got := anyToString(ch)
	if got == "" || got == "null" {
		t.Fatalf("unmarshalable fallback produced empty: %q", got)
	}
}
