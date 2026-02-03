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
	if m.cache.Has(pkg.Name, pkg.Version) {
		archivePath = m.cache.GetPath(pkg.Name, pkg.Version)
	} else {
		result := m.fetcher.Fetch(ctx, pkg)
		if result.Error != nil {
			return nil, result.Error
		}

		archivePath, _ = m.cache.Store(pkg.Name, pkg.Version, result.Path)
	}

	if err := m.extractor.Extract(archivePath, m.packagesDir); err != nil {
		return nil, err
	}

	pkgPath := filepath.Join(m.packagesDir, pkg.Name, pkg.Version)

	binPath, err := findBinary(pkgPath)
	if err != nil {
		return nil, err
	}

	err = m.createSystemLink(binPath, pkg.Name)
	if err != nil {
		return nil, err
	}

	installedPkg := &domain.InstalledPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		URL:         pkg.DownloadURL,
		Path:        pkgPath,
		InstalledAt: time.Now(),
	}

	err = m.state.Add(installedPkg)
	if err != nil {
		return nil, err
	}

	return installedPkg, nil
}

func (m *Manager) Remove(ctx context.Context, pkg domain.Package) (string, string, error) {
	if m.cache.Has(pkg.Name, pkg.Version) {
		cachePath := m.cache.GetPath(pkg.Name, pkg.Version)
		cacheDir := filepath.Dir(filepath.Dir(cachePath))
		if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
			return "", "", err
		}
	}

	installed, installedPkg, _ := m.state.IsInstalled(pkg.Name)
	if !installed {
		return "", "", fmt.Errorf("package %s is not installed", pkg.Name)
	}

	binaryPath := filepath.Join(m.binDir, installedPkg.Name)
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	packageDir := filepath.Join(m.packagesDir, pkg.Name)
	if err := os.RemoveAll(packageDir); err != nil {
		return "", "", err
	}

	err := m.state.Remove(pkg.Name)
	if err != nil {
		return "", "", err
	}

	return installedPkg.Name, installedPkg.Version, nil
}

func (m *Manager) List() ([]string, error) {
	manifest, err := m.state.Load()
	if err != nil {
		return make([]string, 0), err
	}

	packages := make([]string, 0, len(manifest.Packages))
	for _, pkg := range manifest.Packages {
		packageItem := fmt.Sprintf("%s@%s", pkg.Name, pkg.Version)
		packages = append(packages, packageItem)
	}

	return packages, nil
}

func (m *Manager) createSystemLink(path, pkgName string) error {
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	linkPath := filepath.Join(m.binDir, pkgName)

	if _, err := os.Lstat(linkPath); err == nil {
		os.Remove(linkPath)
	}

	return os.Symlink(path, linkPath)
}

func findBinary(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var binDir os.DirEntry

	for _, e := range entries {
		if e.Name() == "bin" && e.IsDir() {
			binDir = e
			break
		}
	}

	if binDir == nil {
		return "", fmt.Errorf("bin directory not found in %s", dir)
	}

	binPath := filepath.Join(dir, binDir.Name())
	binEntries, err := os.ReadDir(binPath)
	if err != nil {
		return "", err
	}

	for _, e := range binEntries {
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

		return fullPath, nil
	}

	return "", fmt.Errorf("no executable found in %s", binPath)
}
