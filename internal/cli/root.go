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
	"github.com/teamcutter/chatr/internal/resolver"
	"github.com/teamcutter/chatr/internal/state"
)

func Execute() error {
	rootCmd := &cobra.Command{Use: "chatr"}
	rootCmd.AddCommand(
		newInstallCmd(),
		newRemoveCmd(),
		newListCmd(),
		newSearchCmd(),
		newClearCmd(),
		newVersionCmd(),
		newNewCommand(),
		newUpgradeCmd(),
	)
	return rootCmd.Execute()
}

func newManager() (*manager.Manager, *config.Config, domain.Registry, *resolver.Resolver, error) {
	return newManagerWithOptions(false)
}

func newManagerWithOptions(cask bool) (*manager.Manager, *config.Config, domain.Registry, *resolver.Resolver, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	c, err := cache.New(cfg.CacheDir)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var reg domain.Registry
	if cask {
		reg = registry.NewCask(cfg.FormulaeDir)
	} else {
		reg = registry.New(cfg.FormulaeDir)
	}

	st, err := state.NewSQLite(cfg.StateDB, cfg.ManifestFile)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	mgr := manager.New(
		fetcher.New(cfg.CacheDir, 1*time.Hour),
		c,
		extractor.New(),
		st,
		cfg.PackagesDir,
		cfg.BinDir,
		cfg.LibDir,
		cfg.AppsDir)

	return mgr, cfg, reg, resolver.New(reg, st), nil
}
