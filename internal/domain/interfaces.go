package domain

import (
	"context"
)

type Fetcher interface {
	Fetch(ctx context.Context, pkg Package) FetchResult
}

type Cache interface {
	Has(name, version string) bool
	GetPath(name, version string) string
	Store(name, version, src string) (string, error)
	Size() (int64, error)
	Clear() error
}

type Extractor interface {
	Extract(src, dest string) error
}

type State interface {
	Load() (*Manifest, error)
	Save(m *Manifest) error
	IsInstalled(name string) (bool, *InstalledPackage, error)
	Add(pkg *InstalledPackage) error
	Remove(name string) error
}

type Registry interface {
	Get(ctx context.Context, name string) (*Formula, error)
	Search(ctx context.Context, query string) ([]Formula, error)
	GetVersions(ctx context.Context, name string) ([]string, error)
}
