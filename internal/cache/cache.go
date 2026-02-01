package cache

import (
	"os"
	"path/filepath"

	"github.com/teamcutter/chatr/internal/domain"
)

type DiskCache struct {
	dir string
}

func New(dir string) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &DiskCache{dir: dir}, nil
}

func (c *DiskCache) GetPath(name, version string) string {
	dir := filepath.Join(c.dir, name, version)
    for _, ext := range domain.Extensions {
        path := filepath.Join(dir, "package" + ext)
        if _, err := os.Stat(path); err == nil {
            return path
        }
    }

    return filepath.Join(dir, "package.tar.gz")
}

func (c *DiskCache) Has(name, version string) bool {
	_, err := os.Stat(c.GetPath(name, version))
	return err == nil
}

func (c *DiskCache) Store(name, version, src string) (string, error) {
	ext := getArchiveExt(src)
	destDir := filepath.Join(c.dir, name, version)
	destPath := filepath.Join(destDir, "package" + ext)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	if err := os.Rename(src, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

func (c *DiskCache) Size() (int64, error) {
	var size int64

	err := filepath.Walk(c.dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return  err
		}

		if !info.IsDir() {
			size += info.Size()
		}
		return  nil
	})

	return size, err
}

func getArchiveExt(path string) string {
	lower := filepath.Base(path)
	for _, ext := range domain.Extensions {
		if len(lower) > len(ext) && lower[len(lower)-len(ext):] == ext {
			return ext
		}
	}

	return ".tar.gz"
}