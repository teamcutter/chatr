package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
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

			wg := &sync.WaitGroup{}
			mu := &sync.Mutex{}
			errCh := make(chan error, len(args))

			green := color.New(color.FgGreen).SprintFunc()
			cyan := color.New(color.FgCyan).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			for _, name := range args {

				wg.Add(1)
				go func(name string) {
					defer wg.Done()

					formula, err := reg.Get(cmd.Context(), name)
					if err != nil {
						errCh <- fmt.Errorf("%s: %v", name, err)
						return
					}

					var installVersion string
					if version == "latest" {
						installVersion = formula.Version
					} else {
						installVersion = version
					}

					pkg, err := mgr.Install(cmd.Context(), domain.Package{
						Name:        formula.Name,
						DownloadURL: formula.URL,
						Version:     installVersion,
						SHA256:      formula.SHA256,
					})
					if err != nil {
						errCh <- fmt.Errorf("%s: %v", name, err)
						return
					}

					mu.Lock()
					fmt.Printf("\n%s %s%s%s\n", green("✓"), bold(pkg.Name), bold("@"), bold(pkg.Version))
					fmt.Printf("  %s %s\n", cyan("cache:"), filepath.Join(cfg.CacheDir, pkg.Name, pkg.Version))
					fmt.Printf("  %s %s\n", cyan("path:"), filepath.Join(cfg.PackagesDir, pkg.Name, pkg.Version))
					mu.Unlock()

				}(name)
			}

			wg.Wait()
			close(errCh)

			var errs []error
			for err := range errCh {
				errs = append(errs, err)
			}

			if len(errs) > 0 {
				fmt.Println()
				for _, e := range errs {
					fmt.Printf("%s %s\n", red("✗"), e)
				}
				return fmt.Errorf("failed to install %d package(s)", len(errs))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
	return cmd
}
