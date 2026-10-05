package extractor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
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

	w, err := openArchiveWriter(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	for _, f := range r.File {
		if err := writeZipEntry(w, f); err != nil {
			return err
		}
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
		if name, ok := topLevelApp(f.Name); ok {
			apps[name] = true
		}
	}

	w, err := openArchiveWriter(dst)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	for appName := range apps {
		if err := w.RemoveAll(appName); err != nil {
			return nil, err
		}
	}

	for _, f := range r.File {
		name, ok := topLevelApp(f.Name)
		if !ok || !apps[name] {
			continue
		}
		if err := writeZipEntry(w, f); err != nil {
			return nil, err
		}
	}

	result := make([]string, 0, len(apps))
	for appName := range apps {
		result = append(result, appName)
	}
	return result, nil
}

// topLevelApp returns the first path component of name if it is a .app bundle
// sitting at the root of the archive.
func topLevelApp(name string) (string, bool) {
	first, _, _ := strings.Cut(strings.TrimPrefix(name, "./"), "/")
	if first == "" || first == ".." || !strings.HasSuffix(first, ".app") {
		return "", false
	}
	return first, true
}

func writeZipEntry(w *archiveWriter, f *zip.File) error {
	info := f.FileInfo()

	if info.IsDir() {
		return w.Mkdir(f.Name)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		return w.Symlink(string(linkTarget), f.Name)
	}

	return w.WriteFile(f.Name, info.Mode(), rc)
}
