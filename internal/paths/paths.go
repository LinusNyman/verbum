// Package paths resolves where vox keeps its data on disk (XDG) and
// answers first-run / free-space questions for the ingest pipeline.
package paths

import (
	"os"
	"path/filepath"
)

// DataDir is ${XDG_DATA_HOME:-~/.local/share}/vox.
func DataDir() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "vox"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "vox"), nil
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
	return filepath.Join(dir, "vox.db"), nil
}

// DBExists reports whether a built database is present (first-run check).
// It never creates anything.
func DBExists() bool {
	dir, err := DataDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "vox.db"))
	return err == nil && !info.IsDir() && info.Size() > 0
}
