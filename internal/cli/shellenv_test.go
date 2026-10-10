package cli

import (
	"os/exec"
	"strings"
	"testing"
)

var testEnv = shellEnv{
	prefix:  "/opt/chatr",
	binDirs: []string{"/opt/chatr/bin", "/home/o'neil/my apps"},
	manDir:  "/opt/chatr/share/man",
	infoDir: "/opt/chatr/share/info",
	zshFunc: "/opt/chatr/share/zsh/site-functions",
}

func TestShellenvInRealShells(t *testing.T) {
	const want = "/opt/chatr/bin:/home/o'neil/my apps:/usr/bin:/bin|/opt/chatr/share/man:|/opt/chatr/share/info:|/opt/chatr"
	for _, shell := range []string{"sh", "bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Skip(shell + " not installed")
			}
			env := testEnv.render(shell)
			script := "set -u; " + env + env + `printf '%s|%s|%s|%s' "$PATH" "$MANPATH" "$INFOPATH" "$CHATR_PREFIX"`
			args := []string{"-c", script}
			if shell == "zsh" {
				args = []string{"-fc", script}
			}
			cmd := exec.Command(shell, args...)
			cmd.Env = []string{"PATH=/usr/bin:/bin"}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s\nscript:\n%s", err, out, env)
			}
			if string(out) != want {
				t.Errorf("got  %q\nwant %q", out, want)
			}
		})
	}
}

func TestShellenvZshAddsCompletionsOnce(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	env := testEnv.render("zsh")
	out, err := exec.Command("zsh", "-fc", "fpath=(/x); "+env+env+`print -r -- "${(j.:.)fpath}"`).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "/opt/chatr/share/zsh/site-functions:/x" {
		t.Errorf("fpath = %q", got)
	}
}

func TestShellenvFish(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish not installed")
	}
	env := testEnv.render("fish")
	cmd := exec.Command("fish", "--no-config", "-c", env+env+`printf '%s|%s|%s' (string join : $PATH) (string join : $MANPATH) $CHATR_PREFIX`)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if want := "/opt/chatr/bin:/home/o'neil/my apps:/usr/bin:/bin|/opt/chatr/share/man:|/opt/chatr"; string(out) != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}

func TestShellenvPOSIXOutputHasNoZshSyntax(t *testing.T) {
	if strings.Contains(testEnv.render("bash"), "fpath") {
		t.Error("bash output must not touch zsh fpath")
	}
}
