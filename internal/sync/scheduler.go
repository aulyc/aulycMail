package sync

import (
	"context"
	"errors"
	"sync"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/logging"
	"github.com/rs/zerolog"
)

// NewMailInfo contains information about newly arrived mail
type NewMailInfo struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	FolderID    string `json:"folderId"`
	Subject     string `json:"subject"`
	FromName    string `json:"fromName"`
	FromEmail   string `json:"fromEmail"`
	Count       int    `json:"count"` // Number of new messages
}

// NewMailCallback is called when new mail arrives
type NewMailCallback func(info NewMailInfo)

// SyncCompletedCallback is called when a sync operation completes (success or error)
type SyncCompletedCallback func(accountID, folderID string, err error)

// SyncCompletedResultCallback is the result-bearing completion callback. It is
// additive so existing consumers of SyncCompletedCallback remain source
// compatible while activity logging can use durable Added/Removed statistics.
type SyncCompletedResultCallback func(accountID, folderID string, result MessageSyncResult, err error)

// AccountSyncFinishedCallback is called once after an account-level scheduler
// run returns. succeeded is true only when the remote folder probe and every
// required message-folder sync completed successfully.
type AccountSyncFinishedCallback func(accountID string, succeeded bool)

// Scheduler handles periodic background sync of email accounts
type Scheduler struct {
	engine       *Engine
	accountStore *account.Store
	folderStore  *folder.Store
	log          zerolog.Logger

	// Callbacks
	newMailCallback       NewMailCallback
	syncCompletedCallback SyncCompletedCallback
	syncResultCallback    SyncCompletedResultCallback
	accountFinished       AccountSyncFinishedCallback
	isConnected           func() bool // optional: skip sync when offline

	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	runningMu     sync.Mutex
	checkInterval time.Duration

	coordinator *Coordinator

	// Compact remote snapshots and attempt times are process-local. They avoid
	// repeated message scans while leaving persisted mail and settings intact.
	probeMu         sync.Mutex
	remoteSnapshots map[string]map[string]RemoteFolderSnapshot
	lastAttempts    map[string]time.Time

	// Per-account cancellation for running syncs
	syncCancels  map[string]context.CancelFunc
	syncCancelMu sync.Mutex
}

// NewScheduler creates a new sync scheduler
func NewScheduler(engine *Engine, accountStore *account.Store, folderStore *folder.Store) *Scheduler {
	return &Scheduler{
		engine:          engine,
		accountStore:    accountStore,
		folderStore:     folderStore,
		log:             logging.WithComponent("sync-scheduler"),
		checkInterval:   1 * time.Minute, // Check every minute if any account is due
		coordinator:     NewCoordinator(),
		remoteSnapshots: make(map[string]map[string]RemoteFolderSnapshot),
		lastAttempts:    make(map[string]time.Time),
		syncCancels:     make(map[string]context.CancelFunc),
	}
}

// SetCoordinator shares account-level trigger coordination with manual and
// wake synchronization paths owned by the app layer.
func (s *Scheduler) SetCoordinator(coordinator *Coordinator) {
	if coordinator != nil {
		s.coordinator = coordinator
	}
}

// SetNewMailCallback sets the callback for new mail notifications
func (s *Scheduler) SetNewMailCallback(callback NewMailCallback) {
	s.newMailCallback = callback
}

// SetSyncCompletedCallback sets the callback for sync completion notifications
func (s *Scheduler) SetSyncCompletedCallback(callback SyncCompletedCallback) {
	s.syncCompletedCallback = callback
}

// SetSyncCompletedResultCallback sets the result-bearing completion callback.
func (s *Scheduler) SetSyncCompletedResultCallback(callback SyncCompletedResultCallback) {
	s.syncResultCallback = callback
}

// SetAccountSyncFinishedCallback sets the account-level lifecycle callback.
func (s *Scheduler) SetAccountSyncFinishedCallback(callback AccountSyncFinishedCallback) {
	s.accountFinished = callback
}

func (s *Scheduler) runAccountSyncLifecycle(accountID string, work func() bool) {
	succeeded := false
	defer func() {
		if s.accountFinished != nil {
			s.accountFinished(accountID, succeeded)
		}
	}()
	succeeded = work()
}

func (s *Scheduler) notifySyncCompleted(accountID, folderID string, result MessageSyncResult, err error) {
	if s.syncResultCallback != nil {
		s.syncResultCallback(accountID, folderID, result, err)
	}
	if s.syncCompletedCallback != nil {
		legacyErr := err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			legacyErr = nil
		}
		s.syncCompletedCallback(accountID, folderID, legacyErr)
	}
}

// SetConnectivityCheck sets a function to check network connectivity.
// When set, the scheduler skips sync ticks when offline to avoid wasted
// connection attempts and unnecessary error logging.
func (s *Scheduler) SetConnectivityCheck(check func() bool) {
	s.isConnected = check
}

// Start starts the background sync scheduler
func (s *Scheduler) Start(ctx context.Context) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		s.log.Warn().Msg("Scheduler already running")
		return
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	s.wg.Add(1)
	go s.run()

	s.log.Info().Msg("Email sync scheduler started")
}

// Stop stops the background sync scheduler
func (s *Scheduler) Stop() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.wg.Wait()
	s.running = false

	s.log.Info().Msg("Email sync scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	// Initial sync on startup (after a short delay to let the app initialize)
	select {
	case <-time.After(10 * time.Second):
		s.syncDueAccounts()
	case <-s.ctx.Done():
		return
	}

	// Periodic check
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncDueAccounts()
		case <-s.ctx.Done():
			return
		}
	}
}

// syncDueAccounts checks all accounts and syncs those that are due
func (s *Scheduler) syncDueAccounts() {
	// Skip sync tick if we know we're offline
	if s.isConnected != nil && !s.isConnected() {
		s.log.Debug().Msg("Skipping sync tick — offline")
		return
	}

	accounts, err := s.accountStore.List()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to list accounts for sync check")
		return
	}

	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}

		// Skip manual-only accounts
		if acc.SyncInterval <= 0 {
			continue
		}

		// Check if sync is due for INBOX
		if !s.isSyncDue(acc) {
			continue
		}

		s.log.Debug().Str("account", acc.Name).Msg("Account is due for sync")

		// Sync in background (don't block the scheduler)
		go s.syncAccountInbox(acc, TriggerScheduled)
	}
}

// isSyncDue returns true if an account's INBOX is due for sync
func (s *Scheduler) isSyncDue(acc *account.Account) bool {
	interval := time.Duration(acc.SyncInterval) * time.Minute
	s.probeMu.Lock()
	attemptedAt := s.lastAttempts[acc.ID]
	s.probeMu.Unlock()

	// Get the INBOX folder for this account
	inbox, err := s.folderStore.GetByType(acc.ID, folder.TypeInbox)
	if err != nil {
		s.log.Warn().Err(err).Str("account", acc.ID).Msg("Failed to get INBOX folder")
		return attemptedAt.IsZero() || time.Since(attemptedAt) >= interval
	}
	if inbox == nil {
		return attemptedAt.IsZero() || time.Since(attemptedAt) >= interval
	}

	// Use the newest successful folder sync or scheduled attempt. Failed probes
	// therefore fall back to the configured account interval instead of being
	// retried by every one-minute scheduler tick.
	var reference time.Time
	if inbox.LastSync != nil {
		reference = *inbox.LastSync
	}
	if attemptedAt.After(reference) {
		reference = attemptedAt
	}
	if reference.IsZero() {
		return true
	}
	elapsed := time.Since(reference)
	return elapsed >= interval
}

// syncAccountInbox syncs the INBOX for an account
func (s *Scheduler) syncAccountInbox(acc *account.Account, trigger Trigger) {
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	err := s.coordinator.Do(parent, acc.ID, trigger, func(ctx context.Context) error {
		// A scheduled request may have waited behind IDLE/manual work. Recheck at
		// execution time so completed foreground work subsumes the pending tick.
		if trigger == TriggerScheduled && !s.isSyncDue(acc) {
			return nil
		}
		if trigger == TriggerScheduled {
			s.probeMu.Lock()
			s.lastAttempts[acc.ID] = time.Now()
			s.probeMu.Unlock()
		}
		s.runAccountSyncLifecycle(acc.ID, func() bool {
			return s.syncAccountInboxWork(ctx, acc, trigger)
		})
		return nil
	})
	if err != nil && !errors.Is(err, ErrCoalesced) && !errors.Is(err, context.Canceled) {
		s.log.Warn().Err(err).Str("account", acc.Name).Msg("Coordinated sync failed")
	}
}

func (s *Scheduler) syncAccountInboxWork(parent context.Context, acc *account.Account, trigger Trigger) bool {

	// Create a cancellable context with timeout for this sync operation
	// 30 minute timeout prevents syncs from running forever if connection hangs
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	s.syncCancelMu.Lock()
	s.syncCancels[acc.ID] = cancel
	s.syncCancelMu.Unlock()

	defer func() {
		// Clean up cancel function
		cancel() // Always call cancel to release timeout resources
		s.syncCancelMu.Lock()
		delete(s.syncCancels, acc.ID)
		s.syncCancelMu.Unlock()
	}()

	s.log.Info().Str("account", acc.Name).Msg("Starting account INBOX sync")
	previousInboxCount := 0
	hadInboxBeforeProbe := false
	if inboxBeforeProbe, inboxErr := s.folderStore.GetByType(acc.ID, folder.TypeInbox); inboxErr == nil && inboxBeforeProbe != nil {
		previousInboxCount = inboxBeforeProbe.TotalCount
		hadInboxBeforeProbe = true
	}

	// First, ensure folders are synced
	folderResult, err := s.engine.SyncFoldersWithResult(ctx, acc.ID)
	if err != nil {
		if ctx.Err() != nil {
			s.log.Info().Str("account", acc.Name).Msg("Sync cancelled during folder sync")
			s.notifySyncCompleted(acc.ID, "", MessageSyncResult{}, ctx.Err())
			return false
		}
		s.log.Error().Err(err).Str("account", acc.Name).Msg("Failed to sync folders")
		s.notifySyncCompleted(acc.ID, "", MessageSyncResult{}, err)
		return false
	}

	forceAllFolders := trigger != TriggerScheduled || !folderResult.Complete
	changedPaths := s.changedFolderPaths(acc.ID, folderResult, forceAllFolders)

	// Get the INBOX folder
	inbox, err := s.folderStore.GetByType(acc.ID, folder.TypeInbox)
	if err != nil {
		s.log.Error().Err(err).Str("account", acc.Name).Msg("Failed to get INBOX folder")
		s.notifySyncCompleted(acc.ID, "", MessageSyncResult{}, err)
		return false
	}
	if inbox == nil {
		s.log.Warn().Str("account", acc.Name).Msg("INBOX folder not found")
		s.notifySyncCompleted(acc.ID, "", MessageSyncResult{}, errors.New("INBOX folder not found"))
		return false
	}
	if !hadInboxBeforeProbe {
		previousInboxCount = inbox.TotalCount
	}

	// STATUS is only a fast-path hint. Some IMAP servers can return a stale
	// STATUS snapshot for Sent or Drafts after another client writes there.
	// Always perform the cheap incremental UID check for the core folders;
	// continue using snapshot deltas for unchanged non-core folders.
	changedPaths[inbox.Path] = true
	if configuredFolders, listErr := s.getAccountSyncFolders(acc); listErr == nil {
		includeRequiredFolderPaths(changedPaths, configuredFolders, forceAllFolders)
	}

	allSucceeded := true
	if changedPaths[inbox.Path] {
		// Sync messages (use account's retention + daily sync strategy settings)
		messageResult, syncErr := s.engine.SyncMessagesWithOptionsResult(ctx, acc.ID, inbox.ID, MessageSyncOptionsFromAccount(acc))
		if syncErr != nil {
			if ctx.Err() != nil {
				s.log.Info().Str("account", acc.Name).Msg("Sync cancelled during message sync")
				s.notifySyncCompleted(acc.ID, inbox.ID, messageResult, ctx.Err())
				return false
			}
			s.log.Error().Err(syncErr).Str("account", acc.Name).Msg("Failed to sync messages")
			s.notifySyncCompleted(acc.ID, inbox.ID, messageResult, syncErr)
			return false
		}

		updatedInbox, updateErr := s.folderStore.Get(inbox.ID)
		if updateErr != nil {
			s.log.Error().Err(updateErr).Str("account", acc.Name).Msg("Failed to get updated INBOX folder")
			s.notifySyncCompleted(acc.ID, inbox.ID, messageResult, updateErr)
			return false
		}
		if updatedInbox != nil && updatedInbox.TotalCount > previousInboxCount {
			newCount := updatedInbox.TotalCount - previousInboxCount
			s.log.Info().
				Str("account", acc.Name).
				Int("newMessages", newCount).
				Msg("New messages arrived")

			// Notify about new mail
			if s.newMailCallback != nil {
				s.newMailCallback(NewMailInfo{
					AccountID:   acc.ID,
					AccountName: acc.Name,
					FolderID:    inbox.ID,
					Count:       newCount,
				})
			}
		}

		s.notifySyncCompleted(acc.ID, inbox.ID, messageResult, nil)
	}

	// Sync additional subscribed/all folders (beyond Inbox)
	if !s.syncAdditionalFolders(ctx, acc, inbox, changedPaths) {
		allSucceeded = false
	}

	if allSucceeded && folderResult.Complete {
		s.storeRemoteSnapshot(acc.ID, folderResult.Snapshots)
	}

	s.log.Debug().Str("account", acc.Name).Int("changedFolders", len(changedPaths)).Msg("Scheduled sync completed")
	return allSucceeded && folderResult.Complete
}

func includeRequiredFolderPaths(changedPaths map[string]bool, configuredFolders []*folder.Folder, forceAll bool) {
	for _, configuredFolder := range configuredFolders {
		if configuredFolder == nil || configuredFolder.NoSelect {
			continue
		}
		if forceAll || isCoreFolderType(configuredFolder.Type) {
			changedPaths[configuredFolder.Path] = true
		}
	}
}

func isCoreFolderType(folderType folder.Type) bool {
	switch folderType {
	case folder.TypeInbox, folder.TypeDrafts, folder.TypeSent:
		return true
	default:
		return false
	}
}

// syncAdditionalFolders syncs subscribed or all folders beyond Inbox,
// based on the account's SyncAllFolders setting.
func (s *Scheduler) syncAdditionalFolders(ctx context.Context, acc *account.Account, inbox *folder.Folder, changedPaths map[string]bool) bool {
	folders, err := s.getAccountSyncFolders(acc)
	if err != nil {
		s.log.Warn().Err(err).Str("account", acc.Name).Msg("Failed to get sync folders")
		return false
	}

	// Filter out Inbox (already synced) and limit to 2 concurrent syncs
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	allSucceeded := true
	var resultMu sync.Mutex

	for _, f := range folders {
		// Skip Inbox — already synced above with notification handling
		if inbox != nil && f.ID == inbox.ID {
			continue
		}
		if !changedPaths[f.Path] {
			continue
		}

		// Check context
		if ctx.Err() != nil {
			resultMu.Lock()
			allSucceeded = false
			resultMu.Unlock()
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(f *folder.Folder) {
			defer wg.Done()
			defer func() { <-sem }()

			result, syncErr := s.engine.SyncMessagesWithOptionsResult(ctx, acc.ID, f.ID, MessageSyncOptionsFromAccount(acc))
			if syncErr != nil {
				resultMu.Lock()
				allSucceeded = false
				resultMu.Unlock()
				if ctx.Err() == nil {
					s.log.Warn().Err(syncErr).Str("folder", f.Path).Msg("Failed to sync additional folder")
				}
			}
			// Notify completion for this folder
			s.notifySyncCompleted(acc.ID, f.ID, result, syncErr)
		}(f)
	}
	wg.Wait()
	return allSucceeded
}

func (s *Scheduler) changedFolderPaths(accountID string, result FolderSyncResult, forceAll bool) map[string]bool {
	changed := make(map[string]bool)
	if forceAll {
		for path, snapshot := range result.Snapshots {
			if !snapshot.NoSelect {
				changed[path] = true
			}
		}
		return changed
	}

	s.probeMu.Lock()
	previous, found := s.remoteSnapshots[accountID]
	s.probeMu.Unlock()
	if !found {
		for path, snapshot := range result.Snapshots {
			if !snapshot.NoSelect {
				changed[path] = true
			}
		}
		return changed
	}

	for path := range ChangedFolderPaths(previous, result.Snapshots) {
		if snapshot, ok := result.Snapshots[path]; ok && !snapshot.NoSelect {
			changed[path] = true
		}
	}
	return changed
}

func (s *Scheduler) storeRemoteSnapshot(accountID string, snapshots map[string]RemoteFolderSnapshot) {
	copyOfSnapshots := make(map[string]RemoteFolderSnapshot, len(snapshots))
	for path, snapshot := range snapshots {
		copyOfSnapshots[path] = snapshot
	}
	s.probeMu.Lock()
	s.remoteSnapshots[accountID] = copyOfSnapshots
	s.probeMu.Unlock()
}

// getAccountSyncFolders returns the folders to sync for an account.
func (s *Scheduler) getAccountSyncFolders(acc *account.Account) ([]*folder.Folder, error) {
	if acc.SyncAllFolders {
		return s.folderStore.ListSelectable(acc.ID)
	}
	if acc.SyncFoldersEnabled {
		return s.folderStore.ListSubscribed(acc.ID)
	}
	// Default: core folders only (backward compatible)
	coreTypes := []folder.Type{folder.TypeInbox, folder.TypeDrafts, folder.TypeSent}
	var folders []*folder.Folder
	for _, ft := range coreTypes {
		f, err := s.folderStore.GetByType(acc.ID, ft)
		if err != nil {
			continue
		}
		if f != nil {
			folders = append(folders, f)
		}
	}
	return folders, nil
}

// TriggerSync manually triggers a sync for a specific account (non-blocking)
func (s *Scheduler) TriggerSync(accountID string) {
	acc, err := s.accountStore.Get(accountID)
	if err != nil {
		s.log.Error().Err(err).Str("accountID", accountID).Msg("Failed to get account for manual sync")
		return
	}

	go s.syncAccountInbox(acc, TriggerManual)
}

// CancelSync cancels any running sync for the specified account
func (s *Scheduler) CancelSync(accountID string) {
	s.syncCancelMu.Lock()
	if cancel, ok := s.syncCancels[accountID]; ok {
		s.log.Info().Str("accountID", accountID).Msg("Cancelling running sync")
		cancel()
	}
	s.syncCancelMu.Unlock()
}

// TriggerSyncAll manually triggers a sync for all enabled accounts (non-blocking)
func (s *Scheduler) TriggerSyncAll() {
	accounts, err := s.accountStore.List()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to list accounts for manual sync")
		return
	}

	for _, acc := range accounts {
		if acc.Enabled {
			go s.syncAccountInbox(acc, TriggerManual)
		}
	}
}

// SyncAccountInboxBlocking syncs INBOX and returns new mail info (blocking)
// This is useful for IDLE-triggered syncs where we want to wait for completion
func (s *Scheduler) SyncAccountInboxBlocking(accountID string) (*NewMailInfo, error) {
	info, _, err := s.SyncAccountInboxBlockingWithResult(accountID)
	return info, err
}

// SyncAccountInboxBlockingWithResult is the result-bearing variant used by the
// IDLE path so its final activity record includes actual local changes.
func (s *Scheduler) SyncAccountInboxBlockingWithResult(accountID string) (*NewMailInfo, MessageSyncResult, error) {
	acc, err := s.accountStore.Get(accountID)
	if err != nil {
		return nil, MessageSyncResult{}, err
	}
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}

	var info *NewMailInfo
	var result MessageSyncResult
	err = s.coordinator.Do(parent, acc.ID, TriggerIdle, func(ctx context.Context) error {
		var syncErr error
		info, result, syncErr = s.syncAccountInboxBlockingWork(ctx, acc)
		return syncErr
	})
	if errors.Is(err, ErrCoalesced) {
		s.log.Debug().Str("account", acc.Name).Msg("IDLE trigger coalesced into pending account sync")
		return nil, MessageSyncResult{}, nil
	}
	return info, result, err
}

func (s *Scheduler) syncAccountInboxBlockingWork(parent context.Context, acc *account.Account) (*NewMailInfo, MessageSyncResult, error) {
	// Create a cancellable context with timeout for this sync operation
	// 30 minute timeout prevents syncs from running forever if connection hangs
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	s.syncCancelMu.Lock()
	s.syncCancels[acc.ID] = cancel
	s.syncCancelMu.Unlock()

	defer func() {
		// Clean up cancel function
		cancel() // Always call cancel to release timeout resources
		s.syncCancelMu.Lock()
		delete(s.syncCancels, acc.ID)
		s.syncCancelMu.Unlock()
	}()

	// Get the INBOX folder
	inbox, err := s.folderStore.GetByType(acc.ID, folder.TypeInbox)
	if err != nil {
		return nil, MessageSyncResult{}, err
	}
	if inbox == nil {
		// Try syncing folders first
		folderResult, err := s.engine.SyncFoldersWithResult(ctx, acc.ID)
		if err != nil {
			if ctx.Err() != nil {
				s.log.Info().Str("account", acc.Name).Msg("Sync cancelled during folder sync")
				return nil, MessageSyncResult{}, ctx.Err()
			}
			return nil, MessageSyncResult{}, err
		}
		if folderResult.Complete {
			s.storeRemoteSnapshot(acc.ID, folderResult.Snapshots)
		}
		inbox, err = s.folderStore.GetByType(acc.ID, folder.TypeInbox)
		if err != nil || inbox == nil {
			return nil, MessageSyncResult{}, err
		}
	}
	previousCount := inbox.TotalCount

	// Sync messages (use account's retention + daily sync strategy settings)
	messageResult, err := s.engine.SyncMessagesWithOptionsResult(ctx, acc.ID, inbox.ID, MessageSyncOptionsFromAccount(acc))
	if err != nil {
		if ctx.Err() != nil {
			s.log.Info().Str("account", acc.Name).Msg("Sync cancelled during message sync")
			return nil, messageResult, ctx.Err()
		}
		return nil, messageResult, err
	}

	updatedInbox, err := s.folderStore.Get(inbox.ID)
	if err != nil {
		return nil, messageResult, err
	}
	if updatedInbox != nil && updatedInbox.TotalCount > previousCount {
		return &NewMailInfo{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			FolderID:    inbox.ID,
			Count:       updatedInbox.TotalCount - previousCount,
		}, messageResult, nil
	}

	return nil, messageResult, nil
}
