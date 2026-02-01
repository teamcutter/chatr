//go:build !darwin

package extractor

import "fmt"

type PKGExtractor struct{}

func NewPKG() *PKGExtractor {
	return &PKGExtractor{}
}

func (pe *PKGExtractor) Extract(src, dst string) error {
	return fmt.Errorf("pkg extraction is only supported on macOS")
}
