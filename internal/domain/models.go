package domain

import "time"

type Package struct {
	Name        string
	Version     string
	Revision    string
	DownloadURL string
	SHA256      string
}

func (p Package) FullVersion() string {
	return formatVersion(p.Version, p.Revision)
}

type FetchResult struct {
	Package string
	Version string
	Path    string
	Error   error
}

type InstalledPackage struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Revision    string    `json:"revision,omitempty"`
	URL         string    `json:"url"`
	Path        string    `json:"path"`
	Binaries    []string  `json:"binaries"`
	InstalledAt time.Time `json:"installed_at"`
}

func (p InstalledPackage) FullVersion() string {
	return formatVersion(p.Version, p.Revision)
}

type Manifest struct {
	Packages map[string]*InstalledPackage `json:"packages"`
}

func NewManifest() *Manifest {
	return &Manifest{Packages: make(map[string]*InstalledPackage)}
}

type RegistryConfig struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Formula struct {
	Name         string
	Description  string
	Homepage     string
	Version      string
	Revision     string
	URL          string
	SHA256       string
	Dependencies []string
}

func (f Formula) FullVersion() string {
	return formatVersion(f.Version, f.Revision)
}
