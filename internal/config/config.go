package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/teamcutter/chatr/internal/domain"
)

type Config struct {
	CacheDir        string                  `toml:"cache_dir"`
	ChatrDir        string                  `toml:"chatr_dir"`
	PackagesDir     string                  `toml:"packages_dir"`
	BinDir          string                  `toml:"bin_dir"`
	ManifestFile    string                  `toml:"manifest_file"`
	MaxParallel     int                     `toml:"max_parallel"`
	Registries      []domain.RegistryConfig `toml:"registries"`
	DefaultRegistry string                  `toml:"default_registry"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".chatr")

	cfg := &Config{
		CacheDir:     filepath.Join(base, "cache"),
		ChatrDir:     base,
		PackagesDir:  filepath.Join(base, "packages"),
		BinDir:       filepath.Join(base, "bin"),
		ManifestFile: filepath.Join(base, "installed.json"),
		MaxParallel:  8,
		Registries: []domain.RegistryConfig{
			{Name: "homebrew", URL: "https://formulae.brew.sh/api/"},
		},
		DefaultRegistry: "homebrew",
	}

	return cfg
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	base := filepath.Join(home, ".chatr")

	configPath := filepath.Join(base, "config.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := Save(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".chatr")

	configPath := filepath.Join(base, "config.toml")

	os.MkdirAll(filepath.Dir(configPath), 0755)
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}
