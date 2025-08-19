package env

import "testing"

func TestEnv_Lookup(t *testing.T) {
	e := Env{Local: map[string]string{"FOO": "bar"}}
	if v, ok := e.Lookup("FOO"); !ok || v != "bar" {
		t.Fatalf("expected FOO=bar, got ok=%v v=%q", ok, v)
	}
	if _, ok := e.Lookup("MISSING"); ok {
		t.Fatalf("expected missing key to return ok=false")
	}
}

func TestEnv_RenderGoTemplate(t *testing.T) {
	e := Env{Global: map[string]string{"username": "alice", "CITY": "seoul"}}
	in := "user={{.username}}, city={{.CITY}}"
	out := e.RenderGoTemplate(in)
	expected := "user=alice, city=seoul"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
	// Missing keys should keep the original string unchanged
	if got := e.RenderGoTemplate("hello {{.missing}}"); got != "hello {{.missing}}" {
		t.Fatalf("expected unchanged when missing key, got %q", got)
	}
}
