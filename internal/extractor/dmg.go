//go:build darwin

package extractor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DMGExtractor struct{}

func NewDMG() *DMGExtractor {
	return &DMGExtractor{}
}

func (de *DMGExtractor) Extract(src, dst string) error {
	return de.mount(src, func(mountPoint string) error {
		entries, err := os.ReadDir(mountPoint)
		if err != nil {
			return fmt.Errorf("dmg: failed to read mount: %w", err)
		}

		for _, entry := range entries {
			src := filepath.Join(mountPoint, entry.Name())
			target := filepath.Join(dst, entry.Name())
			if err := exec.Command("ditto", src, target).Run(); err != nil {
				return fmt.Errorf("dmg: failed to copy %s: %w", entry.Name(), err)
			}
		}
		return nil
	})
}

// ExtractApps extracts only .app bundles from the DMG directly to dst.
func (de *DMGExtractor) ExtractApps(src, dst string) ([]string, error) {
	var apps []string
	err := de.mount(src, func(mountPoint string) error {
		entries, err := os.ReadDir(mountPoint)
		if err != nil {
			return fmt.Errorf("dmg: failed to read mount: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}
			src := filepath.Join(mountPoint, entry.Name())
			target := filepath.Join(dst, entry.Name())
			os.RemoveAll(target)
			if err := exec.Command("ditto", src, target).Run(); err != nil {
				return fmt.Errorf("dmg: failed to copy %s: %w", entry.Name(), err)
			}
			apps = append(apps, entry.Name())
		}
		return nil
	})
	return apps, err
}

func (de *DMGExtractor) mount(src string, fn func(mountPoint string) error) error {
	mountPoint, err := os.MkdirTemp("", "chatr-dmg-*")
	if err != nil {
		return fmt.Errorf("dmg: failed to create mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	if err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mountPoint, src).Run(); err != nil {
		return fmt.Errorf("dmg: failed to mount: %w", err)
	}
	defer exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()

	return fn(mountPoint)
}
