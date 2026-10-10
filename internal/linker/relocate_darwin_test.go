package linker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A Mach-O whose bytes change loses its signature. On Apple Silicon the
// kernel kills it at launch unless relocation re-signs it.
func TestPatchedMachOStillRuns(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("no C compiler")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.c")
	os.WriteFile(src, []byte(`#include <stdio.h>
int main(void) { puts("`+BuildPrefix()+`/Cellar/foo/1.2.3/share/foo/data"); return 0; }
`), 0644)
	keg := filepath.Join(dir, "keg")
	bin := filepath.Join(keg, "libexec", "nested", "foo")
	os.MkdirAll(filepath.Dir(bin), 0755)
	if out, err := exec.Command("cc", "-o", bin, src).CombinedOutput(); err != nil {
		t.Skipf("cc failed: %v\n%s", err, out)
	}

	modified := patchBinaryStrings(keg, BuildPrefix(), "/opt/chatr")
	if len(modified) != 1 {
		t.Fatalf("modified = %v", modified)
	}
	resign(modified)

	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("patched binary failed to run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "/opt/chatr/opt/foo/share/foo/data" {
		t.Errorf("output = %q", got)
	}
}
