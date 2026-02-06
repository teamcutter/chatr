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

	reader, cleanup, err := te.getDecompressor(file)
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
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// https://gist.github.com/leommoore/f9e57ba2aa4bf197ebc5 - this is AWESOME
func (te *TARExtractor) getDecompressor(file *os.File) (io.Reader, func(), error) {
	header := make([]byte, 6)
	n, _ := file.Read(header)
	header = header[:n]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}

	switch {
	case n >= 4 && header[0] == 0x28 && header[1] == 0xb5 && header[2] == 0x2f && header[3] == 0xfd:
		// zstd: 0x28B52FFD
		zr, err := zstd.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("zstd: %w", err)
		}
		return zr, func() { zr.Close() }, nil

	case n >= 2 && header[0] == 0x1f && header[1] == 0x8b:
		// gzip: 0x1F8B
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip: %w", err)
		}
		return gzr, func() { gzr.Close() }, nil

	case n >= 6 && header[0] == 0xfd && header[1] == 0x37 && header[2] == 0x7a && header[3] == 0x58 && header[4] == 0x5a && header[5] == 0x00:
		// xz: 0xFD377A585A00
		xzr, err := xz.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("xz: %w", err)
		}
		return xzr, nil, nil

	case n >= 2 && header[0] == 0x42 && header[1] == 0x5a:
		// bzip2: 0x425A
		return bzip2.NewReader(file), nil, nil

	default:
		// plain tar
		return file, nil, nil
	}
}
