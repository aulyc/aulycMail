package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/message"
)

// ============================================================================
// Message API - Exposed to frontend via Wails bindings
// ============================================================================

const maxInlineMessageSourceBytes = 25 * 1024 * 1024

var errInlineMessageSourceTooLarge = errors.New("message source is too large for inline display")

type MessageSourceResult struct {
	Content  string `json:"content,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	Size     int64  `json:"size"`
	TooLarge bool   `json:"tooLarge"`
}

// GetMessageSource fetches the raw RFC822 source of a message from the IMAP server
func (a *App) GetMessageSource(messageID string) (*MessageSourceResult, error) {
	log := logging.WithComponent("app")
	log.Debug().Str("messageID", messageID).Msg("Fetching message source")

	// Get the message to find the account, folder, and UID
	msg, err := a.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	if msg.Size > maxInlineMessageSourceBytes {
		path, size, err := a.writeMessageSourceFile(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to write message source file: %w", err)
		}
		return &MessageSourceResult{FilePath: path, Size: size, TooLarge: true}, nil
	}

	var rawBytes []byte
	var sourceSize int64
	err = a.withStreamedRawMessagePath(msg, func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		sourceSize = info.Size()
		if sourceSize > maxInlineMessageSourceBytes {
			return errInlineMessageSourceTooLarge
		}
		rawBytes, err = os.ReadFile(path)
		return err
	})
	if err != nil {
		if errors.Is(err, errInlineMessageSourceTooLarge) {
			path, size, streamErr := a.writeMessageSourceFile(msg)
			if streamErr != nil {
				return nil, fmt.Errorf("failed to write message source file: %w", streamErr)
			}
			return &MessageSourceResult{FilePath: path, Size: size, TooLarge: true}, nil
		}
		return nil, fmt.Errorf("failed to fetch message source: %w", err)
	}

	return &MessageSourceResult{Content: string(rawBytes), Size: sourceSize}, nil
}

func (a *App) writeMessageSourceFile(msg *message.Message) (string, int64, error) {
	dir := filepath.Join(a.paths.Data, "message-sources")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", 0, err
	}
	cleanupOldMessageSourceFiles(dir, 24*time.Hour)

	tmp, err := os.CreateTemp(dir, "source-*.eml")
	if err != nil {
		return "", 0, err
	}
	path := tmp.Name()

	var size int64
	streamErr := a.withStreamedRawMessage(msg, func(raw io.Reader) error {
		var err error
		size, err = io.Copy(tmp, raw)
		return err
	})
	closeErr := tmp.Close()
	if streamErr != nil {
		_ = os.Remove(path)
		return "", 0, streamErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", 0, closeErr
	}
	return path, size, nil
}

func cleanupOldMessageSourceFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// FetchMessageBody fetches the body for a message on-demand.
// This is called when a message's body hasn't been fetched yet (BodyFetched = false).
// It fetches the body from IMAP, updates the database, and returns the updated message.
func (a *App) FetchMessageBody(messageID string) (*message.Message, error) {
	log := logging.WithComponent("app")
	log.Debug().Str("messageID", messageID).Msg("Fetching message body on-demand")

	// Get the message first to get the account ID
	msg, err := a.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	// If body is already fetched, just return it
	if msg.BodyFetched {
		return msg, nil
	}
	f, err := a.folderStore.Get(msg.FolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", msg.FolderID)
	}
	if !f.IsSelectable() {
		updated, found, restoreErr := a.restoreMessageBodyFromBackup(msg)
		if restoreErr != nil {
			return nil, restoreErr
		}
		if found {
			return updated, nil
		}
		return nil, unavailableLocalMessageSourceError()
	}

	// Fetch the body from IMAP
	updatedMsg, err := a.syncEngine.FetchMessageBody(a.ctx, msg.AccountID, messageID)
	if err == nil {
		return updatedMsg, nil
	}
	serverErr := err
	updatedMsg, found, restoreErr := a.restoreMessageBodyFromBackup(msg)
	if restoreErr != nil {
		return nil, fmt.Errorf("failed to fetch message body from server (%v) and backup recovery failed: %w", serverErr, restoreErr)
	}
	if found {
		return updatedMsg, nil
	}
	return nil, fmt.Errorf("failed to fetch message body: %w", serverErr)
}

// GetConversations returns conversations (threaded messages) for a folder with pagination
// sortOrder can be "newest" (default) or "oldest"
// filter can be "" (all), "unread", "starred", or "attachments"
func (a *App) GetConversations(accountID, folderID string, offset, limit int, sortOrder, filter string) ([]*message.Conversation, error) {
	if _, err := a.requireSelectableFolder(folderID); err != nil {
		return nil, err
	}
	return a.messageStore.ListConversationsByFolder(folderID, offset, limit, sortOrder, filter)
}

// GetConversationCount returns the total conversation count for a folder
func (a *App) GetConversationCount(accountID, folderID, filter string) (int, error) {
	if _, err := a.requireSelectableFolder(folderID); err != nil {
		return 0, err
	}
	return a.messageStore.CountConversationsByFolder(folderID, filter)
}

// GetUnifiedInboxConversations returns conversations from all inbox folders across all accounts
func (a *App) GetUnifiedInboxConversations(offset, limit int, sortOrder, filter string) ([]*message.Conversation, error) {
	return a.messageStore.ListConversationsUnifiedInbox(offset, limit, sortOrder, filter)
}

// GetUnifiedInboxCount returns the total conversation count across all inbox folders
func (a *App) GetUnifiedInboxCount(filter string) (int, error) {
	return a.messageStore.CountConversationsUnifiedInbox(filter)
}

// GetConversation returns all messages in a conversation/thread
func (a *App) GetConversation(threadID, folderID string) (*message.Conversation, error) {
	log := logging.WithComponent("app")
	log.Debug().
		Str("threadID", threadID).
		Str("folderID", folderID).
		Msg("GetConversation called")
	if _, err := a.requireSelectableFolder(folderID); err != nil {
		return nil, err
	}

	conv, err := a.messageStore.GetConversation(threadID, folderID)
	if err != nil {
		log.Error().Err(err).Msg("GetConversation failed")
		return nil, err
	}

	if conv != nil && conv.Messages != nil {
		for i, m := range conv.Messages {
			log.Debug().
				Int("index", i).
				Str("messageID", m.ID).
				Str("subject", m.Subject).
				Int("bodyTextLen", len(m.BodyText)).
				Int("bodyHTMLLen", len(m.BodyHTML)).
				Str("threadID", m.ThreadID).
				Msg("GetConversation message")
		}
	} else {
		log.Debug().Msg("GetConversation returned nil or no messages")
	}

	return conv, nil
}
