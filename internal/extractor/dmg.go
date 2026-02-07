//go:build darwin

package extractor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type DMGExtractor struct{}

func NewDMG() *DMGExtractor {
	return &DMGExtractor{}
}

func (de *DMGExtractor) Extract(src, dst string) error {
	mountPoint, err := os.MkdirTemp("", "chatr-dmg-*")
	if err != nil {
		return fmt.Errorf("dmg: failed to create mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	attachCmd := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mountPoint, src)
	if err := attachCmd.Run(); err != nil {
		return fmt.Errorf("dmg: failed to mount: %w", err)
	}
	defer exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()

	// Use ditto to preserve extended attributes, resource forks,
	// and code signatures required by macOS .app bundles
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
}
