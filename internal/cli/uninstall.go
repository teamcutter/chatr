package cli

import (
	"fmt"

	"github.com/fatih/color"
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

			green := color.New(color.FgGreen).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			fmt.Printf("Uninstalling %d package(s)...\n", len(args))

			var failed int
			for _, arg := range args {
				err := mgr.Uninstall(cmd.Context(), domain.Package{
					Name:    arg,
					Version: version,
				})

				if err != nil {
					fmt.Printf("\n%s %s: %v\n", red("✗"), arg, err)
					failed++
					continue
				}
				fmt.Printf("\n%s %s@%s\n", green("✓"), bold(arg), bold(version))
			}

			if failed > 0 {
				return fmt.Errorf("failed to uninstall %d package(s)", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	return cmd
}
