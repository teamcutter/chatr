package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newUninstallCmd() *cobra.Command {
    var version string

    cmd := &cobra.Command{
        Use:  "uninstall <name>",
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr, _, _ := newManager()

			fmt.Printf("Uninstalling %s...\n", args[0])
            err := mgr.Uninstall(cmd.Context(), domain.Package{
                Name:    args[0],
                Version: version,
            })
			if err != nil {
				return err
			}
			fmt.Printf("Successfully uninstalled %s@%s\n", args[0], version)
			return nil
        },
    }

    cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
    return cmd
}