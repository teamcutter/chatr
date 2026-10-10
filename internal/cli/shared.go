package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/teamcutter/chatr/internal/config"
	"github.com/teamcutter/chatr/internal/linker"
	"github.com/teamcutter/chatr/internal/ui"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
	dim    = color.New(color.Faint).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

var progress = ui.New(os.Stderr)

func withSpinner(_ context.Context, desc string) (stop func()) {
	return progress.Status(desc)
}

// warnLongPrefix flags installs whose prefix is too long for paths inside
// bottle binaries to be rewritten.
func warnLongPrefix(cfg *config.Config) {
	if linker.CanRelocateBinaries(cfg.Prefix) {
		return
	}
	fmt.Fprintf(progress, "%s prefix %s is longer than %s, so some bottles will not relocate correctly.\n  Move to %s: see \"Moving to /opt/chatr\" in the README.\n",
		yellow("warning:"), cfg.Prefix, linker.BuildPrefix(), config.DefaultPrefix)
}
