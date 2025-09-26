package apirun

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCreateMigration_CreatesTimestampedFileWithTemplate(t *testing.T) {
	dir := t.TempDir()
	p, err := CreateMigration(CreateOptions{Name: "Create User", Dir: dir})
	if err != nil {
		t.Fatalf("CreateMigration error: %v", err)
	}
	// Filename pattern: YYYYMMDDHHMMSS_create_user.yaml
	name := filepath.Base(p)
	re := regexp.MustCompile(`^[0-9]{14}_create_user\.yaml$`)
	if !re.MatchString(name) {
		t.Fatalf("unexpected filename: %s", name)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	content := string(b)
	// Basic template sanity checks
	for _, must := range []string{"up:", "request:", "response:", "result_code:"} {
		if !strings.Contains(content, must) {
			t.Fatalf("created file missing %q. content=\n%s", must, content)
		}
	}
}

func TestMigrator_CreateMigration_DelegatesToPackage(t *testing.T) {
	dir := t.TempDir()
	m := &Migrator{Dir: dir}
	p, err := m.CreateMigration("demo task")
	if err != nil {
		t.Fatalf("Migrator.CreateMigration: %v", err)
	}
	if !strings.HasPrefix(p, dir+string(os.PathSeparator)) {
		t.Fatalf("expected file under dir %s, got %s", dir, p)
	}
}

func TestCreateMigration_ErrorOnEmptyDir(t *testing.T) {
	if _, err := CreateMigration(CreateOptions{Name: "x", Dir: ""}); err == nil {
		t.Fatalf("expected error when Dir is empty")
	}
}
