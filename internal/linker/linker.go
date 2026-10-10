package linker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
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

// BuildPrefix is the prefix Homebrew bottles for this platform are built
// against, and therefore the only prefix relocation rewrites.
func BuildPrefix() string {
	switch {
	case runtime.GOOS == "linux":
		return "/home/linuxbrew/.linuxbrew"
	case runtime.GOARCH == "arm64":
		return "/opt/homebrew"
	default:
		return "/usr/local"
	}
}

// CanRelocateBinaries reports whether paths embedded in bottle binaries fit
// when rewritten to prefix.
func CanRelocateBinaries(prefix string) bool {
	return len(prefix) <= len(BuildPrefix())
}

type RelocateOptions struct {
	Name         string
	Dependencies []string
}

func (l *Linker) Relocate(pkgPath, chatrPrefix string, opts RelocateOptions) error {
	buildPrefix := BuildPrefix()
	chatrCellar := filepath.Join(chatrPrefix, "Cellar")

	replacer := strings.NewReplacer(
		"@@HOMEBREW_PREFIX@@", chatrPrefix,
		"@@HOMEBREW_CELLAR@@", chatrCellar,
		"@@HOMEBREW_REPOSITORY@@", chatrPrefix,
		"@@HOMEBREW_LIBRARY@@", filepath.Join(chatrPrefix, "Library"),
		"@@HOMEBREW_PERL@@", perlPath(chatrPrefix, opts.Name, opts.Dependencies),
		"@@HOMEBREW_JAVA@@", javaHome(chatrPrefix, opts.Dependencies, runtime.GOOS),
		buildPrefix+"/Cellar", chatrCellar,
		buildPrefix, chatrPrefix,
	)

	filepath.WalkDir(pkgPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		relocateTextFile(path, replacer)
		return nil
	})

	modified := patchBinaryStrings(pkgPath, buildPrefix, chatrPrefix)
	l.patchRpath(pkgPath)
	if runtime.GOOS == "darwin" {
		resign(modified)
	}

	return nil
}

func perlPath(prefix, name string, deps []string) string {
	if name == "perl" || slices.Contains(deps, "perl") {
		return filepath.Join(prefix, "opt", "perl", "bin", "perl")
	}
	return "/usr/bin/perl"
}

func javaHome(prefix string, deps []string, goos string) string {
	jdk := "openjdk"
	for _, d := range deps {
		if d == "openjdk" || strings.HasPrefix(d, "openjdk@") {
			jdk = d
			break
		}
	}
	home := filepath.Join(prefix, "opt", jdk, "libexec")
	if goos == "darwin" {
		home = filepath.Join(home, "openjdk.jdk", "Contents", "Home")
	}
	return home
}

func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(512, len(content))], 0) >= 0
}

func relocateTextFile(path string, replacer *strings.Replacer) {
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 || isBinary(content) {
		return
	}
	updated := replacer.Replace(string(content))
	if updated == string(content) {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	os.WriteFile(path, []byte(updated), info.Mode())
}

// patchBinaryStrings rewrites NUL-terminated strings that mention the build
// prefix. Cellar paths become version-agnostic opt paths and the prefix
// becomes chatr's. A string is only replaced when the result fits, padded
// with NULs, in the original space. It returns the files it changed.
func patchBinaryStrings(pkgPath, buildPrefix, chatrPrefix string) []string {
	chatrOpt := filepath.Join(chatrPrefix, "opt")
	needle := []byte(buildPrefix)
	var modified []string

	filepath.WalkDir(pkgPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 || !isBinary(content) {
			return nil
		}

		changed := false
		for off := 0; ; {
			i := bytes.Index(content[off:], needle)
			if i < 0 {
				break
			}
			start := off + i
			end := len(content)
			if n := bytes.IndexByte(content[start:], 0); n >= 0 {
				end = start + n
			}
			old := string(content[start:end])
			updated := rewritePath(old, buildPrefix, chatrPrefix, chatrOpt)
			if updated != old && len(updated) <= len(old) {
				copy(content[start:end], updated)
				clear(content[start+len(updated) : end])
				changed = true
			}
			off = end
		}

		if changed {
			info, err := os.Stat(path)
			if err != nil {
				return nil
			}
			if os.WriteFile(path, content, info.Mode()) == nil {
				modified = append(modified, path)
			}
		}
		return nil
	})

	return modified
}

// rewritePath maps every build-prefix path in s to chatr. Cellar paths
// drop their version and go through opt, e.g.
// /opt/homebrew/Cellar/python@3.13/3.13.1/Frameworks → <prefix>/opt/python@3.13/Frameworks.
func rewritePath(s, buildPrefix, chatrPrefix, chatrOpt string) string {
	cellar := buildPrefix + "/Cellar/"
	var b strings.Builder
	for {
		i := strings.Index(s, cellar)
		if i < 0 {
			break
		}
		b.WriteString(s[:i])
		rest := s[i+len(cellar):]
		name, afterName, _ := strings.Cut(rest, "/")
		b.WriteString(chatrOpt + "/" + name)
		if _, afterVersion, ok := strings.Cut(afterName, "/"); ok {
			s = "/" + afterVersion
		} else {
			s = ""
		}
	}
	b.WriteString(s)
	return strings.ReplaceAll(b.String(), buildPrefix, chatrPrefix)
}

func isMachO(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return false
	}
	switch string(magic) {
	case "\xfe\xed\xfa\xce", "\xce\xfa\xed\xfe",
		"\xfe\xed\xfa\xcf", "\xcf\xfa\xed\xfe",
		"\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca":
		return true
	}
	return false
}

// resign restores ad-hoc signatures that string patching invalidated.
// Apple Silicon refuses to load a Mach-O whose signature does not match.
func resign(paths []string) {
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, path := range paths {
		if !isMachO(path) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			exec.Command("codesign", "--force", "--sign", "-", path).Run()
		}()
	}
	wg.Wait()
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

	rpaths := l.collectRpaths(pkgPath)

	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, path := range binPaths {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			l.patchDarwinBinary(path, rpaths)
		}()
	}
	wg.Wait()

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

	pkgLib := filepath.Join(pkgPath, "lib")
	if _, err := os.Stat(pkgLib); err == nil {
		add(pkgLib)
	}

	// Keg-only dependencies (zlib, openssl, etc.) don't link into prefix lib.
	// Add their opt lib dirs so binaries can find them at runtime.
	optEntries, err := os.ReadDir(l.optDir)
	if err == nil {
		for _, entry := range optEntries {
			optLib := filepath.Join(l.optDir, entry.Name(), "lib")
			if _, err := os.Stat(optLib); err == nil {
				add(optLib)
			}
		}
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

	var changeArgs []string
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
		changeArgs = append(changeArgs, "-change", libRef, newRef)
	}

	var args []string
	args = append(args, changeArgs...)
	for _, rpath := range rpaths {
		args = append(args, "-add_rpath", rpath)
	}

	if len(args) > 0 {
		args = append(args, path)
		exec.Command("install_name_tool", args...).Run()
		exec.Command("codesign", "--force", "--sign", "-", path).Run()
	}

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

func (l *Linker) PrefixPath() string {
	return filepath.Dir(l.optDir)
}
