package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeCaskCache(t *testing.T, dir string, casks []Cask) {
	t.Helper()
	data, err := json.Marshal(casks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "casks.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCaskRegistryConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	writeCaskCache(t, dir, []Cask{
		{Token: "firefox", Name: []string{"Firefox"}, Desc: "Web browser", Version: "1", SHA256: "no_check",
			Artifacts: []json.RawMessage{json.RawMessage(`{"app":["Firefox.app"]}`)}},
		{Token: "iterm2", Name: []string{"iTerm2"}, Desc: "Terminal emulator", Version: "2"},
	})

	reg := NewCask(dir)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			f, err := reg.Get(ctx, "firefox")
			if err != nil {
				t.Error(err)
				return
			}
			if f.SHA256 != "" || len(f.Apps) != 1 || f.Apps[0] != "Firefox.app" {
				t.Errorf("unexpected formula: %+v", f)
			}
		}()
		go func() {
			defer wg.Done()
			res, err := reg.Search(ctx, "term")
			if err != nil {
				t.Error(err)
				return
			}
			if len(res) != 1 || res[0].Name != "iterm2" {
				t.Errorf("search = %+v", res)
			}
		}()
	}
	wg.Wait()
}

func TestFilterAndSortCasksRanking(t *testing.T) {
	casks := []*Cask{
		{Token: "zzz-go", Desc: "mentions go"},
		{Token: "go", Desc: ""},
		{Token: "google-chrome", Desc: "browser"},
	}
	res := filterAndSortCasks(casks, "go")
	want := []string{"go", "google-chrome", "zzz-go"}
	if len(res) != len(want) {
		t.Fatalf("got %d results, want %d", len(res), len(want))
	}
	for i, w := range want {
		if res[i].Name != w {
			t.Errorf("result[%d] = %s, want %s", i, res[i].Name, w)
		}
	}
}
