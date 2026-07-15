package backup

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"aulyc.local/aulycmail/internal/database"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

func TestFindIndexedMessageFileUsesIdentityAndRejectsUnsafePaths(t *testing.T) {
	directory := t.TempDir()
	row := MessageRow{AccountID: "acct", FolderID: "folder", UIDValidity: 9, UID: 42, MessageID: "same@example.com"}
	relPath := "eml/test@example.com/INBOX/message.eml"
	if _, err := WriteFileFromReader(directory, relPath, io.NopCloser(io.LimitReader(&zeroReader{}, 1))); err != nil {
		t.Fatalf("write indexed eml: %v", err)
	}
	idx := &Index{
		Version: IndexVersion,
		Messages: map[string]IndexMessage{
			MessageKey(row): {
				AccountID: "acct", FolderID: "folder", UIDValidity: 9, UID: 42, EMLPath: relPath,
			},
		},
	}
	if err := SaveIndex(directory, idx); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	got, found, err := FindIndexedMessageFile(directory, row.AccountID, row.FolderID, row.UIDValidity, row.UID, row.MessageID)
	if err != nil {
		t.Fatalf("FindIndexedMessageFile: %v", err)
	}
	if !found || got != filepath.Join(directory, filepath.FromSlash(relPath)) {
		t.Fatalf("unexpected lookup result: path=%q found=%v", got, found)
	}

	if _, found, err := FindIndexedMessageFile(directory, "acct", "folder", 9, 999, "missing@example.com"); err != nil || found {
		t.Fatalf("missing identity lookup: found=%v err=%v", found, err)
	}

	entry := idx.Messages[MessageKey(row)]
	entry.EMLPath = "../outside.eml"
	idx.Messages[MessageKey(row)] = entry
	if err := SaveIndex(directory, idx); err != nil {
		t.Fatalf("SaveIndex unsafe entry: %v", err)
	}
	if _, _, err := FindIndexedMessageFile(directory, row.AccountID, row.FolderID, row.UIDValidity, row.UID, row.MessageID); err == nil {
		t.Fatal("expected unsafe indexed path to be rejected")
	}
}

func TestFindIndexedMessageFileFallsBackToSameMessageIDInSelectableFolder(t *testing.T) {
	directory := t.TempDir()
	relPath := "eml/test@example.com/POC/message.eml"
	if _, err := WriteFileFromReader(directory, relPath, io.NopCloser(io.LimitReader(&zeroReader{}, 1))); err != nil {
		t.Fatalf("write indexed eml: %v", err)
	}
	indexedRow := MessageRow{
		AccountID: "acct", FolderID: "selectable-child", UIDValidity: 11, UID: 8,
		MessageID: "<same@example.com>",
	}
	if err := SaveIndex(directory, &Index{
		Version: IndexVersion,
		Messages: map[string]IndexMessage{
			MessageKey(indexedRow): {
				AccountID: "acct", FolderID: "selectable-child", UIDValidity: 11, UID: 8,
				MessageID: "same@example.com", EMLPath: relPath,
			},
		},
	}); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	got, found, err := FindIndexedMessageFile(directory, "acct", "stale-parent", 7, 42, "same@example.com")
	if err != nil {
		t.Fatalf("FindIndexedMessageFile: %v", err)
	}
	if !found || got != filepath.Join(directory, filepath.FromSlash(relPath)) {
		t.Fatalf("same-Message-ID fallback failed: path=%q found=%v", got, found)
	}
}

func TestRunClassifiesUnindexedNonSelectableMessagesWithoutStreaming(t *testing.T) {
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
		"directory", "acct", "Other", "Other", "folder", 7, 0,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	for _, row := range []struct {
		id  string
		uid int
	}{
		{id: "indexed", uid: 1},
		{id: "unavailable", uid: 2},
		{id: "duplicate", uid: 3},
		{id: "envelope", uid: 4},
	} {
		if _, err := db.Exec(
			`INSERT INTO messages (id, account_id, folder_id, uid, message_id, subject, from_name, from_email, date)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.id, "acct", "directory", row.uid, row.id+"@example.com", row.id,
			"Sender", "sender@example.com", "2026-07-15T00:00:00Z",
		); err != nil {
			t.Fatalf("seed message %s: %v", row.id, err)
		}
	}
	if _, err := db.Exec(`UPDATE messages SET message_id = NULL WHERE id = 'envelope'`); err != nil {
		t.Fatalf("clear envelope Message-ID: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"selectable-child", "acct", "POC", "POC", "folder", 11, 1,
	); err != nil {
		t.Fatalf("seed selectable folder: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages (id, account_id, folder_id, uid, message_id, subject, from_name, from_email, date)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"real-envelope", "acct", "selectable-child", 8, nil, "envelope",
		"Sender", "sender@example.com", "2026-07-15T00:00:00Z",
	); err != nil {
		t.Fatalf("seed selectable equivalent: %v", err)
	}

	directory := t.TempDir()
	indexedRow := MessageRow{AccountID: "acct", FolderID: "directory", UIDValidity: 7, UID: 1}
	relPath := "eml/test@example.com/Other/indexed.eml"
	if _, err := WriteFileFromReader(directory, relPath, io.NopCloser(io.LimitReader(&zeroReader{}, 1))); err != nil {
		t.Fatalf("write indexed eml: %v", err)
	}
	duplicatePath := "eml/test@example.com/POC/duplicate.eml"
	if _, err := WriteFileFromReader(directory, duplicatePath, io.NopCloser(io.LimitReader(&zeroReader{}, 1))); err != nil {
		t.Fatalf("write duplicate eml: %v", err)
	}
	envelopePath := "eml/test@example.com/POC/envelope.eml"
	if _, err := WriteFileFromReader(directory, envelopePath, io.NopCloser(io.LimitReader(&zeroReader{}, 1))); err != nil {
		t.Fatalf("write envelope eml: %v", err)
	}
	if err := SaveIndex(directory, &Index{
		Version: IndexVersion,
		Messages: map[string]IndexMessage{
			MessageKey(indexedRow): {
				AccountID: "acct", FolderID: "directory", UIDValidity: 7, UID: 1, EMLPath: relPath,
			},
			"acct:selectable-child:11:8": {
				AccountID: "acct", FolderID: "selectable-child", UIDValidity: 11, UID: 8,
				EMLPath: envelopePath,
			},
			"acct:selectable-child:11:9": {
				AccountID: "acct", FolderID: "selectable-child", UIDValidity: 11, UID: 9,
				MessageID: "duplicate@example.com", EMLPath: duplicatePath,
			},
		},
	}); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	streamCalls := 0
	result, err := Run(context.Background(), db, RunOptions{
		Directory:  directory,
		AccountIDs: []string{"acct"},
		StreamRawMessages: func(context.Context, string, string, []uint32, mailSync.RawMessageStreamHandler) (map[uint32]*mailSync.RawMessageStreamResult, map[uint32]error, error) {
			streamCalls++
			return nil, nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if streamCalls != 0 {
		t.Fatalf("raw message streamer called %d times for non-selectable folder", streamCalls)
	}
	if result.Total != 5 || result.Skipped != 4 || result.Unavailable != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}

	reportBytes, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report Report
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if len(report.UnavailableMessages) != 1 || report.UnavailableMessages[0].UID != 2 {
		t.Fatalf("unexpected unavailable report rows: %#v", report.UnavailableMessages)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("non-selectable row was reported as a failure: %#v", report.Failures)
	}
}

type zeroReader struct{}

func (*zeroReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = 0
	return 1, nil
}
