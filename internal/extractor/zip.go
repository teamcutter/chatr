package extractor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ZIPExtractor struct{}

func NewZIP() *ZIPExtractor {
	return &ZIPExtractor{}
}

func (ze *ZIPExtractor) Extract(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("invalid path in archive: %s", f.Name)
		}

		target := filepath.Join(dst, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			linkTarget, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			if err := os.Symlink(string(linkTarget), target); err != nil {
				return err
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return err
		}

		rc.Close()
		outFile.Close()
	}

	return nil
}

// ExtractApps extracts only .app bundles from the ZIP directly to dst.
func (ze *ZIPExtractor) ExtractApps(src, dst string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	defer r.Close()

	apps := make(map[string]bool)
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) > 0 && strings.HasSuffix(parts[0], ".app") {
			apps[parts[0]] = true
		}
	}

	for appName := range apps {
		os.RemoveAll(filepath.Join(dst, appName))
	}

	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) == 0 || !apps[parts[0]] {
			continue
		}

		if strings.Contains(f.Name, "..") {
			return nil, fmt.Errorf("invalid path in archive: %s", f.Name)
		}

		target := filepath.Join(dst, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}

		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			linkTarget, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			if err := os.Symlink(string(linkTarget), target); err != nil {
				return nil, err
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return nil, err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return nil, err
		}

		rc.Close()
		outFile.Close()
	}

	var result []string
	for appName := range apps {
		result = append(result, appName)
	}
	return result, nil
}
