package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
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

			if err != nil {
				fmt.Printf("%s Failed to update chatr: %v\n", red("✗"), err)
				return fmt.Errorf("failed to update chatr")
			}

			fmt.Printf("%s chatr updated\n", green("✓"))
			return nil
		},
	}
}
