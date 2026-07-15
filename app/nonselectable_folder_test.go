package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func TestNoSelectFolderAPIsRejectBeforeLocalMutation(t *testing.T) {
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
	testApp := &App{folderStore: folderStore, messageStore: messageStore}
	acc := createOfflineCacheTestAccount(t, accountStore, "noselect@example.com")

	directory := &folder.Folder{
		ID: "directory", AccountID: acc.ID, Name: "Other", Path: "Other",
		Type: folder.TypeFolder, NoSelect: true,
	}
	inbox := &folder.Folder{
		ID: "inbox", AccountID: acc.ID, Name: "INBOX", Path: "INBOX",
		Type: folder.TypeInbox,
	}
	for _, f := range []*folder.Folder{directory, inbox} {
		if err := folderStore.Create(f); err != nil {
			t.Fatalf("Create folder %s: %v", f.ID, err)
		}
	}

	hidden := &message.Message{
		ID: "hidden", AccountID: acc.ID, FolderID: directory.ID, UID: 1,
		Subject: "stale local row", Date: time.Now().UTC(), IsRead: false,
	}
	visible := &message.Message{
		ID: "visible", AccountID: acc.ID, FolderID: inbox.ID, UID: 1,
		Subject: "visible row", Date: time.Now().UTC(), IsRead: false,
	}
	for _, msg := range []*message.Message{hidden, visible} {
		if err := messageStore.Create(msg); err != nil {
			t.Fatalf("Create message %s: %v", msg.ID, err)
		}
	}

	if _, err := testApp.GetConversations(acc.ID, directory.ID, 0, 20, "newest", ""); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("GetConversations error = %v, want ErrNotSelectable", err)
	}
	if err := testApp.MarkAsRead([]string{hidden.ID}); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("MarkAsRead error = %v, want ErrNotSelectable", err)
	}
	hiddenAfter, err := messageStore.Get(hidden.ID)
	if err != nil {
		t.Fatalf("Get hidden message: %v", err)
	}
	if hiddenAfter.IsRead {
		t.Fatal("hidden message was mutated before no-select rejection")
	}

	if err := testApp.MoveToFolder([]string{visible.ID}, directory.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("MoveToFolder error = %v, want ErrNotSelectable", err)
	}
	visibleAfter, err := messageStore.Get(visible.ID)
	if err != nil {
		t.Fatalf("Get visible message: %v", err)
	}
	if visibleAfter.FolderID != inbox.ID {
		t.Fatalf("visible message moved to %s before rejection", visibleAfter.FolderID)
	}
}
