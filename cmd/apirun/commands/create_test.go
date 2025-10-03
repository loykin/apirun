package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCreateCmd_GeneratesTimestampedFileWithTemplate(t *testing.T) {
	tdir := t.TempDir()
	migDir := filepath.Join(tdir, "migration")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Minimal config.yaml with migrate_dir
	cfgPath := filepath.Join(tdir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("---\nmigrate_dir: "+migDir+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)

	// Run the command (name: sample_task)
	CreateCmd.SetArgs([]string{"sample task"})
	if err := CreateCmd.RunE(CreateCmd, []string{"sample task"}); err != nil {
		t.Fatalf("CreateCmd.RunE: %v", err)
	}

	// Find created file
	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file created, got %d", len(entries))
	}
	name := entries[0].Name()
	if !regexp.MustCompile(`^[0-9]{14}_sample_task\.yaml$`).MatchString(name) {
		t.Fatalf("unexpected filename: %s", name)
	}

	b, err := os.ReadFile(filepath.Join(migDir, name))
	if err != nil {
		t.Fatalf("read created: %v", err)
	}
	content := string(b)
	for _, must := range []string{"up:", "request:", "response:", "result_code:"} {
		if !strings.Contains(content, must) {
			t.Fatalf("created template missing %q. content=\n%s", must, content)
		}
	}
}
