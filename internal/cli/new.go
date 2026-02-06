package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/version"
)

func newNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Update chatr to the newest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			stop := withSpinner(cmd.Context(), "Updating chatr...")
			c := exec.Command("sh", "-c", "curl -sL https://raw.githubusercontent.com/teamcutter/chatr/main/install.sh | sh")
			err := c.Run()
			stop()

			fmt.Println()
			if err != nil {
				fmt.Printf("%s failed to update chatr\n", red("✗"))
				return err
			}

			fmt.Printf("%s chatr updated to version %s%s%s%s%s\n", green("✓"), bold(version.Version), bold("-"),
				bold(runtime.GOOS), bold("/"), bold(runtime.GOARCH))
			return nil
		},
	}
}
