package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/config"
)

type shellEnv struct {
	prefix  string
	binDirs []string
	manDir  string
	infoDir string
	zshFunc string
}

func newShellenvCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "shellenv [sh|bash|zsh|fish]",
		Short:     "Print shell commands that add chatr to PATH, MANPATH and INFOPATH",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"sh", "bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			shell := filepath.Base(os.Getenv("SHELL"))
			if len(args) == 1 {
				shell = args[0]
			}

			env := shellEnv{
				prefix:  cfg.Prefix,
				binDirs: []string{cfg.BinDir},
				manDir:  filepath.Join(cfg.ShareDir, "man"),
				infoDir: filepath.Join(cfg.ShareDir, "info"),
				zshFunc: filepath.Join(cfg.ShareDir, "zsh", "site-functions"),
			}
			if exe, err := os.Executable(); err == nil {
				if exe, err = filepath.EvalSymlinks(exe); err == nil && !slices.Contains(env.binDirs, filepath.Dir(exe)) {
					env.binDirs = append(env.binDirs, filepath.Dir(exe))
				}
			}

			fmt.Fprint(cmd.OutOrStdout(), env.render(shell))
			return nil
		},
	}
}

func (e shellEnv) render(shell string) string {
	var b strings.Builder
	if shell == "fish" {
		q := fishQuote
		fmt.Fprintf(&b, "set -gx CHATR_PREFIX %s;\n", q(e.prefix))
		fmt.Fprintf(&b, "fish_add_path -gP")
		for _, d := range e.binDirs {
			fmt.Fprintf(&b, " %s", q(d))
		}
		b.WriteString(";\n")
		fmt.Fprintf(&b, "set -q MANPATH; or set -gx MANPATH '';\n")
		fmt.Fprintf(&b, "contains %s $MANPATH; or set -gx MANPATH %s $MANPATH;\n", q(e.manDir), q(e.manDir))
		fmt.Fprintf(&b, "contains %s $INFOPATH; or set -gx INFOPATH %s $INFOPATH;\n", q(e.infoDir), q(e.infoDir))
		return b.String()
	}

	q := shQuote
	fmt.Fprintf(&b, "export CHATR_PREFIX=%s;\n", q(e.prefix))
	for _, d := range slices.Backward(e.binDirs) {
		fmt.Fprintf(&b, "case \":${PATH}:\" in *:%s:*) ;; *) export PATH=%s\":${PATH}\" ;; esac;\n", q(d), q(d))
	}
	fmt.Fprintf(&b, "case \":${MANPATH-}:\" in *:%s:*) ;; *) export MANPATH=%s\":${MANPATH-}\" ;; esac;\n", q(e.manDir), q(e.manDir))
	fmt.Fprintf(&b, "case \":${INFOPATH-}:\" in *:%s:*) ;; *) export INFOPATH=%s\":${INFOPATH-}\" ;; esac;\n", q(e.infoDir), q(e.infoDir))
	if shell == "zsh" {
		fmt.Fprintf(&b, "() { (( ${fpath[(Ie)$1]} )) || fpath=(\"$1\" $fpath) } %s;\n", q(e.zshFunc))
	}
	return b.String()
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func fishQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(s) + "'"
}
