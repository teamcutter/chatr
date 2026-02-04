package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/teamcutter/chatr/internal/domain"
)

type ManifestState struct {
	sync.RWMutex
	path string
}

func New(path string) *ManifestState {
	return &ManifestState{
		path: path,
	}
}

func (m *ManifestState) Load() (*domain.Manifest, error) {
	m.RLock()
	defer m.RUnlock()
	return m.load()
}

func (m *ManifestState) load() (*domain.Manifest, error) {
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return domain.NewManifest(), nil
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

	return &manifest, nil
}

func (m *ManifestState) Save(manifest *domain.Manifest) error {
	m.Lock()
	defer m.Unlock()
	return m.save(manifest)
}

func (m *ManifestState) save(manifest *domain.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

func (m *ManifestState) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	m.RLock()
	defer m.RUnlock()

	manifest, err := m.load()
	if err != nil {
		return false, nil, err
	}

	pkg, exists := manifest.Packages[name]
	if !exists {
		return false, nil, nil
	}

	return true, pkg, nil
}

func (m *ManifestState) Add(pkg *domain.InstalledPackage) error {
	m.Lock()
	defer m.Unlock()

	manifest, err := m.load()
	if err != nil {
		return err
	}

	manifest.Packages[pkg.Name] = pkg
	return m.save(manifest)
}

func (m *ManifestState) Remove(name string) error {
	m.Lock()
	defer m.Unlock()

	manifest, err := m.load()
	if err != nil {
		return err
	}

	delete(manifest.Packages, name)
	return m.save(manifest)
}
