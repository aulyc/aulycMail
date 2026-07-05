package app

import (
	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
	"github.com/aulyc/aulycmail/internal/settings"
)

// ListEnabledExtensions returns the names of all enabled first-party
// extensions. Order is stable but not meaningful — the frontend renders
// rail tabs in coreapi.RailTabRequest.Order order, not list order.
//
// Contacts is the only first-party extension currently registered. As more
// first-party extensions land, add their keys to settings.AllExtensionKeys.
func (a *App) ListEnabledExtensions() ([]string, error) {
	out := make([]string, 0, len(settings.AllExtensionKeys))
	for _, name := range settings.AllExtensionKeys {
		enabled, err := a.settingsStore.IsExtensionEnabled(name)
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// ListExtensionRailTabs returns rail-tab registrations for currently enabled
// extensions only. The frontend uses this to render ExtensionRail.svelte.
// The rail renders when len() >= 1 — one enabled extension plus the implicit
// always-on Mail tab gives the user something to switch between.
func (a *App) ListExtensionRailTabs() ([]coreapi.RailTabRequest, error) {
	if a.uiRegistry == nil {
		return nil, nil
	}
	enabled, err := a.enabledExtensionSet()
	if err != nil {
		return nil, err
	}
	all := a.uiRegistry.ListRailTabs()
	out := make([]coreapi.RailTabRequest, 0, len(all))
	for _, tab := range all {
		if !enabled[tab.ExtensionID] {
			continue
		}
		out = append(out, tab)
	}
	return out, nil
}

// ListAccountSetupHooksForProvider returns hook panels matching the given
// provider. Called by AccountDialog.svelte after a new account is created,
// to render any "Also set up X" panels.
//
// Hooks are returned regardless of whether their extension is currently
// enabled — the hook itself is the discovery surface. Filtering by enabled
// state here would hide first-party features from new users if optional
// extensions are added later.
func (a *App) ListAccountSetupHooksForProvider(provider string) ([]coreapi.AccountSetupHookRequest, error) {
	if a.uiRegistry == nil {
		return nil, nil
	}
	return a.uiRegistry.ListAccountSetupHooksForProvider(provider), nil
}

// enabledExtensionSet returns a set of currently-enabled extension names.
func (a *App) enabledExtensionSet() (map[string]bool, error) {
	set := make(map[string]bool, len(settings.AllExtensionKeys))
	for _, name := range settings.AllExtensionKeys {
		enabled, err := a.settingsStore.IsExtensionEnabled(name)
		if err != nil {
			return nil, err
		}
		if enabled {
			set[name] = true
		}
	}
	return set, nil
}

