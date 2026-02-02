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

	if err := m.extractor.Extract(archivePath, m.packagesDir); err != nil {
		return err
	}

	binPath, err := findBinary(m.packagesDir)
	if err != nil {
		return err
	}

	err = m.createSystemLink(binPath, pkg.Name)
	if err != nil {
		return err
	}

	pkgPath := filepath.Dir(binPath)

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

	var visible []os.DirEntry
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			visible = append(visible, e)
		}
	}

	if len(visible) == 0 {
		return "", fmt.Errorf("no files found in %s", dir)
	}

	for _, e := range visible {
		if e.Name() == "bin" && e.IsDir() {
			binEntries, err := os.ReadDir(filepath.Join(dir, "bin"))
			if err != nil {
				return "", err
			}
			for _, be := range binEntries {
				if !strings.HasPrefix(be.Name(), ".") {
					return filepath.Join(dir, "bin", be.Name()), nil
				}
			}
		}
	}

	if runtime.GOOS == "darwin" {
		for _, e := range visible {
			if strings.HasSuffix(e.Name(), ".app") && e.IsDir() {
				appName := strings.TrimSuffix(e.Name(), ".app")
				cliName := strings.ToLower(appName)

				resourcesDir := filepath.Join(dir, e.Name(), "Contents", "Resources")
				if resEntries, err := os.ReadDir(resourcesDir); err == nil {
					for _, re := range resEntries {
						if re.Name() == cliName && !re.IsDir() {
							binPath := filepath.Join(resourcesDir, re.Name())
							if info, err := os.Stat(binPath); err == nil && info.Mode()&0111 != 0 {
								return binPath, nil
							}
						}
					}
				}

				macosDir := filepath.Join(dir, e.Name(), "Contents", "MacOS")
				binEntries, err := os.ReadDir(macosDir)
				if err != nil {
					return "", err
				}
				for _, be := range binEntries {
					if !strings.HasPrefix(be.Name(), ".") {
						return filepath.Join(macosDir, be.Name()), nil
					}
				}
			}
		}
	}

	return filepath.Join(dir, visible[0].Name()), nil
}
