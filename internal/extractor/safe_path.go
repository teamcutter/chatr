package extractor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// archiveWriter writes archive entries beneath a single directory and refuses
// any entry that would land outside of it. It is backed by os.Root, so
// traversal through "..", absolute names, and symlinks that point outside the
// destination are all rejected by the kernel-level checks rather than by
// string matching on entry names.
type archiveWriter struct {
	root *os.Root
}

func openArchiveWriter(dst string) (*archiveWriter, error) {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return nil, err
	}
	return &archiveWriter{root: root}, nil
}

func (w *archiveWriter) Close() error {
	return w.root.Close()
}

// cleanEntryName normalises an archive entry name to a relative, slash-free
// path inside the destination. It returns ok=false for entries that should be
// skipped (the archive root itself) and an error for entries that try to
// escape.
func cleanEntryName(name string) (string, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "./" {
		return "", false, nil
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", false, fmt.Errorf("invalid path in archive: %s", name)
	}

	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." {
		return "", false, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("invalid path in archive: %s", name)
	}
	return clean, true, nil
}

func (w *archiveWriter) Mkdir(name string) error {
	rel, ok, err := cleanEntryName(name)
	if err != nil || !ok {
		return err
	}
	return w.root.MkdirAll(rel, 0755)
}

// WriteFile creates a regular file from r. Only permission bits are honoured;
// setuid, setgid and sticky bits from the archive are dropped.
func (w *archiveWriter) WriteFile(name string, mode fs.FileMode, r io.Reader) error {
	rel, ok, err := cleanEntryName(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid path in archive: %s", name)
	}

	if dir := filepath.Dir(rel); dir != "." {
		if err := w.root.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	perm := mode.Perm()
	if perm == 0 {
		perm = 0644
	}

	f, err := w.root.OpenFile(rel, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Symlink creates name pointing at target. The target itself is stored as-is
// (bottles legitimately carry absolute and parent-relative links); escaping is
// prevented when something later tries to write through the link, because the
// root refuses to follow symlinks that resolve outside the destination.
func (w *archiveWriter) Symlink(target, name string) error {
	rel, ok, err := cleanEntryName(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid path in archive: %s", name)
	}

	if dir := filepath.Dir(rel); dir != "." {
		if err := w.root.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	_ = w.root.Remove(rel)
	return w.root.Symlink(target, rel)
}

func (w *archiveWriter) RemoveAll(name string) error {
	rel, ok, err := cleanEntryName(name)
	if err != nil || !ok {
		return err
	}
	return w.root.RemoveAll(rel)
}
