package undo

import (
	"fmt"
)

// UndoContext provides dependencies for undo operations
type UndoContext interface {
	// FindLocalMessageIDs finds current local DB message IDs by RFC822 Message-ID and folder
	FindLocalMessageIDs(accountID, folderID string, rfc822MessageIDs []string) ([]string, error)
	// MoveMessagesToFolder moves messages using the full move pipeline (IMAP + local DB)
	MoveMessagesToFolder(messageIDs []string, destFolderID string) error
}

// MoveCommand handles moving messages between folders
type MoveCommand struct {
	BaseCommand
	undoCtx          UndoContext
	accountID        string
	rfc822MessageIDs []string // RFC822 Message-ID headers for reliable lookup
	sourceFolderID   string
	destFolderID     string
}

// NewMoveCommand creates a new MoveCommand
func NewMoveCommand(
	undoCtx UndoContext,
	accountID string,
	rfc822MessageIDs []string,
	sourceFolderID string,
	destFolderID string,
	description string,
) *MoveCommand {
	return &MoveCommand{
		BaseCommand:      NewBaseCommand(description),
		undoCtx:          undoCtx,
		accountID:        accountID,
		rfc822MessageIDs: rfc822MessageIDs,
		sourceFolderID:   sourceFolderID,
		destFolderID:     destFolderID,
	}
}

// Execute performs the action (already done at creation time)
func (c *MoveCommand) Execute() error { return nil }

// Undo reverses the move by finding current messages in the destination folder
// and moving them back using the standard move pipeline.
func (c *MoveCommand) Undo() error {
	// Find current local message IDs by RFC822 Message-ID in the destination folder
	localMsgIDs, err := c.undoCtx.FindLocalMessageIDs(c.accountID, c.destFolderID, c.rfc822MessageIDs)
	if err != nil {
		return fmt.Errorf("failed to find messages: %w", err)
	}
	if len(localMsgIDs) == 0 {
		return fmt.Errorf("messages not found in destination folder")
	}

	// Reuse the full move pipeline (IMAP + local DB + events)
	return c.undoCtx.MoveMessagesToFolder(localMsgIDs, c.sourceFolderID)
}
