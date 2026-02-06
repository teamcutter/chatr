package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, _, _, _ := newManager()

			packages, err := mgr.List()
			if err != nil {
				return err
			}

			if len(packages) == 0 {
				fmt.Printf("\n%s No packages installed\n", dim("○"))
				return nil
			}

			fmt.Printf("Installed packages:\n\n")
			for _, v := range packages {
				fmt.Printf(" %s\n", bold(v))
			}

			return nil
		},
	}

	return cmd
}
