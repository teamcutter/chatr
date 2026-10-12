package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Update chatr to the newest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("bash", "-c", "curl -sL https://raw.githubusercontent.com/teamcutter/chatr/main/install.sh | bash")
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr

			if err := c.Run(); err != nil {
				fmt.Printf("%s Failed to update chatr: %v\n", red("✗"), err)
				return fmt.Errorf("failed to update chatr")
			}

			fmt.Printf("%s chatr updated\n", green("✓"))
			return nil
		},
	}
}
