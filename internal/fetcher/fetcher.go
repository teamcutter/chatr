package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/teamcutter/chatr/internal/domain"
)

type HTTPFetcher struct {
	client    *http.Client
	outputDir string
	timeout   time.Duration
}

func New(outputDir string, timeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		client:    &http.Client{Timeout: timeout},
		outputDir: outputDir,
		timeout:   timeout,
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, pkg domain.Package) domain.FetchResult {
	ext := extFromURL(pkg.DownloadURL)
	filename := fmt.Sprintf("%s-%s%s", pkg.Name, pkg.Version, ext)
	dst := filepath.Join(f.outputDir, filename)

	req, err := http.NewRequestWithContext(ctx, "GET", pkg.DownloadURL, nil)
	if err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.FetchResult{
			Package: pkg.Name,
			Version: pkg.Version,
			Error:   fmt.Errorf("unexpected status: %d", resp.StatusCode),
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	file, err := os.Create(dst)
	if err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}
	defer file.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		fmt.Sprintf("Downloading %s", pkg.Name),
	)

	if _, err := io.Copy(io.MultiWriter(file, bar), resp.Body); err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	if pkg.SHA256 != "" {
		actual, err := computeChecksum(dst)
		if err != nil {
			return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
		}

		if actual != pkg.SHA256 {
			os.Remove(dst)
			return domain.FetchResult{
				Package: pkg.Name,
				Version: pkg.Version,
				Error:   fmt.Errorf("checksum mismatch: expected %s, got %s", pkg.SHA256, actual),
			}
		}
	}

	return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Path: dst}
}

func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extFromURL(rawURL string) string {
	u := path.Base(rawURL)
	for _, ext := range domain.Extensions() {
		if strings.HasSuffix(u, ext) {
			return ext
		}
	}
	return path.Ext(u)
}
