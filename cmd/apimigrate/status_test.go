package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate"
	"github.com/spf13/viper"
)

// captureOutput captures stdout produced by f and returns it as string.
func captureOutput(t *testing.T, f func()) string {
	t.Helper()
	// Save the original stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Run the function
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	f()

	// Restore and close
	_ = w.Close()
	os.Stdout = old
	<-done
	_ = r.Close()

	return buf.String()
}

func TestStatusCmd_EmptyStore_PrintsZeroAndEmpty(t *testing.T) {
	tdir := t.TempDir()

	// Minimal config pointing to temp migrate dir (absolute path)
	cfgPath := writeFile(t, tdir, "config.yaml", "---\nmigrate_dir: "+tdir+"\n")

	// Configure viper
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)

	out := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("statusCmd.RunE error: %v", err)
		}
	})

	want := "current: 0\napplied: []\n"
	if out != want {
		t.Fatalf("unexpected output.\nwant: %q\n got: %q", want, out)
	}
}

func TestStatusCmd_WithAppliedVersions_PrintsCurrentAndList(t *testing.T) {
	tdir := t.TempDir()

	// Create store and apply versions 1 and 3
	dbPath := filepath.Join(tdir, apimigrate.StoreDBFileName)
	cfg := &apimigrate.StoreConfig{}
	cfg.Config.Driver = apimigrate.DriverSqlite
	cfg.Config.DriverConfig = &apimigrate.SqliteConfig{Path: dbPath}
	st, err := apimigrate.OpenStoreFromOptions(filepath.Dir(dbPath), cfg)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions: %v", err)
	}
	if err := st.Apply(1); err != nil {
		t.Fatalf("Apply(1): %v", err)
	}
	if err := st.Apply(3); err != nil {
		t.Fatalf("Apply(3): %v", err)
	}
	_ = st.Close()

	// Config pointing to this migrate dir (absolute path)
	cfgPath := writeFile(t, tdir, "config.yaml", "---\nmigrate_dir: "+tdir+"\n")

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)

	out := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("statusCmd.RunE error: %v", err)
		}
	})

	want := "current: 3\napplied: [1 3]\n"
	if out != want {
		t.Fatalf("unexpected output.\nwant: %q\n got: %q", want, out)
	}
}
