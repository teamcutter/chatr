package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/domain"
)

func newInstallCmd() *cobra.Command {
    var version, sha256 string
    
    cmd := &cobra.Command{
        Use:  "install <name> <url>",
        Args: cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr, cfg, _ := newManager()

            fmt.Printf("Installing %s...\n", args[0])
            err := mgr.Install(cmd.Context(), domain.Package{
                Name:        args[0],
                DownloadURL: args[1],
                Version:     version,
                SHA256:      sha256,
            }) 
            if err != nil {
                return err
            }

            fmt.Printf("Installed %s\n", args[0])
		    fmt.Printf("Cache: %s\n", filepath.Join(cfg.CacheDir, args[0]))
		    fmt.Printf("Packages: %s\n", filepath.Join(cfg.PackagesDir, args[0]))

            return nil
        },
    }

    cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
    cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
    return cmd
}