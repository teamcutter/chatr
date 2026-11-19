package linker

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func prefixFixture(t *testing.T) (*Linker, string) {
	t.Helper()
	root := t.TempDir()
	return New(filepath.Join(root, "Cellar"), filepath.Join(root, "opt"), map[string]string{
		"bin": filepath.Join(root, "bin"), "share": filepath.Join(root, "share"),
		"lib": filepath.Join(root, "lib"), "include": filepath.Join(root, "include"),
	}), root
}

func writePrefixFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertPrefixFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, %v; want %q", path, got, err, want)
	}
}

func TestSharedPrefixDirectories(t *testing.T) {
	l, root := prefixFixture(t)
	dirs := []string{"share/man/man1", "share/zsh/site-functions", "lib/pkgconfig", "include/common"}
	for _, name := range []string{"first", "second"} {
		for _, dir := range dirs {
			writePrefixFile(t, filepath.Join(root, "Cellar", name, "1", dir, name), name)
		}
	}
	// Installs run concurrently in the CLI, including shared-directory creation.
	var wg sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		wg.Go(func() {
			if _, err := l.LinkToPrefix(name, "1"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for _, dir := range dirs {
		for _, name := range []string{"first", "second"} {
			assertPrefixFile(t, filepath.Join(root, dir, name), name)
		}
	}
	if err := l.UnlinkFromPrefix("first", nil); err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		assertPrefixFile(t, filepath.Join(root, dir, "second"), "second")
		assertPrefixFile(t, filepath.Join(root, "Cellar/first/1", dir, "first"), "first")
		if _, err := os.Lstat(filepath.Join(root, dir, "first")); !os.IsNotExist(err) {
			t.Fatalf("first package link still exists: %v", err)
		}
	}
}

func TestLegacyDirectoryLinkMigration(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "merge", true: "rollback"}[conflict], func(t *testing.T) {
			l, root := prefixFixture(t)
			writePrefixFile(t, filepath.Join(root, "Cellar/first/1/share/man/man1/first.1"), "first")
			writePrefixFile(t, filepath.Join(root, "Cellar/second/1/share/man/man1/aaa.1"), "second")
			if conflict {
				writePrefixFile(t, filepath.Join(root, "Cellar/second/1/share/man/man1/first.1"), "conflict")
			}
			if err := os.MkdirAll(filepath.Join(root, "share"), 0755); err != nil {
				t.Fatal(err)
			}
			legacy := "../Cellar/first/1/share/man"
			if err := os.Symlink(legacy, filepath.Join(root, "share/man")); err != nil {
				t.Fatal(err)
			}
			_, err := l.LinkToPrefix("second", "1")
			if conflict {
				if err == nil || !strings.Contains(err.Error(), "link conflict") {
					t.Fatalf("expected conflict, got %v", err)
				}
				if got, err := os.Readlink(filepath.Join(root, "share/man")); err != nil || got != legacy {
					t.Fatalf("legacy link not restored: %q, %v", got, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := l.LinkToPrefix("second", "1"); err != nil {
					t.Fatalf("relink: %v", err)
				}
				if err := l.UnlinkFromPrefix("first", []string{"share"}); err != nil {
					t.Fatal(err)
				}
				assertPrefixFile(t, filepath.Join(root, "share/man/man1/aaa.1"), "second")
			}
			assertPrefixFile(t, filepath.Join(root, "Cellar/first/1/share/man/man1/first.1"), "first")
			if _, err := os.Lstat(filepath.Join(root, "Cellar/first/1/share/man/man1/aaa.1")); !os.IsNotExist(err) {
				t.Fatalf("modified first package's keg: %v", err)
			}
		})
	}
}

func TestPrefixConflictPreservesExistingFiles(t *testing.T) {
	l, root := prefixFixture(t)
	writePrefixFile(t, filepath.Join(root, "bin/taken"), "user file")
	writePrefixFile(t, filepath.Join(root, "Cellar/pkg/1/bin/aaa"), "new")
	writePrefixFile(t, filepath.Join(root, "Cellar/pkg/1/bin/taken"), "replacement")
	if _, err := l.LinkToPrefix("pkg", "1"); err == nil || !strings.Contains(err.Error(), "link conflict") {
		t.Fatalf("expected conflict, got %v", err)
	}
	assertPrefixFile(t, filepath.Join(root, "bin/taken"), "user file")
	if _, err := os.Lstat(filepath.Join(root, "bin/aaa")); !os.IsNotExist(err) {
		t.Fatalf("partial link not rolled back: %v", err)
	}
}

func TestUnlinkRelativeAndForeignLinks(t *testing.T) {
	l, root := prefixFixture(t)
	writePrefixFile(t, filepath.Join(root, "Cellar/pkg/1/share/man/pkg.1"), "pkg")
	writePrefixFile(t, filepath.Join(root, "Cellar/pkg-other/1/share/man/other.1"), "other")
	if err := os.MkdirAll(filepath.Join(root, "share"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"man":   "../Cellar/pkg/1/share/man",
		"other": "../Cellar/pkg-other/1/share/man",
	} {
		if err := os.Symlink(target, filepath.Join(root, "share", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.UnlinkFromPrefix("pkg", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "share/man")); !os.IsNotExist(err) {
		t.Fatalf("legacy link remains: %v", err)
	}
	assertPrefixFile(t, filepath.Join(root, "share/other/other.1"), "other")
	assertPrefixFile(t, filepath.Join(root, "Cellar/pkg/1/share/man/pkg.1"), "pkg")
}

func TestPrefixRejectsForeignDirectoryLink(t *testing.T) {
	l, root := prefixFixture(t)
	writePrefixFile(t, filepath.Join(root, "Cellar/pkg/1/share/man/pkg.1"), "pkg")
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "share"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "share/man")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.LinkToPrefix("pkg", "1"); err == nil {
		t.Fatal("expected conflict with foreign directory link")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("foreign directory changed: %v, %v", entries, err)
	}
}
