package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/sync/errgroup"
)

func newInstallCmd() *cobra.Command {
	var version, sha256 string

	cmd := &cobra.Command{
		Use:  "install <name>...",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, reg, err := newManager()
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

					stop := withSpinner(cmd.Context(), fmt.Sprintf("Fetching %s...", name))
					formula, err := reg.Get(ctx, name)
					stop()
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: %v", name, err))
						mu.Unlock()
						return nil
					}

					installVersion := formula.Version
					if version != "latest" {
						installVersion = version
					}

					pkg, err := mgr.Install(ctx, domain.Package{
						Name:        formula.Name,
						DownloadURL: formula.URL,
						Version:     installVersion,
						SHA256:      formula.SHA256,
					})
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Errorf("%s: %v", name, err))
						mu.Unlock()
						return nil
					}

					mu.Lock()
					success = append(success, fmt.Sprintf("%s %s%s%s\n  %s %s\n  %s %s",
						green("✓"), bold(pkg.Name), bold("@"), bold(pkg.Version),
						cyan("cache:"), filepath.Join(cfg.CacheDir, pkg.Name, pkg.Version),
						cyan("path:"), filepath.Join(cfg.PackagesDir, pkg.Name, pkg.Version)))
					mu.Unlock()

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
