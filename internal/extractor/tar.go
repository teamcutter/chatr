package extractor

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type TARExtractor struct{}

func NewTAR() *TARExtractor {
	return &TARExtractor{}
}

func (te *TARExtractor) Extract(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, cleanup, err := te.getDecompressor(src, file)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

func (te *TARExtractor) getDecompressor(src string, file *os.File) (io.Reader, func(), error) {
	lower := strings.ToLower(src)

	switch {
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		zr, err := zstd.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("zstd: %w", err)
		}
		return zr, func() { zr.Close() }, nil

	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip: %w", err)
		}
		return gzr, func() { gzr.Close() }, nil

	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		xzr, err := xz.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("xz: %w", err)
		}
		return xzr, nil, nil

	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return bzip2.NewReader(file), nil, nil

	case strings.HasSuffix(lower, ".tar"):
		return file, nil, nil

	default:
		return nil, nil, fmt.Errorf("unsupported archive format: %s", src)
	}
}
