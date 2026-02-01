package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/cache"
	"github.com/teamcutter/chatr/internal/config"
	"github.com/teamcutter/chatr/internal/extractor"
	"github.com/teamcutter/chatr/internal/fetcher"
	"github.com/teamcutter/chatr/internal/manager"
	"github.com/teamcutter/chatr/internal/state"
)

func Execute() error {
	rootCmd := &cobra.Command{Use: "chatr"}
	rootCmd.AddCommand(
		newInstallCmd(),
		newUninstallCmd(),
		newListCmd(),
	)
	return rootCmd.Execute()
}

func newManager() (*manager.Manager, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	c, err := cache.New(cfg.CacheDir)
	if err != nil {
		return nil, nil, err
	}

	return manager.New(
		fetcher.New(cfg.CacheDir, 1 * time.Hour),
		c,
		extractor.New(),
		state.New(cfg.ManifestFile),
		cfg.PackagesDir,
		cfg.BinDir), cfg, err
}
