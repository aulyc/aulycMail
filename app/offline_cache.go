package app

import (
	"fmt"
	"strings"
	"time"

	"aulyc.local/aulycmail/internal/logging"
)

// OfflineBodyCacheClearResult summarizes a local offline-body cleanup.
type OfflineBodyCacheClearResult struct {
	FoldersScanned     int   `json:"folders"`
	BodiesCleared      int64 `json:"bodiesCleared"`
	AttachmentsDeleted int64 `json:"attachmentsDeleted"`
}

// ClearOfflineBodyCache clears cached message bodies and parsed attachments for
// one account while keeping the local message index/envelope rows intact.
func (a *App) ClearOfflineBodyCache(accountID string) (*OfflineBodyCacheClearResult, error) {
	log := logging.WithComponent("app.offline_cache")
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id is required")
	}
	if a.accountStore == nil || a.folderStore == nil || a.messageStore == nil || a.attachmentStore == nil {
		return nil, fmt.Errorf("app stores are not ready")
	}

	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	if acc == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}

	if a.syncScheduler != nil {
		a.syncScheduler.CancelSync(accountID)
	}
	cancelledFolderSyncs := 0
	a.syncMu.Lock()
	for syncKey, cancel := range a.syncContexts {
		if strings.HasPrefix(syncKey, accountID+":") {
			cancel()
			delete(a.syncContexts, syncKey)
			cancelledFolderSyncs++
		}
	}
	a.syncMu.Unlock()
	if cancelledFolderSyncs > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	result := &OfflineBodyCacheClearResult{}
	for _, f := range folders {
		if f == nil {
			continue
		}
		bodiesCleared, err := a.messageStore.ClearBodiesForFolder(f.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to clear bodies for folder %s: %w", f.Path, err)
		}
		attachmentsDeleted, err := a.attachmentStore.DeleteAttachmentsForFolder(f.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete attachments for folder %s: %w", f.Path, err)
		}

		result.FoldersScanned++
		result.BodiesCleared += bodiesCleared
		result.AttachmentsDeleted += attachmentsDeleted
	}

	log.Info().
		Str("accountID", accountID).
		Int("cancelledFolderSyncs", cancelledFolderSyncs).
		Int("folders", result.FoldersScanned).
		Int64("bodiesCleared", result.BodiesCleared).
		Int64("attachmentsDeleted", result.AttachmentsDeleted).
		Msg("Cleared offline body cache")
	return result, nil
}
