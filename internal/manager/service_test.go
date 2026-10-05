package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/linker"
)

type fakeFetcher struct{ calls int }

func (f *fakeFetcher) Fetch(ctx context.Context, pkg domain.Package) domain.FetchResult {
	f.calls++
	return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Path: "/dev/null"}
}

type fakeCache struct{ path string }

func (c *fakeCache) Has(name, version string) bool                   { return true }
func (c *fakeCache) GetPath(name, version string) string             { return c.path }
func (c *fakeCache) Store(name, version, src string) (string, error) { return c.path, nil }
func (c *fakeCache) Size() (int64, error)                            { return 0, nil }
func (c *fakeCache) Clear() error                                    { return nil }

type fakeExtractor struct {
	apps  []string
	calls int
}

func (e *fakeExtractor) Extract(src, dst string) error { return nil }
func (e *fakeExtractor) ExtractApps(src, dst string) ([]string, error) {
	e.calls++
	for _, app := range e.apps {
		if err := os.MkdirAll(filepath.Join(dst, app), 0755); err != nil {
			return nil, err
		}
	}
	return e.apps, nil
}

type memState struct {
	pkgs map[string]*domain.InstalledPackage
}

func newMemState() *memState { return &memState{pkgs: map[string]*domain.InstalledPackage{}} }

func (s *memState) Load() (*domain.Manifest, error) { return &domain.Manifest{Packages: s.pkgs}, nil }
func (s *memState) Save(m *domain.Manifest) error   { s.pkgs = m.Packages; return nil }
func (s *memState) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	p, ok := s.pkgs[name]
	return ok, p, nil
}
func (s *memState) Add(pkg *domain.InstalledPackage) error { s.pkgs[pkg.Name] = pkg; return nil }
func (s *memState) Remove(name string) error               { delete(s.pkgs, name); return nil }
func (s *memState) Flush() error                           { return nil }
func (s *memState) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	return s.pkgs, nil
}
func (s *memState) BeginInstall(pkg *domain.InstalledPackage) error { return nil }

func newTestManager(t *testing.T, apps []string) (*Manager, string, *fakeFetcher, *fakeExtractor) {
	t.Helper()
	base := t.TempDir()
	appsDir := filepath.Join(base, "Applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatal(err)
	}
	lnkr := linker.New(filepath.Join(base, "Cellar"), filepath.Join(base, "opt"), map[string]string{})
	fetcher := &fakeFetcher{}
	extractor := &fakeExtractor{apps: apps}
	m := New(fetcher, &fakeCache{path: filepath.Join(base, "pkg.zip")}, extractor, newMemState(), lnkr, appsDir)
	return m, appsDir, fetcher, extractor
}

func caskPkg(force bool) domain.Package {
	return domain.Package{
		Name:        "foo",
		Version:     "1.0",
		FullVersion: "1.0",
		IsCask:      true,
		Apps:        []string{"Foo.app"},
		Force:       force,
	}
}

func TestInstallCaskRefusesToReplaceForeignApp(t *testing.T) {
	m, appsDir, _, extractor := newTestManager(t, []string{"Foo.app"})
	if err := os.MkdirAll(filepath.Join(appsDir, "Foo.app"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := m.Install(context.Background(), caskPkg(false))
	if err == nil {
		t.Fatal("expected error when app already exists")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force: %v", err)
	}
	if extractor.calls != 0 {
		t.Errorf("extractor should not have run, calls = %d", extractor.calls)
	}
	if installed, _, _ := m.IsInstalled("foo"); installed {
		t.Error("package must not be recorded as installed")
	}
}

func TestInstallCaskForceReplacesExistingApp(t *testing.T) {
	m, appsDir, _, extractor := newTestManager(t, []string{"Foo.app"})
	if err := os.MkdirAll(filepath.Join(appsDir, "Foo.app"), 0755); err != nil {
		t.Fatal(err)
	}

	pkg, err := m.Install(context.Background(), caskPkg(true))
	if err != nil {
		t.Fatalf("Install with Force: %v", err)
	}
	if extractor.calls != 1 {
		t.Errorf("extractor calls = %d, want 1", extractor.calls)
	}
	if len(pkg.Apps) != 1 || pkg.Apps[0] != "Foo.app" {
		t.Errorf("Apps = %v", pkg.Apps)
	}
}

func TestInstallCaskWhenAppAbsent(t *testing.T) {
	m, appsDir, _, _ := newTestManager(t, []string{"Foo.app"})

	if _, err := m.Install(context.Background(), caskPkg(false)); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appsDir, "Foo.app")); err != nil {
		t.Errorf("app not placed in apps dir: %v", err)
	}
	if installed, _, _ := m.IsInstalled("foo"); !installed {
		t.Error("package should be recorded as installed")
	}
}
