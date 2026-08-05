package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/settings"
)

func TestBackupViewerAPIsBeforeAndAfterSQLiteIndex(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.ctx = context.Background()
	a.settingsStore = settings.NewStore(a.db)
	directory := t.TempDir()
	if err := a.settingsStore.Set(settings.KeyBackupDirectory, directory); err != nil {
		t.Fatal(err)
	}

	firstRaw := backupViewerRawMessage(
		"Alice <alice@example.com>",
		"Bob <bob@example.com>",
		"Quarterly report",
		"Sat, 04 Jul 2026 12:00:00 +0800",
		"The searchable quarterly body",
		"report.pdf",
	)
	secondRaw := backupViewerRawMessage(
		"Carol <carol@example.com>",
		"Bob <bob@example.com>",
		"Earlier note",
		"Fri, 03 Jul 2026 12:00:00 +0800",
		"A second searchable body",
		"",
	)
	firstPath := "eml/alice@example.com/INBOX/first.eml"
	secondPath := "eml/carol@example.com/Sent/second.eml"
	for _, fixture := range []struct {
		path string
		raw  string
	}{{firstPath, firstRaw}, {secondPath, secondRaw}} {
		if _, err := mailBackup.WriteFileFromReader(directory, fixture.path, strings.NewReader(fixture.raw)); err != nil {
			t.Fatal(err)
		}
	}
	idx := &mailBackup.Index{
		Version:  mailBackup.IndexVersion,
		Messages: map[string]mailBackup.IndexMessage{},
	}
	idx.Messages["first"] = mailBackup.IndexMessage{
		AccountID:      "account-1",
		AccountEmail:   "alice@example.com",
		FolderID:       "inbox-1",
		FolderPath:     "INBOX",
		UIDValidity:    1,
		UID:            10,
		Subject:        "Quarterly report",
		Date:           "Sat, 04 Jul 2026 12:00:00 +0800",
		EMLPath:        firstPath,
		Size:           len(firstRaw),
		HasAttachments: mailBackup.BoolPtr(true),
	}
	idx.Messages["second"] = mailBackup.IndexMessage{
		AccountID:      "account-2",
		AccountEmail:   "carol@example.com",
		FolderID:       "sent-2",
		FolderPath:     "Sent",
		UIDValidity:    2,
		UID:            20,
		Subject:        "Earlier note",
		Date:           "Fri, 03 Jul 2026 12:00:00 +0800",
		EMLPath:        secondPath,
		Size:           len(secondRaw),
		HasAttachments: mailBackup.BoolPtr(false),
	}
	if err := mailBackup.SaveIndex(directory, idx); err != nil {
		t.Fatal(err)
	}

	catalog, err := a.GetBackupViewerCatalog("")
	if err != nil || catalog.Directory != directory || catalog.MessageCount != 2 || catalog.IndexReady || !catalog.NeedsIndex {
		t.Fatalf("JSON catalog = %#v, %v", catalog, err)
	}
	if len(catalog.Accounts) != 2 || catalog.Accounts[0].AccountEmail != "alice@example.com" || catalog.Accounts[1].AccountEmail != "carol@example.com" {
		t.Fatalf("catalog accounts = %#v", catalog.Accounts)
	}

	page, err := a.ListBackupViewerMessages("", "", "oldest", -4, 1)
	if err != nil || page.Total != 2 || len(page.Messages) != 1 || page.Messages[0].Key != "second" || !page.HasMore {
		t.Fatalf("JSON list page = %#v, %v", page, err)
	}
	page, err = a.ListBackupViewerMessages(directory, "ALICE@example.com", "newest", 0, 5000)
	if err != nil || page.Total != 1 || len(page.Messages) != 1 || page.Messages[0].AttachmentCount != 1 {
		t.Fatalf("filtered JSON list page = %#v, %v", page, err)
	}
	page, err = a.ListBackupViewerMessages(directory, "", "newest", 99, 10)
	if err != nil || page.Total != 2 || len(page.Messages) != 0 || page.HasMore {
		t.Fatalf("past-end JSON list page = %#v, %v", page, err)
	}

	page, err = a.SearchBackupViewerMessages(directory, "", "quarterly", 0, 0)
	if err != nil || page.Total != 1 || page.Messages[0].Key != "first" {
		t.Fatalf("JSON search page = %#v, %v", page, err)
	}
	page, err = a.SearchBackupViewerMessages(directory, "carol@example.com", "sent", 0, 10)
	if err != nil || page.Total != 1 || page.Messages[0].Key != "second" {
		t.Fatalf("JSON account search page = %#v, %v", page, err)
	}

	detail, err := a.GetBackupViewerMessage(directory, "first")
	if err != nil || detail.Subject != "Quarterly report" || detail.BodyText != "The searchable quarterly body" || len(detail.Attachments) != 1 || detail.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("message detail = %#v, %v", detail, err)
	}
	if _, _, err := a.backupViewerIndexedMessagePath(directory, "missing"); err == nil {
		t.Fatal("missing backup key must fail")
	}
	if _, err := a.SaveBackupViewerAttachmentAs(directory, "first", -1); err == nil {
		t.Fatal("negative backup attachment index must fail")
	}
	if _, err := a.SaveBackupViewerAttachmentAs(directory, "first", 9); err == nil {
		t.Fatal("out-of-range backup attachment index must fail before showing a dialog")
	}

	indexed, err := a.BuildBackupViewerIndex("")
	if err != nil || indexed != 2 {
		t.Fatalf("BuildBackupViewerIndex = %d, %v", indexed, err)
	}
	catalog, err = a.GetBackupViewerCatalog(directory)
	if err != nil || !catalog.IndexReady || catalog.NeedsIndex || catalog.MessageCount != 2 {
		t.Fatalf("SQLite catalog = %#v, %v", catalog, err)
	}
	page, err = a.ListBackupViewerMessages(directory, "alice@example.com", "newest", 0, 10)
	if err != nil || page.Total != 1 || page.Messages[0].Key != "first" || page.Messages[0].AttachmentCount != 1 {
		t.Fatalf("SQLite list page = %#v, %v", page, err)
	}
	page, err = a.SearchBackupViewerMessages(directory, "", "searchable quarterly", 0, 10)
	if err != nil || page.Total != 1 || page.Messages[0].Key != "first" || page.Messages[0].Snippet == "" {
		t.Fatalf("SQLite search page = %#v, %v", page, err)
	}
}

func TestBackupViewerEmptyAndInvalidDirectories(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.ctx = context.Background()
	a.settingsStore = settings.NewStore(a.db)

	catalog, err := a.GetBackupViewerCatalog("")
	if err != nil || catalog.MessageCount != 0 || catalog.Directory != "" {
		t.Fatalf("empty catalog = %#v, %v", catalog, err)
	}
	if page, err := a.ListBackupViewerMessages("", "", "", 0, 0); err != nil || page.Total != 0 {
		t.Fatalf("empty list = %#v, %v", page, err)
	}
	if page, err := a.SearchBackupViewerMessages("", "", "anything", 0, 0); err != nil || page.Total != 0 {
		t.Fatalf("empty search = %#v, %v", page, err)
	}
	if _, err := a.BuildBackupViewerIndex(""); err == nil {
		t.Fatal("building without a configured directory must fail")
	}
	if _, err := a.GetBackupViewerMessage("", "key"); err == nil {
		t.Fatal("reading without a configured directory must fail")
	}

	directory := t.TempDir()
	catalog, err = a.GetBackupViewerCatalog(directory)
	if err != nil || catalog.Directory != directory || catalog.MessageCount != 0 || catalog.NeedsIndex {
		t.Fatalf("directory without index catalog = %#v, %v", catalog, err)
	}
	if _, err := a.BuildBackupViewerIndex(directory); err == nil {
		t.Fatal("building without a JSON index must fail")
	}

	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetBackupViewerCatalog(filePath); err == nil {
		t.Fatal("a file path must not be accepted as a backup directory")
	}
}

func backupViewerRawMessage(from, to, subject, date, body, attachmentName string) string {
	lines := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + date,
		"MIME-Version: 1.0",
	}
	if attachmentName == "" {
		lines = append(lines, "Content-Type: text/plain; charset=utf-8", "", body)
		return strings.Join(lines, "\r\n")
	}
	lines = append(lines,
		`Content-Type: multipart/mixed; boundary="viewer"`,
		"",
		"--viewer",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
		"--viewer",
		`Content-Type: application/pdf; name="`+attachmentName+`"`,
		`Content-Disposition: attachment; filename="`+attachmentName+`"`,
		"",
		"PDFDATA",
		"--viewer--",
		"",
	)
	return strings.Join(lines, "\r\n")
}
