package registry

import (
	"slices"
	"testing"
)

func TestBottleTags(t *testing.T) {
	cases := []struct {
		name         string
		goos, goarch string
		macMajor     int
		want         []string
	}{
		{"apple silicon newest", "darwin", "arm64", 27, []string{"arm64_golden_gate", "arm64_tahoe", "arm64_sequoia", "arm64_sonoma", "arm64_ventura", "arm64_monterey", "arm64_big_sur", "all"}},
		{"apple silicon tahoe skips newer", "darwin", "arm64", 26, []string{"arm64_tahoe", "arm64_sequoia", "arm64_sonoma", "arm64_ventura", "arm64_monterey", "arm64_big_sur", "all"}},
		{"intel sonoma", "darwin", "amd64", 14, []string{"sonoma", "ventura", "monterey", "big_sur", "all"}},
		{"future macOS falls back to newest known", "darwin", "arm64", 30, []string{"arm64_golden_gate", "arm64_tahoe", "arm64_sequoia", "arm64_sonoma", "arm64_ventura", "arm64_monterey", "arm64_big_sur", "all"}},
		{"unknown version tries everything", "darwin", "arm64", 0, []string{"arm64_golden_gate", "arm64_tahoe", "arm64_sequoia", "arm64_sonoma", "arm64_ventura", "arm64_monterey", "arm64_big_sur", "all"}},
		{"linux amd64", "linux", "amd64", 0, []string{"x86_64_linux", "all"}},
		{"linux arm64 uses Homebrew's tag", "linux", "arm64", 0, []string{"arm64_linux", "all"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bottleTags(c.goos, c.goarch, c.macMajor); !slices.Equal(got, c.want) {
				t.Errorf("got %v\nwant %v", got, c.want)
			}
		})
	}
}

func TestParseMacOSMajor(t *testing.T) {
	cases := map[string]int{
		"27.0.1":  27,
		"26.1":    26,
		"15.6":    15,
		"16.0":    26,
		"10.16":   11,
		"":        0,
		"garbage": 0,
	}
	for in, want := range cases {
		if got := parseMacOSMajor(in); got != want {
			t.Errorf("parseMacOSMajor(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestToFormulaWithoutBottleHasNoURL(t *testing.T) {
	h := New(t.TempDir())
	f := &Formulae{Name: "portable-zlib"}
	if got := h.toFormula(f); got.URL != "" {
		t.Errorf("formula without bottle got URL %q, want empty", got.URL)
	}
}
