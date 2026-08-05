//go:build !darwin

package platform

// MenuLabels holds the localized strings for the custom application menu.
type MenuLabels struct {
	Settings, BackupViewer, CheckUpdate, About, Quit, Edit, Undo, Redo, Cut, Copy, Paste, Delete string
}

// SetMenuHandler is a no-op outside macOS (the custom NSMenu is macOS-only).
func SetMenuHandler(_ func(action string)) {}

// InstallAppMenu is a no-op outside macOS.
func InstallAppMenu(_ MenuLabels) {}
