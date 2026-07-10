// Package platform provides platform-specific functionality
package platform

import (
	"os"
	"path/filepath"
)

const appName = "aulycmail"

// Paths holds the application data paths
type Paths struct {
	Config string // Configuration files
	Data   string // Persistent data (database, attachment downloads)
	Cache  string // Cached data (can be deleted)
}

// GetPaths returns macOS application paths.
func GetPaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	appSupport := filepath.Join(home, "Library", "Application Support", "aulycmail")
	caches := filepath.Join(home, "Library", "Caches", "aulycmail")

	return &Paths{
		Config: appSupport,
		Data:   appSupport,
		Cache:  caches,
	}, nil
}

// EnsureDirectories creates the root directories if they don't exist.
// Subdirectories (e.g. attachment downloads) are created on demand by
// their owners.
func (p *Paths) EnsureDirectories() error {
	dirs := []string{
		p.Config,
		p.Data,
		p.Cache,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return nil
}

// DatabasePath returns the path to the main database
func (p *Paths) DatabasePath() string {
	return filepath.Join(p.Data, "aulycmail.db")
}

// AttachmentsPath returns the path to the attachments directory
func (p *Paths) AttachmentsPath() string {
	return filepath.Join(p.Data, "attachments")
}
