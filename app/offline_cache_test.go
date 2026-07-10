package app

import (
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func TestClearOfflineBodyCacheClearsOnlySelectedAccount(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	folderStore := folder.NewStore(db)
	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	testApp := &App{
		accountStore:    accountStore,
		folderStore:     folderStore,
		messageStore:    messageStore,
		attachmentStore: attachmentStore,
	}

	selected := createOfflineCacheTestAccount(t, accountStore, "selected@example.com")
	other := createOfflineCacheTestAccount(t, accountStore, "other@example.com")
	selectedFolder := createOfflineCacheTestFolder(t, folderStore, selected.ID, "selected-inbox")
	otherFolder := createOfflineCacheTestFolder(t, folderStore, other.ID, "other-inbox")
	selectedMessage := createOfflineCacheTestMessage(t, messageStore, selected.ID, selectedFolder.ID, "selected-message")
	otherMessage := createOfflineCacheTestMessage(t, messageStore, other.ID, otherFolder.ID, "other-message")
	createOfflineCacheTestAttachment(t, attachmentStore, selectedMessage.ID, "selected-attachment")
	createOfflineCacheTestAttachment(t, attachmentStore, otherMessage.ID, "other-attachment")

	result, err := testApp.ClearOfflineBodyCache(selected.ID)
	if err != nil {
		t.Fatalf("ClearOfflineBodyCache: %v", err)
	}
	if result.FoldersScanned != 1 {
		t.Fatalf("FoldersScanned = %d, want 1", result.FoldersScanned)
	}
	if result.BodiesCleared != 1 {
		t.Fatalf("BodiesCleared = %d, want 1", result.BodiesCleared)
	}
	if result.AttachmentsDeleted != 1 {
		t.Fatalf("AttachmentsDeleted = %d, want 1", result.AttachmentsDeleted)
	}

	assertOfflineCacheBodyState(t, db, selectedMessage.ID, 0)
	assertOfflineCacheAttachmentCount(t, db, selectedMessage.ID, 0)
	assertOfflineCacheBodyState(t, db, otherMessage.ID, 1)
	assertOfflineCacheAttachmentCount(t, db, otherMessage.ID, 1)
}

func createOfflineCacheTestAccount(t *testing.T, store *account.Store, email string) *account.Account {
	t.Helper()
	acc, err := store.Create(&account.AccountConfig{
		Name:             email,
		DisplayName:      "Test User",
		Email:            email,
		Username:         email,
		IMAPHost:         "imap.example.com",
		IMAPPort:         993,
		IMAPSecurity:     account.SecurityTLS,
		NoOutgoingServer: true,
		AuthType:         account.AuthPassword,
	})
	if err != nil {
		t.Fatalf("create account %s: %v", email, err)
	}
	return acc
}

func createOfflineCacheTestFolder(t *testing.T, store *folder.Store, accountID, id string) *folder.Folder {
	t.Helper()
	f := &folder.Folder{
		ID:         id,
		AccountID:  accountID,
		Name:       "INBOX",
		Path:       "INBOX",
		Type:       folder.TypeInbox,
		Subscribed: true,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("create folder %s: %v", id, err)
	}
	return f
}

func createOfflineCacheTestMessage(t *testing.T, store *message.Store, accountID, folderID, id string) *message.Message {
	t.Helper()
	msg := &message.Message{
		ID:             id,
		AccountID:      accountID,
		FolderID:       folderID,
		UID:            1,
		MessageID:      id + "@example.com",
		Subject:        "Cached message",
		FromName:       "Sender",
		FromEmail:      "sender@example.com",
		Date:           time.Now().UTC(),
		Snippet:        "cached snippet",
		Size:           100,
		HasAttachments: true,
		BodyText:       "cached body",
		BodyHTML:       "<p>cached body</p>",
		BodyFetched:    true,
		ReceivedAt:     time.Now().UTC(),
	}
	if err := store.Create(msg); err != nil {
		t.Fatalf("create message %s: %v", id, err)
	}
	return msg
}

func createOfflineCacheTestAttachment(t *testing.T, store *message.AttachmentStore, messageID, id string) {
	t.Helper()
	if err := store.Create(&message.Attachment{
		ID:          id,
		MessageID:   messageID,
		Filename:    id + ".png",
		ContentType: "image/png",
		Size:        3,
		ContentID:   id,
		IsInline:    true,
		Content:     []byte("img"),
	}); err != nil {
		t.Fatalf("create attachment %s: %v", id, err)
	}
}

func assertOfflineCacheBodyState(t *testing.T, db *database.DB, messageID string, wantFetched int) {
	t.Helper()
	var bodyFetched int
	var bodyText, bodyHTML any
	if err := db.QueryRow(
		`SELECT body_fetched, body_text, body_html FROM messages WHERE id = ?`,
		messageID,
	).Scan(&bodyFetched, &bodyText, &bodyHTML); err != nil {
		t.Fatalf("query message %s: %v", messageID, err)
	}
	if bodyFetched != wantFetched {
		t.Fatalf("message %s body_fetched = %d, want %d", messageID, bodyFetched, wantFetched)
	}
	if wantFetched == 0 && (bodyText != nil || bodyHTML != nil) {
		t.Fatalf("message %s body cache not cleared: text=%v html=%v", messageID, bodyText, bodyHTML)
	}
}

func assertOfflineCacheAttachmentCount(t *testing.T, db *database.DB, messageID string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE message_id = ?`, messageID).Scan(&count); err != nil {
		t.Fatalf("query attachment count for %s: %v", messageID, err)
	}
	if count != want {
		t.Fatalf("attachment count for %s = %d, want %d", messageID, count, want)
	}
}
