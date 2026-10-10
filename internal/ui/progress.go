package ui

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/term"
)

const (
	frameInterval = 80 * time.Millisecond
	minBarWidth   = 10
	maxBarWidth   = 30
	maxNameWidth  = 32
	speedWindow   = 500 * time.Millisecond
)

var (
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cyan          = color.New(color.FgCyan).SprintFunc()
	dim           = color.New(color.Faint).SprintFunc()
	bold          = color.New(color.Bold).SprintFunc()
)

type Renderer struct {
	mu       sync.Mutex
	w        io.Writer
	tty      bool
	width    func() int
	entries  []*entry
	rendered int
	stop     chan struct{}
	frame    int
}

type entry struct {
	name    string
	total   int64
	done    atomic.Int64
	spinner bool

	lastBytes int64
	lastTime  time.Time
	speed     float64
}

type tracker struct {
	r *Renderer
	e *entry
}

func (t *tracker) Add(n int64) { t.e.done.Add(n) }
func (t *tracker) Done()       { t.r.remove(t.e) }

func New(w io.Writer) *Renderer {
	r := &Renderer{w: w, width: func() int { return 80 }}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) && os.Getenv("TERM") != "dumb" {
		r.tty = true
		r.width = func() int {
			if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
				return cols
			}
			return 80
		}
	}
	return r
}

func (r *Renderer) Start(name string, total int64) domain.Tracker {
	e := r.add(name, total, false)
	if !r.tty {
		if total > 0 {
			fmt.Fprintf(r.w, "Downloading %s (%s)\n", name, formatBytes(total))
		} else {
			fmt.Fprintf(r.w, "Downloading %s\n", name)
		}
	}
	return &tracker{r: r, e: e}
}

func (r *Renderer) Spin(desc string) func() {
	e := r.add(desc, -1, true)
	var once sync.Once
	return func() { once.Do(func() { r.remove(e) }) }
}

func (r *Renderer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.tty || r.rendered == 0 {
		return r.w.Write(p)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[%dA\r\x1b[0J", r.rendered)
	b.Write(p)
	if len(p) > 0 && p[len(p)-1] != '\n' {
		b.WriteByte('\n')
	}
	io.WriteString(r.w, b.String())
	r.rendered = 0
	r.draw()
	return len(p), nil
}

func (r *Renderer) add(name string, total int64, spinner bool) *entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := &entry{name: name, total: total, spinner: spinner, lastTime: time.Now()}
	r.entries = append(r.entries, e)
	if r.tty && r.stop == nil {
		r.startLoop()
	}
	return e
}

func (r *Renderer) remove(e *entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := slices.Index(r.entries, e); i >= 0 {
		r.entries = slices.Delete(r.entries, i, i+1)
	}
	if len(r.entries) == 0 && r.stop != nil {
		close(r.stop)
		r.stop = nil
		r.draw()
	}
}

func (r *Renderer) startLoop() {
	stop := make(chan struct{})
	r.stop = stop
	r.draw()
	go func() {
		ticker := time.NewTicker(frameInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r.mu.Lock()
				if r.stop == stop {
					r.draw()
				}
				r.mu.Unlock()
			}
		}
	}()
}

func (r *Renderer) draw() {
	lines := r.lines(time.Now())
	var b strings.Builder
	if r.rendered > 0 {
		fmt.Fprintf(&b, "\x1b[%dA", r.rendered)
	}
	b.WriteString("\r")
	for _, l := range lines {
		b.WriteString("\x1b[2K")
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString("\x1b[0J")
	io.WriteString(r.w, b.String())
	r.rendered = len(lines)
	r.frame++
}

func (r *Renderer) lines(now time.Time) []string {
	glyph := spinnerFrames[r.frame%len(spinnerFrames)]
	width := r.width()

	var downloads, spinners []*entry
	nameW := 0
	for _, e := range r.entries {
		if e.spinner {
			spinners = append(spinners, e)
			continue
		}
		downloads = append(downloads, e)
		nameW = max(nameW, len(e.name))
	}
	nameW = min(nameW, maxNameWidth)

	var out []string
	for _, e := range spinners {
		out = append(out, fmt.Sprintf("%s %s", cyan(glyph), e.name))
	}
	if len(downloads) == 0 {
		return out
	}

	noun := "packages"
	if len(downloads) == 1 {
		noun = "package"
	}
	out = append(out, fmt.Sprintf("%s %s %s", cyan(glyph), bold("Downloading"), dim(fmt.Sprintf("%d %s", len(downloads), noun))))

	for _, e := range downloads {
		e.sample(now)
		done := e.done.Load()
		name := truncate(e.name, nameW)

		var stats string
		if e.total > 0 {
			stats = fmt.Sprintf("%s/%s", formatBytes(done), formatBytes(e.total))
		} else {
			stats = formatBytes(done)
		}
		if e.speed > 0 {
			stats += "  " + formatBytes(int64(e.speed)) + "/s"
		}

		barW := width - 2 - nameW - 2 - 2 - len(stats) - 1
		barW = max(minBarWidth, min(maxBarWidth, barW))

		var bar string
		if e.total > 0 {
			filled := int(float64(barW) * float64(done) / float64(e.total))
			filled = max(0, min(barW, filled))
			bar = cyan(strings.Repeat("━", filled)) + dim(strings.Repeat("━", barW-filled))
		} else {
			pos := r.frame % barW
			bar = dim(strings.Repeat("━", pos)) + cyan("━") + dim(strings.Repeat("━", barW-pos-1))
		}

		out = append(out, fmt.Sprintf("  %-*s  %s  %s", nameW, name, bar, dim(stats)))
	}
	return out
}

func (e *entry) sample(now time.Time) {
	elapsed := now.Sub(e.lastTime)
	if elapsed < speedWindow {
		return
	}
	done := e.done.Load()
	inst := float64(done-e.lastBytes) / elapsed.Seconds()
	if e.speed == 0 {
		e.speed = inst
	} else {
		e.speed = 0.7*e.speed + 0.3*inst
	}
	e.lastBytes = done
	e.lastTime = now
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func formatBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	val := float64(n)
	for _, suffix := range []string{"kB", "MB", "GB", "TB"} {
		val /= unit
		if val < unit {
			return fmt.Sprintf("%.1f %s", val, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", val/unit)
}
