package app

import (
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Undo API - Exposed to frontend via Wails bindings
// ============================================================================

// Undo reverses the most recent undoable action
// Returns the description of what was undone, or error if nothing to undo
func (a *App) Undo() (string, error) {
	cmd := a.undoStack.Pop()
	if cmd == nil {
		return "", fmt.Errorf("nothing to undo")
	}

	if err := cmd.Undo(); err != nil {
		return "", fmt.Errorf("undo failed: %w", err)
	}

	// Emit event to refresh UI
	wailsRuntime.EventsEmit(a.ctx, "undo:completed", cmd.Description())

	return cmd.Description(), nil
}

// ============================================================================
// UndoContext Implementation - Required for undo.Command operations
// ============================================================================

// FindLocalMessageIDs implements undo.UndoContext
// Finds current local DB message IDs by RFC822 Message-ID header and folder
func (a *App) FindLocalMessageIDs(accountID, folderID string, rfc822MessageIDs []string) ([]string, error) {
	return a.messageStore.GetIDsByMessageIDs(accountID, folderID, rfc822MessageIDs)
}

// MoveMessagesToFolder implements undo.UndoContext
// Delegates to the standard MoveToFolder pipeline (IMAP + local DB + events)
func (a *App) MoveMessagesToFolder(messageIDs []string, destFolderID string) error {
	return a.MoveToFolder(messageIDs, destFolderID)
}
