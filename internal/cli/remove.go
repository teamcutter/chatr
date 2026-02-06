package cli

import (
	"fmt"
	"strings"

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
				installed, err := mgr.List()
				if err != nil {
					return err
				}
				if len(installed) == 0 {
					fmt.Printf("%s No packages installed\n", dim("○"))
					return nil
				}
				packages = make([]string, 0, len(installed))
				for _, pkg := range installed {
					name := pkg[:strings.LastIndex(pkg, "-")]
					packages = append(packages, name)
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
