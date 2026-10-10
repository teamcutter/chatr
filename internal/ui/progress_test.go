package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func newTestRenderer(tty bool) (*Renderer, *bytes.Buffer) {
	var buf bytes.Buffer
	r := New(&buf)
	r.tty = tty
	r.width = func() int { return 80 }
	return r, &buf
}

func TestNonTTYPrintsOneLinePerDownload(t *testing.T) {
	r, buf := newTestRenderer(false)

	tr := r.Start("jq", 1500)
	tr.Add(1500)
	tr.Done()

	stop := r.Status("Resolving jq...")
	stop()

	got := buf.String()
	if got != "Downloading jq (1.5 kB)\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestTTYFrameListsEveryActiveDownload(t *testing.T) {
	r, buf := newTestRenderer(true)

	a := r.Start("jq", 1000)
	b := r.Start("oniguruma", 4000)
	a.Add(500)
	b.Add(1000)
	stopStatus := r.Status("Installing pcre2")

	buf.Reset()
	r.mu.Lock()
	r.draw()
	r.mu.Unlock()

	frame := buf.String()
	for _, want := range []string{"Downloading", "2 packages", "jq", "oniguruma", "500 B/1.0 kB", "1.0 kB/4.0 kB", "━"} {
		if !strings.Contains(frame, want) {
			t.Errorf("frame missing %q:\n%s", want, frame)
		}
	}
	if r.rendered != 4 {
		t.Errorf("rendered = %d, want 4", r.rendered)
	}
	if strings.Index(frame, "Installing pcre2") < strings.Index(frame, "oniguruma") {
		t.Errorf("status line should render below downloads:\n%s", frame)
	}

	a.Done()
	b.Done()
	stopStatus()

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) != 0 || r.stop != nil || r.rendered != 0 {
		t.Errorf("renderer not idle after Done: entries=%d stop=%v rendered=%d", len(r.entries), r.stop, r.rendered)
	}
}

func TestTTYWriteInsertsAboveRegion(t *testing.T) {
	r, buf := newTestRenderer(true)

	tr := r.Start("jq", 1000)
	buf.Reset()

	r.Write([]byte("warning: something"))

	got := buf.String()
	if !strings.HasPrefix(got, "\x1b[2A\r\x1b[0Jwarning: something\n") {
		t.Errorf("warning not inserted above region:\n%q", got)
	}
	if !strings.Contains(got[len("\x1b[2A\r\x1b[0Jwarning: something\n"):], "jq") {
		t.Errorf("region not redrawn after warning:\n%q", got)
	}
	tr.Done()
}

func TestSpeedSampling(t *testing.T) {
	e := &entry{lastTime: time.Now().Add(-time.Second)}
	e.done.Store(2000)
	e.sample(time.Now())
	if e.speed < 1900 || e.speed > 2100 {
		t.Errorf("speed = %v, want ~2000", e.speed)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:             "0 B",
		999:           "999 B",
		1000:          "1.0 kB",
		1536:          "1.5 kB",
		12_345_678:    "12.3 MB",
		2_000_000_000: "2.0 GB",
	}
	for n, want := range cases {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
