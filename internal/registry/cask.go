package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/teamcutter/chatr/internal/domain"
)

type CaskRegistry struct {
	sync.RWMutex
	client   *http.Client
	cacheDir string
}

type Cask struct {
	Token     string            `json:"token"`
	Name      []string          `json:"name"`
	Desc      string            `json:"desc"`
	Homepage  string            `json:"homepage"`
	URL       string            `json:"url"`
	Version   string            `json:"version"`
	SHA256    string            `json:"sha256"`
	Artifacts []json.RawMessage `json:"artifacts"`
}

func NewCask(cacheDir string) *CaskRegistry {
	return &CaskRegistry{
		client:   &http.Client{},
		cacheDir: cacheDir,
	}
}

func (c *CaskRegistry) Get(ctx context.Context, name string) (*domain.Formula, error) {
	url := baseUrl + "cask/" + name + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "chatr")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching cask: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("cask %q not found", name)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var cask Cask
	if err := json.NewDecoder(resp.Body).Decode(&cask); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return toFormulaCask(&cask), nil
}

func (c *CaskRegistry) Search(ctx context.Context, query string) ([]domain.Formula, error) {
	var casks []Cask

	if cached, ok := c.getFromCache(time.Minute * 10); ok {
		if err := json.Unmarshal(cached, &casks); err == nil {
			return filterAndSortCasks(casks, query), nil
		}
	}

	url := baseUrl + "cask.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "chatr")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching casks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if err := json.Unmarshal(data, &casks); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	_ = c.storeToCache(data)

	return filterAndSortCasks(casks, query), nil
}

func (c *CaskRegistry) GetVersion(ctx context.Context, name string) (string, error) {
	formula, err := c.Get(ctx, name)
	if err != nil {
		return "", err
	}
	return formula.Version, nil
}

func (c *CaskRegistry) getFromCache(ttl time.Duration) ([]byte, bool) {
	c.RLock()
	defer c.RUnlock()

	path := filepath.Join(c.cacheDir, "casks.json")
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

func (c *CaskRegistry) storeToCache(data []byte) error {
	c.Lock()
	defer c.Unlock()

	path := filepath.Join(c.cacheDir, "casks.json")
	return os.WriteFile(path, data, 0644)
}

func filterAndSortCasks(casks []Cask, query string) []domain.Formula {
	query = strings.ToLower(query)
	var results []domain.Formula
	for _, c := range casks {
		if strings.Contains(strings.ToLower(c.Token), query) ||
			strings.Contains(strings.ToLower(c.Desc), query) {
			results = append(results, *toFormulaCask(&c))
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

func toFormulaCask(c *Cask) *domain.Formula {
	sha256 := c.SHA256
	if sha256 == "no_check" {
		sha256 = ""
	}

	apps := parseArtifacts(c.Artifacts)

	desc := c.Desc
	if len(c.Name) > 0 {
		desc = c.Name[0]
		if c.Desc != "" {
			desc += " — " + c.Desc
		}
	}

	return &domain.Formula{
		Name:        c.Token,
		Description: desc,
		Homepage:    c.Homepage,
		Version:     c.Version,
		URL:         c.URL,
		SHA256:      sha256,
		IsCask:      true,
		Apps:        apps,
	}
}

func parseArtifacts(artifacts []json.RawMessage) []string {
	var apps []string
	for _, raw := range artifacts {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}

		if appData, ok := obj["app"]; ok {
			var appList []string
			if err := json.Unmarshal(appData, &appList); err == nil {
				apps = append(apps, appList...)
			}
		}
	}
	return apps
}
