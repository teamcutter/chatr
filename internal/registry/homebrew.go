package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

const baseUrl string = "https://formulae.brew.sh/api/"

type HomebrewRegistry struct {
	sync.RWMutex
	client   *http.Client
	cacheDir string
	index    map[string]*Formulae
	indexMu  sync.Once
	indexErr error
}

type Formulae struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Desc     string `json:"desc"`
	Homepage string `json:"homepage"`
	Versions struct {
		Stable string `json:"stable"`
		Head   string `json:"head"`
	} `json:"versions"`
	Revision int `json:"revision"`
	URLs     struct {
		Stable struct {
			URL      string `json:"url"`
			Checksum string `json:"checksum"`
		} `json:"stable"`
	} `json:"urls"`
	Bottle struct {
		Stable struct {
			Files map[string]struct {
				URL    string `json:"url"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
		} `json:"stable"`
	} `json:"bottle"`
	Dependencies []string `json:"dependencies"`
}

func New(cacheDir string) *HomebrewRegistry {
	return &HomebrewRegistry{
		client:   &http.Client{},
		cacheDir: cacheDir,
	}
}

func (h *HomebrewRegistry) decodeIndex(r io.Reader) (map[string]*Formulae, error) {
	dec := json.NewDecoder(r)

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	index := make(map[string]*Formulae)
	for dec.More() {
		var f Formulae
		if err := dec.Decode(&f); err != nil {
			return nil, err
		}
		index[f.Name] = &f
	}

	return index, nil
}

func (h *HomebrewRegistry) loadIndex(ctx context.Context) error {
	h.indexMu.Do(func() {
		if cached, ok := h.getFromCache(10 * time.Minute); ok {
			index, err := h.decodeIndex(bytes.NewReader(cached))
			if err == nil {
				h.index = index
				return
			}
		}

		url := baseUrl + "formula.json"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			h.indexErr = fmt.Errorf("creating request: %w", err)
			return
		}
		req.Header.Set("User-Agent", "chatr")

		resp, err := h.client.Do(req)
		if err != nil {
			h.indexErr = fmt.Errorf("fetching formulae: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.indexErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			return
		}

		var buf bytes.Buffer
		reader := io.TeeReader(resp.Body, &buf)

		index, err := h.decodeIndex(reader)
		if err != nil {
			h.indexErr = fmt.Errorf("decoding response: %w", err)
			return
		}

		h.index = index
		_ = h.storeToCache(buf.Bytes())
	})
	return h.indexErr
}

func (h *HomebrewRegistry) Get(ctx context.Context, name string) (*domain.Formula, error) {
	if h.index != nil {
		if f, ok := h.index[name]; ok {
			return h.toFormula(f), nil
		}
	}

	url := baseUrl + "formula/" + name + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "chatr")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching formula: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("formula %q not found", name)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var f Formulae
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return h.toFormula(&f), nil
}

func (h *HomebrewRegistry) Search(ctx context.Context, query string) ([]domain.Formula, error) {
	if err := h.loadIndex(ctx); err != nil {
		return nil, err
	}

	formulae := make([]Formulae, 0, len(h.index))
	for _, f := range h.index {
		formulae = append(formulae, *f)
	}

	return h.filterAndSort(formulae, query), nil
}

func (h *HomebrewRegistry) GetVersion(ctx context.Context, name string) (string, error) {
	formula, err := h.Get(ctx, name)
	if err != nil {
		return "", err
	}

	return formula.Version, nil
}

func (h *HomebrewRegistry) filterAndSort(formulae []Formulae, query string) []domain.Formula {
	query = strings.ToLower(query)
	var results []domain.Formula
	for _, f := range formulae {
		if strings.Contains(strings.ToLower(f.Name), query) ||
			strings.Contains(strings.ToLower(f.Desc), query) {
			results = append(results, *h.toFormula(&f))
		}
	}

	slices.SortFunc(results, func(a, b domain.Formula) int {
		nameA := strings.ToLower(a.Name)
		nameB := strings.ToLower(b.Name)

		if (nameA == query) != (nameB == query) {
			if nameA == query {
				return -1
			}
			return 1
		}

		if strings.HasPrefix(nameA, query) != strings.HasPrefix(nameB, query) {
			if strings.HasPrefix(nameA, query) {
				return -1
			}
		}

		return strings.Compare(nameA, nameB)
	})

	return results
}

func (h *HomebrewRegistry) getFromCache(ttl time.Duration) ([]byte, bool) {
	h.RLock()
	defer h.RUnlock()

	path := filepath.Join(h.cacheDir, "formulae.json")
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}

	if time.Since(info.ModTime()) > ttl {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	return data, true
}

func (h *HomebrewRegistry) storeToCache(data []byte) error {
	h.Lock()
	defer h.Unlock()

	path := filepath.Join(h.cacheDir, "formulae.json")
	return os.WriteFile(path, data, 0644)
}

func (h *HomebrewRegistry) toFormula(f *Formulae) *domain.Formula {
	var url, sha256 string

	for _, p := range getPlatformCandidates() {
		if file, ok := f.Bottle.Stable.Files[p]; ok {
			url = file.URL
			sha256 = file.SHA256
			break
		}
	}

	if url == "" && f.URLs.Stable.URL != "" {
		url = f.URLs.Stable.URL
		sha256 = f.URLs.Stable.Checksum
	}

	return &domain.Formula{
		Name:         f.Name,
		Description:  f.Desc,
		Homepage:     f.Homepage,
		Version:      f.Versions.Stable,
		Revision:     strconv.Itoa(f.Revision),
		URL:          url,
		SHA256:       sha256,
		Dependencies: f.Dependencies,
	}
}

func getPlatformCandidates() []string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			candidates = []string{"arm64_sequoia", "arm64_sonoma", "arm64_ventura", "arm64_monterey"}
		} else {
			candidates = []string{"sequoia", "sonoma", "ventura", "monterey"}
		}
	case "linux":
		if runtime.GOARCH == "amd64" {
			candidates = []string{"x86_64_linux"}
		} else if runtime.GOARCH == "arm64" {
			candidates = []string{"aarch64_linux"}
		}
	}
	return append(candidates, "all")
}
