package app

import (
	"fmt"

	"aulyc.local/aulycmail/internal/appstate"
)

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

// Build metadata defaults are intentionally generic. Make injects the values
// derived from version.json and Git into every normal or release build.
var (
	Version     = "0.0.0-dev"
	BuildNumber = "0"
	CommitSHA   = "unknown"
)

// VersionLabel returns the user-facing version. A zero build number identifies
// an ad-hoc local build and is omitted from the label.
func VersionLabel() string {
	if BuildNumber == "" || BuildNumber == "0" {
		return Version
	}
	return fmt.Sprintf("%s (build %s)", Version, BuildNumber)
}

// AppInfo contains application metadata
type AppInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	BuildNumber    string `json:"buildNumber"`
	CommitSHA      string `json:"commitSHA"`
	DisplayVersion string `json:"displayVersion"`
	Description    string `json:"description"`
	Website        string `json:"website"`
	License        string `json:"license"`
}

// GetAppInfo returns application metadata for the About settings page
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:           "aulycMail",
		Version:        Version,
		BuildNumber:    BuildNumber,
		CommitSHA:      CommitSHA,
		DisplayVersion: VersionLabel(),
		Description:    "A lightweight desktop e-mail client",
		Website:        "https://www.aulyc.com",
		License:        "Proprietary",
	}
}

// GetPendingMailto returns and clears any pending mailto: URL data.
// This is used when aulycMail is launched with a mailto: URL argument.
func (a *App) GetPendingMailto() *MailtoData {
	data := a.PendingMailto
	a.PendingMailto = nil // Clear after reading
	return data
}
