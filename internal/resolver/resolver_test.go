package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/teamcutter/chatr/internal/domain"
)

type fakeRegistry map[string]*domain.Formula

func (r fakeRegistry) Get(_ context.Context, name string) (*domain.Formula, error) {
	if f, ok := r[name]; ok {
		return f, nil
	}
	return nil, errors.New("not found")
}
func (r fakeRegistry) Search(context.Context, string) ([]domain.Formula, error) { return nil, nil }
func (r fakeRegistry) GetVersion(context.Context, string) (string, error)       { return "", nil }
func (r fakeRegistry) Update(context.Context) (int, error)                      { return 0, nil }

type emptyState struct{}

func (emptyState) IsInstalled(string) (bool, *domain.InstalledPackage, error) { return false, nil, nil }
func (emptyState) Add(*domain.InstalledPackage) error                         { return nil }
func (emptyState) Remove(string) error                                        { return nil }
func (emptyState) Flush() error                                               { return nil }
func (emptyState) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	return nil, nil
}
func (emptyState) BeginInstall(*domain.InstalledPackage) error { return nil }

func TestResolveFailsWhenDependencyHasNoBottle(t *testing.T) {
	reg := fakeRegistry{
		"app": {Name: "app", URL: "https://example/app.tar.gz", Dependencies: []string{"lib"}},
		"lib": {Name: "lib"},
	}
	_, err := New(reg, emptyState{}).Resolve(context.Background(), "app")
	if !errors.Is(err, ErrNoBottle) {
		t.Fatalf("err = %v, want ErrNoBottle", err)
	}
}

func TestResolveAllowsCaskWithURL(t *testing.T) {
	reg := fakeRegistry{"firefox": {Name: "firefox", URL: "https://example/f.dmg", IsCask: true}}
	got, err := New(reg, emptyState{}).Resolve(context.Background(), "firefox")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
}
