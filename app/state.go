package app

import "aulyc.local/aulycmail/internal/appstate"

// ============================================================================
// UI State Persistence
// ============================================================================

// GetUIState retrieves the last saved UI state
func (a *App) GetUIState() (*appstate.UIState, error) {
	return a.appStateStore.GetUIState()
}

// SaveUIState persists the current UI state
func (a *App) SaveUIState(state *appstate.UIState) error {
	return a.appStateStore.SaveUIState(state)
}

// ============================================================================
// App Info API - Exposed to frontend via Wails bindings
// ============================================================================

// Version is consumed by the About settings page and the --version CLI flag. The
// default is the local/test version; release builds override it with Go
// linker flags derived from wails.json.
var Version = "0.3.91-dev"

// AppInfo contains application metadata
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Website     string `json:"website"`
	License     string `json:"license"`
}

// GetAppInfo returns application metadata for the About settings page
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:        "aulycmail",
		Version:     Version,
		Description: "A lightweight desktop e-mail client",
		Website:     "https://aulyc.com/aulycmail",
		License:     "Proprietary",
	}
}

// GetPendingMailto returns and clears any pending mailto: URL data.
// This is used when aulycmail is launched with a mailto: URL argument.
func (a *App) GetPendingMailto() *MailtoData {
	data := a.PendingMailto
	a.PendingMailto = nil // Clear after reading
	return data
}
