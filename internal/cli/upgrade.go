package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/resolver"
	"golang.org/x/sync/errgroup"
)

func newUpgradeCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "upgrade [name...]",
		Short: "Upgrade installed packages to latest version",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				return nil
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, _, formulaRes, err := newManagerWithOptions(false)
			if err != nil {
				return err
			}
			_, _, _, caskRes, err := newManagerWithOptions(true)
			if err != nil {
				return err
			}

			mgr.Reconcile()

			installed, err := mgr.ListInstalled()
			if err != nil {
				return err
			}

			if len(installed) == 0 {
				fmt.Printf("%s No packages installed\n", dim("○"))
				return nil
			}

			names := args
			if all {
				for name, pkg := range installed {
					if pkg.IsDep {
						continue
					}
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

					var res *resolver.Resolver
					if installedPkg.IsCask {
						res = caskRes
					} else {
						res = formulaRes
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
						pkg, err := mgr.Install(ctx, domain.Package{
							Name:        rp.Formula.Name,
							Version:     rp.Formula.Version,
							Revision:    rp.Formula.Revision,
							FullVersion: rp.Formula.FullVersion(),
							DownloadURL: rp.Formula.URL,
							SHA256:      rp.Formula.SHA256,
							IsDep:       true,
						})
						if err != nil {
							mu.Lock()
							upgraded = append(upgraded, fmt.Sprintf("  %s %s: %v %s",
								dim("↳"), rp.Formula.Name, err, dim("(skipped)")))
							mu.Unlock()
							continue
						}
						depNames = append(depNames, rp.Formula.Name)
						mu.Lock()
						upgraded = append(upgraded, fmt.Sprintf("  %s %s%s%s %s",
							dim("↳"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()), dim("(dependency)")))
						mu.Unlock()
					}

					rootFormula := &resolved[len(resolved)-1].Formula

					if installedPkg.FullVersion() == rootFormula.FullVersion() {
						mu.Lock()
						upToDate = append(upToDate, name)
						mu.Unlock()
						return nil
					}

					oldVersion := installedPkg.FullVersion()

					pkg, err := mgr.Upgrade(ctx, domain.Package{
						Name:        name,
						Version:     installedPkg.Version,
						FullVersion: installedPkg.FullVersion(),
						IsCask:      installedPkg.IsCask,
					}, domain.Package{
						Name:        rootFormula.Name,
						Version:     rootFormula.Version,
						Revision:    rootFormula.Revision,
						FullVersion: rootFormula.FullVersion(),
						DownloadURL: rootFormula.URL,
						SHA256:      rootFormula.SHA256,
						IsCask:      rootFormula.IsCask,
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

	cmd.Flags().BoolVar(&all, "all", false, "Upgrade all installed packages")
	return cmd
}
