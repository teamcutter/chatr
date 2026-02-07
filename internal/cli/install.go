package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/resolver"
	"golang.org/x/sync/errgroup"
)

func newInstallCmd() *cobra.Command {
	var sha256 string
	var cask bool

	cmd := &cobra.Command{
		Use:   "install <name>...",
		Short: "Install packages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, _, res, err := newManagerWithOptions(cask)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			mu := &sync.Mutex{}
			var errs []error

			resolved := make([][]resolver.ResolvedPackage, len(args))

			rg, rctx := errgroup.WithContext(ctx)
			rg.SetLimit(min(len(args), cfg.MaxParallel))

			for i, name := range args {
				rg.Go(func() error {
					stop := withSpinner(rctx, fmt.Sprintf("Resolving %s...", name))
					pkgs, err := res.Resolve(rctx, name)
					stop()
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: %v", name, err))
						mu.Unlock()
						return nil
					}
					resolved[i] = pkgs
					return nil
				})
			}
			_ = rg.Wait()

			seen := make(map[string]bool)
			var plan []resolver.ResolvedPackage
			rootDeps := make(map[string][]string)

			for _, pkgs := range resolved {
				if len(pkgs) == 0 {
					continue
				}

				rootName := pkgs[len(pkgs)-1].Formula.Name

				for _, rp := range pkgs {
					name := rp.Formula.Name
					if rp.IsDep || rp.AlreadyInstalled {
						rootDeps[rootName] = append(rootDeps[rootName], name)
					}
					if !seen[name] {
						seen[name] = true
						plan = append(plan, rp)
					}
				}
			}

			output := make(map[string]string)
			outMu := &sync.Mutex{}

			ig, ictx := errgroup.WithContext(ctx)
			ig.SetLimit(cfg.MaxParallel)

			for _, rp := range plan {
				ig.Go(func() error {
					formula := rp.Formula

					if rp.AlreadyInstalled {
						outMu.Lock()
						output[formula.Name] = fmt.Sprintf("  %s %s %s",
							dim("↳"), formula.Name, dim("(already installed)"))
						outMu.Unlock()
						return nil
					}

					checksum := formula.SHA256
					if !rp.IsDep && sha256 != "" {
						checksum = sha256
					}

					pkg, err := mgr.Install(ictx, domain.Package{
						Name:        formula.Name,
						Version:     formula.Version,
						Revision:    formula.Revision,
						DownloadURL: formula.URL,
						SHA256:      checksum,
						IsDep:       rp.IsDep,
						IsCask:      formula.IsCask,
					})
					if err != nil {
						outMu.Lock()
						if strings.Contains(err.Error(), "already installed") {
							output[formula.Name] = fmt.Sprintf("%s %s already installed", yellow("!"), bold(formula.Name))
						} else if rp.IsDep {
							output[formula.Name] = fmt.Sprintf("  %s %s: %v %s",
								dim("↳"), formula.Name, err, dim("(skipped)"))
						} else {
							mu.Lock()
							errs = append(errs, fmt.Errorf("%s: %v", formula.Name, err))
							mu.Unlock()
						}
						outMu.Unlock()
						return nil
					}

					outMu.Lock()
					if rp.IsDep {
						output[formula.Name] = fmt.Sprintf("  %s %s%s%s %s",
							dim("↳"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()), dim("(dependency)"))
					} else if pkg.IsCask {
						lines := fmt.Sprintf("%s %s%s%s %s",
							green("✓"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()), dim("(cask)"))
						for _, app := range pkg.Apps {
							lines += fmt.Sprintf("\n  %s %s", cyan("app:"), filepath.Join(cfg.AppsDir, app))
						}
						output[formula.Name] = lines
					} else {
						output[formula.Name] = fmt.Sprintf("%s %s%s%s\n  %s %s\n  %s %s",
							green("✓"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()),
							cyan("cache:"), filepath.Join(cfg.CacheDir, pkg.Name, pkg.FullVersion()),
							cyan("path:"), filepath.Join(cfg.PackagesDir, pkg.Name, pkg.FullVersion()))
					}
					outMu.Unlock()
					return nil
				})
			}
			_ = ig.Wait()

			for root, deps := range rootDeps {
				if len(deps) > 0 {
					mgr.SetDependencies(root, deps)
				}
			}

			fmt.Println()
			for _, rp := range plan {
				if msg, ok := output[rp.Formula.Name]; ok {
					fmt.Println(msg)
				}
			}

			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Printf("%s %s\n", red("✗"), e)
				}
				return fmt.Errorf("failed to install %d package(s)\n", len(errs))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
	cmd.Flags().BoolVar(&cask, "cask", false, "Install a cask (macOS application)")
	return cmd
}
