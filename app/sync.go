package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	gosync "sync"
	"time"

	"aulyc.local/aulycmail/internal/activitylog"
	"aulyc.local/aulycmail/internal/certificate"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/logging"
	syncengine "aulyc.local/aulycmail/internal/sync"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Sync API - Exposed to frontend via Wails bindings
// ============================================================================

// SyncFolder synchronizes messages for a folder with the IMAP server
func (a *App) SyncFolder(accountID, folderID string) error {
	const debounceMs = 500
	log := logging.WithComponent("app")

	// Use composite key to allow multiple folders to sync concurrently
	syncKey := accountID + ":" + folderID

	a.syncMu.Lock()

	// Check debounce - if last request for this folder was within 500ms, skip
	if lastReq, exists := a.syncLastRequest[syncKey]; exists {
		if time.Since(lastReq) < time.Duration(debounceMs)*time.Millisecond {
			a.syncMu.Unlock()
			log.Debug().Str("account", accountID).Str("folder", folderID).Msg("SyncFolder debounced, skipping")
			return nil // Silently ignore
		}
	}
	a.syncLastRequest[syncKey] = time.Now()

	// Cancel existing sync for this specific folder if any
	if cancel, exists := a.syncContexts[syncKey]; exists {
		log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Cancelling existing sync for restart")
		cancel()
		// Small delay to let goroutines clean up
		a.syncMu.Unlock()
		time.Sleep(100 * time.Millisecond)
		a.syncMu.Lock()
	}

	// Create new cancellable context for this sync
	ctx, cancel := context.WithCancel(a.ctx)
	a.syncContexts[syncKey] = cancel

	a.syncMu.Unlock()

	// NOTE: Don't cleanup syncContexts here - body sync runs in goroutine
	// and needs the context to remain cancellable. Cleanup happens in the goroutine.

	// Get account to determine sync period
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		a.recordSyncActivity(accountID, folderID, syncengine.MessageSyncResult{}, activitylog.StatusFailed, err)
		return fmt.Errorf("failed to get account: %w", err)
	}
	syncOptions := syncengine.MessageSyncOptionsFromAccount(acc)

	// Use ctx (not a.ctx) for sync operations so they can be cancelled
	messageResult, err := a.syncEngine.SyncMessagesWithOptionsResult(ctx, accountID, folderID, syncOptions)
	if err != nil {
		// Check if this was a cancellation
		if ctx.Err() != nil {
			log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Sync cancelled")
			a.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusCancelled, ctx.Err())
			return ctx.Err()
		}
		// Check for certificate error - emit special event for TOFU dialog
		var certErr *certificate.Error
		if errors.As(err, &certErr) {
			log.Warn().Str("folder", folderID).Str("fingerprint", certErr.Info.Fingerprint).Msg("Untrusted certificate during sync")
			wailsRuntime.EventsEmit(a.ctx, "certificate:untrusted", map[string]interface{}{
				"accountId":   accountID,
				"certificate": certErr.Info,
			})
			a.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusFailed, err)
			return err
		}

		// Actual error - emit error event
		log.Error().Err(err).Str("folder", folderID).Msg("Header sync failed")
		wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
			"error":     err.Error(),
		})
		a.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusFailed, err)
		return err
	}

	// Checkpoint WAL after heavy sync operation
	if checkpointErr := a.db.Checkpoint(); checkpointErr != nil {
		log.Warn().Err(checkpointErr).Msg("WAL checkpoint after SyncFolder failed")
	}

	// Emit folder count change event so frontend updates sidebar
	if folderObj, folderErr := a.folderStore.Get(folderID); folderErr == nil && folderObj != nil {
		log.Debug().
			Str("folderID", folderID).
			Int("unreadCount", folderObj.UnreadCount).
			Msg("Emitting folders:countsChanged after sync")
		wailsRuntime.EventsEmit(a.ctx, "folders:countsChanged", map[string]int{
			folderID: folderObj.UnreadCount,
		})
	}
	a.refreshUnreadBadges()

	bodyFetch := syncengine.BodyFetchOptionsFromAccount(acc)
	if !bodyFetch.Enabled {
		a.syncMu.Lock()
		if currentCancel, exists := a.syncContexts[syncKey]; exists && fmt.Sprintf("%p", currentCancel) == fmt.Sprintf("%p", cancel) {
			delete(a.syncContexts, syncKey)
		}
		a.syncMu.Unlock()

		wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
		})
		a.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusSuccess, nil)
		return nil
	}

	// Start background body fetching (emits progress events for "bodies" phase)
	// Pass ctx so body fetch can also be cancelled
	go func(syncCtx context.Context, syncDays int, cancelFn context.CancelFunc, key string, result syncengine.MessageSyncResult) {
		// Cleanup sync context when goroutine completes
		defer func() {
			a.syncMu.Lock()
			// Only delete if it's still our cancel function (not replaced by newer sync)
			if currentCancel, exists := a.syncContexts[key]; exists && fmt.Sprintf("%p", currentCancel) == fmt.Sprintf("%p", cancelFn) {
				delete(a.syncContexts, key)
			}
			a.syncMu.Unlock()
		}()

		// Panic recovery - ensure we always emit an event so UI doesn't get stuck
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("folder", folderID).Msg("Body fetch goroutine panicked")
				a.recordSyncActivity(accountID, folderID, result, activitylog.StatusPartial, fmt.Errorf("body fetch panic: %v", r))
				wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     fmt.Sprintf("body fetch panic: %v", r),
				})
			}
		}()

		bodyErr := a.syncEngine.FetchBodiesInBackground(syncCtx, accountID, folderID, syncDays)

		if bodyErr != nil {
			if syncCtx.Err() != nil {
				// Cancelled - not an error, still emit synced so spinner stops
				log.Debug().Str("folder", folderID).Msg("Background body fetch cancelled")
				wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
				})
				a.recordSyncActivity(accountID, folderID, result, activitylog.StatusCancelled, syncCtx.Err())
			} else {
				// Actual error - emit error event instead of synced
				log.Error().Err(bodyErr).Str("folder", folderID).Msg("Background body fetch failed")
				wailsRuntime.EventsEmit(a.ctx, "folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     bodyErr.Error(),
				})
				a.recordSyncActivity(accountID, folderID, result, activitylog.StatusPartial, bodyErr)
			}
		} else {
			// Success
			wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
				"accountId": accountID,
				"folderId":  folderID,
			})
			a.recordSyncActivity(accountID, folderID, result, activitylog.StatusSuccess, nil)
		}
	}(ctx, bodyFetch.Days, cancel, syncKey, messageResult)

	return nil
}

func syncActivitySummary(result syncengine.MessageSyncResult, status string) string {
	switch status {
	case activitylog.StatusCancelled:
		return fmt.Sprintf("同步已取消 · 新增 %d 封 · 移除 %d 封", result.Added, result.Removed)
	case activitylog.StatusPartial:
		return fmt.Sprintf("部分完成 · 新增 %d 封 · 移除 %d 封", result.Added, result.Removed)
	case activitylog.StatusFailed:
		return "同步失败"
	default:
		return fmt.Sprintf("同步成功 · 新增 %d 封 · 移除 %d 封", result.Added, result.Removed)
	}
}

func (a *App) recordSyncActivity(accountID, folderID string, result syncengine.MessageSyncResult, status string, syncErr error) {
	if status == activitylog.StatusFailed && result.Added+result.Removed > 0 {
		status = activitylog.StatusPartial
	}
	accountEmail := ""
	if acc, err := a.accountStore.Get(accountID); err == nil && acc != nil {
		accountEmail = acc.Email
	}
	folderName := folderID
	if f, err := a.folderStore.Get(folderID); err == nil && f != nil {
		folderName = f.Path
	}
	if strings.TrimSpace(folderName) == "" {
		folderName = "邮箱"
	}
	detail := ""
	if syncErr != nil {
		detail = syncErr.Error()
	}
	entry := activitylog.Entry{
		Type:    activitylog.TypeSync,
		Status:  status,
		Title:   "同步 " + folderName,
		Summary: syncActivitySummary(result, status),
		Detail:  detail,
		Payload: map[string]any{
			"accountEmail": accountEmail,
			"folderName":   folderName,
			"scope":        folderName,
			"added":        result.Added,
			"removed":      result.Removed,
		},
	}
	if err := a.appendActivityLog(entry); err != nil {
		log := logging.WithComponent("app.sync")
		log.Warn().Err(err).Msg("Failed to persist sync activity log")
	}
}

// ForceSyncFolder clears body content and attachments for a folder, then re-syncs.
// This is useful when attachments weren't extracted properly (e.g., after a fix)
// or when message content needs to be re-parsed.
func (a *App) ForceSyncFolder(accountID, folderID string) error {
	log := logging.WithComponent("app")
	log.Info().Str("accountID", accountID).Str("folderID", folderID).Msg("Starting force re-sync of folder")

	// Step 1: Clear body content for all messages in the folder
	bodiesCleared, err := a.messageStore.ClearBodiesForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to clear bodies: %w", err)
	}
	log.Info().Int64("bodiesCleared", bodiesCleared).Msg("Cleared message bodies")

	// Step 2: Delete attachments for all messages in the folder
	attachmentsDeleted, err := a.attachmentStore.DeleteAttachmentsForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to delete attachments: %w", err)
	}
	log.Info().Int64("attachmentsDeleted", attachmentsDeleted).Msg("Deleted attachments")

	// Step 3: Trigger normal folder sync (which will re-fetch bodies and extract attachments)
	return a.SyncFolder(accountID, folderID)
}

// SyncAccountComplete performs a comprehensive sync of an account:
// 1. Syncs folder list from IMAP
// 2. Syncs core folders' messages (Inbox, Drafts, Sent)
func (a *App) SyncAccountComplete(accountID string) error {
	log := logging.WithComponent("app.masterSync")
	log.Info().Str("accountID", accountID).Msg("Starting complete account sync")

	// Check if sync was cancelled before starting
	a.syncMu.Lock()
	cancelled := a.syncCancelled
	a.syncMu.Unlock()
	if cancelled {
		err := context.Canceled
		a.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, activitylog.StatusCancelled, err)
		return fmt.Errorf("sync cancelled: %w", err)
	}

	// 1. Sync folder list first (required for message sync)
	if err := a.SyncFolders(accountID); err != nil {
		status := activitylog.StatusFailed
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = activitylog.StatusCancelled
		}
		a.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, status, err)
		return fmt.Errorf("folder sync failed: %w", err)
	}
	// Reaching here means IMAP connected + authenticated — stamp last-OK.
	a.recordAccountConnOK(accountID)

	// 2. Determine which folders to sync based on account settings
	foldersToSync, err := a.getSyncFolders(accountID)
	if err != nil {
		a.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, activitylog.StatusFailed, err)
		return fmt.Errorf("failed to determine sync folders: %w", err)
	}

	log.Info().Str("accountID", accountID).Int("folders", len(foldersToSync)).Msg("Syncing folders")

	// Sync with semaphore (max 2 concurrent to leave pool room for user actions)
	sem := make(chan struct{}, 2)
	var syncErrors []string
	var errorsMu gosync.Mutex
	var wg gosync.WaitGroup

	for _, f := range foldersToSync {
		// Check if sync was cancelled between folders
		a.syncMu.Lock()
		cancelled := a.syncCancelled
		a.syncMu.Unlock()
		if cancelled {
			log.Info().Str("accountID", accountID).Msg("Sync cancelled, stopping folder loop")
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(f *folder.Folder) {
			defer recoverPanic("app.sync", "sync folder")
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			log.Info().Str("path", f.Path).Str("id", f.ID).Msg("Syncing folder")
			if syncErr := a.SyncFolder(accountID, f.ID); syncErr != nil {
				log.Warn().Err(syncErr).Str("folder", f.Path).Msg("Message sync failed")
				errorsMu.Lock()
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", f.Path, syncErr))
				errorsMu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	if len(syncErrors) > 0 {
		return fmt.Errorf("some folders failed to sync: %s", strings.Join(syncErrors, "; "))
	}

	log.Info().Str("accountID", accountID).Msg("Complete account sync finished")
	return nil
}

// getSyncFolders returns the folders that should be automatically synced for an account.
// Three-way logic:
//   - SyncAllFolders=true → all folders
//   - SyncFoldersEnabled=true → subscribed folders (respects IMAP subscriptions)
//   - default → core only (Inbox, Drafts, Sent) — backward compatible
func (a *App) getSyncFolders(accountID string) ([]*folder.Folder, error) {
	acct, err := a.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if acct.SyncAllFolders {
		return a.folderStore.List(accountID)
	}
	if acct.SyncFoldersEnabled {
		return a.folderStore.ListSubscribed(accountID)
	}
	return a.getCoreOnlyFolders(accountID)
}

// getCoreOnlyFolders returns core folders (Inbox, Drafts, Sent) — the default sync behavior.
func (a *App) getCoreOnlyFolders(accountID string) ([]*folder.Folder, error) {
	coreTypes := []folder.Type{folder.TypeInbox, folder.TypeDrafts, folder.TypeSent}
	var folders []*folder.Folder
	for _, ft := range coreTypes {
		f, err := a.folderStore.GetByType(accountID, ft)
		if err != nil {
			continue
		}
		if f != nil {
			folders = append(folders, f)
		}
	}
	return folders, nil
}

// SyncAllComplete syncs all enabled accounts completely.
// This is the master sync function called from the sidebar sync button.
func (a *App) SyncAllComplete() error {
	log := logging.WithComponent("app.masterSync")

	// Reset cancellation flag for this sync run
	a.syncMu.Lock()
	a.syncCancelled = false
	a.syncMu.Unlock()

	// Skip if we know we're offline — avoids connection errors and the
	// error indicator that would appear on the sidebar account menu.
	// Emit folder:synced for each account so the frontend clears its
	// syncing state (otherwise it stays stuck on the spinner).
	if a.networkMonitor != nil && !a.networkMonitor.IsConnected() {
		log.Info().Msg("Skipping complete sync — offline")
		accounts, listErr := a.accountStore.List()
		if listErr == nil {
			for _, acc := range accounts {
				if !acc.Enabled {
					continue
				}
				inbox, inboxErr := a.folderStore.GetByType(acc.ID, folder.TypeInbox)
				if inboxErr == nil && inbox != nil {
					wailsRuntime.EventsEmit(a.ctx, "folder:synced", map[string]interface{}{
						"accountId": acc.ID,
						"folderId":  inbox.ID,
					})
				}
			}
		}
		return nil
	}

	log.Info().Msg("Starting complete sync of all accounts and contacts")

	accounts, err := a.accountStore.List()
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	var errors []string

	// First: Sync each email account (sequentially to avoid overwhelming IMAP)
	// Email sync is the primary use case and runs without database contention
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}

		// Check if sync was cancelled between accounts
		a.syncMu.Lock()
		cancelled := a.syncCancelled
		a.syncMu.Unlock()
		if cancelled {
			log.Info().Msg("Sync cancelled, stopping account loop")
			break
		}

		if err := a.SyncAccountComplete(acc.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", acc.Email, err))
			// Continue with other accounts
		}
	}

	// Restart IDLE connections — they may have died during network changes
	// or exhausted their reconnect attempts. StartAccount is a no-op for
	// accounts that already have a healthy IDLE connection.
	a.restartIDLE()

	if len(errors) > 0 {
		return fmt.Errorf("sync errors: %s", strings.Join(errors, "; "))
	}

	log.Info().Msg("Complete sync of all accounts and contacts finished")
	return nil
}

// CancelFolderSync cancels a running sync for a specific folder
func (a *App) CancelFolderSync(accountID, folderID string) {
	log := logging.WithComponent("app")
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	syncKey := accountID + ":" + folderID
	if cancel, exists := a.syncContexts[syncKey]; exists {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling folder sync")
		cancel()
		delete(a.syncContexts, syncKey)
	}
}

// CancelAllSyncs cancels all running syncs and force-closes pool connections.
// Force-closing is needed because context cancellation cannot interrupt blocked
// TCP reads on dead sockets (e.g., after network changes). ForceClose kills the
// sockets immediately so goroutines unblock and emit folder:synced events.
func (a *App) CancelAllSyncs() {
	log := logging.WithComponent("app")
	a.syncMu.Lock()

	// Set cancellation flag so SyncAllComplete/SyncAccountComplete loops stop
	a.syncCancelled = true

	for syncKey, cancel := range a.syncContexts {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling sync")
		cancel()
	}
	a.syncContexts = make(map[string]context.CancelFunc)

	a.syncMu.Unlock()

	// Force-close all pool connections to unblock goroutines stuck on dead
	// TCP sockets. This uses ForceClose (no graceful logout) so it returns
	// instantly even if connections are unresponsive.
	if a.imapPool != nil {
		a.imapPool.CloseAll()
	}
}
