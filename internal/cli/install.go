package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/sync/errgroup"
)

func newInstallCmd() *cobra.Command {
	var version, sha256 string

	cmd := &cobra.Command{
		Use:   "install <name>...",
		Short: "Install packages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, _, res, err := newManager()
			if err != nil {
				return err
			}

			// Btw can use workers pool but
			// it doesn't seem to be needed here
			g, ctx := errgroup.WithContext(cmd.Context())
			g.SetLimit(min(len(args), cfg.MaxParallel))

			mu := &sync.Mutex{}
			var errs []error
			var success []string

			for _, name := range args {
				g.Go(func() error {

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
					var rootName string

					for _, rp := range resolved {
						formula := rp.Formula

						installVersion := formula.Version
						installRevision := formula.Revision
						if !rp.IsDep && version != "latest" {
							installVersion = version
							installRevision = ""
						}

						checksum := formula.SHA256
						if !rp.IsDep && sha256 != "" {
							checksum = sha256
						}

						pkg, err := mgr.Install(ctx, domain.Package{
							Name:        formula.Name,
							Version:     installVersion,
							Revision:    installRevision,
							DownloadURL: formula.URL,
							SHA256:      checksum,
							IsDep:       rp.IsDep,
						})
						if err != nil {
							mu.Lock()
							if strings.Contains(err.Error(), "already installed") {
								success = append(success, fmt.Sprintf("%s %s already installed", yellow("!"), bold(formula.Name)))
							} else if rp.IsDep {
								success = append(success, fmt.Sprintf("  %s %s: %v %s",
									dim("↳"), formula.Name, err, dim("(skipped)")))
							} else {
								errs = append(errs, fmt.Errorf("%s: %v", formula.Name, err))
							}
							mu.Unlock()
							if !rp.IsDep {
								return nil
							}
							continue
						}

						mu.Lock()
						if rp.IsDep {
							depNames = append(depNames, formula.Name)
							success = append(success, fmt.Sprintf("  %s %s%s%s %s",
								dim("↳"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()), dim("(dependency)")))
						} else {
							rootName = pkg.Name
							success = append(success, fmt.Sprintf("%s %s%s%s\n  %s %s\n  %s %s",
								green("✓"), bold(pkg.Name), bold("-"), bold(pkg.FullVersion()),
								cyan("cache:"), filepath.Join(cfg.CacheDir, pkg.Name, pkg.FullVersion()),
								cyan("path:"), filepath.Join(cfg.PackagesDir, pkg.Name, pkg.FullVersion())))
						}
						mu.Unlock()
					}

					if rootName != "" && len(depNames) > 0 {
						mgr.SetDependencies(rootName, depNames)
					}

					return nil
				})
			}

			_ = g.Wait()

			fmt.Println()
			for _, s := range success {
				fmt.Printf("%s\n", s)
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

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
	return cmd
}
