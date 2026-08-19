// Package paths resolves where verbum keeps its data on disk (XDG) and
// answers first-run / free-space questions for the ingest pipeline.
package paths

import (
	"os"
	"path/filepath"
)

// DataDir is ${XDG_DATA_HOME:-~/.local/share}/verbum.
func DataDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "verbum")
	migrateLegacy(base, dir)
	return dir, nil
}

// migrateLegacy re-homes a v0.1.x `vox` data tree onto the new name, so anyone
// upgrading keeps their ~1.6 GB database instead of re-downloading it. Both
// renames are best-effort: on failure the old tree is left untouched and the
// CLI just reports "no dictionary yet". Delete once v0.1.x is out of use.
func migrateLegacy(base, dir string) {
	if _, err := os.Stat(dir); err == nil {
		return // already on the new name
	}
	legacy := filepath.Join(base, "vox")
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return
	}
	if err := os.Rename(legacy, dir); err != nil {
		return
	}
	os.Rename(filepath.Join(dir, "vox.db"), filepath.Join(dir, "verbum.db"))
}

// DBPath returns the database path, creating the parent directory.
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "verbum.db"), nil
}

// DBExists reports whether a built database is present (first-run check).
// It never creates anything.
func DBExists() bool {
	dir, err := DataDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "verbum.db"))
	return err == nil && !info.IsDir() && info.Size() > 0
}
