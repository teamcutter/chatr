package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	CacheDir string `toml:"cache_dir"`
	ChatrDir string `toml:"chatr_dir"`
	PackagesDir string `toml:"packages_dir"`
	BinDir string `toml:"bin_dir"`
	ManifestFile string `toml:"manifest_file"`
	Registries []Registry `toml:"registries"`
	DefaultRegistry string `toml:"default_registry"`
}

type Registry struct {
	Name string `toml:"name"`
	URL string `toml:"url"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()

	cfg := &Config{
		CacheDir: filepath.Join(home, ".chatr", "cache"),
		ChatrDir: filepath.Join(home, ".chatr"),
		BinDir: filepath.Join(home, ".chatr", "bin"),
		ManifestFile: filepath.Join(home, ".chatr", "installed.json"),
		PackagesDir: filepath.Join(home, ".chatr", "packages"),
		Registries: []Registry{
			{Name: "default", URL: "https://registery.chatr.dev"},
		},
		DefaultRegistry: "default",
	}

	err := Save(cfg) 
	if err != nil {
		fmt.Println(err) // TODO: Improve
	}

	return cfg
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	configPath := filepath.Join(home, ".chatr", "config.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	home, _ := os.UserHomeDir()

	configPath := filepath.Join(home, ".chatr", "config.toml")

	os.MkdirAll(filepath.Dir(configPath), 0755)
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
} 