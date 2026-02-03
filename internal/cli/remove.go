package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newRemoveCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:  "remove <name>",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, _, _ := newManager()

			var failed int
			for _, arg := range args {
				rmName, rmVersion, err := mgr.Remove(cmd.Context(), domain.Package{
					Name:    arg,
					Version: version,
				})
				if err != nil {
					fmt.Printf("%s %s: %v\n", red("✗"), arg, err)
					failed++
					continue
				}
				fmt.Printf("%s %s%s%s removed\n", green("✓"), bold(rmName), bold("@"), bold(rmVersion))
			}

			if failed > 0 {
				return fmt.Errorf("failed to remove %d package(s)", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	return cmd
}
