package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/cache"
	"github.com/teamcutter/chatr/internal/config"
	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/extractor"
	"github.com/teamcutter/chatr/internal/fetcher"
	"github.com/teamcutter/chatr/internal/manager"
	"github.com/teamcutter/chatr/internal/registry"
	"github.com/teamcutter/chatr/internal/state"
)

func Execute() error {
	rootCmd := &cobra.Command{Use: "chatr"}
	rootCmd.AddCommand(
		newInstallCmd(),
		newRemoveCmd(),
		newListCmd(),
		newSearchCmd(),
	)
	return rootCmd.Execute()
}

func newManager() (*manager.Manager, *config.Config, domain.Registry, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}

	c, err := cache.New(cfg.CacheDir)
	if err != nil {
		return nil, nil, nil, err
	}

	reg := registry.New(cfg.CacheDir)

	return manager.New(
		fetcher.New(cfg.CacheDir, 1*time.Hour),
		c,
		extractor.New(),
		state.New(cfg.ManifestFile),
		cfg.PackagesDir,
		cfg.BinDir), cfg, reg, err
}
