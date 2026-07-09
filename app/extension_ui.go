package app

import (
	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

// ListEnabledExtensions returns the names of built-in non-mail rail panes.
// Contacts is always available; there is no extension enable/disable setting.
func (a *App) ListEnabledExtensions() ([]string, error) {
	return []string{"contacts"}, nil
}

// ListExtensionRailTabs returns built-in rail-tab registrations. The frontend
// uses this to render ExtensionRail.svelte next to the always-on Mail tab.
func (a *App) ListExtensionRailTabs() ([]coreapi.RailTabRequest, error) {
	if a.uiRegistry == nil {
		return nil, nil
	}
	return a.uiRegistry.ListRailTabs(), nil
}
