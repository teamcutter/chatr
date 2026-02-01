package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newInstallCmd() *cobra.Command {
	var version, sha256 string

	cmd := &cobra.Command{
		Use:  "install <name@url>...",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, cfg, _ := newManager()

			for _, arg := range args {
				parts := strings.SplitN(arg, "@", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid format %q: expected name@url", arg)
				}

				fmt.Println(strings.Repeat("=", 50))
				fmt.Printf("Installing %s...\n", parts[0])
				err := mgr.Install(cmd.Context(), domain.Package{
					Name:        parts[0],
					DownloadURL: parts[1],
					Version:     version,
					SHA256:      sha256,
				})
				if err != nil {
					return err
				}

				fmt.Printf("Installed %s\n", parts[0])
				fmt.Printf("Cache: %s\n", filepath.Join(cfg.CacheDir, parts[0]))
				fmt.Printf("Packages: %s\n", filepath.Join(cfg.PackagesDir, parts[0]))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
	cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
	return cmd
}
