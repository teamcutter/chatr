package cli

import (
	"context"
	"os"

	"github.com/fatih/color"
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
	return progress.Spin(desc)
}
