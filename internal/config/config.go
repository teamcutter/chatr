package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

var configMu sync.Mutex

// DefaultPrefix is no longer than any Homebrew build prefix, which lets
// paths baked into bottle binaries be rewritten in place.
const DefaultPrefix = "/opt/chatr"

type Config struct {
	Prefix        string `toml:"prefix"`
	CacheDir      string `toml:"cache_dir"`
	ChatrDir      string `toml:"chatr_dir"`
	PackagesDir   string `toml:"packages_dir"`
	BinDir        string `toml:"bin_dir,omitempty"`
	LibDir        string `toml:"lib_dir,omitempty"`
	AppsDir       string `toml:"apps_dir"`
	FormulaeDir   string `toml:"formulae_dir"`
	ManifestFile  string `toml:"manifest_file"`
	StateDB       string `toml:"state_db"`
	MaxParallel   int    `toml:"max_parallel"`
	CellarDir     string `toml:"cellar_dir,omitempty"`
	OptDir        string `toml:"opt_dir,omitempty"`
	IncludeDir    string `toml:"include_dir,omitempty"`
	ShareDir      string `toml:"share_dir,omitempty"`
	EtcDir        string `toml:"etc_dir,omitempty"`
	VarDir        string `toml:"var_dir,omitempty"`
	FrameworksDir string `toml:"frameworks_dir,omitempty"`
}

// prefixDirs maps each prefix-relative directory to its config key and field.
func (c *Config) prefixDirs() []struct {
	key, sub string
	field    *string
} {
	return []struct {
		key, sub string
		field    *string
	}{
		{"bin_dir", "bin", &c.BinDir},
		{"lib_dir", "lib", &c.LibDir},
		{"cellar_dir", "Cellar", &c.CellarDir},
		{"opt_dir", "opt", &c.OptDir},
		{"include_dir", "include", &c.IncludeDir},
		{"share_dir", "share", &c.ShareDir},
		{"etc_dir", "etc", &c.EtcDir},
		{"var_dir", "var", &c.VarDir},
		{"frameworks_dir", "Frameworks", &c.FrameworksDir},
	}
}

func (c *Config) applyPrefix(isSet func(key string) bool) {
	for _, d := range c.prefixDirs() {
		if !isSet(d.key) {
			*d.field = filepath.Join(c.Prefix, d.sub)
		}
	}
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".chatr")

	cfg := &Config{
		Prefix:       DefaultPrefix,
		CacheDir:     filepath.Join(base, "cache"),
		ChatrDir:     base,
		PackagesDir:  filepath.Join(base, "packages"),
		AppsDir:      "/Applications",
		FormulaeDir:  filepath.Join(base, "formulae"),
		ManifestFile: filepath.Join(base, "installed.json"),
		StateDB:      filepath.Join(base, "state.db"),
		MaxParallel:  6,
	}
	cfg.applyPrefix(func(string) bool { return false })

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

	return decode(configPath, cfg)
}

// decode overlays the config file on the defaults. Package directories that
// the file does not set are derived from the prefix. Older files have no
// prefix key but list every directory, so their prefix is inferred from the
// Cellar location and nothing moves.
func decode(path string, cfg *Config) (*Config, error) {
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, err
	}
	if !md.IsDefined("prefix") && md.IsDefined("cellar_dir") {
		cfg.Prefix = filepath.Dir(cfg.CellarDir)
	}
	cfg.applyPrefix(func(key string) bool { return md.IsDefined(key) })
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

	out := *cfg
	for _, d := range out.prefixDirs() {
		*d.field = ""
	}
	return toml.NewEncoder(f).Encode(out)
}
