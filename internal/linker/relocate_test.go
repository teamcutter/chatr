package linker

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRewritePath(t *testing.T) {
	const build, chatr, opt = "/opt/homebrew", "/opt/chatr", "/opt/chatr/opt"
	cases := map[string]string{
		"/opt/homebrew/Cellar/python@3.13/3.13.1/Frameworks/Python": "/opt/chatr/opt/python@3.13/Frameworks/Python",
		"/opt/homebrew/Cellar/foo/1.0":                              "/opt/chatr/opt/foo",
		"/opt/homebrew/lib/libz.dylib":                              "/opt/chatr/lib/libz.dylib",
		"/opt/homebrew/share:/opt/homebrew/etc":                     "/opt/chatr/share:/opt/chatr/etc",
		"/usr/local/lib":                                            "/usr/local/lib",
	}
	for in, want := range cases {
		if got := rewritePath(in, build, chatr, opt); got != want {
			t.Errorf("rewritePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPatchBinaryStringsPadsAndSkipsOversized(t *testing.T) {
	dir := t.TempDir()
	content := []byte("\x00\x00HEAD\x00/opt/homebrew/lib/libz.dylib\x00/usr/local/keep\x00/opt/homebrew/x\x00TAIL")
	bin := filepath.Join(dir, "bin", "tool")
	os.MkdirAll(filepath.Dir(bin), 0755)
	os.WriteFile(bin, content, 0755)

	modified := patchBinaryStrings(dir, "/opt/homebrew", "/opt/chatr")
	if len(modified) != 1 || modified[0] != bin {
		t.Fatalf("modified = %v", modified)
	}
	got, _ := os.ReadFile(bin)
	if len(got) != len(content) {
		t.Fatalf("file size changed: %d -> %d", len(content), len(got))
	}
	for _, want := range []string{"/opt/chatr/lib/libz.dylib\x00\x00\x00\x00", "/usr/local/keep\x00", "/opt/chatr/x\x00", "TAIL"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	long := "/a/much/longer/prefix/than/homebrew"
	os.WriteFile(bin, content, 0755)
	if modified := patchBinaryStrings(dir, "/opt/homebrew", long); len(modified) != 0 {
		t.Errorf("strings that cannot fit must be left alone, modified %v", modified)
	}
}

func TestRelocateTextPlaceholdersAcrossKeg(t *testing.T) {
	keg := t.TempDir()
	files := map[string]string{
		"libexec/git-core/git-web--browse": "#!@@HOMEBREW_PERL@@\nprefix=@@HOMEBREW_PREFIX@@ cellar=@@HOMEBREW_CELLAR@@\n",
		"bin/tool":                         "JAVA_HOME=@@HOMEBREW_JAVA@@\nlib=@@HOMEBREW_LIBRARY@@ repo=@@HOMEBREW_REPOSITORY@@\n",
		"sbin/daemon":                      "conf=" + BuildPrefix() + "/etc/daemon.conf\nkeg=" + BuildPrefix() + "/Cellar/daemon/1.0\n",
	}
	for rel, body := range files {
		p := filepath.Join(keg, rel)
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(body), 0755)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("@@HOMEBREW_PREFIX@@"), 0644)
	os.Symlink(outside, filepath.Join(keg, "bin", "link"))

	l := New(filepath.Join(keg, "Cellar"), filepath.Join(keg, "opt"), map[string]string{"lib": filepath.Join(keg, "lib")})
	l.Relocate(keg, "/opt/chatr", RelocateOptions{Name: "git", Dependencies: []string{"pcre2", "openjdk@21"}})

	read := func(rel string) string { b, _ := os.ReadFile(filepath.Join(keg, rel)); return string(b) }

	if got := read("libexec/git-core/git-web--browse"); got != "#!/usr/bin/perl\nprefix=/opt/chatr cellar=/opt/chatr/Cellar\n" {
		t.Errorf("libexec script: %q", got)
	}
	java := javaHome("/opt/chatr", []string{"openjdk@21"}, runtime.GOOS)
	if got := read("bin/tool"); got != "JAVA_HOME="+java+"\nlib=/opt/chatr/Library repo=/opt/chatr\n" {
		t.Errorf("bin/tool: %q", got)
	}
	if got := read("sbin/daemon"); got != "conf=/opt/chatr/etc/daemon.conf\nkeg=/opt/chatr/Cellar/daemon/1.0\n" {
		t.Errorf("sbin script: %q", got)
	}
	if b, _ := os.ReadFile(outside); string(b) != "@@HOMEBREW_PREFIX@@" {
		t.Errorf("relocation followed a symlink out of the keg: %q", b)
	}
}

func TestPlaceholderTargets(t *testing.T) {
	if got := perlPath("/p", "perl", nil); got != "/p/opt/perl/bin/perl" {
		t.Errorf("perl formula: %s", got)
	}
	if got := perlPath("/p", "x", []string{"perl"}); got != "/p/opt/perl/bin/perl" {
		t.Errorf("perl dependency: %s", got)
	}
	if got := perlPath("/p", "x", nil); got != "/usr/bin/perl" {
		t.Errorf("system perl: %s", got)
	}
	if got := javaHome("/p", []string{"openjdk@17"}, "darwin"); got != "/p/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home" {
		t.Errorf("darwin java: %s", got)
	}
	if got := javaHome("/p", nil, "linux"); got != "/p/opt/openjdk/libexec" {
		t.Errorf("linux java: %s", got)
	}
}

func TestCanRelocateBinaries(t *testing.T) {
	if !CanRelocateBinaries("/opt/chatr") {
		t.Error("/opt/chatr must fit every Homebrew build prefix")
	}
	if CanRelocateBinaries("/Users/someone/.chatr") {
		t.Error("a home-directory prefix is longer than the build prefix")
	}
}
