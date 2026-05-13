package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
	"github.com/teamcutter/chatr/internal/linker"
)

type Manager struct {
	fetcher   domain.Fetcher
	cache     domain.Cache
	extractor domain.Extractor
	state     domain.State
	linker    *linker.Linker
	appsDir   string
}

func New(
	fetcher domain.Fetcher,
	cache domain.Cache,
	extractor domain.Extractor,
	state domain.State,
	lnkr *linker.Linker,
	appsDir string,
) *Manager {
	return &Manager{
		fetcher:   fetcher,
		cache:     cache,
		extractor: extractor,
		state:     state,
		linker:    lnkr,
		appsDir:   appsDir,
	}
}

func (m *Manager) Install(ctx context.Context, pkg domain.Package) (*domain.InstalledPackage, error) {
	if installed, _, _ := m.state.IsInstalled(pkg.Name); installed {
		return nil, fmt.Errorf("package %s already installed", pkg.Name)
	}

	var archivePath string
	if m.cache.Has(pkg.Name, pkg.FullVersion) {
		archivePath = m.cache.GetPath(pkg.Name, pkg.FullVersion)
	} else {
		result := m.fetcher.Fetch(ctx, pkg)
		if result.Error != nil {
			return nil, result.Error
		}

		var err error
		archivePath, err = m.cache.Store(pkg.Name, pkg.FullVersion, result.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to cache %s: %w", pkg.Name, err)
		}
	}

	pkgPath := m.linker.CellarPath(pkg.Name, pkg.FullVersion)

	pendingPkg := &domain.InstalledPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Revision:    pkg.Revision,
		URL:         pkg.DownloadURL,
		Path:        pkgPath,
		IsDep:       pkg.IsDep,
		IsCask:      pkg.IsCask,
		KegOnly:     pkg.KegOnly,
		InstalledAt: time.Now(),
	}

	if err := m.state.BeginInstall(pendingPkg); err != nil {
		return nil, fmt.Errorf("failed to begin install: %w", err)
	}

	var appNames []string

	if pkg.IsCask {
		apps, err := m.extractor.ExtractApps(archivePath, m.appsDir)
		if err != nil {
			return nil, err
		}
		appNames = apps
	} else {
		os.RemoveAll(pkgPath)

		if err := m.extractor.Extract(archivePath, m.linker.CellarPath("", "")); err != nil {
			return nil, err
		}

		pkgPath = m.linker.CellarPath(pkg.Name, pkg.FullVersion)

		if err := m.linker.Relocate(pkgPath, m.linker.PrefixPath()); err != nil {
		}

		if err := m.linker.CreateOptLink(pkg.Name, pkg.FullVersion); err != nil {
			return nil, fmt.Errorf("failed to create opt link: %w", err)
		}

		var linkedDirs []string
		var err error
		if !pkg.KegOnly {
			linkedDirs, err = m.linker.LinkToPrefix(pkg.Name, pkg.FullVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to link to prefix: %w", err)
			}
		}
		pendingPkg.LinkedDirs = linkedDirs
	}

	installedPkg := &domain.InstalledPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Revision:    pkg.Revision,
		URL:         pkg.DownloadURL,
		Path:        pkgPath,
		Binaries:    pendingPkg.Binaries,
		Libs:        pendingPkg.Libs,
		Apps:        appNames,
		IsDep:       pkg.IsDep,
		IsCask:      pkg.IsCask,
		KegOnly:     pkg.KegOnly,
		LinkedDirs:  pendingPkg.LinkedDirs,
		InstalledAt: pendingPkg.InstalledAt,
	}

	if err := m.state.Add(installedPkg); err != nil {
		return nil, err
	}

	return installedPkg, nil
}

func (m *Manager) Remove(ctx context.Context, pkg domain.Package) (*domain.InstalledPackage, error) {
	installed, installedPkg, _ := m.state.IsInstalled(pkg.Name)
	if !installed {
		return nil, fmt.Errorf("package %s is not installed", pkg.Name)
	}

	if installedPkg.IsCask {
		for _, appName := range installedPkg.Apps {
			appPath := filepath.Join(m.appsDir, appName)
			if err := os.RemoveAll(appPath); err != nil {
				return nil, fmt.Errorf("failed to remove app %s: %w", appName, err)
			}
		}
	} else {
		m.linker.UnlinkFromPrefix(pkg.Name, installedPkg.LinkedDirs)
		m.linker.RemoveOptLink(pkg.Name)
		os.RemoveAll(m.linker.CellarPath(pkg.Name, ""))
	}

	if err := m.state.Remove(pkg.Name); err != nil {
		return nil, err
	}

	for _, dep := range installedPkg.Dependencies {
		if m.isDependencyOf(dep, pkg.Name) {
			continue
		}
		m.Remove(ctx, domain.Package{Name: dep})
	}

	return installedPkg, nil
}

func (m *Manager) isDependencyOf(dep, excludeName string) bool {
	installed, err := m.state.ListInstalled()
	if err != nil {
		return false
	}
	for name, pkg := range installed {
		if name == excludeName {
			continue
		}
		if slices.Contains(pkg.Dependencies, dep) {
			return true
		}
	}
	return false
}

func (m *Manager) SetDependencies(name string, deps []string) error {
	_, pkg, err := m.state.IsInstalled(name)
	if err != nil || pkg == nil {
		return err
	}
	pkg.Dependencies = deps
	return m.state.Add(pkg)
}

func (m *Manager) Upgrade(ctx context.Context, oldPackage domain.Package, newPackage domain.Package) (*domain.InstalledPackage, error) {
	_, oldInstalled, _ := m.state.IsInstalled(oldPackage.Name)
	var oldDeps []string
	if oldInstalled != nil {
		oldDeps = oldInstalled.Dependencies
	}

	var archivePath string
	if m.cache.Has(newPackage.Name, newPackage.FullVersion) {
		archivePath = m.cache.GetPath(newPackage.Name, newPackage.FullVersion)
	} else {
		result := m.fetcher.Fetch(ctx, newPackage)
		if result.Error != nil {
			return nil, result.Error
		}
		var err error
		archivePath, err = m.cache.Store(newPackage.Name, newPackage.FullVersion, result.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to cache %s: %w", newPackage.Name, err)
		}
	}

	pkgPath := m.linker.CellarPath(newPackage.Name, newPackage.FullVersion)

	pendingPkg := &domain.InstalledPackage{
		Name:         newPackage.Name,
		Version:      newPackage.Version,
		Revision:     newPackage.Revision,
		URL:          newPackage.DownloadURL,
		Path:         pkgPath,
		Dependencies: oldDeps,
		IsDep:        newPackage.IsDep,
		IsCask:       newPackage.IsCask,
		KegOnly:      newPackage.KegOnly,
		InstalledAt:  time.Now(),
	}

	if err := m.state.BeginInstall(pendingPkg); err != nil {
		return nil, fmt.Errorf("failed to begin upgrade: %w", err)
	}

	if oldInstalled != nil {
		if oldInstalled.IsCask {
			for _, appName := range oldInstalled.Apps {
				appPath := filepath.Join(m.appsDir, appName)
				os.RemoveAll(appPath)
			}
		} else {
			m.linker.UnlinkFromPrefix(oldPackage.Name, oldInstalled.LinkedDirs)
			m.linker.RemoveOptLink(oldPackage.Name)
			os.RemoveAll(m.linker.CellarPath(oldPackage.Name, oldInstalled.FullVersion()))
		}
	}

	var appNames []string

	if newPackage.IsCask {
		apps, err := m.extractor.ExtractApps(archivePath, m.appsDir)
		if err != nil {
			return nil, err
		}
		appNames = apps
	} else {
		os.RemoveAll(pkgPath)

		if err := m.extractor.Extract(archivePath, m.linker.CellarPath("", "")); err != nil {
			return nil, err
		}

		pkgPath = m.linker.CellarPath(newPackage.Name, newPackage.FullVersion)

		if err := m.linker.Relocate(pkgPath, m.linker.PrefixPath()); err != nil {
		}

		if err := m.linker.CreateOptLink(newPackage.Name, newPackage.FullVersion); err != nil {
			return nil, fmt.Errorf("failed to create opt link: %w", err)
		}

		var linkedDirs []string
		var err error
		if !newPackage.KegOnly {
			linkedDirs, err = m.linker.LinkToPrefix(newPackage.Name, newPackage.FullVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to link to prefix: %w", err)
			}
		}
		pendingPkg.LinkedDirs = linkedDirs
	}

	installedPkg := &domain.InstalledPackage{
		Name:         newPackage.Name,
		Version:      newPackage.Version,
		Revision:     newPackage.Revision,
		URL:          newPackage.DownloadURL,
		Path:         pkgPath,
		Binaries:     pendingPkg.Binaries,
		Libs:         pendingPkg.Libs,
		Apps:         appNames,
		Dependencies: oldDeps,
		IsDep:        newPackage.IsDep,
		IsCask:       newPackage.IsCask,
		KegOnly:      newPackage.KegOnly,
		LinkedDirs:   pendingPkg.LinkedDirs,
		InstalledAt:  pendingPkg.InstalledAt,
	}

	if err := m.state.Add(installedPkg); err != nil {
		return nil, err
	}

	return installedPkg, nil
}

func (m *Manager) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	return m.state.ListInstalled()
}

func (m *Manager) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	return m.state.IsInstalled(name)
}

func (m *Manager) Reconcile() []string {
	installed, err := m.state.ListInstalled()
	if err != nil {
		return nil
	}

	var removed []string
	for name, pkg := range installed {
		if !pkg.IsCask {
			continue
		}
		for _, app := range pkg.Apps {
			appPath := filepath.Join(m.appsDir, app)
			if _, err := os.Stat(appPath); os.IsNotExist(err) {
				m.state.Remove(name)
				removed = append(removed, name)
				break
			}
		}
	}
	return removed
}

func (m *Manager) Flush() error {
	return m.state.Flush()
}

func (m *Manager) Clear(ctx context.Context) error {
	return m.cache.Clear()
}
