package domain

import "time"

type Package struct {
	Name        string
	Version     string
	Revision    string
	FullVersion string
	DownloadURL string
	SHA256      string
	IsDep       bool
	IsCask      bool
}

type FetchResult struct {
	Package string
	Version string
	Path    string
	Error   error
}

type InstalledPackage struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Revision     string    `json:"revision,omitempty"`
	URL          string    `json:"url"`
	Path         string    `json:"path"`
	Binaries     []string  `json:"binaries"`
	Apps         []string  `json:"apps,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
	IsDep        bool      `json:"is_dep,omitempty"`
	IsCask       bool      `json:"is_cask,omitempty"`
	InstalledAt  time.Time `json:"installed_at"`
}

func (p InstalledPackage) FullVersion() string {
	return FormatVersion(p.Version, p.Revision)
}

type Manifest struct {
	Packages map[string]*InstalledPackage `json:"packages"`
}

func NewManifest() *Manifest {
	return &Manifest{Packages: make(map[string]*InstalledPackage)}
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
	IsCask       bool
	Apps         []string
}

func (f Formula) FullVersion() string {
	return FormatVersion(f.Version, f.Revision)
}
