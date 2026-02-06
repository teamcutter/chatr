package manager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	libDir      string
}

func New(
	fetcher domain.Fetcher,
	cache domain.Cache,
	extractor domain.Extractor,
	state domain.State,
	packagesDir, binDir, libDir string,
) *Manager {

	return &Manager{
		fetcher:     fetcher,
		cache:       cache,
		extractor:   extractor,
		state:       state,
		packagesDir: packagesDir,
		binDir:      binDir,
		libDir:      libDir,
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

	libs := findLibraries(pkgPath)
	for _, libPath := range libs {
		libName := filepath.Base(libPath)
		m.createLibSymlink(libPath, libName)
		patchRpath(libPath, m.libDir)
	}

	binaries := findBinaries(pkgPath)

	var binaryNames []string
	for _, binPath := range binaries {
		binName := filepath.Base(binPath)
		if err := m.createSymlink(binPath, binName); err != nil {
			return nil, err
		}
		binaryNames = append(binaryNames, binName)
		patchRpath(binPath, m.libDir)
	}

	installedPkg := &domain.InstalledPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Revision:    pkg.Revision,
		URL:         pkg.DownloadURL,
		Path:        pkgPath,
		Binaries:    binaryNames,
		IsDep:       pkg.IsDep,
		InstalledAt: time.Now(),
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

	for _, binName := range installedPkg.Binaries {
		binaryPath := filepath.Join(m.binDir, binName)
		if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	libs := findLibraries(installedPkg.Path)
	for _, libPath := range libs {
		linkPath := filepath.Join(m.libDir, filepath.Base(libPath))
		os.Remove(linkPath)
	}

	packageDir := filepath.Join(m.packagesDir, pkg.Name)
	if err := os.RemoveAll(packageDir); err != nil {
		return nil, err
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
		for _, d := range pkg.Dependencies {
			if d == dep {
				return true
			}
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
	_, err := m.Remove(ctx, oldPackage)
	if err != nil {
		return nil, err
	}

	installedPackage, err := m.Install(ctx, newPackage)
	if err != nil {
		return nil, err
	}

	return installedPackage, nil
}

func (m *Manager) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	return m.state.ListInstalled()
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
		if pkg.IsDep {
			continue
		}
		packageItem := fmt.Sprintf("%s-%s", pkg.Name, pkg.FullVersion())
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

func (m *Manager) createLibSymlink(src, libName string) {
	os.MkdirAll(m.libDir, 0755)

	linkPath := filepath.Join(m.libDir, libName)
	if _, err := os.Lstat(linkPath); err == nil {
		os.Remove(linkPath)
	}

	os.Symlink(src, linkPath)
}

func patchRpath(path, libDir string) {
	switch runtime.GOOS {
	case "darwin":
		patchDarwin(path, libDir)
	case "linux":
		exec.Command("patchelf", "--set-rpath", libDir, path).Run()
	}
}

func patchDarwin(path, libDir string) {
	out, err := exec.Command("otool", "-L", path).Output()
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, " (compatibility") {
			continue
		}
		libRef := strings.TrimSpace(strings.Split(line, " (compatibility")[0])

		if strings.HasPrefix(libRef, "/usr/lib/") ||
			strings.HasPrefix(libRef, "/System/") ||
			strings.HasPrefix(libRef, "@rpath/") ||
			strings.HasPrefix(libRef, "@loader_path/") ||
			strings.HasPrefix(libRef, "@executable_path/") {
			continue
		}

		newRef := "@rpath/" + filepath.Base(libRef)
		exec.Command("install_name_tool", "-change", libRef, newRef, path).Run()
	}

	exec.Command("install_name_tool", "-add_rpath", libDir, path).Run()
	exec.Command("codesign", "--force", "--sign", "-", path).Run()
}

func findLibraries(dir string) []string {
	libDir := filepath.Join(dir, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil
	}

	var libs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.") {
			libs = append(libs, filepath.Join(libDir, name))
		}
	}
	return libs
}

func findBinaries(dir string) []string {
	candidates := []string{
		filepath.Join(dir, "bin"),
		filepath.Join(dir, "libexec", "bin"),
		filepath.Join(dir, "libexec"),
	}

	for _, binPath := range candidates {
		if executables := findExecutablesIn(binPath); len(executables) > 0 {
			return executables
		}
	}

	return nil
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
