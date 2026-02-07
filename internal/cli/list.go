package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/sync/errgroup"
)

func newListCmd() *cobra.Command {
	var cask bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, reg, _, err := newManagerWithOptions(cask)
			if err != nil {
				return err
			}

			if removed := mgr.Reconcile(); len(removed) > 0 {
				for _, name := range removed {
					fmt.Printf("%s %s removed externally\n", dim("○"), name)
				}
				fmt.Println()
			}

			installed, err := mgr.ListInstalled()
			if err != nil {
				return err
			}

			var packages []*domain.InstalledPackage
			for _, pkg := range installed {
				if pkg.IsDep {
					continue
				}
				if cask && !pkg.IsCask {
					continue
				}
				if !cask && pkg.IsCask {
					continue
				}
				packages = append(packages, pkg)
			}

			if len(packages) == 0 {
				label := "packages"
				if cask {
					label = "casks"
				}
				fmt.Printf("\n%s No %s installed\n", dim("○"), label)
				return nil
			}

			ctx := cmd.Context()
			latest := make(map[string]string)
			mu := &sync.Mutex{}

			g, gctx := errgroup.WithContext(ctx)
			g.SetLimit(cfg.MaxParallel)

			for _, pkg := range packages {
				g.Go(func() error {
					ver, err := reg.GetVersion(gctx, pkg.Name)
					if err != nil {
						return nil
					}
					mu.Lock()
					latest[pkg.Name] = ver
					mu.Unlock()
					return nil
				})
			}
			_ = g.Wait()

			label := "Installed packages:"
			if cask {
				label = "Installed casks:"
			}
			fmt.Printf("%s\n\n", label)

			for _, pkg := range packages {
				line := fmt.Sprintf(" %s", bold(fmt.Sprintf("%s-%s", pkg.Name, pkg.FullVersion())))
				if ver, ok := latest[pkg.Name]; ok && ver != pkg.Version {
					line += fmt.Sprintf("  %s", yellow(fmt.Sprintf("↑ %s", ver)))
				}
				fmt.Println(line)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cask, "cask", false, "List installed casks")
	return cmd
}
