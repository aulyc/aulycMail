package app

import "github.com/aulyc/aulycmail/internal/platform"

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
	platform.SetDockBadge(count)

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
