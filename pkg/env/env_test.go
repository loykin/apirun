package env

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewAndFromStringMap(t *testing.T) {
	e := New()
	if e == nil || e.Auth == nil || e.Global == nil || e.Local == nil {
		t.Fatalf("New should init all maps")
	}
	m := FromStringMap(map[string]string{"a": "1", "b": "2"})
	if m["a"].String() != "1" || m["b"].String() != "2" {
		t.Fatalf("FromStringMap mismatch: %#v", m)
	}
}

func TestSetGetStringAndNormalize(t *testing.T) {
	e := New()
	cases := []struct{ mp string }{{"global"}, {"GLOBAL"}, {"auth"}, {"local"}, {"unknown"}}
	for _, c := range cases {
		if err := e.SetString(c.mp, "k", "v"); err != nil {
			t.Fatalf("SetString error for %s: %v", c.mp, err)
		}
		if got := e.GetString(c.mp, "k"); got != "v" {
			t.Fatalf("GetString mismatch for %s: %q", c.mp, got)
		}
	}
}

func TestSealUnseal(t *testing.T) {
	e := New()
	e.Seal()
	if err := e.SetString("global", "k", "v"); err == nil {
		t.Fatalf("expected error when sealed")
	}
	e.Unseal()
	if err := e.SetString("global", "k", "v"); err != nil {
		t.Fatalf("unexpected error after Unseal: %v", err)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	e := New()
	_ = e.SetString("global", "g", "1")
	_ = e.SetString("local", "l", "2")
	// place a lazy value in auth
	called := int32(0)
	e.Auth["tok"] = e.MakeLazy(func(_ *Env) (string, error) {
		atomic.AddInt32(&called, 1)
		return "T", nil
	})
	cl := e.Clone()
	// modify original after clone
	_ = e.SetString("global", "g", "X")
	_ = e.SetString("local", "l", "Y")
	if cl.GetString("global", "g") != "1" || cl.GetString("local", "l") != "2" {
		t.Fatalf("clone should preserve prior values: g=%s l=%s", cl.GetString("global", "g"), cl.GetString("local", "l"))
	}
	// lazy copied by reference: rendering through clone should call resolver once
	s := cl.RenderGoTemplate("{{.auth.tok}}-{{.auth.tok}}")
	if s != "T-T" {
		t.Fatalf("unexpected render: %q", s)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("lazy should be evaluated once, called=%d", called)
	}
}

func TestLookupPrecedence(t *testing.T) {
	e := New()
	_ = e.SetString("global", "x", "G")
	_ = e.SetString("local", "x", "L")
	if v, ok := e.Lookup("x"); !ok || v != "L" {
		t.Fatalf("Lookup should prefer local over global: v=%q ok=%v", v, ok)
	}
}

func TestRenderGoTemplateBasics(t *testing.T) {
	e := &Env{Global: FromStringMap(map[string]string{"name": "world", "api": "http://x"}), Local: FromStringMap(map[string]string{"id": "42"})}
	// also expose auth
	e.Auth = Map{"token": Str("abc")}
	if got := e.RenderGoTemplate("hello {{.env.name}} #{{.env.id}} tok={{.auth.token}}"); got != "hello world #42 tok=abc" {
		t.Fatalf("render mismatch: %q", got)
	}
	// non-template stays as-is
	if got := e.RenderGoTemplate("plain"); got != "plain" {
		t.Fatalf("non-template changed: %q", got)
	}
	// malformed template returns input
	in := "{{.bad"
	if got := e.RenderGoTemplate(in); got != in {
		t.Fatalf("malformed should return input: %q", got)
	}
}

func TestRenderGoTemplateErr(t *testing.T) {
	e := &Env{Global: FromStringMap(map[string]string{"a": "1"})}
	// ok case
	got, err := e.RenderGoTemplateErr("A={{.env.a}}")
	if err != nil || got != "A=1" {
		t.Fatalf("RenderGoTemplateErr ok mismatch: got=%q err=%v", got, err)
	}
	// missing key -> error
	_, err = e.RenderGoTemplateErr("{{.env.missing}}")
	if err == nil {
		t.Fatalf("expected error for missing key")
	}
	// parse error -> error
	_, err = e.RenderGoTemplateErr("{{.bad")
	if err == nil {
		t.Fatalf("expected error for parse failure")
	}
}

// Concurrency/race-oriented tests
func TestConcurrentSetGet(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	stop := int32(0)
	// writers mutate the base env
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for atomic.LoadInt32(&stop) == 0 {
				_ = e.SetString("global", fmt.Sprintf("k%d", id), fmt.Sprintf("v%d", id))
			}
		}(i)
	}
	// readers/renderers work on snapshots (Clone acquires RLock internally)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&stop) == 0 {
				cl := e.Clone()
				_ = cl.GetString("global", "k1")
				_, _ = cl.Lookup("k2")
				_ = cl.RenderGoTemplate("{{.env.k0}}-{{.env.k1}}-{{.env.k2}}")
			}
		}()
	}
	// run briefly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = e.SetString("local", "x", "y")
		}
	}()
	// stop writers/readers
	atomic.StoreInt32(&stop, 1)
	wg.Wait()
}

func TestParallelSubtests(t *testing.T) {
	e := New()
	// Seal global to avoid concurrent writes to same map while reading in parallel.
	_ = e.SetString("global", "name", "parallel")
	e.Seal()
	subs := []string{"a", "b", "c", "d"}
	for _, s := range subs {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			// Each subtest uses its own clone to avoid concurrent mutation of the same maps.
			cl := e.Clone()
			cl.Unseal()
			_ = cl.SetString("local", "k"+s, s)
			out := cl.RenderGoTemplate("{{.env.name}}-{{.env.k" + s + "}}")
			if out == "" {
				t.Fatalf("unexpected empty render")
			}
		})
	}
}

// Ensure Env.UnmarshalYAML decodes mapping under `env` into Local map.
func TestEnv_UnmarshalYAML_LocalMapping(t *testing.T) {
	type wrap struct {
		Env *Env `yaml:"env"`
	}
	var w wrap
	data := []byte("env:\n  a: '1'\n  b: '2'\n")
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if w.Env == nil || w.Env.Local == nil {
		t.Fatalf("expected Env and Local to be non-nil")
	}
	if w.Env.Local["a"].String() != "1" || w.Env.Local["b"].String() != "2" {
		t.Fatalf("unexpected Local values: a=%q b=%q", w.Env.Local["a"].String(), w.Env.Local["b"].String())
	}
}

// Cover Lookup edge cases: nil env, missing keys, precedence.
func TestEnv_Lookup_EdgeCases(t *testing.T) {
	var e *Env
	if v, ok := e.Lookup("x"); ok || v != "" {
		t.Fatalf("nil env Lookup should return empty,false; got %q,%v", v, ok)
	}
	e = New()
	// missing key
	if v, ok := e.Lookup("missing"); ok || v != "" {
		t.Fatalf("missing key should be empty,false; got %q,%v", v, ok)
	}
	// precedence local > global
	_ = e.SetString("global", "k", "G")
	_ = e.SetString("local", "k", "L")
	if v, ok := e.Lookup("k"); !ok || v != "L" {
		t.Fatalf("Lookup precedence failed: got %q,%v", v, ok)
	}
}

// Cover VarLazy.Value and error path in String() resolver.
func TestVarLazy_ValueAndError(t *testing.T) {
	e := New()
	calls := 0
	lv := e.MakeLazy(func(_ *Env) (string, error) {
		calls++
		return "TOK", nil
	})
	// Value triggers resolver once and returns value and nil err
	v, err := lv.Value()
	if err != nil || v != "TOK" {
		t.Fatalf("Value() mismatch: v=%q err=%v", v, err)
	}
	// Second call should not increment calls
	_, _ = lv.Value()
	if calls != 1 {
		t.Fatalf("resolver should be called once; calls=%d", calls)
	}

	// Error path: resolver returns error; String should cache empty and error
	errLazy := e.MakeLazy(func(_ *Env) (string, error) { return "", errors.New("boom") })
	if s := errLazy.String(); s != "" {
		t.Fatalf("expected empty string on error; got %q", s)
	}
	if _, err := errLazy.Value(); err == nil {
		t.Fatalf("expected error from Value after failure")
	}
}
