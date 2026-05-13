package linker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Linker struct {
	cellarDir  string
	optDir     string
	prefixDirs map[string]string
}

func New(cellarDir, optDir string, prefixDirs map[string]string) *Linker {
	return &Linker{
		cellarDir:  cellarDir,
		optDir:     optDir,
		prefixDirs: prefixDirs,
	}
}

func (l *Linker) CreateOptLink(name, version string) error {
	optLink := filepath.Join(l.optDir, name)
	cellarPath := filepath.Join(l.cellarDir, name, version)

	if err := os.MkdirAll(filepath.Dir(optLink), 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(cellarPath, 0755); err != nil {
		return err
	}

	target := filepath.Join("..", "Cellar", name, version)
	relativeTarget, err := filepath.Rel(filepath.Dir(optLink), filepath.Join(l.optDir, target))
	if err != nil {
		return err
	}

	if _, err := os.Lstat(optLink); err == nil {
		os.Remove(optLink)
	}

	return os.Symlink(relativeTarget, optLink)
}

func (l *Linker) RemoveOptLink(name string) error {
	optLink := filepath.Join(l.optDir, name)
	if _, err := os.Lstat(optLink); err == nil {
		return os.Remove(optLink)
	}
	return nil
}

func (l *Linker) LinkToPrefix(name, version string) ([]string, error) {
	cellarPath := filepath.Join(l.cellarDir, name, version)
	seenDirs := make(map[string]bool)
	var linked []string

	for dirName, prefixDir := range l.prefixDirs {
		cellarSubdir := filepath.Join(cellarPath, dirName)
		if _, err := os.Stat(cellarSubdir); os.IsNotExist(err) {
			continue
		}

		if err := os.MkdirAll(prefixDir, 0755); err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(cellarSubdir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			entryName := entry.Name()
			src := filepath.Join(cellarSubdir, entryName)
			linkPath := filepath.Join(prefixDir, entryName)

			if _, err := os.Lstat(linkPath); err == nil {
				os.Remove(linkPath)
			}

			if err := os.Symlink(src, linkPath); err != nil {
				continue
			}

			if !seenDirs[dirName] {
				seenDirs[dirName] = true
				linked = append(linked, dirName)
			}
		}
	}

	return linked, nil
}

func (l *Linker) UnlinkFromPrefix(name string, linkedDirs []string) error {
	cellarPath := filepath.Join(l.cellarDir, name)
	absCellar, err := filepath.Abs(cellarPath)
	if err != nil {
		return err
	}

	dirs := l.prefixDirs
	if len(linkedDirs) > 0 {
		dirs = make(map[string]string)
		for _, d := range linkedDirs {
			if prefixDir, ok := l.prefixDirs[d]; ok {
				dirs[d] = prefixDir
			}
		}
	}

	for _, prefixDir := range dirs {
		entries, err := os.ReadDir(prefixDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			linkPath := filepath.Join(prefixDir, entry.Name())

			linkTarget, err := os.Readlink(linkPath)
			if err != nil {
				continue
			}

			absTarget, err := filepath.Abs(linkTarget)
			if err != nil {
				continue
			}

			if strings.HasPrefix(absTarget, absCellar+string(os.PathSeparator)) {
				os.Remove(linkPath)
			}
		}
	}

	return nil
}

func (l *Linker) Relocate(pkgPath, chatrPrefix string) error {
	chatrCellar := filepath.Join(chatrPrefix, "Cellar")

	cellarPrefixes := []string{
		"/opt/homebrew/Cellar",
		"/usr/local/Cellar",
		"/home/linuxbrew/.linuxbrew/Cellar",
	}
	homebrewPrefixes := []string{
		"/opt/homebrew",
		"/usr/local",
		"/home/linuxbrew/.linuxbrew",
	}

	placeholders := map[string]string{
		"@@HOMEBREW_PREFIX@@": chatrPrefix,
		"@@HOMEBREW_CELLAR@@": chatrCellar,
	}

	for dirName := range l.prefixDirs {
		dirPath := filepath.Join(pkgPath, dirName)
		l.walkAndReplace(dirPath, placeholders, cellarPrefixes, homebrewPrefixes, chatrPrefix, chatrCellar)
	}

	binPath := filepath.Join(pkgPath, "bin")
	l.walkAndReplace(binPath, placeholders, cellarPrefixes, homebrewPrefixes, chatrPrefix, chatrCellar)
	l.patchRpath(pkgPath)

	return nil
}

func (l *Linker) walkAndReplace(dirPath string, placeholders map[string]string, cellarPrefixes, homebrewPrefixes []string, chatrPrefix, chatrCellar string) error {
	return filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !l.isTextFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		modified := false
		str := string(content)

		for placeholder, replacement := range placeholders {
			if strings.Contains(str, placeholder) {
				str = strings.ReplaceAll(str, placeholder, replacement)
				modified = true
			}
		}

		for _, cellarPrefix := range cellarPrefixes {
			if strings.Contains(str, cellarPrefix) {
				str = strings.ReplaceAll(str, cellarPrefix, chatrCellar)
				modified = true
			}
		}

		for _, homebrewPrefix := range homebrewPrefixes {
			if strings.Contains(str, homebrewPrefix) {
				str = strings.ReplaceAll(str, homebrewPrefix, chatrPrefix)
				modified = true
			}
		}

		if modified {
			os.WriteFile(path, []byte(str), 0644)
		}

		return nil
	})
}

func (l *Linker) isTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	if n == 0 {
		return false
	}

	if bytes.Contains(buffer[:n], []byte{0x00}) {
		return false
	}

	return true
}

func (l *Linker) patchRpath(pkgPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return l.patchDarwin(pkgPath)
	case "linux":
		return l.patchLinux(pkgPath)
	}
	return nil
}

func (l *Linker) patchDarwin(pkgPath string) error {
	binDirs := []string{
		filepath.Join(pkgPath, "bin"),
		filepath.Join(pkgPath, "libexec", "bin"),
		filepath.Join(pkgPath, "libexec"),
	}

	var binPaths []string
	for _, binDir := range binDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			binPaths = append(binPaths, filepath.Join(binDir, entry.Name()))
		}
	}

	libDir := filepath.Join(pkgPath, "lib")
	entries, err := os.ReadDir(libDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			binPaths = append(binPaths, filepath.Join(libDir, entry.Name()))
		}
	}

	// Scan Frameworks for binaries and dylibs (e.g., Python.framework)
	frameworksDir := filepath.Join(pkgPath, "Frameworks")
	filepath.WalkDir(frameworksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Mode()&0111 != 0 {
			binPaths = append(binPaths, path)
		}
		return nil
	})

	// Collect rpaths: prefix lib + package's own lib + framework version dirs
	rpaths := l.collectRpaths(pkgPath)

	for _, path := range binPaths {
		l.patchDarwinBinary(path, rpaths)
	}

	return nil
}

func (l *Linker) collectRpaths(pkgPath string) []string {
	seen := make(map[string]bool)
	var rpaths []string

	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			rpaths = append(rpaths, p)
		}
	}

	// Prefix lib (for dependencies linked to prefix)
	add(l.prefixDirs["lib"])

	// Package's own lib
	pkgLib := filepath.Join(pkgPath, "lib")
	if _, err := os.Stat(pkgLib); err == nil {
		add(pkgLib)
	}

	// Framework version dirs (e.g., Frameworks/Python.framework/Versions/3.12/)
	frameworksDir := filepath.Join(pkgPath, "Frameworks")
	entries, err := os.ReadDir(frameworksDir)
	if err != nil {
		return rpaths
	}
	for _, fw := range entries {
		if !fw.IsDir() {
			continue
		}
		versionsDir := filepath.Join(frameworksDir, fw.Name(), "Versions")
		versions, err := os.ReadDir(versionsDir)
		if err != nil {
			continue
		}
		for _, ver := range versions {
			if !ver.IsDir() {
				continue
			}
			add(filepath.Join(versionsDir, ver.Name()))
		}
	}

	return rpaths
}

func (l *Linker) patchDarwinBinary(path string, rpaths []string) error {
	out, err := exec.Command("otool", "-L", path).Output()
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, " (compatibility") {
			continue
		}
		libRef := strings.TrimSpace(strings.Split(line, " (compatibility")[0])

		if strings.HasPrefix(libRef, "/usr/lib/") ||
			strings.HasPrefix(libRef, "/System/") ||
			strings.HasPrefix(libRef, "@rpath/") ||
			strings.HasPrefix(libRef, "@loader_path/") ||
			strings.HasPrefix(libRef, "@executable_path/") {
			continue
		}

		newRef := "@rpath/" + filepath.Base(libRef)
		exec.Command("install_name_tool", "-change", libRef, newRef, path).Run()
	}

	for _, rpath := range rpaths {
		exec.Command("install_name_tool", "-add_rpath", rpath, path).Run()
	}

	exec.Command("codesign", "--force", "--sign", "-", path).Run()

	return nil
}

func (l *Linker) patchLinux(pkgPath string) error {
	if _, err := exec.LookPath("patchelf"); err != nil {
		fmt.Fprintln(os.Stderr, "warning: patchelf not found, binaries may not work")
		return nil
	}

	binDirs := []string{
		filepath.Join(pkgPath, "bin"),
		filepath.Join(pkgPath, "libexec", "bin"),
		filepath.Join(pkgPath, "libexec"),
	}

	var binPaths []string
	for _, binDir := range binDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			binPaths = append(binPaths, filepath.Join(binDir, entry.Name()))
		}
	}

	interp := l.findSystemInterpreter()
	if interp != "" {
		for _, path := range binPaths {
			exec.Command("patchelf", "--set-interpreter", interp, path).Run()
		}
	}

	for _, path := range binPaths {
		exec.Command("patchelf", "--set-rpath", l.prefixDirs["lib"], path).Run()
	}

	return nil
}

func (l *Linker) findSystemInterpreter() string {
	out, err := exec.Command("patchelf", "--print-interpreter", "/bin/sh").Output()
	if err == nil {
		if interp := strings.TrimSpace(string(out)); interp != "" {
			return interp
		}
	}
	return ""
}

func (l *Linker) CellarPath(name, version string) string {
	return filepath.Join(l.cellarDir, name, version)
}

func (l *Linker) OptPath(name string) string {
	return filepath.Join(l.optDir, name)
}

func (l *Linker) PrefixPath() string {
	return filepath.Dir(l.optDir)
}
