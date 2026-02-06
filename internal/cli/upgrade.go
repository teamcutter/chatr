package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/sync/errgroup"
)

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade [name...]",
		Short: "Upgrade installed packages to latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, _, res, err := newManager()
			if err != nil {
				return err
			}

			installed, err := mgr.ListInstalled()
			if err != nil {
				return err
			}

			if len(installed) == 0 {
				fmt.Printf("\n%s No packages installed\n", dim("○"))
				return nil
			}

			names := args
			if len(names) == 0 {
				for name := range installed {
					names = append(names, name)
				}
			}

			g, ctx := errgroup.WithContext(cmd.Context())
			g.SetLimit(min(len(names), cfg.MaxParallel))

			mu := &sync.Mutex{}
			var errs []error
			var upgraded []string
			var upToDate []string

			for _, name := range names {
				g.Go(func() error {
					installedPkg, ok := installed[name]
					if !ok {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: not installed", name))
						mu.Unlock()
						return nil
					}

					stop := withSpinner(ctx, fmt.Sprintf("Resolving %s...", name))
					resolved, err := res.Resolve(ctx, name)
					stop()
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: %v", name, err))
						mu.Unlock()
						return nil
					}

					var depNames []string

					for _, rp := range resolved {
						if !rp.IsDep {
							continue
						}
						formula := rp.Formula
						pkg, err := mgr.Install(ctx, domain.Package{
							Name:        formula.Name,
							Version:     formula.Version,
							Revision:    formula.Revision,
							DownloadURL: formula.URL,
							SHA256:      formula.SHA256,
							IsDep:       true,
						})
						if err != nil {
							mu.Lock()
							upgraded = append(upgraded, fmt.Sprintf("  %s %s: %v %s",
								dim("↳"), formula.Name, err, dim("(skipped)")))
							mu.Unlock()
							continue
						}
						depNames = append(depNames, formula.Name)
						mu.Lock()
						upgraded = append(upgraded, fmt.Sprintf("  %s %s%s%s %s",
							dim("↳"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()), dim("(dependency)")))
						mu.Unlock()
					}

					var rootFormula *domain.Formula
					for _, rp := range resolved {
						if !rp.IsDep {
							rootFormula = &rp.Formula
							break
						}
					}

					if rootFormula == nil || installedPkg.FullVersion() == rootFormula.FullVersion() {
						mu.Lock()
						upToDate = append(upToDate, name)
						mu.Unlock()
						return nil
					}

					oldVersion := installedPkg.FullVersion()

					pkg, err := mgr.Upgrade(ctx, domain.Package{
						Name:    name,
						Version: installedPkg.Version,
					}, domain.Package{
						Name:        rootFormula.Name,
						Version:     rootFormula.Version,
						Revision:    rootFormula.Revision,
						DownloadURL: rootFormula.URL,
						SHA256:      rootFormula.SHA256,
					})
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: %v", name, err))
						mu.Unlock()
						return nil
					}

					if len(depNames) > 0 {
						mgr.SetDependencies(pkg.Name, depNames)
					}

					mu.Lock()
					upgraded = append(upgraded, fmt.Sprintf("%s %s%s%s → %s\n  %s %s\n  %s %s",
						green("✓"), bold(pkg.Name), bold("-"), bold(oldVersion), bold(pkg.FullVersion()),
						cyan("cache:"), filepath.Join(cfg.CacheDir, pkg.Name, pkg.FullVersion()),
						cyan("path:"), filepath.Join(cfg.PackagesDir, pkg.Name, pkg.FullVersion())))
					mu.Unlock()

					return nil
				})
			}

			_ = g.Wait()

			fmt.Println()
			for _, s := range upgraded {
				fmt.Printf("%s\n", s)
			}
			for _, name := range upToDate {
				fmt.Printf("%s %s already up-to-date\n", dim("○"), name)
			}

			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Printf("%s %s\n", red("✗"), e)
				}
				return fmt.Errorf("failed to upgrade %d package(s)", len(errs))
			}

			return nil
		},
	}

	return cmd
}
