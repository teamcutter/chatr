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

			fmt.Println()
			if err != nil {
				return fmt.Errorf("%s failed to update chatr: %w", red("✗"), err)
			}

			fmt.Printf("%s chatr updated successfully!\n", green("✓"))
			return nil
		},
	}
}
