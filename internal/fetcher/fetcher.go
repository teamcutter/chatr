package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

type HTTPFetcher struct {
	client    *http.Client
	outputDir string
	timeout   time.Duration
	// warn receives human-readable notices such as "checksum skipped".
	warn io.Writer
	// retries is the number of additional attempts for transient failures.
	retries int
	// backoff is the base delay between attempts; it grows linearly.
	backoff time.Duration
	// progress, when set, receives one tracker per download.
	progress domain.Progress
}

type trackerWriter struct{ t domain.Tracker }

func (w trackerWriter) Write(p []byte) (int, error) {
	w.t.Add(int64(len(p)))
	return len(p), nil
}

func New(outputDir string, timeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     30 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
		outputDir: outputDir,
		timeout:   timeout,
		warn:      os.Stderr,
		retries:   2,
		backoff:   500 * time.Millisecond,
	}
}

func (f *HTTPFetcher) SetProgress(p domain.Progress) { f.progress = p }

func (f *HTTPFetcher) SetWarnWriter(w io.Writer) { f.warn = w }

// isTransient reports whether a request should be retried.
func isTransient(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// do performs req, retrying transient failures. The returned response has an
// open body that the caller must close.
func (f *HTTPFetcher) do(ctx context.Context, newReq func() (*http.Request, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= f.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(f.backoff * time.Duration(attempt)):
			}
		}

		req, err := newReq()
		if err != nil {
			return nil, err
		}
		resp, err := f.client.Do(req)
		if !isTransient(resp, err) {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			resp.Body.Close()
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (f *HTTPFetcher) Fetch(ctx context.Context, pkg domain.Package) domain.FetchResult {
	ext := extFromURL(pkg.DownloadURL)
	filename := fmt.Sprintf("%s-%s%s", pkg.Name, pkg.FullVersion, ext)
	dst := filepath.Join(f.outputDir, filename)

	resp, err := f.do(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, pkg.DownloadURL, nil)
	})
	if err != nil {
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	if resp.StatusCode == http.StatusUnauthorized && strings.Contains(pkg.DownloadURL, "ghcr.io") {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		token, err := f.getGHCRToken(ctx, wwwAuth)
		if err != nil {
			return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
		}
		resp, err = f.do(ctx, func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.DownloadURL, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			return req, nil
		})
		if err != nil {
			return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
		}
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

	writers := []io.Writer{file}
	if f.progress != nil {
		t := f.progress.Start(pkg.Name, resp.ContentLength)
		defer t.Done()
		writers = append(writers, trackerWriter{t})
	}

	h := sha256.New()
	if pkg.SHA256 != "" {
		writers = append(writers, h)
	} else if f.warn != nil {
		fmt.Fprintf(f.warn, "warning: %s has no checksum, skipping verification\n", pkg.Name)
	}

	if _, err := io.Copy(io.MultiWriter(writers...), resp.Body); err != nil {
		file.Close()
		os.Remove(dst)
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	if err := file.Close(); err != nil {
		os.Remove(dst)
		return domain.FetchResult{Package: pkg.Name, Version: pkg.Version, Error: err}
	}

	if pkg.SHA256 != "" {
		actual := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(actual, pkg.SHA256) {
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

// Detailed here
// https://stackoverflow.com/questions/79168476/how-to-get-api-token-to-github-container-registry
func (f *HTTPFetcher) getGHCRToken(ctx context.Context, wwwAuth string) (string, error) {
	tokenURL, err := tokenURLFromChallenge(wwwAuth)
	if err != nil {
		return "", err
	}

	resp, err := f.do(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}

// tokenURLFromChallenge builds the token endpoint URL from a
// `Bearer realm="...",service="...",scope="..."` challenge. Parameters are
// query-encoded so values containing ':' or '/' (as GHCR scopes do) are safe.
func tokenURLFromChallenge(wwwAuth string) (string, error) {
	params := make(map[string]string)
	for _, part := range strings.Split(strings.TrimPrefix(strings.TrimSpace(wwwAuth), "Bearer "), ",") {
		part = strings.TrimSpace(part)
		if key, val, ok := strings.Cut(part, "="); ok {
			params[key] = strings.Trim(val, `"`)
		}
	}

	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("missing realm in WWW-Authenticate challenge")
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid realm %q: %w", realm, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("refusing non-https token realm %q", realm)
	}

	q := u.Query()
	if v := params["service"]; v != "" {
		q.Set("service", v)
	}
	if v := params["scope"]; v != "" {
		q.Set("scope", v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func extFromURL(rawURL string) string {
	// Since our main registry is brew
	// it provides blobs and they are always tar.gz
	if strings.Contains(rawURL, "ghcr.io") && strings.Contains(rawURL, "/blobs/") {
		return ".tar.gz"
	}

	u := path.Base(rawURL)
	for _, ext := range domain.Extensions() {
		if strings.HasSuffix(u, ext) {
			return ext
		}
	}
	return path.Ext(u)
}
