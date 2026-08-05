package message

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/database"
)

func TestAttachmentStoreRoundTripAndInlineContent(t *testing.T) {
	messageStore, accountID, folderID := newBodyFailedTestStore(t)
	if err := messageStore.Create(&Message{
		ID:        "message-with-attachments",
		AccountID: accountID,
		FolderID:  folderID,
		UID:       41,
		Date:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Create message: %v", err)
	}

	store := NewAttachmentStore(messageStore.db)
	regularContent := []byte("regular attachment must not be stored inline")
	inlineContent := []byte("synthetic png bytes")
	regular := &Attachment{
		ID:          "regular-attachment",
		MessageID:   "message-with-attachments",
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Size:        len(regularContent),
		Content:     regularContent,
	}
	inline := &Attachment{
		ID:          "inline-attachment",
		MessageID:   "message-with-attachments",
		Filename:    "logo.png",
		ContentType: "image/png",
		Size:        len(inlineContent),
		ContentID:   "logo@example.com",
		IsInline:    true,
		Content:     inlineContent,
	}
	if err := store.Create(regular); err != nil {
		t.Fatalf("Create regular attachment: %v", err)
	}
	if err := store.Create(inline); err != nil {
		t.Fatalf("Create inline attachment: %v", err)
	}

	attachments, err := store.GetByMessage("message-with-attachments")
	if err != nil {
		t.Fatalf("GetByMessage: %v", err)
	}
	if len(attachments) != 2 || attachments[0].ID != regular.ID || attachments[1].ID != inline.ID {
		t.Fatalf("attachments = %#v, want filename-ordered regular/inline records", attachments)
	}
	gotInline, err := store.Get(inline.ID)
	if err != nil {
		t.Fatalf("Get inline attachment: %v", err)
	}
	if gotInline == nil || !gotInline.IsInline || gotInline.ContentID != inline.ContentID {
		t.Fatalf("inline attachment = %#v", gotInline)
	}
	if missing, err := store.Get("missing"); err != nil || missing != nil {
		t.Fatalf("Get missing = (%#v, %v), want nil/nil", missing, err)
	}

	inlineData, err := store.GetInlineByMessage("message-with-attachments")
	if err != nil {
		t.Fatalf("GetInlineByMessage: %v", err)
	}
	wantDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(inlineContent)
	if len(inlineData) != 1 || inlineData[inline.ContentID] != wantDataURL {
		t.Fatalf("inline data = %#v, want %q", inlineData, wantDataURL)
	}

	var storedRegular, storedInline []byte
	if err := messageStore.db.QueryRow("SELECT content FROM attachments WHERE id = ?", regular.ID).Scan(&storedRegular); err != nil {
		t.Fatalf("read regular content: %v", err)
	}
	if err := messageStore.db.QueryRow("SELECT content FROM attachments WHERE id = ?", inline.ID).Scan(&storedInline); err != nil {
		t.Fatalf("read inline content: %v", err)
	}
	if len(storedRegular) != 0 || !bytes.Equal(storedInline, inlineContent) {
		t.Fatalf("stored content regular=%q inline=%q", storedRegular, storedInline)
	}

	if err := store.UpdateLocalPath(regular.ID, "/tmp/document.pdf"); err != nil {
		t.Fatalf("UpdateLocalPath: %v", err)
	}
	updated, err := store.Get(regular.ID)
	if err != nil || updated == nil || updated.LocalPath != "/tmp/document.pdf" {
		t.Fatalf("updated attachment = (%#v, %v)", updated, err)
	}

	if err := store.DeleteByMessage("message-with-attachments"); err != nil {
		t.Fatalf("DeleteByMessage: %v", err)
	}
	remaining, err := store.GetByMessage("message-with-attachments")
	if err != nil || len(remaining) != 0 {
		t.Fatalf("attachments after delete = (%#v, %v)", remaining, err)
	}
}

func TestAttachmentStoreBatchContinuesAfterDuplicateAndDeletesByFolder(t *testing.T) {
	messageStore, accountID, folderID := newBodyFailedTestStore(t)
	const otherFolderID = "other-folder"
	if _, err := messageStore.db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES (?, ?, ?, ?, ?)`,
		otherFolderID, accountID, "Archive", "Archive", "archive",
	); err != nil {
		t.Fatalf("seed other folder: %v", err)
	}
	now := time.Now().UTC()
	if err := messageStore.UpsertBatch([]*Message{
		{ID: "folder-message", AccountID: accountID, FolderID: folderID, UID: 51, Date: now},
		{ID: "other-message", AccountID: accountID, FolderID: otherFolderID, UID: 52, Date: now},
	}); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	store := NewAttachmentStore(messageStore.db)
	duplicate := &Attachment{ID: "duplicate", MessageID: "folder-message", Filename: "a.txt", ContentType: "text/plain"}
	if err := store.Create(duplicate); err != nil {
		t.Fatalf("seed duplicate: %v", err)
	}
	if err := store.CreateBatch(nil); err != nil {
		t.Fatalf("CreateBatch nil: %v", err)
	}
	if err := store.CreateBatch([]*Attachment{
		duplicate,
		{ID: "folder-good", MessageID: "folder-message", Filename: "b.txt", ContentType: "text/plain"},
		{ID: "other-good", MessageID: "other-message", Filename: "c.txt", ContentType: "text/plain"},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if got, _ := store.Get("folder-good"); got == nil {
		t.Fatal("valid attachment after duplicate was not committed")
	}
	if got, _ := store.Get("other-good"); got == nil {
		t.Fatal("other-folder attachment was not committed")
	}

	deleted, err := store.DeleteAttachmentsForFolder(folderID)
	if err != nil {
		t.Fatalf("DeleteAttachmentsForFolder: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if got, _ := store.Get("duplicate"); got != nil {
		t.Fatalf("folder attachment remains: %#v", got)
	}
	if got, _ := store.Get("other-good"); got == nil {
		t.Fatal("attachment from another folder was deleted")
	}
}

func TestNewAttachmentStoreUpgradesLegacyContentColumn(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "legacy-attachments.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE attachments (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size INTEGER NOT NULL,
			content_id TEXT,
			is_inline INTEGER NOT NULL DEFAULT 0,
			local_path TEXT
		)
	`); err != nil {
		t.Fatalf("create legacy attachments table: %v", err)
	}

	_ = NewAttachmentStore(db)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('attachments') WHERE name = 'content'`).Scan(&count); err != nil {
		t.Fatalf("inspect content column: %v", err)
	}
	if count != 1 {
		t.Fatalf("content column count = %d, want 1", count)
	}
}
