package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/teamcutter/chatr/internal/cache"
	"github.com/teamcutter/chatr/internal/domain"
)

type writingFetcher struct {
	dir   string
	body  string
	err   error
	calls int
}

func (f *writingFetcher) Fetch(_ context.Context, pkg domain.Package) domain.FetchResult {
	f.calls++
	if f.err != nil {
		return domain.FetchResult{Package: pkg.Name, Error: f.err}
	}
	p := filepath.Join(f.dir, pkg.Name+".tar.gz")
	os.WriteFile(p, []byte(f.body), 0644)
	return domain.FetchResult{Package: pkg.Name, Path: p}
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func newArchiveManager(t *testing.T, cached string, f *writingFetcher) (*Manager, domain.Package) {
	t.Helper()
	c, err := cache.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.dir = t.TempDir()
	pkg := domain.Package{Name: "jq", FullVersion: "1.8.2"}
	if cached != "" {
		src := filepath.Join(t.TempDir(), "jq.tar.gz")
		os.WriteFile(src, []byte(cached), 0644)
		if _, err := c.Store(pkg.Name, pkg.FullVersion, src); err != nil {
			t.Fatal(err)
		}
	}
	return &Manager{fetcher: f, cache: c}, pkg
}

func readArchive(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestArchiveReusesVerifiedCache(t *testing.T) {
	f := &writingFetcher{body: "fresh"}
	m, pkg := newArchiveManager(t, "good", f)
	pkg.SHA256 = sum("good")

	path, err := m.archive(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 || readArchive(t, path) != "good" {
		t.Errorf("fetch calls = %d, archive = %q; want the cached copy", f.calls, readArchive(t, path))
	}
}

func TestArchiveReplacesStaleCache(t *testing.T) {
	f := &writingFetcher{body: "rebuilt"}
	m, pkg := newArchiveManager(t, "old bottle", f)
	pkg.SHA256 = sum("rebuilt")

	path, err := m.archive(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || readArchive(t, path) != "rebuilt" {
		t.Errorf("fetch calls = %d, archive = %q; want a fresh download", f.calls, readArchive(t, path))
	}
}

func TestArchiveEnforcesChecksumOnCacheHit(t *testing.T) {
	mismatch := errors.New("checksum mismatch")
	f := &writingFetcher{err: mismatch}
	m, pkg := newArchiveManager(t, "cached", f)
	pkg.SHA256 = sum("what --sha256 expects")

	if _, err := m.archive(context.Background(), pkg); !errors.Is(err, mismatch) {
		t.Fatalf("err = %v, want the download's checksum error", err)
	}
	if m.cache.Has(pkg.Name, pkg.FullVersion) {
		t.Error("the mismatched cached archive should have been discarded")
	}
}

func TestArchiveWithoutChecksumUsesCache(t *testing.T) {
	f := &writingFetcher{body: "fresh"}
	m, pkg := newArchiveManager(t, "no_check cask", f)

	path, err := m.archive(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 || readArchive(t, path) != "no_check cask" {
		t.Errorf("fetch calls = %d; a package without a checksum should use the cache", f.calls)
	}
}
