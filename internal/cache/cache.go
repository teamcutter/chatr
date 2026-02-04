package cache

import (
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/teamcutter/chatr/internal/domain"
)

type DiskCache struct {
	sync.RWMutex
	dir string
}

func New(dir string) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &DiskCache{dir: dir}, nil
}

func (c *DiskCache) GetPath(name, version string) string {
	c.RLock()
	defer c.RUnlock()
	return c.getPath(name, version)
}

func (c *DiskCache) getPath(name, version string) string {
	actual := version
	if version == "latest" {
		entries, _ := os.ReadDir(filepath.Join(c.dir, name))
		var versions []string
		for _, e := range entries {
			if e.IsDir() {
				versions = append(versions, e.Name())
			}
		}
		if len(versions) > 0 {
			sort.Strings(versions)
			actual = versions[len(versions)-1]
		}
	}

	dir := filepath.Join(c.dir, name, actual)
	for _, ext := range domain.Extensions() {
		path := filepath.Join(dir, "package"+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return filepath.Join(dir, "package.tar.gz")
}

func (c *DiskCache) Has(name, version string) bool {
	c.RLock()
	defer c.RUnlock()
	_, err := os.Stat(c.getPath(name, version))
	return err == nil
}

func (c *DiskCache) Store(name, version, src string) (string, error) {
	c.Lock()
	defer c.Unlock()

	ext := getArchiveExt(src)
	destDir := filepath.Join(c.dir, name, version)
	destPath := filepath.Join(destDir, "package"+ext)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	if err := os.Rename(src, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

func (c *DiskCache) Size() (int64, error) {
	c.RLock()
	defer c.RUnlock()

	var size int64

	err := filepath.Walk(c.dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func (c *DiskCache) Clear() error {
	c.Lock()
	defer c.Unlock()

	return os.RemoveAll(c.dir)
}

func getArchiveExt(path string) string {
	lower := filepath.Base(path)
	for _, ext := range domain.Extensions() {
		if len(lower) > len(ext) && lower[len(lower)-len(ext):] == ext {
			return ext
		}
	}

	return ".tar.gz"
}
