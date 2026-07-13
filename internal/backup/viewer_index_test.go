package backup

import (
	"strings"
	"testing"
	"time"
)

func TestViewerIndexListAndSearch(t *testing.T) {
	dir := t.TempDir()
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: =?UTF-8?B?5rWL6K+V5YWo5paH57Si5byV?=",
		"Date: Sat, 04 Jul 2026 12:00:00 +0800",
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"这里是一段可以被全文搜索命中的纯文本正文",
		"--mixed",
		"Content-Type: application/pdf; name=\"report.pdf\"",
		"Content-Disposition: attachment; filename=\"report.pdf\"",
		"",
		"PDFDATA",
		"--mixed--",
		"",
	}, "\r\n")

	relPath := "eml/alice@example.com/INBOX/message.eml"
	if _, err := WriteFileFromReader(dir, relPath, strings.NewReader(raw)); err != nil {
		t.Fatalf("write eml: %v", err)
	}

	viewerIndex, err := OpenViewerIndex(dir)
	if err != nil {
		t.Fatalf("open viewer index: %v", err)
	}
	defer viewerIndex.Close()

	entry := IndexMessage{
		AccountID:      "account",
		AccountEmail:   "alice@example.com",
		FolderID:       "folder",
		FolderPath:     "INBOX",
		Subject:        "测试全文索引",
		Date:           "Sat, 04 Jul 2026 12:00:00 +0800",
		EMLPath:        relPath,
		Size:           len(raw),
		HasAttachments: BoolPtr(true),
		ExportedAt:     "2026-07-04T04:00:00Z",
	}
	if err := viewerIndex.UpsertMessageFromFile(dir, "key-1", entry); err != nil {
		t.Fatalf("upsert message: %v", err)
	}

	listPage, err := viewerIndex.ListMessages("", "newest", 0, 200)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if listPage.Total != 1 || len(listPage.Messages) != 1 {
		t.Fatalf("list page mismatch: %#v", listPage)
	}
	if listPage.Messages[0].AttachmentCount != 1 {
		t.Fatalf("attachment count mismatch: %#v", listPage.Messages[0])
	}

	searchPage, err := viewerIndex.SearchMessages("", "全文搜索", 0, 50)
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if searchPage.Total != 1 || len(searchPage.Messages) != 1 {
		t.Fatalf("search page mismatch: %#v", searchPage)
	}
	if searchPage.Messages[0].Key != "key-1" {
		t.Fatalf("search key mismatch: %#v", searchPage.Messages[0])
	}
}

func TestViewerIndexUpsertTruncatedMultipartReturnsWithoutHanging(t *testing.T) {
	dir := t.TempDir()
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: truncated multipart",
		"Date: Sat, 04 Jul 2026 12:00:00 +0800",
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"searchable body",
		"--mixed",
		"Content-Type: application/octet-stream; name=large.bin",
		"Content-Disposition: attachment; filename=large.bin",
		"",
		strings.Repeat("A", viewerIndexMaxRawBytes+1024),
	}, "\r\n")

	relPath := "eml/alice@example.com/INBOX/truncated.eml"
	if _, err := WriteFileFromReader(dir, relPath, strings.NewReader(raw)); err != nil {
		t.Fatalf("write eml: %v", err)
	}

	viewerIndex, err := OpenViewerIndex(dir)
	if err != nil {
		t.Fatalf("open viewer index: %v", err)
	}
	defer viewerIndex.Close()

	entry := IndexMessage{
		AccountID:      "account",
		AccountEmail:   "alice@example.com",
		FolderID:       "folder",
		FolderPath:     "INBOX",
		Subject:        "truncated multipart",
		Date:           "Sat, 04 Jul 2026 12:00:00 +0800",
		EMLPath:        relPath,
		Size:           len(raw),
		HasAttachments: BoolPtr(true),
		ExportedAt:     "2026-07-04T04:00:00Z",
	}
	done := make(chan error, 1)
	go func() {
		done <- viewerIndex.UpsertMessageFromFile(dir, "key-truncated", entry)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("upsert truncated multipart: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upsert truncated multipart did not return")
	}

	page, err := viewerIndex.ListMessages("", "newest", 0, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if page.Total != 1 || len(page.Messages) != 1 || page.Messages[0].Key != "key-truncated" {
		t.Fatalf("truncated message was not indexed: %#v", page)
	}
}
