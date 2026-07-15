package sync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func TestRestoreMessageBodyFromReaderPersistsSanitizedBodyAndAttachmentsAtomically(t *testing.T) {
	db, engine, messageStore, attachmentStore := newRestoreTestEngine(t)
	seedRestoreMessage(t, db, "message-1", "expected@example.com")
	if err := attachmentStore.Create(&message.Attachment{
		ID: "old-attachment", MessageID: "message-1", Filename: "old.txt", ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("seed old attachment: %v", err)
	}

	raw := strings.Join([]string{
		"Message-ID: <expected@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>Recovered body</p><script>alert(1)</script>",
		"--mixed",
		"Content-Type: application/pdf; name=report.pdf",
		"Content-Disposition: attachment; filename=report.pdf",
		"Content-Transfer-Encoding: base64",
		"",
		"UERGREFUQQ==",
		"--mixed--",
		"",
	}, "\r\n")

	updated, err := engine.RestoreMessageBodyFromReader(context.Background(), "message-1", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("RestoreMessageBodyFromReader: %v", err)
	}
	if !updated.BodyFetched || !strings.Contains(updated.BodyHTML, "Recovered body") || strings.Contains(updated.BodyHTML, "<script") {
		t.Fatalf("unexpected recovered body: fetched=%v html=%q", updated.BodyFetched, updated.BodyHTML)
	}
	var bodyFailed int
	if err := db.QueryRow(`SELECT body_failed FROM messages WHERE id = ?`, "message-1").Scan(&bodyFailed); err != nil {
		t.Fatalf("read body_failed: %v", err)
	}
	if bodyFailed != 0 {
		t.Fatalf("body_failed = %d, want 0", bodyFailed)
	}
	attachments, err := attachmentStore.GetByMessage("message-1")
	if err != nil {
		t.Fatalf("GetByMessage: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachments were not replaced atomically: %#v", attachments)
	}

	_ = messageStore
}

func TestRestoreMessageBodyFromReaderRejectsIdentityMismatchWithoutMutation(t *testing.T) {
	db, engine, _, attachmentStore := newRestoreTestEngine(t)
	seedRestoreMessage(t, db, "message-2", "expected@example.com")
	if err := attachmentStore.Create(&message.Attachment{
		ID: "keep-attachment", MessageID: "message-2", Filename: "keep.txt", ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}

	raw := "Message-ID: <different@example.com>\r\nContent-Type: text/plain\r\n\r\nwrong message"
	if _, err := engine.RestoreMessageBodyFromReader(context.Background(), "message-2", strings.NewReader(raw)); !IsMessageIdentityMismatchError(err) {
		t.Fatalf("error = %v, want MessageIdentityMismatchError", err)
	}

	var bodyFetched, bodyFailed int
	if err := db.QueryRow(`SELECT body_fetched, body_failed FROM messages WHERE id = ?`, "message-2").Scan(&bodyFetched, &bodyFailed); err != nil {
		t.Fatalf("read body state: %v", err)
	}
	if bodyFetched != 0 || bodyFailed != 1 {
		t.Fatalf("body state mutated after mismatch: fetched=%d failed=%d", bodyFetched, bodyFailed)
	}
	attachments, err := attachmentStore.GetByMessage("message-2")
	if err != nil {
		t.Fatalf("GetByMessage: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "keep.txt" {
		t.Fatalf("attachments mutated after mismatch: %#v", attachments)
	}
}

func TestValidateMessageIdentityFromReaderDoesNotReadLargeBody(t *testing.T) {
	db, engine, _, _ := newRestoreTestEngine(t)
	seedRestoreMessage(t, db, "message-3", "expected@example.com")
	raw := "Message-ID: <expected@example.com>\r\nContent-Type: application/octet-stream\r\n\r\n" + strings.Repeat("x", maxHeaderLiteralBytes+1)

	if err := engine.ValidateMessageIdentityFromReader(context.Background(), "message-3", strings.NewReader(raw)); err != nil {
		t.Fatalf("ValidateMessageIdentityFromReader read into large body: %v", err)
	}
}

func newRestoreTestEngine(t *testing.T) (*database.DB, *Engine, *message.Store, *message.AttachmentStore) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"acct", "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"folder", "acct", "Other", "Other", "folder", 7, 0,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	engine := NewEngine(nil, account.NewStore(db), folder.NewStore(db), messageStore, attachmentStore, nil)
	return db, engine, messageStore, attachmentStore
}

func seedRestoreMessage(t *testing.T, db *database.DB, id, rfcMessageID string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO messages (id, account_id, folder_id, uid, message_id, subject, from_name, from_email, date, body_fetched, body_failed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "acct", "folder", 1, rfcMessageID, "Recover me", "Sender", "sender@example.com", "2026-07-15T00:00:00Z", 0, 1,
	); err != nil {
		t.Fatalf("seed message: %v", err)
	}
}
