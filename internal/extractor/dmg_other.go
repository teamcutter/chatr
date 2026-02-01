//go:build !darwin

package extractor

import "fmt"

type DMGExtractor struct{}

func NewDMG() *DMGExtractor {
	return &DMGExtractor{}
}

func (de *DMGExtractor) Extract(src, dst string) error {
	return fmt.Errorf("dmg extraction is only supported on macOS")
}
