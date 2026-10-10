package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultUsesShortPrefix(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Prefix != DefaultPrefix || cfg.CellarDir != "/opt/chatr/Cellar" || cfg.BinDir != "/opt/chatr/bin" {
		t.Errorf("unexpected defaults: prefix=%s cellar=%s bin=%s", cfg.Prefix, cfg.CellarDir, cfg.BinDir)
	}
	if !strings.HasSuffix(cfg.StateDB, filepath.Join(".chatr", "state.db")) {
		t.Errorf("state db should stay in the home directory, got %s", cfg.StateDB)
	}
}

func TestLegacyConfigKeepsHomeLayout(t *testing.T) {
	p := writeConfig(t, `
cellar_dir = "/home/u/.chatr/Cellar"
opt_dir = "/home/u/.chatr/opt"
bin_dir = "/somewhere/else/bin"
`)
	cfg, err := decode(p, "/home/u", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "/home/u/.chatr" || cfg.CellarDir != "/home/u/.chatr/Cellar" {
		t.Errorf("prefix=%s cellar=%s, want the ~/.chatr layout", cfg.Prefix, cfg.CellarDir)
	}
	if cfg.BinDir != "/home/u/.chatr/bin" {
		t.Errorf("bin_dir is not a setting, got %s", cfg.BinDir)
	}
}

func TestPrefixIsNotConfigurable(t *testing.T) {
	cases := map[string]string{
		"prefix key":        `prefix = "/opt/x"`,
		"custom cellar_dir": `cellar_dir = "/data/Cellar"`,
		"custom bin_dir":    `bin_dir = "/usr/local/bin"`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := decode(writeConfig(t, body), "/home/u", DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Prefix != DefaultPrefix || cfg.CellarDir != "/opt/chatr/Cellar" || cfg.BinDir != "/opt/chatr/bin" {
				t.Errorf("prefix=%s cellar=%s bin=%s, want /opt/chatr", cfg.Prefix, cfg.CellarDir, cfg.BinDir)
			}
		})
	}
}

func TestOtherSettingsStillApply(t *testing.T) {
	cfg, err := decode(writeConfig(t, "max_parallel = 2\napps_dir = \"/Users/u/Applications\""), "/home/u", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxParallel != 2 || cfg.AppsDir != "/Users/u/Applications" {
		t.Errorf("max_parallel=%d apps_dir=%s", cfg.MaxParallel, cfg.AppsDir)
	}
}

func TestSaveWritesNoPrefixSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".chatr", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"prefix", "cellar_dir", "bin_dir", "opt_dir"} {
		if strings.Contains(string(data), key) {
			t.Errorf("saved config contains %s:\n%s", key, data)
		}
	}
}
