package cli

import (
	"fmt"
	"strings"

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

			for _, arg := range args {

				fmt.Println(strings.Repeat("=", 50))
				fmt.Printf("Uninstalling %s...\n", arg)
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
