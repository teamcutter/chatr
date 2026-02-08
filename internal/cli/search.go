package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var show int
	var cask bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, reg, _, err := newManagerWithOptions(cask)
			if err != nil {
				return err
			}

			stop := withSpinner(cmd.Context(), fmt.Sprintf("Searching for %s...", args[0]))
			results, err := reg.Search(cmd.Context(), args[0])
			stop()
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Printf("%s No results found for %q\n", dim("○"), args[0])
				return nil
			}

			size := min(len(results), show)

			fmt.Printf("Showing %s of %s results for %q\n\n", green(size), green(len(results)), args[0])

			for i := range size {
				fmt.Printf("%s %s\n", green("●"), bold(results[i].Name))
				fmt.Printf("  %s %s\n", cyan("version:"), results[i].FullVersion())
				if results[i].Description != "" {
					fmt.Printf("  %s %s\n", cyan("desc:"), results[i].Description)
				}
				if results[i].Homepage != "" {
					fmt.Printf("  %s %s\n", cyan("url:"), dim(results[i].Homepage))
				}
				fmt.Println()
			}

			if len(results) > size {
				fmt.Printf("%s %d more available, use %s to see all\n", dim("..."), len(results)-size, cyan(fmt.Sprintf("--show %d", len(results))))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&show, "show", "s", 5, "Shows first n packages")
	cmd.Flags().BoolVar(&cask, "cask", false, "Search casks (macOS applications)")
	return cmd
}
