//go:build darwin

package extractor

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type PKGExtractor struct{}

func NewPKG() *PKGExtractor {
	return &PKGExtractor{}
}

func (pe *PKGExtractor) Extract(src, dst string) error {
	expandDir, err := os.MkdirTemp("", "chatr-pkg-*")
	if err != nil {
		return fmt.Errorf("pkg: failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(expandDir)

	expandCmd := exec.Command("pkgutil", "--expand", src, expandDir)
	if err := expandCmd.Run(); err != nil {
		return fmt.Errorf("pkg: failed to expand: %w", err)
	}

	return pe.extractPayloads(expandDir, dst)
}

func (pe *PKGExtractor) extractPayloads(expandDir, dst string) error {
	entries, err := os.ReadDir(expandDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(expandDir, entry.Name())

		if entry.IsDir() {
			payloadPath := filepath.Join(path, "Payload")
			if _, err := os.Stat(payloadPath); err == nil {
				if err := pe.extractCPIO(payloadPath, dst); err != nil {
					return err
				}
			}
		} else if entry.Name() == "Payload" {
			if err := pe.extractCPIO(path, dst); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pe *PKGExtractor) extractCPIO(payloadPath, dst string) error {
	file, err := os.Open(payloadPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var reader io.Reader = file

	header := make([]byte, 2)
	if _, err := file.Read(header); err != nil {
		return err
	}
	file.Seek(0, io.SeekStart)

	if header[0] == 0x1f && header[1] == 0x8b {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("pkg: gzip: %w", err)
		}
		defer gzr.Close()
		reader = gzr
	}

	cpioCmd := exec.Command("cpio", "-idm", "--quiet")
	cpioCmd.Dir = dst
	cpioCmd.Stdin = reader

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return cpioCmd.Run()
}
