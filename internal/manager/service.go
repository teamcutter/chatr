package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

type Manager struct {
	fetcher   domain.Fetcher
	cache     domain.Cache
	extractor domain.Extractor
	state domain.State
	packagesDir string
	binDir string
}

func New(
	fetcher domain.Fetcher, 
	cache domain.Cache, 
	extractor domain.Extractor, 
	state domain.State,
	packagesDir, binDir string,
	) *Manager {
		
		return &Manager{
			fetcher: fetcher,
			cache: cache,
			extractor: extractor,
			state: state,
			packagesDir: packagesDir,
			binDir: binDir,
		}
	} 


func (m *Manager) Install(ctx context.Context, pkg domain.Package) error {
	if installed, _, _ := m.state.IsInstalled(pkg.Name); installed {
		return fmt.Errorf("package %s already installed", pkg.Name)
	}

	var archivePath string
	if m.cache.Has(pkg.Name, pkg.Version) {
		archivePath = m.cache.GetPath(pkg.Name, pkg.Version)
	} else {
		result := m.fetcher.Fetch(ctx, pkg)
		if result.Error != nil {
			return result.Error
		}

		archivePath, _ = m.cache.Store(pkg.Name, pkg.Version, result.Path)
	}

	extractDir := filepath.Join(m.packagesDir, pkg.Name, pkg.Version)
	if err := m.extractor.Extract(archivePath, extractDir); err != nil {
		return  err
	}

	// TODO: Improve
	var binPath string
	entries, err := os.ReadDir(extractDir)
	if err != nil {
    	return err
	}
	if len(entries) == 0 {
    	return fmt.Errorf("no files found in %s", extractDir)
	}
	if entries[0].Name() == "bin" {
		binFile, err := os.ReadDir(filepath.Join(extractDir, entries[0].Name()))
		if err != nil {
			return err
		}
		binPath = filepath.Join(extractDir, entries[0].Name(), binFile[0].Name())
	} else {
		binPath = filepath.Join(extractDir, entries[0].Name())
	}

	err = m.createSystemLink(binPath)
	if err != nil {
		return err
	}

	pkgPath := filepath.Join(extractDir, entries[0].Name())

	return m.state.Add(domain.InstalledPackage{
        Name:        pkg.Name,
        Version:     pkg.Version,
        URL:         pkg.DownloadURL,
        Path:        pkgPath,
        InstalledAt: time.Now(),
    })
}

func (m *Manager) Uninstall(ctx context.Context, pkg domain.Package) error {
	if m.cache.Has(pkg.Name, pkg.Version) {
		cachePath := m.cache.GetPath(pkg.Name, pkg.Version)
		cacheDir := filepath.Dir(filepath.Dir(cachePath))
		if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	installed, installedPkg, _ := m.state.IsInstalled(pkg.Name)
	if !installed {
		return fmt.Errorf("package %s is not installed", pkg.Name)
	}

	binaryPath := filepath.Join(m.binDir, installedPkg.Name)
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	packageDir := filepath.Join(m.packagesDir, pkg.Name)
	if err := os.RemoveAll(packageDir); err != nil {
		return err
	}
	
	return m.state.Remove(pkg.Name)
}

func (m *Manager) List() ([]string, error) {
	manifest, err := m.state.Load()
	if err != nil {
		return make([]string, 0), err
	}

	packages := make([]string, 0, len(manifest.Packages))
	for _, pkg := range manifest.Packages {
		packageItem := fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
		packages = append(packages, packageItem)
	}

	return packages, nil
}

func (m *Manager) createSystemLink(path string) error {
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	binaryName := filepath.Base(path)
	linkPath := filepath.Join(m.binDir, binaryName)

	if _, err := os.Lstat(linkPath); err == nil {
		os.Remove(linkPath)
	}

	return os.Symlink(path, linkPath)
}