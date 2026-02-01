package domain

import "time"

type Package struct {
	Name        string
	Version     string
	DownloadURL string
	SHA256      string
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
	URL         string    `json:"url"`
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installed_at"`
}

type Manifest struct {
	Packages map[string]InstalledPackage `json:"packages"`
}

func NewManifest() *Manifest {
	return &Manifest{Packages: make(map[string]InstalledPackage)}
}

type Formula struct {
	Name         string
	Description  string
	Homepage     string
	Version      string
	URLs         map[string]string
	SHA256       map[string]string
	Dependencies []string
}
