package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/platform"
	"aulyc.local/aulycmail/internal/settings"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

func TestFetchMessageBodyRecoversNonSelectableMessageFromConfiguredBackup(t *testing.T) {
	testApp, msg, backupDir := newBackupRecoveryTestApp(t)
	raw := strings.Join([]string{
		"Message-ID: <recover@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Recovered from backup",
		"--mixed",
		"Content-Type: text/plain; name=notes.txt",
		"Content-Disposition: attachment; filename=notes.txt",
		"",
		"attachment",
		"--mixed--",
		"",
	}, "\r\n")
	writeBackupRecoveryEML(t, backupDir, msg, raw)

	updated, err := testApp.FetchMessageBody(msg.ID)
	if err != nil {
		t.Fatalf("FetchMessageBody: %v", err)
	}
	if !updated.BodyFetched || !strings.Contains(updated.BodyText, "Recovered from backup") {
		t.Fatalf("body was not recovered: %#v", updated)
	}
	attachments, err := testApp.GetAttachments(msg.ID)
	if err != nil {
		t.Fatalf("GetAttachments: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "notes.txt" {
		t.Fatalf("attachment metadata was not recovered: %#v", attachments)
	}
	attachmentPath := filepath.Join(t.TempDir(), "notes.txt")
	savedPath, err := testApp.DownloadAttachment(attachments[0].ID, attachmentPath)
	if err != nil {
		t.Fatalf("DownloadAttachment: %v", err)
	}
	attachmentContent, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read downloaded attachment: %v", err)
	}
	if string(attachmentContent) != "attachment" {
		t.Fatalf("downloaded attachment content = %q", attachmentContent)
	}
	source, err := testApp.GetMessageSource(msg.ID)
	if err != nil {
		t.Fatalf("GetMessageSource: %v", err)
	}
	if source.Content != raw || source.TooLarge {
		t.Fatalf("message source was not recovered from backup: %#v", source)
	}

	var streamed string
	if err := testApp.withStreamedRawMessagePath(msg, func(path string) error {
		content, err := os.ReadFile(path)
		if err == nil {
			streamed = string(content)
		}
		return err
	}); err != nil {
		t.Fatalf("withStreamedRawMessagePath: %v", err)
	}
	if streamed != raw {
		t.Fatal("attachment raw-message path did not use the indexed backup")
	}
}

func TestFetchMessageBodyExplainsWhenOnlyLocalEnvelopeRemains(t *testing.T) {
	testApp, msg, _ := newBackupRecoveryTestApp(t)

	_, err := testApp.FetchMessageBody(msg.ID)
	if !errors.Is(err, errLocalMessageSourceUnavailable) {
		t.Fatalf("error = %v, want errLocalMessageSourceUnavailable", err)
	}
}

func TestFetchMessageBodyRecoversFromUniqueSelectableEnvelopeEquivalent(t *testing.T) {
	testApp, msg, backupDir := newBackupRecoveryTestApp(t)
	if _, err := testApp.db.Exec(`UPDATE messages SET message_id = NULL WHERE id = ?`, msg.ID); err != nil {
		t.Fatalf("clear stale Message-ID: %v", err)
	}
	msg.MessageID = ""
	f := &folder.Folder{
		ID: "selectable-child", AccountID: msg.AccountID, Name: "POC", Path: "POC",
		Type: folder.TypeFolder, UIDValidity: 11,
	}
	if err := testApp.folderStore.Create(f); err != nil {
		t.Fatalf("Create selectable folder: %v", err)
	}
	equivalent := *msg
	equivalent.ID = "real-message"
	equivalent.FolderID = f.ID
	equivalent.UID = 8
	if err := testApp.messageStore.Create(&equivalent); err != nil {
		t.Fatalf("Create equivalent message: %v", err)
	}
	raw := "Content-Type: text/plain; charset=utf-8\r\n\r\nRecovered by unique envelope"
	relPath := "eml/backup-recovery@example.com/POC/equivalent.eml"
	if _, err := mailBackup.WriteFileFromReader(backupDir, relPath, strings.NewReader(raw)); err != nil {
		t.Fatalf("WriteFileFromReader: %v", err)
	}
	row := mailBackup.MessageRow{
		AccountID: equivalent.AccountID, FolderID: equivalent.FolderID,
		UIDValidity: f.UIDValidity, UID: equivalent.UID,
	}
	if err := mailBackup.SaveIndex(backupDir, &mailBackup.Index{
		Version: mailBackup.IndexVersion,
		Messages: map[string]mailBackup.IndexMessage{
			mailBackup.MessageKey(row): {
				AccountID: equivalent.AccountID, FolderID: equivalent.FolderID,
				UIDValidity: f.UIDValidity, UID: equivalent.UID, EMLPath: relPath,
			},
		},
	}); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	updated, err := testApp.FetchMessageBody(msg.ID)
	if err != nil {
		t.Fatalf("FetchMessageBody: %v", err)
	}
	if !strings.Contains(updated.BodyText, "Recovered by unique envelope") {
		t.Fatalf("body was not recovered from equivalent: %#v", updated)
	}
}

func newBackupRecoveryTestApp(t *testing.T) (*App, *message.Message, string) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	folderStore := folder.NewStore(db)
	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	settingsStore := settings.NewStore(db)
	acc := createOfflineCacheTestAccount(t, accountStore, "backup-recovery@example.com")
	f := &folder.Folder{
		ID: "directory", AccountID: acc.ID, Name: "Other", Path: "Other",
		Type: folder.TypeFolder, UIDValidity: 7, NoSelect: true,
	}
	if err := folderStore.Create(f); err != nil {
		t.Fatalf("Create folder: %v", err)
	}
	msg := &message.Message{
		ID: "recover-message", AccountID: acc.ID, FolderID: f.ID, UID: 42,
		MessageID: "recover@example.com", Subject: "Recover", FromName: "Sender",
		FromEmail: "sender@example.com",
	}
	if err := messageStore.Create(msg); err != nil {
		t.Fatalf("Create message: %v", err)
	}
	if _, err := db.Exec(`UPDATE messages SET body_failed = 1 WHERE id = ?`, msg.ID); err != nil {
		t.Fatalf("mark body failed: %v", err)
	}

	backupDir := t.TempDir()
	dataDir := t.TempDir()
	if err := settingsStore.Set(settings.KeyBackupDirectory, backupDir); err != nil {
		t.Fatalf("set backup directory: %v", err)
	}
	engine := mailSync.NewEngine(nil, accountStore, folderStore, messageStore, attachmentStore, nil)
	return &App{
		ctx: context.Background(), db: db, accountStore: accountStore, folderStore: folderStore,
		messageStore: messageStore, attachmentStore: attachmentStore, settingsStore: settingsStore,
		syncEngine: engine, paths: &platform.Paths{Config: dataDir, Data: dataDir, Cache: dataDir},
	}, msg, backupDir
}

func writeBackupRecoveryEML(t *testing.T, directory string, msg *message.Message, raw string) {
	t.Helper()
	row := mailBackup.MessageRow{
		AccountID: msg.AccountID, FolderID: "selectable-child", UIDValidity: 11, UID: 8,
		MessageID: msg.MessageID,
	}
	relPath := "eml/backup-recovery@example.com/Other/recover.eml"
	if _, err := mailBackup.WriteFileFromReader(directory, relPath, strings.NewReader(raw)); err != nil {
		t.Fatalf("WriteFileFromReader: %v", err)
	}
	if err := mailBackup.SaveIndex(directory, &mailBackup.Index{
		Version: mailBackup.IndexVersion,
		Messages: map[string]mailBackup.IndexMessage{
			mailBackup.MessageKey(row): {
				AccountID: msg.AccountID, FolderID: "selectable-child", UIDValidity: 11, UID: 8,
				MessageID: msg.MessageID, EMLPath: relPath,
			},
		},
	}); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}
}
