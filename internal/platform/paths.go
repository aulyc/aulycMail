// Package platform provides platform-specific functionality
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "aulycmail"

// Paths holds the application data paths
type Paths struct {
	Config string // Configuration files
	Data   string // Persistent data (database, attachment downloads)
	Cache  string // Cached data (can be deleted)
}

// GetPaths returns platform-specific paths for the application
func GetPaths() (*Paths, error) {
	switch runtime.GOOS {
	case "linux":
		return getLinuxPaths()
	case "darwin":
		return getDarwinPaths()
	case "windows":
		return getWindowsPaths()
	default:
		// Fallback to Linux-style paths
		return getLinuxPaths()
	}
}

// getLinuxPaths returns XDG-compliant paths for Linux
func getLinuxPaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// XDG Base Directory Specification
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}

	return &Paths{
		Config: filepath.Join(configHome, appName),
		Data:   filepath.Join(dataHome, appName),
		Cache:  filepath.Join(cacheHome, appName),
	}, nil
}

// getDarwinPaths returns macOS-style paths
func getDarwinPaths() (*Paths, error) {
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

// getWindowsPaths returns Windows-style paths
func getWindowsPaths() (*Paths, error) {
	// APPDATA is for roaming data (synced across machines)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	// LOCALAPPDATA is for local data (not synced)
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}

	return &Paths{
		Config: filepath.Join(appData, "aulycmail"),
		Data:   filepath.Join(appData, "aulycmail"),
		Cache:  filepath.Join(localAppData, "aulycmail", "Cache"),
	}, nil
}

// IsFlatpak returns true if the application is running inside a Flatpak sandbox.
func IsFlatpak() bool {
	return os.Getenv("FLATPAK_ID") != ""
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

