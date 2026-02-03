package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, _, _ := newManager()

			bold := color.New(color.Bold).SprintFunc()
			dim := color.New(color.Faint).SprintFunc()

			packages, err := mgr.List()
			if err != nil {
				return err
			}

			if len(packages) == 0 {
				fmt.Printf("%s No packages installed\n", dim("○"))
				return nil
			}

			fmt.Printf("Installed packages:\n\n")
			for _, v := range packages {
				fmt.Printf("  %s\n", bold(v))
			}

			return nil
		},
	}

	return cmd
}
