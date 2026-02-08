package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/teamcutter/chatr/internal/domain"
)

type ManifestState struct {
	mu       sync.RWMutex
	path     string
	manifest *domain.Manifest
}

func New(path string) (*ManifestState, error) {
	m := &ManifestState{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		m.manifest = domain.NewManifest()
		return m, nil
	}
	if err != nil {
		return nil, err
	}

	var manifest domain.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Packages == nil {
		manifest.Packages = make(map[string]*domain.InstalledPackage)
	}
	m.manifest = &manifest
	return m, nil
}

func (m *ManifestState) Load() (*domain.Manifest, error) {
	return m.manifest, nil
}

func (m *ManifestState) Save(manifest *domain.Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifest = manifest
	return m.flush()
}

func (m *ManifestState) flush() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

func (m *ManifestState) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkg, exists := m.manifest.Packages[name]
	if !exists {
		return false, nil, nil
	}
	return true, pkg, nil
}

func (m *ManifestState) Add(pkg *domain.InstalledPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifest.Packages[pkg.Name] = pkg
	return m.flush()
}

func (m *ManifestState) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make(map[string]*domain.InstalledPackage, len(m.manifest.Packages))
	for k, v := range m.manifest.Packages {
		cp[k] = v
	}
	return cp, nil
}

func (m *ManifestState) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.manifest.Packages, name)
	return m.flush()
}
