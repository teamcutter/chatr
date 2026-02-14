package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

var configMu sync.Mutex

type Config struct {
	CacheDir     string `toml:"cache_dir"`
	ChatrDir     string `toml:"chatr_dir"`
	PackagesDir  string `toml:"packages_dir"`
	BinDir       string `toml:"bin_dir"`
	LibDir       string `toml:"lib_dir"`
	AppsDir      string `toml:"apps_dir"`
	ManifestFile string `toml:"manifest_file"`
	StateDB      string `toml:"state_db"`
	MaxParallel  int    `toml:"max_parallel"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".chatr")

	cfg := &Config{
		CacheDir:     filepath.Join(base, "cache"),
		ChatrDir:     base,
		PackagesDir:  filepath.Join(base, "packages"),
		BinDir:       filepath.Join(base, "bin"),
		LibDir:       filepath.Join(base, "lib"),
		AppsDir:      "/Applications",
		ManifestFile: filepath.Join(base, "installed.json"),
		StateDB:      filepath.Join(base, "state.db"),
		MaxParallel:  6,
	}

	return cfg
}

func Load() (*Config, error) {
	configMu.Lock()
	defer configMu.Unlock()

	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	base := filepath.Join(home, ".chatr")
	configPath := filepath.Join(base, "config.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := save(cfg); err != nil {
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
	configMu.Lock()
	defer configMu.Unlock()
	return save(cfg)
}

func save(cfg *Config) error {
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
