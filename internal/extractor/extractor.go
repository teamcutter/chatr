package extractor

import (
	"fmt"
	"strings"
)

type Extractor struct {
	tar *TARExtractor
	zip *ZIPExtractor
	dmg *DMGExtractor
	pkg *PKGExtractor
}

func New() *Extractor {
	return &Extractor{
		tar: NewTAR(),
		zip: NewZIP(),
		dmg: NewDMG(),
		pkg: NewPKG(),
	}
}

func (e *Extractor) Extract(src, dst string) error {
	lower := strings.ToLower(src)

	switch {
	case strings.HasSuffix(lower, ".zip"):
		return e.zip.Extract(src, dst)
	case strings.HasSuffix(lower, ".dmg"):
		return e.dmg.Extract(src, dst)
	case strings.HasSuffix(lower, ".pkg"):
		return e.pkg.Extract(src, dst)
	case isTarArchive(lower):
		return e.tar.Extract(src, dst)
	default:
		return fmt.Errorf("unsupported archive format: %s", src)
	}
}

func isTarArchive(name string) bool {
	tarExts := []string{".tar.gz", ".tar.zst", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tzst", ".tbz2", ".tar"}
	for _, ext := range tarExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
