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

func TestLegacyConfigKeepsItsPaths(t *testing.T) {
	p := writeConfig(t, `
cellar_dir = "/home/u/.chatr/Cellar"
opt_dir = "/home/u/.chatr/opt"
bin_dir = "/home/u/.chatr/bin"
`)
	cfg, err := decode(p, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "/home/u/.chatr" {
		t.Errorf("prefix = %s, want inferred /home/u/.chatr", cfg.Prefix)
	}
	if cfg.LibDir != "/home/u/.chatr/lib" {
		t.Errorf("unset dirs should follow the inferred prefix, lib = %s", cfg.LibDir)
	}
	if cfg.BinDir != "/home/u/.chatr/bin" {
		t.Errorf("bin = %s", cfg.BinDir)
	}
}

func TestPrefixDrivesUnsetDirs(t *testing.T) {
	p := writeConfig(t, `
prefix = "/opt/x"
etc_dir = "/etc/x"
`)
	cfg, err := decode(p, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CellarDir != "/opt/x/Cellar" || cfg.OptDir != "/opt/x/opt" {
		t.Errorf("cellar=%s opt=%s", cfg.CellarDir, cfg.OptDir)
	}
	if cfg.EtcDir != "/etc/x" {
		t.Errorf("explicit etc_dir lost: %s", cfg.EtcDir)
	}
}

func TestSaveOmitsDerivedDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".chatr", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `prefix = "/opt/chatr"`) || strings.Contains(s, "cellar_dir") {
		t.Errorf("saved config should carry prefix only:\n%s", s)
	}
}
