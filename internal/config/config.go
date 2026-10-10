package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

var configMu sync.Mutex

// DefaultPrefix is no longer than any Homebrew build prefix, which lets
// paths baked into bottle binaries be rewritten in place. It is not a user
// setting.
const DefaultPrefix = "/opt/chatr"

type Config struct {
	Prefix        string `toml:"-"`
	CacheDir      string `toml:"cache_dir"`
	ChatrDir      string `toml:"chatr_dir"`
	PackagesDir   string `toml:"packages_dir"`
	BinDir        string `toml:"-"`
	LibDir        string `toml:"-"`
	AppsDir       string `toml:"apps_dir"`
	FormulaeDir   string `toml:"formulae_dir"`
	ManifestFile  string `toml:"manifest_file"`
	StateDB       string `toml:"state_db"`
	MaxParallel   int    `toml:"max_parallel"`
	CellarDir     string `toml:"-"`
	OptDir        string `toml:"-"`
	IncludeDir    string `toml:"-"`
	ShareDir      string `toml:"-"`
	EtcDir        string `toml:"-"`
	VarDir        string `toml:"-"`
	FrameworksDir string `toml:"-"`
}

func (c *Config) setPrefix(prefix string) {
	c.Prefix = prefix
	c.BinDir = filepath.Join(prefix, "bin")
	c.LibDir = filepath.Join(prefix, "lib")
	c.CellarDir = filepath.Join(prefix, "Cellar")
	c.OptDir = filepath.Join(prefix, "opt")
	c.IncludeDir = filepath.Join(prefix, "include")
	c.ShareDir = filepath.Join(prefix, "share")
	c.EtcDir = filepath.Join(prefix, "etc")
	c.VarDir = filepath.Join(prefix, "var")
	c.FrameworksDir = filepath.Join(prefix, "Frameworks")
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".chatr")

	cfg := &Config{
		CacheDir:     filepath.Join(base, "cache"),
		ChatrDir:     base,
		PackagesDir:  filepath.Join(base, "packages"),
		AppsDir:      "/Applications",
		FormulaeDir:  filepath.Join(base, "formulae"),
		ManifestFile: filepath.Join(base, "installed.json"),
		StateDB:      filepath.Join(base, "state.db"),
		MaxParallel:  6,
	}
	cfg.setPrefix(DefaultPrefix)

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

	return decode(configPath, home, cfg)
}

// decode overlays the config file on the defaults. Directory keys inside the
// prefix are not settings. The one exception is a config written before
// /opt/chatr, whose cellar_dir is exactly ~/.chatr/Cellar: its packages were
// relocated for ~/.chatr and stay there until the user migrates.
func decode(path, home string, cfg *Config) (*Config, error) {
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	var legacy struct {
		CellarDir string `toml:"cellar_dir"`
	}
	if _, err := toml.DecodeFile(path, &legacy); err != nil {
		return nil, err
	}
	legacyPrefix := filepath.Join(home, ".chatr")
	if legacy.CellarDir == filepath.Join(legacyPrefix, "Cellar") {
		cfg.setPrefix(legacyPrefix)
	}

	return cfg, nil
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
