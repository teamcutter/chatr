package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newRemoveCmd() *cobra.Command {
	var version string
	var all bool

	cmd := &cobra.Command{
		Use:   "remove [name...]",
		Short: "Remove installed packages",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				return nil
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, _, _, _ := newManager()

			packages := args
			if all {
				installed, err := mgr.ListInstalled()
				if err != nil {
					return err
				}
				packages = make([]string, 0, len(installed))
				for name, pkg := range installed {
					if pkg.IsDep {
						continue
					}
					packages = append(packages, name)
				}
				if len(packages) == 0 {
					fmt.Printf("%s No packages installed\n", dim("○"))
					return nil
				}
			}

			fmt.Println()
			var failed int
			for _, arg := range packages {
				removedPackage, err := mgr.Remove(cmd.Context(), domain.Package{
					Name:    arg,
					Version: version,
				})
				if err != nil {
					fmt.Printf("%s %s: %v\n", red("✗"), arg, err)
					failed++
					continue
				}
				fmt.Printf("%s %s%s%s removed (with %s dependencies)\n", green("✓"), bold(removedPackage.Name), bold("-"), bold(removedPackage.FullVersion()), green(len(removedPackage.Dependencies)))
			}

			if err := mgr.Flush(); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			if failed > 0 {
				return fmt.Errorf("failed to remove %d package(s)", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	cmd.Flags().BoolVar(&all, "all", false, "Remove all installed packages")
	return cmd
}
