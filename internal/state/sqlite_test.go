package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

func TestRecoverRemovesPendingCaskAppsFromAppsDir(t *testing.T) {
	base := t.TempDir()
	appsDir := filepath.Join(base, "Applications")
	cwd := filepath.Join(base, "cwd")
	for _, d := range []string{filepath.Join(appsDir, "Foo.app"), filepath.Join(cwd, "Foo.app")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(cwd)

	dbPath := filepath.Join(base, "state.db")
	manifest := filepath.Join(base, "installed.json")

	st, err := NewSQLite(dbPath, manifest, appsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginInstall(&domain.InstalledPackage{
		Name:        "foo",
		Version:     "1.0",
		URL:         "https://example.invalid/foo.zip",
		Path:        appsDir,
		Apps:        []string{"Foo.app"},
		IsCask:      true,
		InstalledAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	// Reopening runs recover() for the pending row.
	st, err = NewSQLite(dbPath, manifest, appsDir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := os.Stat(filepath.Join(appsDir, "Foo.app")); !os.IsNotExist(err) {
		t.Errorf("pending app in apps dir should have been removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "Foo.app")); err != nil {
		t.Errorf("app in working directory must be left alone, stat err = %v", err)
	}
	if installed, _, _ := st.IsInstalled("foo"); installed {
		t.Error("pending package should not be reported as installed")
	}
}
