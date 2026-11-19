package linker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Prefix links point to individual files in the Cellar. Their targets record
// ownership, including for installations made before recursive linking.
func (l *Linker) LinkToPrefix(name, version string) (_ []string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var undo []func() error
	defer func() {
		if err != nil {
			for i := len(undo) - 1; i >= 0; i-- {
				err = errors.Join(err, undo[i]())
			}
		}
	}()
	var dirs []string
	for dir := range l.prefixDirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	var linked []string
	for _, dir := range dirs {
		src := filepath.Join(l.cellarDir, name, version, dir)
		if _, err := os.Lstat(src); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		if err := l.mergePrefix(src, l.prefixDirs[dir], &undo); err != nil {
			return nil, err
		}
		linked = append(linked, dir)
	}
	return linked, nil
}

func (l *Linker) mergePrefix(src, dst string, undo *[]func() error) error {
	source, err := os.Lstat(src)
	if err != nil {
		return err
	}
	target, err := os.Lstat(dst)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !source.IsDir() {
		if target != nil {
			if target.Mode()&os.ModeSymlink != 0 {
				old, err := linkTarget(dst)
				if err != nil {
					return err
				}
				absSrc, err := filepath.Abs(src)
				if err != nil {
					return err
				}
				if old == absSrc {
					return nil
				}
			}
			return fmt.Errorf("link conflict at %s: already exists (installing %s)", dst, src)
		}
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(absSrc, dst); err != nil {
			return err
		}
		*undo = append(*undo, func() error { return os.Remove(dst) })
		return nil
	}

	if target != nil && target.Mode()&os.ModeSymlink != 0 {
		// Expand a legacy directory link without writing through it into the
		// other package's keg. Never expand links to directories outside Cellar.
		old, err := linkTarget(dst)
		if err != nil {
			return err
		}
		resolved, err := filepath.EvalSymlinks(old)
		if err != nil {
			return fmt.Errorf("link conflict at %s: %w", dst, err)
		}
		cellar, err := filepath.EvalSymlinks(l.cellarDir)
		if err != nil {
			return err
		}
		info, err := os.Stat(old)
		if err != nil {
			return err
		}
		if !info.IsDir() || !within(cellar, resolved) {
			return fmt.Errorf("link conflict at %s: not a Cellar directory link", dst)
		}
		raw, err := os.Readlink(dst)
		if err != nil {
			return err
		}
		if err := os.Remove(dst); err != nil {
			return err
		}
		*undo = append(*undo, func() error { return os.Symlink(raw, dst) })
		if err := os.Mkdir(dst, 0755); err != nil {
			return err
		}
		*undo = append(*undo, func() error { return os.Remove(dst) })
		if err := l.mergeChildren(old, dst, undo); err != nil {
			return err
		}
	} else if target != nil && !target.IsDir() {
		return fmt.Errorf("link conflict at %s: expected a directory", dst)
	} else if target == nil {
		if err := os.Mkdir(dst, 0755); err != nil {
			return err
		}
		*undo = append(*undo, func() error { return os.Remove(dst) })
	}
	return l.mergeChildren(src, dst, undo)
}

func (l *Linker) mergeChildren(src, dst string, undo *[]func() error) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := l.mergePrefix(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), undo); err != nil {
			return err
		}
	}
	return nil
}

func linkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Abs(target)
}

func within(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (l *Linker) UnlinkFromPrefix(name string, linkedDirs []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	owner, err := filepath.Abs(filepath.Join(l.cellarDir, name))
	if err != nil {
		return err
	}
	if len(linkedDirs) == 0 {
		for dir := range l.prefixDirs {
			linkedDirs = append(linkedDirs, dir)
		}
	}
	var errs []error
	for _, dir := range linkedDirs {
		root, ok := l.prefixDirs[dir]
		if !ok {
			continue
		}
		// WalkDir does not follow symlinks: remove our links, never their targets.
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink == 0 {
				return nil
			}
			target, err := linkTarget(path)
			if err != nil {
				return err
			}
			if within(owner, target) {
				return os.Remove(path)
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
