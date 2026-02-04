package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Update chatr to the newest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("sh", "-c", "curl -sL https://raw.githubusercontent.com/teamcutter/chatr/main/install.sh | sh")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}
