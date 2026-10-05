package extractor

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// entry describes one archive member for the test archive builders.
type entry struct {
	name    string
	mode    fs.FileMode
	body    string
	dir     bool
	symlink string // non-empty: entry is a symlink to this target
}

func writeTarGz(t *testing.T, path string, entries []entry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: int64(e.mode)}
		switch {
		case e.dir:
			h.Typeflag = tar.TypeDir
		case e.symlink != "":
			h.Typeflag = tar.TypeSymlink
			h.Linkname = e.symlink
		default:
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path string, entries []entry) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		switch {
		case e.dir:
			if h.Name[len(h.Name)-1] != '/' {
				h.Name += "/"
			}
			h.SetMode(fs.ModeDir | e.mode)
		case e.symlink != "":
			h.SetMode(fs.ModeSymlink | e.mode)
		default:
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		body := e.body
		if e.symlink != "" {
			body = e.symlink
		}
		if !e.dir {
			if _, err := w.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

// extractFn runs one extractor implementation against an archive built from entries.
type extractFn func(t *testing.T, entries []entry, dst string) error

var implementations = map[string]extractFn{
	"tar-go": func(t *testing.T, entries []entry, dst string) error {
		src := filepath.Join(t.TempDir(), "a.tar.gz")
		writeTarGz(t, src, entries)
		return NewTAR().extractGo(src, dst)
	},
	"zip": func(t *testing.T, entries []entry, dst string) error {
		src := filepath.Join(t.TempDir(), "a.zip")
		writeZip(t, src, entries)
		return NewZIP().Extract(src, dst)
	},
}

func forEachImpl(t *testing.T, fn func(t *testing.T, extract extractFn)) {
	for name, impl := range implementations {
		t.Run(name, func(t *testing.T) { fn(t, impl) })
	}
}

func TestExtractNormalArchive(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		dst := t.TempDir()
		err := extract(t, []entry{
			{name: "pkg/", dir: true, mode: 0755},
			{name: "pkg/bin/tool", mode: 0755, body: "#!/bin/sh\n"},
			{name: "pkg/README", mode: 0644, body: "hi"},
			{name: "pkg/bin/alias", mode: 0777, symlink: "tool"},
			{name: "pkg/lib/abs", mode: 0777, symlink: "/usr/lib/libSystem.B.dylib"},
		}, dst)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}

		got, err := os.ReadFile(filepath.Join(dst, "pkg/README"))
		if err != nil || string(got) != "hi" {
			t.Fatalf("README = %q, %v", got, err)
		}

		info, err := os.Stat(filepath.Join(dst, "pkg/bin/tool"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("tool mode = %o, want 0755", info.Mode().Perm())
		}

		if target, err := os.Readlink(filepath.Join(dst, "pkg/bin/alias")); err != nil || target != "tool" {
			t.Errorf("alias -> %q, %v; want tool", target, err)
		}
		// Absolute symlink targets are preserved: bottles rely on them.
		if target, err := os.Readlink(filepath.Join(dst, "pkg/lib/abs")); err != nil || target != "/usr/lib/libSystem.B.dylib" {
			t.Errorf("abs -> %q, %v", target, err)
		}
	})
}

func TestExtractRejectsParentTraversal(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		outer := t.TempDir()
		dst := filepath.Join(outer, "dst")
		err := extract(t, []entry{
			{name: "../escaped", mode: 0644, body: "x"},
		}, dst)
		if err == nil {
			t.Fatal("expected error for ../ entry")
		}
		if _, statErr := os.Stat(filepath.Join(outer, "escaped")); statErr == nil {
			t.Fatal("file was written outside destination")
		}
	})
}

func TestExtractRejectsAbsolutePath(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		dst := t.TempDir()
		marker := filepath.Join(t.TempDir(), "marker")
		err := extract(t, []entry{
			{name: marker, mode: 0644, body: "x"},
		}, dst)
		if err == nil {
			t.Fatal("expected error for absolute entry")
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Fatal("file was written at absolute path")
		}
	})
}

func TestExtractRejectsWriteThroughEscapingSymlink(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		outside := t.TempDir()
		dst := t.TempDir()
		err := extract(t, []entry{
			{name: "link", mode: 0777, symlink: outside},
			{name: "link/pwned", mode: 0644, body: "x"},
		}, dst)
		if err == nil {
			t.Fatal("expected error when writing through a symlink that leaves dst")
		}
		if _, statErr := os.Stat(filepath.Join(outside, "pwned")); statErr == nil {
			t.Fatal("file was written through escaping symlink")
		}
	})
}

func TestExtractAllowsSymlinkInsideDestination(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		dst := t.TempDir()
		err := extract(t, []entry{
			{name: "real/", dir: true, mode: 0755},
			{name: "link", mode: 0777, symlink: "real"},
			{name: "link/file", mode: 0644, body: "ok"},
		}, dst)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(dst, "real/file")); err != nil || string(got) != "ok" {
			t.Fatalf("real/file = %q, %v", got, err)
		}
	})
}

func TestExtractStripsSpecialModeBits(t *testing.T) {
	forEachImpl(t, func(t *testing.T, extract extractFn) {
		dst := t.TempDir()
		err := extract(t, []entry{
			{name: "suid", mode: fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky | 0755, body: "x"},
		}, dst)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		info, err := os.Stat(filepath.Join(dst, "suid"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
			t.Errorf("special bits survived: %v", info.Mode())
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("perm = %o, want 0755", info.Mode().Perm())
		}
	})
}

func TestZipExtractAppsOnlyTopLevelBundles(t *testing.T) {
	dst := t.TempDir()
	src := filepath.Join(t.TempDir(), "cask.zip")
	writeZip(t, src, []entry{
		{name: "Foo.app/", dir: true, mode: 0755},
		{name: "Foo.app/Contents/MacOS/Foo", mode: 0755, body: "bin"},
		{name: "README.txt", mode: 0644, body: "skip me"},
		{name: "nested/Bar.app/Contents/Info.plist", mode: 0644, body: "skip me too"},
	})

	apps, err := NewZIP().ExtractApps(src, dst)
	if err != nil {
		t.Fatalf("ExtractApps: %v", err)
	}
	if !slices.Equal(apps, []string{"Foo.app"}) {
		t.Fatalf("apps = %v, want [Foo.app]", apps)
	}
	if _, err := os.Stat(filepath.Join(dst, "Foo.app/Contents/MacOS/Foo")); err != nil {
		t.Errorf("Foo.app not extracted: %v", err)
	}
	for _, skipped := range []string{"README.txt", "nested"} {
		if _, err := os.Stat(filepath.Join(dst, skipped)); err == nil {
			t.Errorf("%s should not have been extracted", skipped)
		}
	}
}

func TestCleanEntryName(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		ok      bool
		wantErr bool
	}{
		{"a/b", "a/b", true, false},
		{"./a/b", "a/b", true, false},
		{"a/./b/", "a/b", true, false},
		{"a/../b", "b", true, false},
		{"foo..bar", "foo..bar", true, false},
		{".", "", false, false},
		{"./", "", false, false},
		{"", "", false, false},
		{"..", "", false, true},
		{"../x", "", false, true},
		{"a/../../x", "", false, true},
		{"/etc/passwd", "", false, true},
	}
	for _, c := range cases {
		got, ok, err := cleanEntryName(c.in)
		if (err != nil) != c.wantErr || ok != c.ok || got != c.want {
			t.Errorf("cleanEntryName(%q) = (%q, %v, %v); want (%q, %v, err=%v)",
				c.in, got, ok, err, c.want, c.ok, c.wantErr)
		}
	}
}
