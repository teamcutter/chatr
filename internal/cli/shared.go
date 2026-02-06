package cli

import (
	"context"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
	dim    = color.New(color.Faint).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

func withSpinner(ctx context.Context, desc string) (stop func()) {
	spinner := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionClearOnFinish(),
	)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				spinner.Finish()
				return
			default:
				spinner.Add(1)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return func() {
		close(done)
		spinner.Finish()
	}
}
