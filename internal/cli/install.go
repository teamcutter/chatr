package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newInstallCmd() *cobra.Command {
	var version, sha256 string

	cmd := &cobra.Command{
		Use:  "install <name@url>...",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, err := newManager()
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

			for _, arg := range args {
				parts := strings.SplitN(arg, "@", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid format %q: expected name@url", arg)
				}

				wg.Add(1)
				go func(name, url string) {
					defer wg.Done()

					err := mgr.Install(cmd.Context(), domain.Package{
						Name:        name,
						DownloadURL: url,
						Version:     version,
						SHA256:      sha256,
					})
					if err != nil {
						errCh <- fmt.Errorf("%s: %v", name, err)
						return
					}

					mu.Lock()
					fmt.Printf("\n%s %s\n", green("✓"), bold(name))
					fmt.Printf("  %s %s\n", cyan("cache:"), filepath.Join(cfg.CacheDir, name))
					fmt.Printf("  %s %s\n", cyan("path:"), filepath.Join(cfg.PackagesDir, name))
					mu.Unlock()

				}(parts[0], parts[1])
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
