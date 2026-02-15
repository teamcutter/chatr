package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/teamcutter/chatr/internal/domain"
)

const schema = `
CREATE TABLE IF NOT EXISTS packages (
    name         TEXT PRIMARY KEY,
    version      TEXT NOT NULL,
    revision     TEXT DEFAULT '',
    url          TEXT NOT NULL,
    path         TEXT NOT NULL,
    binaries     TEXT NOT NULL DEFAULT '[]',
    libs         TEXT NOT NULL DEFAULT '[]',
    apps         TEXT NOT NULL DEFAULT '[]',
    dependencies TEXT NOT NULL DEFAULT '[]',
    is_dep       INTEGER NOT NULL DEFAULT 0,
    is_cask      INTEGER NOT NULL DEFAULT 0,
    installed_at TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'installed'
);
`

type SQLiteState struct {
	mu           sync.RWMutex
	db           *sql.DB
	dbPath       string
	manifestPath string
}

func NewSQLite(dbPath, manifestPath string) (*SQLiteState, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	s := &SQLiteState{
		db:           db,
		dbPath:       dbPath,
		manifestPath: manifestPath,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	if err := s.recover(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to recover: %w", err)
	}

	return s, nil
}

func (s *SQLiteState) migrate() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM packages").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if _, err := os.Stat(s.manifestPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest domain.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, pkg := range manifest.Packages {
		if err := s.insertPkg(tx, pkg, "installed"); err != nil {
			return fmt.Errorf("failed to insert %s: %w", pkg.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	backupPath := s.manifestPath + ".bak"
	if err := os.Rename(s.manifestPath, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to backup manifest: %v\n", err)
	}

	return nil
}

func (s *SQLiteState) recover() error {
	rows, err := s.db.Query("SELECT name, path, apps, is_cask FROM packages WHERE status = 'pending'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var pending []struct {
		name   string
		path   string
		apps   string
		isCask bool
	}

	for rows.Next() {
		var p struct {
			name   string
			path   string
			apps   string
			isCask bool
		}
		if err := rows.Scan(&p.name, &p.path, &p.apps, &p.isCask); err != nil {
			return err
		}
		pending = append(pending, p)
	}

	for _, p := range pending {
		fmt.Fprintf(os.Stderr, "recovering from interrupted install: %s\n", p.name)

		if p.isCask {
			var apps []string
			if err := json.Unmarshal([]byte(p.apps), &apps); err == nil {
				for _, app := range apps {
					os.RemoveAll(app)
				}
			}
		} else {
			os.RemoveAll(p.path)
		}

		if _, err := s.db.Exec("DELETE FROM packages WHERE name = ?", p.name); err != nil {
			return fmt.Errorf("failed to delete pending package %s: %w", p.name, err)
		}
	}

	return nil
}

func (s *SQLiteState) insertPkg(tx *sql.Tx, pkg *domain.InstalledPackage, status string) error {
	binaries, _ := json.Marshal(pkg.Binaries)
	libs, _ := json.Marshal(pkg.Libs)
	apps, _ := json.Marshal(pkg.Apps)
	deps, _ := json.Marshal(pkg.Dependencies)

	_, err := tx.Exec(`
		INSERT OR REPLACE INTO packages
		(name, version, revision, url, path, binaries, libs, apps, dependencies, is_dep, is_cask, installed_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pkg.Name, pkg.Version, pkg.Revision, pkg.URL, pkg.Path,
		string(binaries), string(libs), string(apps), string(deps),
		boolToInt(pkg.IsDep), boolToInt(pkg.IsCask),
		pkg.InstalledAt.Format(time.RFC3339), status)
	return err
}

func (s *SQLiteState) Load() (*domain.Manifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pkgs, err := s.listInstalled()
	if err != nil {
		return nil, err
	}

	return &domain.Manifest{Packages: pkgs}, nil
}

func (s *SQLiteState) Save(m *domain.Manifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM packages"); err != nil {
		return err
	}

	for _, pkg := range m.Packages {
		if err := s.insertPkg(tx, pkg, "installed"); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return s.exportJSON()
}

func (s *SQLiteState) IsInstalled(name string) (bool, *domain.InstalledPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pkg, err := s.getPkg(name)
	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, pkg, nil
}

func (s *SQLiteState) getPkg(name string) (*domain.InstalledPackage, error) {
	var pkg domain.InstalledPackage
	var binaries, libs, apps, deps, installedAt, status string
	var isDep, isCask int

	err := s.db.QueryRow(`
		SELECT name, version, revision, url, path, binaries, libs, apps, dependencies,
		       is_dep, is_cask, installed_at, status
		FROM packages WHERE name = ? AND status = 'installed'`, name).Scan(
		&pkg.Name, &pkg.Version, &pkg.Revision, &pkg.URL, &pkg.Path,
		&binaries, &libs, &apps, &deps, &isDep, &isCask, &installedAt, &status)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(binaries), &pkg.Binaries)
	json.Unmarshal([]byte(libs), &pkg.Libs)
	json.Unmarshal([]byte(apps), &pkg.Apps)
	json.Unmarshal([]byte(deps), &pkg.Dependencies)
	pkg.IsDep = isDep == 1
	pkg.IsCask = isCask == 1
	pkg.InstalledAt, _ = time.Parse(time.RFC3339, installedAt)

	return &pkg, nil
}

func (s *SQLiteState) Add(pkg *domain.InstalledPackage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.insertPkg(tx, pkg, "installed"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteState) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM packages WHERE name = ?", name)
	return err
}

func (s *SQLiteState) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exportJSON()
}

func (s *SQLiteState) ListInstalled() (map[string]*domain.InstalledPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listInstalled()
}

func (s *SQLiteState) listInstalled() (map[string]*domain.InstalledPackage, error) {
	rows, err := s.db.Query(`
		SELECT name, version, revision, url, path, binaries, libs, apps, dependencies,
		       is_dep, is_cask, installed_at
		FROM packages WHERE status = 'installed'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkgs := make(map[string]*domain.InstalledPackage)
	for rows.Next() {
		var pkg domain.InstalledPackage
		var binaries, libs, apps, deps, installedAt string
		var isDep, isCask int

		if err := rows.Scan(&pkg.Name, &pkg.Version, &pkg.Revision, &pkg.URL, &pkg.Path,
			&binaries, &libs, &apps, &deps, &isDep, &isCask, &installedAt); err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(binaries), &pkg.Binaries)
		json.Unmarshal([]byte(libs), &pkg.Libs)
		json.Unmarshal([]byte(apps), &pkg.Apps)
		json.Unmarshal([]byte(deps), &pkg.Dependencies)
		pkg.IsDep = isDep == 1
		pkg.IsCask = isCask == 1
		pkg.InstalledAt, _ = time.Parse(time.RFC3339, installedAt)

		pkgs[pkg.Name] = &pkg
	}

	return pkgs, nil
}

func (s *SQLiteState) BeginInstall(pkg *domain.InstalledPackage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.insertPkg(tx, pkg, "pending"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteState) exportJSON() error {
	pkgs, err := s.listInstalled()
	if err != nil {
		return err
	}

	manifest := domain.Manifest{Packages: pkgs}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.manifestPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(s.manifestPath, data, 0644)
}

func (s *SQLiteState) Close() error {
	return s.db.Close()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
