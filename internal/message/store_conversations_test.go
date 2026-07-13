package message

import (
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/database"
)

func newConversationTestStore(t *testing.T) (*Store, string, map[string]string) {
	t.Helper()

	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const accountID = "acct-conversation"
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	folders := map[string]string{
		"inbox":  "folder-inbox",
		"drafts": "folder-drafts",
		"sent":   "folder-sent",
	}
	for folderType, folderID := range folders {
		if _, err := db.Exec(
			`INSERT INTO folders (id, account_id, name, path, folder_type)
			 VALUES (?, ?, ?, ?, ?)`,
			folderID, accountID, folderType, folderType, folderType,
		); err != nil {
			t.Fatalf("seed %s folder: %v", folderType, err)
		}
	}

	return NewStore(db), accountID, folders
}

func TestGetConversationDraftsOnlyIncludesDraftFolder(t *testing.T) {
	store, accountID, folders := newConversationTestStore(t)
	const threadID = "thread@example.com"
	baseDate := time.Date(2026, time.July, 13, 14, 0, 0, 0, time.UTC)

	messages := []*Message{
		{ID: "draft-1", AccountID: accountID, FolderID: folders["drafts"], UID: 1, MessageID: "draft-1@example.com", ThreadID: threadID, Subject: "Draft", Date: baseDate, IsDraft: true},
		{ID: "draft-2", AccountID: accountID, FolderID: folders["drafts"], UID: 2, MessageID: "draft-2@example.com", ThreadID: threadID, Subject: "Draft", Date: baseDate.Add(time.Minute), IsDraft: true},
		{ID: "sent-1", AccountID: accountID, FolderID: folders["sent"], UID: 1, MessageID: "sent-1@example.com", ThreadID: threadID, Subject: "Draft", Date: baseDate.Add(2 * time.Minute)},
	}
	for _, msg := range messages {
		if err := store.Create(msg); err != nil {
			t.Fatalf("Create %s: %v", msg.ID, err)
		}
	}

	conversation, err := store.GetConversation(threadID, folders["drafts"])
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if conversation == nil {
		t.Fatal("GetConversation returned nil")
	}
	if conversation.MessageCount != 2 {
		t.Fatalf("MessageCount = %d, want 2", conversation.MessageCount)
	}
	if len(conversation.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(conversation.Messages))
	}
	for _, msg := range conversation.Messages {
		if msg.FolderID != folders["drafts"] {
			t.Errorf("message %s came from folder %s, want drafts folder", msg.ID, msg.FolderID)
		}
	}
}
