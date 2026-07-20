package app

import "aulyc.local/aulycmail/internal/platform"

// refreshUnreadBadges updates native unread-count surfaces to the total number
// of unread messages across all badge-eligible folders. Safe to call from any
// goroutine and at any point in the lifecycle (it no-ops before the message store
// is ready and on unsupported platforms). Call it wherever unread counts change.
func (a *App) refreshUnreadBadges() {
	if a == nil || a.messageStore == nil {
		return
	}
	count, err := a.messageStore.GetBadgeUnreadCount()
	if err != nil {
		return
	}
	if a.dockBadgeEnabled.Load() {
		platform.SetDockBadge(count)
	} else {
		// Clear rather than retaining a label that macOS cannot currently show.
		platform.SetDockBadge(0)
	}

	if a.settingsStore == nil {
		return
	}
	enabled, err := a.settingsStore.GetMenuBarIcon()
	if err != nil || !enabled {
		platform.SetStatusItemVisible(false, platform.StatusItemLabels{})
		return
	}
	platform.SetStatusItemVisible(true, a.statusItemLabels())
	platform.SetStatusItemUnreadCount(count)
}
