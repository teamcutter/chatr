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
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, _ := newManager()

			fmt.Printf("Uninstalling %d package(s)...\n", len(args))
			for _, arg := range args {

				err := mgr.Uninstall(cmd.Context(), domain.Package{
					Name:    arg,
					Version: version,
				})

				if err != nil {
					return err
				}
				fmt.Printf("Successfully uninstalled %s@%s\n", arg, version)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	return cmd
}
