package app

import (
	"fmt"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
)

// resolveSpecialFolder is the single source of truth for resolving manual
// special-folder mappings. Missing, stale, or hierarchy-only mappings fall
// back to the server-detected selectable folder; storage failures do not.
func resolveSpecialFolder(accountStore *account.Store, folderStore *folder.Store, accountID string, folderType folder.Type) (*folder.Folder, error) {
	if accountStore == nil {
		return nil, fmt.Errorf("account store is not ready")
	}
	if folderStore == nil {
		return nil, fmt.Errorf("folder store is not ready")
	}

	acc, err := accountStore.Get(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	mappedPath := acc.GetFolderMapping(string(folderType))
	if mappedPath != "" {
		mapped, err := folderStore.GetByPath(accountID, mappedPath)
		if err != nil {
			return nil, err
		}
		if mapped.IsSelectable() {
			return mapped, nil
		}
	}

	return folderStore.GetByType(accountID, folderType)
}
