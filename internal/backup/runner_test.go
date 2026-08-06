package backup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestRunExportsRawMessagesReportsFailuresAndResumesIncrementally(t *testing.T) {
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
	for _, f := range []struct {
		id, name, path string
	}{
		{id: "archive", name: "Archive", path: "Archive"},
		{id: "inbox", name: "Inbox", path: "INBOX"},
	} {
		if _, err := db.Exec(
			`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			f.id, "acct", f.name, f.path, "folder", 77, 1,
		); err != nil {
			t.Fatalf("seed folder %s: %v", f.id, err)
		}
	}
	for _, m := range []struct {
		id, folderID, subject string
		uid                   int
		hasAttachments        bool
	}{
		{id: "batch-failure", folderID: "archive", uid: 4, subject: "Batch failure"},
		{id: "exported", folderID: "inbox", uid: 1, subject: "Exported subject", hasAttachments: true},
		{id: "missing", folderID: "inbox", uid: 2, subject: "Missing subject"},
		{id: "single-failure", folderID: "inbox", uid: 3, subject: "Single failure"},
	} {
		if _, err := db.Exec(
			`INSERT INTO messages (
				id, account_id, folder_id, uid, message_id, subject, from_name, from_email,
				date, size, has_attachments
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.id, "acct", m.folderID, m.uid, m.id+"@example.com", m.subject,
			"Sender", "sender@example.com", "2026-07-15T00:00:00Z", 123, m.hasAttachments,
		); err != nil {
			t.Fatalf("seed message %s: %v", m.id, err)
		}
	}

	rawByUID := map[uint32]string{
		1: "From: Sender <sender@example.com>\r\nTo: Test <test@example.com>\r\nSubject: Exported subject\r\nMessage-ID: <exported@example.com>\r\nDate: Wed, 15 Jul 2026 00:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nneedle backup body\r\n",
		2: "From: Sender <sender@example.com>\r\nTo: Test <test@example.com>\r\nSubject: Recovered missing\r\n\r\nmissing recovered\r\n",
		3: "From: Sender <sender@example.com>\r\nTo: Test <test@example.com>\r\nSubject: Recovered failure\r\n\r\nfailure recovered\r\n",
		4: "From: Sender <sender@example.com>\r\nTo: Test <test@example.com>\r\nSubject: Recovered batch\r\n\r\nbatch recovered\r\n",
	}
	directory := t.TempDir()
	firstPass := true
	var chunks [][]uint32
	var progress []Progress
	stream := func(_ context.Context, _ string, folderID string, uids []uint32, handle mailSync.RawMessageStreamHandler) (map[uint32]*mailSync.RawMessageStreamResult, map[uint32]error, error) {
		chunks = append(chunks, append([]uint32(nil), uids...))
		if firstPass && folderID == "archive" {
			return nil, nil, errors.New("mailbox offline")
		}
		results := make(map[uint32]*mailSync.RawMessageStreamResult)
		failures := make(map[uint32]error)
		for _, uid := range uids {
			if firstPass {
				switch uid {
				case 2:
					// No result is the server-missing case the runner must classify.
					continue
				case 3:
					failures[uid] = errors.New("corrupt raw message")
					continue
				}
			}
			written, err := handle(uid, strings.NewReader(rawByUID[uid]))
			if err != nil {
				failures[uid] = err
				continue
			}
			results[uid] = &mailSync.RawMessageStreamResult{BytesWritten: written, ReportedSize: int64(len(rawByUID[uid]))}
		}
		return results, failures, nil
	}

	result, err := Run(context.Background(), db, RunOptions{
		Directory:         directory,
		StartedAt:         "2026-07-15T00:00:00Z",
		AccountIDs:        []string{"acct"},
		RawFetchBatchSize: 2,
		StreamRawMessages: stream,
		EmitProgress:      func(p Progress) { progress = append(progress, p) },
	})
	if err != nil {
		t.Fatalf("Run first pass: %v", err)
	}
	if result.Mode != "full" || result.Total != 4 || result.Exported != 1 || result.Skipped != 0 || result.Missing != 1 || result.Unavailable != 0 || result.Failed != 2 {
		t.Fatalf("unexpected first result: %#v", result)
	}
	if len(chunks) != 3 || len(chunks[0]) != 1 || len(chunks[1]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("unexpected stream chunks: %#v", chunks)
	}
	if len(progress) < 3 || progress[0].Stage != ProgressStageChecking || progress[1].Stage != ProgressStageExporting {
		t.Fatalf("unexpected progress: %#v", progress)
	}
	lastProgress := progress[len(progress)-1]
	if lastProgress.Current != 4 || lastProgress.Exported != 1 || lastProgress.Missing != 1 || lastProgress.Failed != 2 {
		t.Fatalf("unexpected final progress: %#v", lastProgress)
	}

	idx, found, err := LoadIndex(directory)
	if err != nil || !found {
		t.Fatalf("LoadIndex after export: found=%v err=%v", found, err)
	}
	exportedKey := "acct:inbox:77:1"
	exportedEntry, ok := idx.Messages[exportedKey]
	if !ok || exportedEntry.HasAttachments == nil || !*exportedEntry.HasAttachments {
		t.Fatalf("missing exported index entry: %#v", exportedEntry)
	}
	emlPath, err := IndexedFilePath(directory, exportedEntry.EMLPath)
	if err != nil {
		t.Fatalf("IndexedFilePath: %v", err)
	}
	eml, err := os.ReadFile(emlPath)
	if err != nil {
		t.Fatalf("read exported EML: %v", err)
	}
	if string(eml) != rawByUID[1] {
		t.Fatalf("exported EML differs:\n%s", eml)
	}

	viewer, err := OpenViewerIndex(directory)
	if err != nil {
		t.Fatalf("OpenViewerIndex: %v", err)
	}
	page, err := viewer.SearchMessages("test@example.com", "needle", 0, 10)
	if closeErr := viewer.Close(); closeErr != nil {
		t.Fatalf("close viewer index: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if page.Total != 1 || len(page.Messages) != 1 || page.Messages[0].Subject != "Exported subject" {
		t.Fatalf("unexpected viewer search page: %#v", page)
	}

	reportBytes, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read first report: %v", err)
	}
	var report Report
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		t.Fatalf("decode first report: %v", err)
	}
	if len(report.MissingMessages) != 1 || report.MissingMessages[0].UID != 2 || len(report.Failures) != 2 {
		t.Fatalf("unexpected first report: %#v", report)
	}

	firstPass = false
	chunks = nil
	result, err = Run(context.Background(), db, RunOptions{
		Directory:         directory,
		StartedAt:         "2026-07-16T00:00:00Z",
		AccountIDs:        []string{"acct"},
		RawFetchBatchSize: 2,
		StreamRawMessages: stream,
	})
	if err != nil {
		t.Fatalf("Run incremental pass: %v", err)
	}
	if result.Mode != "incremental" || result.Total != 4 || result.Exported != 3 || result.Skipped != 1 || result.Missing != 0 || result.Failed != 0 {
		t.Fatalf("unexpected incremental result: %#v", result)
	}
	idx, found, err = LoadIndex(directory)
	if err != nil || !found || len(idx.Messages) != 4 {
		t.Fatalf("incremental index: found=%v messages=%d err=%v", found, len(idx.Messages), err)
	}
}

func TestRunValidatesStreamerAndHonorsCancellation(t *testing.T) {
	directory := t.TempDir()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if _, err := Run(context.Background(), db, RunOptions{Directory: directory}); err == nil || !strings.Contains(err.Error(), "streamer is not configured") {
		t.Fatalf("missing streamer error = %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES ('acct', 'Test', 'test@example.com', 'imap.example.com', 'smtp.example.com', 'test@example.com')`,
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
		 VALUES ('inbox', 'acct', 'Inbox', 'INBOX', 'inbox', 1, 1)`,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages (id, account_id, folder_id, uid, subject, date)
		 VALUES ('message', 'acct', 'inbox', 1, 'Subject', '2026-07-15T00:00:00Z')`,
	); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Run(ctx, db, RunOptions{
		Directory:  directory,
		AccountIDs: []string{"acct"},
		StreamRawMessages: func(context.Context, string, string, []uint32, mailSync.RawMessageStreamHandler) (map[uint32]*mailSync.RawMessageStreamResult, map[uint32]error, error) {
			t.Fatal("streamer must not run after cancellation")
			return nil, nil, nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run cancellation error = %v, want context.Canceled", err)
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
