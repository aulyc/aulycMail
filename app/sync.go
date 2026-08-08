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
)

const defaultAccountSyncTimeout = 30 * time.Minute

// ============================================================================
// Sync API - Exposed to frontend via Wails bindings
// ============================================================================

// SyncFolder synchronizes messages for a folder with the IMAP server
func (s *SyncBridge) SyncFolder(accountID, folderID string) error {
	const debounceWindow = 500 * time.Millisecond
	syncKey := accountID + ":" + folderID
	s.mu.Lock()
	if lastRequest, exists := s.lastRequest[syncKey]; exists && time.Since(lastRequest) < debounceWindow {
		s.mu.Unlock()
		log := logging.WithComponent("app")
		log.Debug().Str("account", accountID).Str("folder", folderID).Msg("SyncFolder debounced, skipping")
		return nil
	}
	s.lastRequest[syncKey] = time.Now()
	s.mu.Unlock()

	return s.coordinateAccountSync(accountID, syncengine.TriggerManual, func(ctx context.Context) error {
		return s.syncFolderDirect(ctx, accountID, folderID)
	})
}

func (s *SyncBridge) syncFolderDirect(parent context.Context, accountID, folderID string) error {
	log := logging.WithComponent("app")
	folderObj, err := s.app.requireSelectableFolder(folderID)
	if err != nil {
		return err
	}
	if folderObj.AccountID != accountID {
		return fmt.Errorf("folder %s does not belong to account %s", folderID, accountID)
	}

	// Use composite key to allow multiple folders to sync concurrently
	syncKey := accountID + ":" + folderID

	s.mu.Lock()

	// Cancel existing sync for this specific folder if any
	if cancel, exists := s.contexts[syncKey]; exists {
		log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Cancelling existing sync for restart")
		cancel()
		// Small delay to let goroutines clean up
		s.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
	}

	if parent == nil {
		parent = s.app.ctx
		if parent == nil {
			parent = context.Background()
		}
	}

	// Create a cancellable context for the header phase. It inherits the
	// coordinator/account deadline so a preempted or timed-out sync cannot keep
	// consuming a connection after its caller has already stopped waiting.
	ctx, cancel := context.WithCancel(parent)
	s.contexts[syncKey] = cancel

	s.mu.Unlock()
	headerContextOwned := true
	defer func() {
		if !headerContextOwned {
			return
		}
		cancel()
		s.mu.Lock()
		if currentCancel, exists := s.contexts[syncKey]; exists && sameCancelFunc(currentCancel, cancel) {
			delete(s.contexts, syncKey)
		}
		s.mu.Unlock()
	}()

	// Header-phase contexts are cleaned on every return. If background body
	// fetching starts, ownership is transferred to a detached body context.

	// Get account to determine sync period
	acc, err := s.app.accountStore.Get(accountID)
	if err != nil {
		s.recordSyncActivity(accountID, folderID, syncengine.MessageSyncResult{}, activitylog.StatusFailed, err)
		return fmt.Errorf("failed to get account: %w", err)
	}
	syncOptions := syncengine.MessageSyncOptionsFromAccount(acc)

	// Use ctx (not s.app.ctx) for sync operations so they can be cancelled
	messageResult, err := s.app.syncEngine.SyncMessagesWithOptionsResult(ctx, accountID, folderID, syncOptions)
	if err != nil {
		// Check if this was a cancellation
		if ctx.Err() != nil {
			log.Debug().Str("account", accountID).Str("folder", folderID).Msg("Sync cancelled")
			s.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusCancelled, ctx.Err())
			return ctx.Err()
		}
		// Check for certificate error - emit special event for TOFU dialog
		var certErr *certificate.Error
		if errors.As(err, &certErr) {
			log.Warn().Str("folder", folderID).Str("fingerprint", certErr.Info.Fingerprint).Msg("Untrusted certificate during sync")
			s.app.emitEvent("certificate:untrusted", map[string]interface{}{
				"accountId":   accountID,
				"certificate": certErr.Info,
			})
			s.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusFailed, err)
			return err
		}

		// Actual error - emit error event
		log.Error().Err(err).Str("folder", folderID).Msg("Header sync failed")
		s.app.emitEvent("folder:syncError", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
			"error":     err.Error(),
		})
		s.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusFailed, err)
		return err
	}

	// Checkpoint WAL after heavy sync operation
	if checkpointErr := s.app.db.Checkpoint(); checkpointErr != nil {
		log.Warn().Err(checkpointErr).Msg("WAL checkpoint after SyncFolder failed")
	}

	// Emit folder count change event so frontend updates sidebar
	if folderObj, folderErr := s.app.folderStore.Get(folderID); folderErr == nil && folderObj != nil {
		log.Debug().
			Str("folderID", folderID).
			Int("unreadCount", folderObj.UnreadCount).
			Msg("Emitting folders:countsChanged after sync")
		s.app.emitEvent("folders:countsChanged", map[string]int{
			folderID: folderObj.UnreadCount,
		})
	}
	s.app.refreshUnreadBadges()

	bodyFetch := syncengine.BodyFetchOptionsFromAccount(acc)
	if !bodyFetch.Enabled {
		s.app.emitEvent("folder:synced", map[string]interface{}{
			"accountId": accountID,
			"folderId":  folderID,
		})
		s.recordSyncActivity(accountID, folderID, messageResult, activitylog.StatusSuccess, nil)
		return nil
	}

	// Body fetching intentionally outlives the coordinator request that handled
	// the header phase. Transfer the cancellation slot to a fresh app-lifetime
	// context before returning; explicit CancelFolderSync/CancelAllSyncs still
	// cancel it, while a normal coordinator cleanup no longer aborts it.
	bodyParent := s.app.ctx
	if bodyParent == nil {
		bodyParent = context.Background()
	}
	bodyCtx, bodyCancel := context.WithCancel(bodyParent)
	s.mu.Lock()
	currentCancel, exists := s.contexts[syncKey]
	if !exists || !sameCancelFunc(currentCancel, cancel) || ctx.Err() != nil {
		s.mu.Unlock()
		bodyCancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return context.Canceled
	}
	s.contexts[syncKey] = bodyCancel
	s.mu.Unlock()
	headerContextOwned = false
	cancel()

	// Start background body fetching (emits progress events for "bodies" phase)
	// Pass bodyCtx so body fetch can also be cancelled explicitly.
	go func(syncCtx context.Context, syncDays int, cancelFn context.CancelFunc, key string, result syncengine.MessageSyncResult) {
		defer cancelFn()
		// Cleanup sync context when goroutine completes
		defer func() {
			s.mu.Lock()
			// Only delete if it's still our cancel function (not replaced by newer sync)
			if currentCancel, exists := s.contexts[key]; exists && sameCancelFunc(currentCancel, cancelFn) {
				delete(s.contexts, key)
			}
			s.mu.Unlock()
		}()

		// Panic recovery - ensure we always emit an event so UI doesn't get stuck
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("folder", folderID).Msg("Body fetch goroutine panicked")
				s.recordSyncActivity(accountID, folderID, result, activitylog.StatusPartial, fmt.Errorf("body fetch panic: %v", r))
				s.app.emitEvent("folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     fmt.Sprintf("body fetch panic: %v", r),
				})
			}
		}()

		bodyErr := s.app.syncEngine.FetchBodiesInBackground(syncCtx, accountID, folderID, syncDays)

		if bodyErr != nil {
			if syncCtx.Err() != nil {
				// Cancelled - not an error, still emit synced so spinner stops
				log.Debug().Str("folder", folderID).Msg("Background body fetch cancelled")
				s.app.emitEvent("folder:synced", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
				})
				s.recordSyncActivity(accountID, folderID, result, activitylog.StatusCancelled, syncCtx.Err())
			} else {
				// Actual error - emit error event instead of synced
				log.Error().Err(bodyErr).Str("folder", folderID).Msg("Background body fetch failed")
				s.app.emitEvent("folder:syncError", map[string]interface{}{
					"accountId": accountID,
					"folderId":  folderID,
					"error":     bodyErr.Error(),
				})
				s.recordSyncActivity(accountID, folderID, result, activitylog.StatusPartial, bodyErr)
			}
		} else {
			// Success
			s.app.emitEvent("folder:synced", map[string]interface{}{
				"accountId": accountID,
				"folderId":  folderID,
			})
			s.recordSyncActivity(accountID, folderID, result, activitylog.StatusSuccess, nil)
		}
	}(bodyCtx, bodyFetch.Days, bodyCancel, syncKey, messageResult)

	return nil
}

func sameCancelFunc(left, right context.CancelFunc) bool {
	return fmt.Sprintf("%p", left) == fmt.Sprintf("%p", right)
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

func (s *SyncBridge) recordSyncActivity(accountID, folderID string, result syncengine.MessageSyncResult, status string, syncErr error) {
	if status == activitylog.StatusFailed && result.Added+result.Removed > 0 {
		status = activitylog.StatusPartial
	}
	accountEmail := ""
	if acc, err := s.app.accountStore.Get(accountID); err == nil && acc != nil {
		accountEmail = acc.Email
	}
	folderName := folderID
	if f, err := s.app.folderStore.Get(folderID); err == nil && f != nil {
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
	if err := s.app.appendActivityLog(entry); err != nil {
		log := logging.WithComponent("app.sync")
		log.Warn().Err(err).Msg("Failed to persist sync activity log")
	}
}

// ForceSyncFolder clears body content and attachments for a folder, then re-syncs.
// This is useful when attachments weren't extracted properly (e.g., after a fix)
// or when message content needs to be re-parsed.
func (s *SyncBridge) ForceSyncFolder(accountID, folderID string) error {
	return s.coordinateAccountSync(accountID, syncengine.TriggerManual, func(ctx context.Context) error {
		return s.forceSyncFolderDirect(ctx, accountID, folderID)
	})
}

func (s *SyncBridge) forceSyncFolderDirect(ctx context.Context, accountID, folderID string) error {
	log := logging.WithComponent("app")
	log.Info().Str("accountID", accountID).Str("folderID", folderID).Msg("Starting force re-sync of folder")

	// Validate ownership before deleting any cached bodies or attachments.
	// A stale or mismatched frontend request must never clear another
	// account's offline data and only then discover the mismatch.
	folderObj, err := s.app.requireSelectableFolder(folderID)
	if err != nil {
		return err
	}
	if folderObj.AccountID != accountID {
		return fmt.Errorf("folder %s does not belong to account %s", folderID, accountID)
	}

	// Step 1: Clear body content for all messages in the folder
	bodiesCleared, err := s.app.messageStore.ClearBodiesForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to clear bodies: %w", err)
	}
	log.Info().Int64("bodiesCleared", bodiesCleared).Msg("Cleared message bodies")

	// Step 2: Delete attachments for all messages in the folder
	attachmentsDeleted, err := s.app.attachmentStore.DeleteAttachmentsForFolder(folderID)
	if err != nil {
		return fmt.Errorf("failed to delete attachments: %w", err)
	}
	log.Info().Int64("attachmentsDeleted", attachmentsDeleted).Msg("Deleted attachments")

	// Step 3: Trigger normal folder sync (which will re-fetch bodies and extract attachments)
	return s.syncFolderDirect(ctx, accountID, folderID)
}

// SyncAccountComplete performs a comprehensive sync of an account:
// 1. Syncs folder list from IMAP
// 2. Syncs core folders' messages (Inbox, Drafts, Sent)
func (s *SyncBridge) SyncAccountComplete(accountID string) error {
	return s.coordinateAccountSync(accountID, syncengine.TriggerManual, func(ctx context.Context) error {
		return s.runAccountSyncWithTimeout(ctx, accountID, func(syncCtx context.Context) error {
			return s.syncAccountCompleteDirect(syncCtx, accountID)
		})
	})
}

func (s *SyncBridge) syncAccountCompleteDirect(ctx context.Context, accountID string) error {
	log := logging.WithComponent("app.masterSync")
	log.Info().Str("accountID", accountID).Msg("Starting complete account sync")

	// Check if sync was cancelled before starting
	s.mu.Lock()
	cancelled := s.cancelled
	s.mu.Unlock()
	if cancelled {
		err := context.Canceled
		s.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, activitylog.StatusCancelled, err)
		return fmt.Errorf("sync cancelled: %w", err)
	}

	// 1. Sync folder list first (required for message sync)
	if err := s.app.syncFoldersDirect(ctx, accountID); err != nil {
		status := activitylog.StatusFailed
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = activitylog.StatusCancelled
		}
		s.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, status, err)
		return fmt.Errorf("folder sync failed: %w", err)
	}
	// Reaching here means IMAP connected + authenticated — stamp last-OK.
	s.app.recordAccountConnOK(accountID)

	// 2. Determine which folders to sync based on account settings
	foldersToSync, err := s.getSyncFolders(accountID)
	if err != nil {
		s.recordSyncActivity(accountID, "", syncengine.MessageSyncResult{}, activitylog.StatusFailed, err)
		return fmt.Errorf("failed to determine sync folders: %w", err)
	}

	log.Info().Str("accountID", accountID).Int("folders", len(foldersToSync)).Msg("Syncing folders")

	// Sync with semaphore (max 2 concurrent to leave pool room for user actions)
	sem := make(chan struct{}, 2)
	var syncErrors []string
	var errorsMu gosync.Mutex
	var wg gosync.WaitGroup

folderLoop:
	for _, f := range foldersToSync {
		// Check if sync was cancelled between folders
		s.mu.Lock()
		cancelled := s.cancelled
		s.mu.Unlock()
		if cancelled {
			log.Info().Str("accountID", accountID).Msg("Sync cancelled, stopping folder loop")
			break
		}

		select {
		case sem <- struct{}{}: // Acquire semaphore
		case <-ctx.Done():
			errorsMu.Lock()
			syncErrors = append(syncErrors, ctx.Err().Error())
			errorsMu.Unlock()
			break folderLoop
		}
		wg.Add(1)
		go func(f *folder.Folder) {
			defer recoverPanic("app.sync", "sync folder")
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			log.Info().Str("path", f.Path).Str("id", f.ID).Msg("Syncing folder")
			if syncErr := s.syncFolderDirect(ctx, accountID, f.ID); syncErr != nil {
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
func (s *SyncBridge) getSyncFolders(accountID string) ([]*folder.Folder, error) {
	acct, err := s.app.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if acct.SyncAllFolders {
		return s.app.folderStore.ListSelectable(accountID)
	}
	if acct.SyncFoldersEnabled {
		return s.app.folderStore.ListSubscribed(accountID)
	}
	return s.getCoreOnlyFolders(accountID)
}

// getCoreOnlyFolders returns core folders (Inbox, Drafts, Sent) — the default sync behavior.
func (s *SyncBridge) getCoreOnlyFolders(accountID string) ([]*folder.Folder, error) {
	coreTypes := []folder.Type{folder.TypeInbox, folder.TypeDrafts, folder.TypeSent}
	var folders []*folder.Folder
	for _, ft := range coreTypes {
		f, err := s.app.folderStore.GetByType(accountID, ft)
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
func (s *SyncBridge) SyncAllComplete() error {
	return s.syncAllComplete(syncengine.TriggerManual)
}

func (s *SyncBridge) syncAllComplete(trigger syncengine.Trigger) error {
	log := logging.WithComponent("app.masterSync")

	// Reset cancellation flag for this sync run
	s.mu.Lock()
	s.cancelled = false
	s.mu.Unlock()

	// Skip if we know we're offline — avoids connection errors and the
	// error indicator that would appear on the sidebar account menu.
	// Emit folder:synced for each account so the frontend clears its
	// syncing state (otherwise it stays stuck on the spinner).
	if s.app.networkMonitor != nil && !s.app.networkMonitor.IsConnected() {
		log.Info().Msg("Skipping complete sync — offline")
		accounts, listErr := s.app.accountStore.List()
		if listErr == nil {
			for _, acc := range accounts {
				if !acc.Enabled {
					continue
				}
				inbox, inboxErr := s.app.folderStore.GetByType(acc.ID, folder.TypeInbox)
				if inboxErr == nil && inbox != nil {
					s.app.emitEvent("folder:synced", map[string]interface{}{
						"accountId": acc.ID,
						"folderId":  inbox.ID,
					})
				}
			}
		}
		return nil
	}

	log.Info().Msg("Starting complete sync of all accounts and contacts")

	accounts, err := s.app.accountStore.List()
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
		s.mu.Lock()
		cancelled := s.cancelled
		s.mu.Unlock()
		if cancelled {
			log.Info().Msg("Sync cancelled, stopping account loop")
			break
		}

		if err := s.coordinateAccountSync(acc.ID, trigger, func(ctx context.Context) error {
			return s.runAccountSyncWithTimeout(ctx, acc.ID, func(syncCtx context.Context) error {
				return s.syncAccountCompleteDirect(syncCtx, acc.ID)
			})
		}); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", acc.Email, err))
			// Continue with other accounts
		}
	}

	// Restart IDLE connections — they may have died during network changes
	// or exhausted their reconnect attempts. StartAccount is a no-op for
	// accounts that already have a healthy IDLE connection.
	s.app.restartIDLE()

	if len(errors) > 0 {
		return fmt.Errorf("sync errors: %s", strings.Join(errors, "; "))
	}

	log.Info().Msg("Complete sync of all accounts and contacts finished")
	return nil
}

func (s *SyncBridge) coordinateAccountSync(accountID string, trigger syncengine.Trigger, work func(context.Context) error) error {
	parent := s.app.ctx
	if parent == nil {
		parent = context.Background()
	}
	if s.app.syncCoordinator == nil {
		return work(parent)
	}
	err := s.app.syncCoordinator.Do(parent, accountID, trigger, work)
	if errors.Is(err, syncengine.ErrCoalesced) {
		return nil
	}
	return err
}

func (s *SyncBridge) runAccountSyncWithTimeout(
	parent context.Context,
	accountID string,
	work func(context.Context) error,
) error {
	if parent == nil {
		parent = context.Background()
	}
	timeout := s.accountSyncTimeout
	if timeout <= 0 {
		timeout = defaultAccountSyncTimeout
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	watchdogDone := make(chan struct{})
	stopWatchdog := context.AfterFunc(ctx, func() {
		defer close(watchdogDone)
		log := logging.WithComponent("app.masterSync")
		log.Warn().
			Err(ctx.Err()).
			Str("accountID", accountID).
			Dur("timeout", timeout).
			Msg("Account sync deadline reached; closing stale IMAP connections")
		if s.app.imapPool != nil {
			s.app.imapPool.CloseAccount(accountID)
		}
	})

	err := work(ctx)
	contextErr := ctx.Err()
	if !stopWatchdog() {
		<-watchdogDone
	}
	cancel()

	if errors.Is(contextErr, context.DeadlineExceeded) {
		return fmt.Errorf("account sync timed out: %w", context.DeadlineExceeded)
	}
	if contextErr != nil {
		return contextErr
	}
	if err != nil {
		return err
	}
	return nil
}

// CancelFolderSync cancels a running sync for a specific folder
func (s *SyncBridge) CancelFolderSync(accountID, folderID string) {
	log := logging.WithComponent("app")
	s.mu.Lock()
	defer s.mu.Unlock()

	syncKey := accountID + ":" + folderID
	if cancel, exists := s.contexts[syncKey]; exists {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling folder sync")
		cancel()
		delete(s.contexts, syncKey)
	}
}

// CancelAllSyncs cancels all running syncs and force-closes pool connections.
// Force-closing is needed because context cancellation cannot interrupt blocked
// TCP reads on dead sockets (e.g., after network changes). ForceClose kills the
// sockets immediately so goroutines unblock and emit folder:synced events.
func (s *SyncBridge) CancelAllSyncs() {
	log := logging.WithComponent("app")
	s.mu.Lock()

	// Set cancellation flag so SyncAllComplete/SyncAccountComplete loops stop
	s.cancelled = true

	for syncKey, cancel := range s.contexts {
		log.Info().Str("syncKey", syncKey).Msg("Cancelling sync")
		cancel()
	}
	s.contexts = make(map[string]context.CancelFunc)

	s.mu.Unlock()

	// Force-close all pool connections to unblock goroutines stuck on dead
	// TCP sockets. This uses ForceClose (no graceful logout) so it returns
	// instantly even if connections are unresponsive.
	if s.app.imapPool != nil {
		s.app.imapPool.CloseAll()
	}
}
