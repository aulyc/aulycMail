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

// undoContextAdapter keeps undo's narrow interface off the Wails-bound App
// method set while still delegating to the normal local/IMAP move pipeline.
type undoContextAdapter struct {
	app *App
}

func (c undoContextAdapter) FindLocalMessageIDs(accountID, folderID string, rfc822MessageIDs []string) ([]string, error) {
	return c.app.messageStore.GetIDsByMessageIDs(accountID, folderID, rfc822MessageIDs)
}

func (c undoContextAdapter) MoveMessagesToFolder(messageIDs []string, destFolderID string) error {
	return c.app.MoveToFolder(messageIDs, destFolderID)
}
