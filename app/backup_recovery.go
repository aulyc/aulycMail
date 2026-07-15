package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/settings"
)

var errLocalMessageSourceUnavailable = errors.New("local message source unavailable")

func (a *App) indexedBackupMessagePath(msg *message.Message) (string, bool, error) {
	if msg == nil {
		return "", false, errors.New("message is nil")
	}
	directory, err := a.settingsStore.Get(settings.KeyBackupDirectory)
	if err != nil {
		return "", false, fmt.Errorf("failed to read configured backup directory: %w", err)
	}
	if strings.TrimSpace(directory) == "" {
		return "", false, nil
	}
	f, err := a.folderStore.Get(msg.FolderID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get message folder: %w", err)
	}
	if f == nil {
		return "", false, fmt.Errorf("folder not found: %s", msg.FolderID)
	}
	path, found, err := mailBackup.FindIndexedMessageFile(
		directory,
		msg.AccountID,
		msg.FolderID,
		f.UIDValidity,
		msg.UID,
		msg.MessageID,
	)
	if err != nil || found || f.IsSelectable() {
		return path, found, err
	}

	equivalent, err := a.messageStore.FindUniqueSelectableEquivalent(msg.ID)
	if err != nil {
		return "", false, err
	}
	if equivalent == nil {
		return "", false, nil
	}
	equivalentFolder, err := a.folderStore.Get(equivalent.FolderID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get equivalent message folder: %w", err)
	}
	if equivalentFolder == nil || !equivalentFolder.IsSelectable() {
		return "", false, nil
	}
	return mailBackup.FindIndexedMessageFile(
		directory,
		equivalent.AccountID,
		equivalent.FolderID,
		equivalentFolder.UIDValidity,
		equivalent.UID,
		equivalent.MessageID,
	)
}

func (a *App) restoreMessageBodyFromBackup(msg *message.Message) (*message.Message, bool, error) {
	path, found, err := a.indexedBackupMessagePath(msg)
	if err != nil || !found {
		return nil, found, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, true, fmt.Errorf("failed to open indexed backup message: %w", err)
	}
	defer file.Close()
	updated, err := a.syncEngine.RestoreMessageBodyFromReader(a.ctx, msg.ID, file)
	if err != nil {
		return nil, true, fmt.Errorf("failed to restore message from indexed backup: %w", err)
	}
	return updated, true, nil
}

func unavailableLocalMessageSourceError() error {
	return fmt.Errorf("%w: only the local envelope record remains; the server mailbox and indexed backup raw message are unavailable", errLocalMessageSourceUnavailable)
}
