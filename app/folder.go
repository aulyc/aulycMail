package app

import (
	"context"
	"fmt"

	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/logging"
	syncengine "aulyc.local/aulycmail/internal/sync"
)

// ============================================================================
// Folder API - Exposed to frontend via Wails bindings
// ============================================================================

// GetFolders returns selectable mailboxes for pickers and settings.
// Hierarchy-only IMAP folders remain available through GetFolderTree.
func (a *App) GetFolders(accountID string) ([]*folder.Folder, error) {
	folders, err := a.folderStore.ListSelectable(accountID)
	if err != nil {
		return nil, err
	}

	// Sort folders (special folders first)
	folder.SortFolders(folders)
	return folders, nil
}

// GetFolderTree returns folders as a tree structure
func (a *App) GetFolderTree(accountID string) ([]*folder.FolderTree, error) {
	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return nil, err
	}

	// Sort folders (INBOX first, then special folders, then alphabetically)
	folder.SortFolders(folders)
	return folder.BuildTree(folders), nil
}

// syncFolders synchronizes the folder list with the IMAP server.
func (a *App) syncFolders(accountID string) error {
	return a.coordinateAccountSync(accountID, syncengine.TriggerManual, func(ctx context.Context) error {
		return a.syncFoldersDirect(ctx, accountID)
	})
}

func (a *App) syncFoldersDirect(ctx context.Context, accountID string) error {
	log := logging.WithComponent("app")
	if ctx == nil {
		ctx = a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
	}
	err := a.syncEngine.SyncFolders(ctx, accountID)
	if err == nil {
		// Checkpoint WAL after heavy sync operation
		if checkpointErr := a.db.Checkpoint(); checkpointErr != nil {
			log.Warn().Err(checkpointErr).Msg("WAL checkpoint after SyncFolders failed")
		}
	}
	return err
}

// GetAccountFoldersForMapping returns all folders for an account (for folder mapping UI).
// Triggers a folder sync if no folders exist yet.
func (a *App) GetAccountFoldersForMapping(accountID string) ([]*folder.Folder, error) {
	log := logging.WithComponent("app")

	folders, err := a.folderStore.ListSelectable(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	// If no folders, trigger sync first
	if len(folders) == 0 {
		log.Info().Str("accountID", accountID).Msg("No folders found, triggering sync")
		if err := a.syncFolders(accountID); err != nil {
			return nil, fmt.Errorf("failed to sync folders: %w", err)
		}
		folders, err = a.folderStore.ListSelectable(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to list folders after sync: %w", err)
		}
	}

	// Sort using existing logic
	folder.SortFolders(folders)
	return folders, nil
}

// GetAutoDetectedFolders returns the auto-detected special folders for an account.
// Returns map of folder type -> folder path (e.g., {"sent": "Sent Mail", "trash": "Deleted Items"}).
func (a *App) GetAutoDetectedFolders(accountID string) (map[string]string, error) {
	folders, err := a.folderStore.ListSelectable(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	result := make(map[string]string)
	for _, f := range folders {
		if f.Type != folder.TypeFolder && f.Type != folder.TypeInbox {
			result[string(f.Type)] = f.Path
		}
	}
	return result, nil
}

// getSpecialFolder returns the folder for a special type, checking account mappings first.
// If no mapping is set, falls back to auto-detected folder type.
func (a *App) getSpecialFolder(accountID string, folderType folder.Type) (*folder.Folder, error) {
	return resolveSpecialFolder(a.accountStore, a.folderStore, accountID, folderType)
}

// SubscribeFolder subscribes to an IMAP folder for automatic sync.
func (a *App) SubscribeFolder(accountID, folderID string) error {
	log := logging.WithComponent("app")

	f, err := a.folderStore.Get(folderID)
	if err != nil {
		return fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	if f.AccountID != accountID {
		return fmt.Errorf("folder %s does not belong to account %s", folderID, accountID)
	}
	if err := f.RequireSelectable(); err != nil {
		return err
	}

	// Subscribe on IMAP server
	conn, connErr := a.syncEngine.GetPoolConnection(a.ctx, accountID)
	if connErr != nil {
		return fmt.Errorf("failed to get IMAP connection: %w", connErr)
	}
	defer a.syncEngine.ReleasePoolConnection(conn)

	if subErr := conn.Client().Subscribe(f.Path); subErr != nil {
		return fmt.Errorf("IMAP SUBSCRIBE failed: %w", subErr)
	}

	// Update local cache
	if dbErr := a.folderStore.UpdateSubscribed(folderID, true); dbErr != nil {
		log.Warn().Err(dbErr).Msg("Failed to update local subscription cache")
	}

	log.Info().Str("folder", f.Path).Msg("Subscribed to folder")
	return nil
}

// UnsubscribeFolder unsubscribes from an IMAP folder for automatic sync.
func (a *App) UnsubscribeFolder(accountID, folderID string) error {
	log := logging.WithComponent("app")

	f, err := a.folderStore.Get(folderID)
	if err != nil {
		return fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	if f.AccountID != accountID {
		return fmt.Errorf("folder %s does not belong to account %s", folderID, accountID)
	}
	if err := f.RequireSelectable(); err != nil {
		return err
	}

	// Unsubscribe on IMAP server
	conn, connErr := a.syncEngine.GetPoolConnection(a.ctx, accountID)
	if connErr != nil {
		return fmt.Errorf("failed to get IMAP connection: %w", connErr)
	}
	defer a.syncEngine.ReleasePoolConnection(conn)

	if subErr := conn.Client().Unsubscribe(f.Path); subErr != nil {
		return fmt.Errorf("IMAP UNSUBSCRIBE failed: %w", subErr)
	}

	// Update local cache
	if dbErr := a.folderStore.UpdateSubscribed(folderID, false); dbErr != nil {
		log.Warn().Err(dbErr).Msg("Failed to update local subscription cache")
	}

	log.Info().Str("folder", f.Path).Msg("Unsubscribed from folder")
	return nil
}

// SubscribeAllFolders subscribes to all IMAP folders for an account.
func (a *App) SubscribeAllFolders(accountID string) error {
	log := logging.WithComponent("app")

	folders, err := a.folderStore.ListSelectable(accountID)
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	conn, connErr := a.syncEngine.GetPoolConnection(a.ctx, accountID)
	if connErr != nil {
		return fmt.Errorf("failed to get IMAP connection: %w", connErr)
	}
	defer a.syncEngine.ReleasePoolConnection(conn)

	for _, f := range folders {
		if subErr := conn.Client().Subscribe(f.Path); subErr != nil {
			log.Warn().Err(subErr).Str("folder", f.Path).Msg("Failed to subscribe")
			continue
		}
		if err := a.folderStore.UpdateSubscribed(f.ID, true); err != nil {
			log.Warn().Err(err).Str("folder", f.Path).Msg("Failed to update subscribed flag")
		}
	}

	log.Info().Str("accountID", accountID).Int("count", len(folders)).Msg("Subscribed to all folders")
	return nil
}

func (a *App) requireSelectableFolder(folderID string) (*folder.Folder, error) {
	f, err := a.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return nil, err
	}
	return f, nil
}
