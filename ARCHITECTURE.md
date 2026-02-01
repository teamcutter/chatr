# Chatr Architecture

Clean/hexagonal architecture for a production-grade package manager.

## Project Structure

```
chatr/
├── cmd/chatr/main.go          # Entry point
├── internal/
│   ├── domain/                # Core (zero dependencies)
│   │   ├── models.go          # Package, Manifest, Formula
│   │   └── interfaces.go      # Fetcher, Cache, Extractor, State
│   ├── fetcher/               # HTTP download + SHA256 verification
│   │   └── fetcher.go         # HTTPFetcher implements domain.Fetcher
│   ├── cache/                 # Download cache
│   │   └── cache.go           # DiskCache implements domain.Cache
│   ├── extractor/             # Archive extraction
│   │   └── tar.go             # TARExtractor implements domain.Extractor
│   ├── state/                 # Installed packages tracking
│   │   └── manifest.go        # ManifestState implements domain.State
│   ├── manager/               # Orchestration
│   │   └── service.go         # Wires everything together
│   ├── config/                # User configuration
│   │   └── config.go          # TOML config loader
│   └── cli/                   # Cobra commands
│       ├── root.go
│       ├── install.go
│       ├── uninstall.go
│       └── list.go
└── go.mod
```

## Dependency Direction

```
         cmd/chatr
              │
              ▼
          internal/cli
              │
              ▼
        internal/manager
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
 fetcher   cache    extractor
 state     config
    │         │         │
    └─────────┼─────────┘
              ▼
       internal/domain  ◄── No external dependencies
```

All arrows point inward. Domain has zero imports.

---

## Domain Layer

### models.go

```go
type Package struct {
    Name        string
    Version     string
    DownloadURL string
    SHA256      string
}

type FetchResult struct {
    Package string
    Version string
    Path    string
    Error   error
}

type InstalledPackage struct {
    Name        string    `json:"name"`
    Version     string    `json:"version"`
    URL         string    `json:"url"`
    Path        string    `json:"path"`
    InstalledAt time.Time `json:"installed_at"`
}

type Manifest struct {
    Packages map[string]InstalledPackage `json:"packages"`
}
```

### interfaces.go

```go
type Fetcher interface {
    Fetch(ctx context.Context, pkg Package) FetchResult
}

type Cache interface {
    Has(name, version string) bool
    GetPath(name, version string) string
    Store(name, version, src string) (string, error)
}

type Extractor interface {
    Extract(src, dest string) error
}

type State interface {
    Load() (*Manifest, error)
    Save(m *Manifest) error
    IsInstalled(name string) (bool, *InstalledPackage, error)
    Add(pkg InstalledPackage) error
    Remove(name string) error
}
```

---

## Infrastructure Layer

### fetcher/fetcher.go

```go
type HTTPFetcher struct {
    client    *http.Client
    outputDir string
}

func (f *HTTPFetcher) Fetch(ctx context.Context, pkg domain.Package) domain.FetchResult {
    // 1. Create HTTP request with context
    // 2. Download to outputDir with progress bar
    // 3. If pkg.SHA256 != "", verify checksum
    // 4. Return path or error
}
```

### cache/cache.go

```go
type DiskCache struct {
    dir string
}

func (c *DiskCache) Has(name, version string) bool {
    // Check if ~/.chatr/cache/{name}/{version}/package.tar.gz exists
}

func (c *DiskCache) Store(name, version, src string) (string, error) {
    // Move downloaded file to cache directory
}
```

### extractor/tar.go

```go
type TARExtractor struct{}

func (e *TARExtractor) Extract(src, dest string) error {
    // 1. Open tar.gz file
    // 2. Validate paths (no ".." for security)
    // 3. Extract files preserving permissions
}
```

### state/manifest.go

```go
type ManifestState struct {
    path string  // ~/.chatr/installed.json
}

func (m *ManifestState) Load() (*domain.Manifest, error) {
    // Read and parse JSON, return empty manifest if not exists
}

func (m *ManifestState) Add(pkg domain.InstalledPackage) error {
    // Load -> add package -> save
}
```

---

## Application Layer

### manager/service.go

```go
type Manager struct {
    fetcher     domain.Fetcher
    cache       domain.Cache
    extractor   domain.Extractor
    state       domain.State
    packagesDir string
    binDir      string
}

func (m *Manager) Install(ctx context.Context, pkg domain.Package) error {
    // 1. Check if already installed
    if installed, _, _ := m.state.IsInstalled(pkg.Name); installed {
        return ErrAlreadyInstalled
    }

    // 2. Check cache
    var archivePath string
    if m.cache.Has(pkg.Name, pkg.Version) {
        archivePath = m.cache.GetPath(pkg.Name, pkg.Version)
    } else {
        // 3. Fetch from URL (includes SHA256 verification)
        result := m.fetcher.Fetch(ctx, pkg)
        if result.Error != nil {
            return result.Error
        }
        // 4. Store in cache
        archivePath, _ = m.cache.Store(pkg.Name, pkg.Version, result.Path)
    }

    // 5. Extract to packages dir
    extractDir := filepath.Join(m.packagesDir, pkg.Name, pkg.Version)
    if err := m.extractor.Extract(archivePath, extractDir); err != nil {
        return err
    }

    // 6. Create symlinks in bin/
    m.createSymlinks(extractDir)

    // 7. Update state
    return m.state.Add(domain.InstalledPackage{
        Name:        pkg.Name,
        Version:     pkg.Version,
        URL:         pkg.DownloadURL,
        Path:        extractDir,
        InstalledAt: time.Now(),
    })
}

func (m *Manager) Uninstall(name string) error {
    // 1. Check if installed
    // 2. Remove package directory
    // 3. Remove symlinks
    // 4. Update state
}

func (m *Manager) List() ([]domain.InstalledPackage, error) {
    // Load manifest and return all packages
}
```

---

## CLI Layer

### cli/root.go

```go
func Execute() error {
    rootCmd := &cobra.Command{Use: "chatr"}
    rootCmd.AddCommand(
        newInstallCmd(),
        newUninstallCmd(),
        newListCmd(),
    )
    return rootCmd.Execute()
}

func newManager() (*manager.Manager, error) {
    cfg, _ := config.Load()

    return manager.New(manager.Config{
        Fetcher:     fetcher.New(cfg.CacheDir, 30*time.Second),
        Cache:       cache.New(cfg.CacheDir),
        Extractor:   extractor.New(),
        State:       state.New(cfg.ChatrDir),
        PackagesDir: cfg.PackagesDir,
        BinDir:      cfg.BinDir,
    }), nil
}
```

### cli/install.go

```go
func newInstallCmd() *cobra.Command {
    var version, sha256 string

    cmd := &cobra.Command{
        Use:  "install <name> <url>",
        Args: cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr, _ := newManager()
            return mgr.Install(cmd.Context(), domain.Package{
                Name:        args[0],
                DownloadURL: args[1],
                Version:     version,
                SHA256:      sha256,
            })
        },
    }

    cmd.Flags().StringVarP(&version, "version", "v", "latest", "Package version")
    cmd.Flags().StringVar(&sha256, "sha256", "", "Expected SHA256 checksum")
    return cmd
}
```

---

## Install Flow

```
chatr install jq https://... --version 1.7.1 --sha256 abc123
                │
                ▼
        ┌───────────────┐
        │ state.        │
        │ IsInstalled?  │──yes──▶ Error: already installed
        └───────┬───────┘
                │ no
                ▼
        ┌───────────────┐
        │ cache.Has?    │──yes──▶ Use cached archive
        └───────┬───────┘
                │ no
                ▼
        ┌───────────────┐
        │ fetcher.Fetch │
        │ (download +   │
        │  SHA256)      │
        └───────┬───────┘
                │
                ▼
        ┌───────────────┐
        │ cache.Store   │
        └───────┬───────┘
                │
                ▼
        ┌───────────────┐
        │ extractor.    │
        │ Extract       │
        └───────┬───────┘
                │
                ▼
        ┌───────────────┐
        │ Create bin/   │
        │ symlinks      │
        └───────┬───────┘
                │
                ▼
        ┌───────────────┐
        │ state.Add     │
        └───────────────┘
```

---

## File Layout

```
~/.chatr/
├── bin/                    # Symlinks (add to PATH)
│   ├── jq -> ../packages/jq/1.7.1/jq
│   └── rg -> ../packages/rg/14.1.0/rg
├── packages/               # Extracted binaries
│   └── jq/
│       └── 1.7.1/
│           └── jq
├── cache/                  # Downloaded archives
│   └── jq/
│       └── 1.7.1/
│           └── package.tar.gz
├── installed.json          # Package database
└── config.toml             # Configuration
```

---

## Why This Architecture?

| Benefit | How |
|---------|-----|
| Testable | Mock interfaces in unit tests |
| Swappable | Replace HTTPFetcher with S3Fetcher |
| Single responsibility | Each package does one thing |
| No circular deps | Domain has zero imports |
