package app

import (
	"errors"
	"strings"

	"aulyc.local/aulycmail/internal/activitylog"
)

var errActivityLogStoreUnavailable = errors.New("activity log store is not initialized")

const maxActivityLogDetailRunes = 4096

// ListActivityLogs returns a filtered, newest-first page of durable activity
// records for the settings activity-log view.
func (a *App) ListActivityLogs(query activitylog.Query) (activitylog.Page, error) {
	if a.activityLogStore == nil {
		return activitylog.Page{}, errActivityLogStoreUnavailable
	}
	return a.activityLogStore.List(query)
}

// GetLatestActivityLog returns the newest record matching a type and optional
// backup directory.
func (a *App) GetLatestActivityLog(activityType, directory string) (*activitylog.Entry, error) {
	if a.activityLogStore == nil {
		return nil, errActivityLogStoreUnavailable
	}
	return a.activityLogStore.Latest(activityType, directory)
}

// ClearActivityLogs removes records matching the same filters used by the list
// view. A zero-value query explicitly clears all activity records.
func (a *App) ClearActivityLogs(query activitylog.Query) (int64, error) {
	if a.activityLogStore == nil {
		return 0, errActivityLogStoreUnavailable
	}
	return a.activityLogStore.Clear(query)
}

// appendActivityLog is the single write path used by completed background
// tasks. Store.Append assigns the canonical ID/time and applies retention before
// the persisted record is announced to the frontend.
func (a *App) appendActivityLog(entry activitylog.Entry) error {
	if a.activityLogStore == nil {
		return errActivityLogStoreUnavailable
	}
	entry.Detail = strings.TrimSpace(entry.Detail)
	detailRunes := []rune(entry.Detail)
	if len(detailRunes) > maxActivityLogDetailRunes {
		entry.Detail = string(detailRunes[:maxActivityLogDetailRunes]) + "…"
	}
	if err := a.activityLogStore.Append(&entry); err != nil {
		return err
	}
	a.emitEvent("activity-log:created", entry)
	return nil
}
