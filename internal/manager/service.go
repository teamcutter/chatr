package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

type Manager struct {
	fetcher     domain.Fetcher
	cache       domain.Cache
	extractor   domain.Extractor
	state       domain.State
	packagesDir string
	binDir      string
}

func New(
	fetcher domain.Fetcher,
	cache domain.Cache,
	extractor domain.Extractor,
	state domain.State,
	packagesDir, binDir string,
) *Manager {

	return &Manager{
		fetcher:     fetcher,
		cache:       cache,
		extractor:   extractor,
		state:       state,
		packagesDir: packagesDir,
		binDir:      binDir,
	}
}

func (m *Manager) Install(ctx context.Context, pkg domain.Package) (*domain.InstalledPackage, error) {
	if installed, _, _ := m.state.IsInstalled(pkg.Name); installed {
		return nil, fmt.Errorf("package %s already installed", pkg.Name)
	}

	var archivePath string
	if m.cache.Has(pkg.Name, pkg.FullVersion()) {
		archivePath = m.cache.GetPath(pkg.Name, pkg.FullVersion())
	} else {
		result := m.fetcher.Fetch(ctx, pkg)
		if result.Error != nil {
			return nil, result.Error
		}

		archivePath, _ = m.cache.Store(pkg.Name, pkg.FullVersion(), result.Path)
	}

	// Ensure that if any previous installations
	// failed, we extract into clear dir
	pkgPath := filepath.Join(m.packagesDir, pkg.Name, pkg.FullVersion())
	os.RemoveAll(pkgPath)

	if err := m.extractor.Extract(archivePath, m.packagesDir); err != nil {
		return nil, err
	}

	binaries, err := findBinaries(pkgPath)
	if err != nil {
		os.RemoveAll(pkgPath)
		return nil, err
	}

	var binaryNames []string
	for _, binPath := range binaries {
		binName := filepath.Base(binPath)
		if err := m.createSymlink(binPath, binName); err != nil {
			return nil, err
		}
		binaryNames = append(binaryNames, binName)
	}

	installedPkg := &domain.InstalledPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Revision:    pkg.Revision,
		URL:         pkg.DownloadURL,
		Path:        pkgPath,
		Binaries:    binaryNames,
		InstalledAt: time.Now(),
	}

	err = m.state.Add(installedPkg)
	if err != nil {
		return nil, err
	}

	return installedPkg, nil
}

func (m *Manager) Remove(ctx context.Context, pkg domain.Package) (string, string, error) {
	installed, installedPkg, _ := m.state.IsInstalled(pkg.Name)
	if !installed {
		return "", "", fmt.Errorf("package %s is not installed", pkg.Name)
	}

	for _, binName := range installedPkg.Binaries {
		binaryPath := filepath.Join(m.binDir, binName)
		if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
			return "", "", err
		}
	}

	packageDir := filepath.Join(m.packagesDir, pkg.Name)
	if err := os.RemoveAll(packageDir); err != nil {
		return "", "", err
	}

	err := m.state.Remove(pkg.Name)
	if err != nil {
		return "", "", err
	}

	return installedPkg.Name, installedPkg.FullVersion(), nil
}

func (m *Manager) Clear(ctx context.Context) error {
	return m.cache.Clear()
}

func (m *Manager) List() ([]string, error) {
	manifest, err := m.state.Load()
	if err != nil {
		return make([]string, 0), err
	}

	packages := make([]string, 0, len(manifest.Packages))
	for _, pkg := range manifest.Packages {
		packageItem := fmt.Sprintf("%s@%s", pkg.Name, pkg.FullVersion())
		packages = append(packages, packageItem)
	}

	return packages, nil
}

func (m *Manager) createSymlink(path, binName string) error {
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	linkPath := filepath.Join(m.binDir, binName)

	if _, err := os.Lstat(linkPath); err == nil {
		os.Remove(linkPath)
	}

	return os.Symlink(path, linkPath)
}

func findBinaries(dir string) ([]string, error) {
	candidates := []string{
		filepath.Join(dir, "bin"),
		filepath.Join(dir, "libexec", "bin"),
		filepath.Join(dir, "libexec"),
	}

	for _, binPath := range candidates {
		if executables := findExecutablesIn(binPath); len(executables) > 0 {
			return executables, nil
		}
	}

	return nil, fmt.Errorf("no executables found in %s", dir)
}

func findExecutablesIn(binPath string) []string {
	entries, err := os.ReadDir(binPath)
	if err != nil {
		return nil
	}

	var executables []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
			continue
		}

		fullPath := filepath.Join(binPath, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
			continue
		}

		executables = append(executables, fullPath)
	}

	return executables
}
