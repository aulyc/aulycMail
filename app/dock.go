package app

import "github.com/aulyc/aulycmail/internal/platform"

// refreshDockBadge updates the macOS Dock tile badge to the total number of
// unread messages across all inbox folders. Safe to call from any goroutine and
// at any point in the lifecycle (it no-ops before the message store is ready and
// on non-macOS platforms). Call it wherever unread counts change.
func (a *App) refreshDockBadge() {
	if a == nil || a.messageStore == nil {
		return
	}
	count, err := a.messageStore.GetBadgeUnreadCount()
	if err != nil {
		return
	}
	platform.SetDockBadge(count)
}
