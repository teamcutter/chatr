package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/linker"
)

// stubState is an in-memory domain.State whose Remove can be made to fail.
type stubState struct {
	pkgs      map[string]*domain.InstalledPackage
	removeErr error
}

func (s *stubState) Load() (*domain.Manifest, error) { return &domain.Manifest{Packages: s.pkgs}, nil }
func (s *stubState) Save(m *domain.Manifest) error   { s.pkgs = m.Packages; return nil }
func (s *stubState) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	p, ok := s.pkgs[name]
	return ok, p, nil
}
func (s *stubState) Add(pkg *domain.InstalledPackage) error { s.pkgs[pkg.Name] = pkg; return nil }
func (s *stubState) Remove(name string) error {
	if s.removeErr != nil {
		return s.removeErr
	}
	delete(s.pkgs, name)
	return nil
}
func (s *stubState) Flush() error { return nil }
func (s *stubState) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	return s.pkgs, nil
}
func (s *stubState) BeginInstall(pkg *domain.InstalledPackage) error { return nil }

func newRemoveFixture(t *testing.T) (*Manager, *stubState, string) {
	t.Helper()
	base := t.TempDir()
	appsDir := filepath.Join(base, "Applications")
	for _, app := range []string{"Foo.app", "Bar.app", "Shared.app"} {
		if err := os.MkdirAll(filepath.Join(appsDir, app), 0755); err != nil {
			t.Fatal(err)
		}
	}
	st := &stubState{pkgs: map[string]*domain.InstalledPackage{
		"foo":    {Name: "foo", IsCask: true, Apps: []string{"Foo.app"}, Dependencies: []string{"bar", "shared"}},
		"bar":    {Name: "bar", IsCask: true, IsDep: true, Apps: []string{"Bar.app"}},
		"shared": {Name: "shared", IsCask: true, IsDep: true, Apps: []string{"Shared.app"}},
		"other":  {Name: "other", IsCask: true, Dependencies: []string{"shared"}},
	}}
	lnkr := linker.New(filepath.Join(base, "Cellar"), filepath.Join(base, "opt"), map[string]string{})
	return New(nil, nil, nil, st, lnkr, appsDir), st, appsDir
}

func TestRemoveCascadesToUnsharedDependencies(t *testing.T) {
	m, st, appsDir := newRemoveFixture(t)

	removed, err := m.Remove(context.Background(), domain.Package{Name: "foo"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.Name != "foo" {
		t.Errorf("removed = %s", removed.Name)
	}

	for _, gone := range []string{"foo", "bar"} {
		if _, ok := st.pkgs[gone]; ok {
			t.Errorf("%s should be removed from state", gone)
		}
	}
	if _, ok := st.pkgs["shared"]; !ok {
		t.Error("shared is still needed by 'other' and must stay installed")
	}

	for _, app := range []string{"Foo.app", "Bar.app"} {
		if _, err := os.Stat(filepath.Join(appsDir, app)); !os.IsNotExist(err) {
			t.Errorf("%s should be deleted, stat err = %v", app, err)
		}
	}
	if _, err := os.Stat(filepath.Join(appsDir, "Shared.app")); err != nil {
		t.Errorf("Shared.app must remain: %v", err)
	}
}

func TestRemoveLeavesFilesWhenStateUpdateFails(t *testing.T) {
	m, st, appsDir := newRemoveFixture(t)
	st.removeErr = errors.New("disk full")

	if _, err := m.Remove(context.Background(), domain.Package{Name: "foo"}); err == nil {
		t.Fatal("expected error from state")
	}
	if _, err := os.Stat(filepath.Join(appsDir, "Foo.app")); err != nil {
		t.Errorf("Foo.app must not be deleted when state update fails: %v", err)
	}
	if _, ok := st.pkgs["foo"]; !ok {
		t.Error("foo should still be in state")
	}
}

func TestRemoveNotInstalled(t *testing.T) {
	m, _, _ := newRemoveFixture(t)
	if _, err := m.Remove(context.Background(), domain.Package{Name: "nope"}); err == nil {
		t.Fatal("expected error for unknown package")
	}
}
