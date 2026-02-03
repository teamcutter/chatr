package cli

import (
	"fmt"

	"github.com/fatih/color"
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

			green := color.New(color.FgGreen).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			fmt.Printf("Removing %d package(s)...\n", len(args))

			var failed int
			for _, arg := range args {
				rmName, rmVersion, err := mgr.Remove(cmd.Context(), domain.Package{
					Name:    arg,
					Version: version,
				})

				if err != nil {
					fmt.Printf("\n%s %s: %v\n", red("✗"), arg, err)
					failed++
					continue
				}
				fmt.Printf("\n%s %s%s%s\n", green("✓"), bold(rmName), bold("@"), bold(rmVersion))
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
