package fetcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

func newTestFetcher(t *testing.T) (*HTTPFetcher, *bytes.Buffer) {
	t.Helper()
	f := New(t.TempDir(), 10*time.Second)
	var warnings bytes.Buffer
	f.warn = &warnings
	f.backoff = time.Millisecond
	return f, &warnings
}

func TestTokenURLFromChallengeEncodesScope(t *testing.T) {
	got, err := tokenURLFromChallenge(`Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:homebrew/core/go:pull"`)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://ghcr.io/token?scope=repository%3Ahomebrew%2Fcore%2Fgo%3Apull&service=ghcr.io"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestTokenURLFromChallengeRejectsBadRealm(t *testing.T) {
	for _, challenge := range []string{
		`Bearer service="x"`,
		`Bearer realm="http://evil.example/token"`,
	} {
		if _, err := tokenURLFromChallenge(challenge); err == nil {
			t.Errorf("expected error for %q", challenge)
		}
	}
}

func TestFetchVerifiesChecksumAndCleansUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("payload"))
	}))
	defer srv.Close()

	f, warnings := newTestFetcher(t)
	res := f.Fetch(context.Background(), domain.Package{
		Name: "foo", Version: "1", FullVersion: "1",
		DownloadURL: srv.URL + "/foo-1.tar.gz",
		SHA256:      strings.Repeat("0", 64),
	})
	if res.Error == nil || !strings.Contains(res.Error.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", res.Error)
	}
	if _, err := os.Stat(res.Path); res.Path != "" && err == nil {
		t.Error("partial download should have been removed")
	}
	if warnings.Len() != 0 {
		t.Errorf("unexpected warning: %s", warnings.String())
	}

	sum := sha256.Sum256([]byte("payload"))
	res = f.Fetch(context.Background(), domain.Package{
		Name: "foo", Version: "1", FullVersion: "1",
		DownloadURL: srv.URL + "/foo-1.tar.gz",
		SHA256:      strings.ToUpper(hex.EncodeToString(sum[:])),
	})
	if res.Error != nil {
		t.Fatalf("Fetch: %v", res.Error)
	}
	if data, err := os.ReadFile(res.Path); err != nil || string(data) != "payload" {
		t.Fatalf("downloaded = %q, %v", data, err)
	}
}

func TestFetchWarnsWhenChecksumMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("payload"))
	}))
	defer srv.Close()

	f, warnings := newTestFetcher(t)
	res := f.Fetch(context.Background(), domain.Package{
		Name: "foo", Version: "1", FullVersion: "1", DownloadURL: srv.URL + "/foo.zip",
	})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if !strings.Contains(warnings.String(), "skipping verification") {
		t.Errorf("expected warning, got %q", warnings.String())
	}
}

func TestFetchRetriesTransientErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f, _ := newTestFetcher(t)
	res := f.Fetch(context.Background(), domain.Package{
		Name: "foo", Version: "1", FullVersion: "1", DownloadURL: srv.URL + "/foo.zip",
	})
	if res.Error != nil {
		t.Fatalf("expected success after retries, got %v", res.Error)
	}
	if hits.Load() != 3 {
		t.Errorf("hits = %d, want 3", hits.Load())
	}
}

func TestFetchDoesNotRetryPermanentErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f, _ := newTestFetcher(t)
	res := f.Fetch(context.Background(), domain.Package{
		Name: "foo", Version: "1", FullVersion: "1", DownloadURL: srv.URL + "/foo.zip",
	})
	if res.Error == nil {
		t.Fatal("expected error")
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}
