//go:build darwin

package extractor

import (
	"fmt"
	"io"
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

	return copyDir(mountPoint, dst)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(linkTarget) {
				return nil
			}
			return os.Symlink(linkTarget, targetPath)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
