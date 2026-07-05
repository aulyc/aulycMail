package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBackupMessageKeyIncludesUIDValidity(t *testing.T) {
	first := backupMessageRow{
		AccountID:   "account-1",
		FolderID:    "folder-1",
		UIDValidity: 100,
		UID:         42,
	}
	second := first
	second.UIDValidity = 200

	if backupMessageKey(first) == backupMessageKey(second) {
		t.Fatal("expected backup keys to differ when UIDVALIDITY changes")
	}
}

func TestBackupMessageRelativePathIncludesStableMetadata(t *testing.T) {
	row := backupMessageRow{
		AccountEmail: "user@example.com",
		FolderPath:   "INBOX/Reports",
		UIDValidity:  42,
		UID:          99,
		Subject:      "Q2/Risk:Plan",
		Date:         time.Date(2026, 7, 4, 12, 13, 14, 0, time.UTC),
	}

	got := backupMessageRelativePath(row)
	want := "eml/user@example.com/INBOX/Reports/20260704-121314_uv42_uid99_Q2_Risk_Plan.eml"
	if got != want {
		t.Fatalf("backup path mismatch\nwant: %s\n got: %s", want, got)
	}
	if strings.Contains(got, "Risk:Plan") {
		t.Fatalf("expected invalid filename characters to be sanitized: %s", got)
	}
}

func TestNormalizeBackupScope(t *testing.T) {
	if got := normalizeBackupScope(backupScopeSelected); got != backupScopeSelected {
		t.Fatalf("expected selected scope, got %q", got)
	}
	for _, input := range []string{"", "unknown", backupScopeAll} {
		if got := normalizeBackupScope(input); got != backupScopeAll {
			t.Fatalf("expected all scope for %q, got %q", input, got)
		}
	}
}

func TestUniqueNonEmptyStrings(t *testing.T) {
	got := uniqueNonEmptyStrings([]string{" a ", "", "b", "a", " b ", "c"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deduped strings mismatch\nwant: %#v\n got: %#v", want, got)
	}
}

func TestBackupIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	idx := &backupIndex{
		Version:   backupIndexVersion,
		CreatedAt: "2026-07-04T00:00:00Z",
		UpdatedAt: "2026-07-04T00:00:00Z",
		Messages: map[string]backupIndexMessage{
			"account:folder:1:2": {
				AccountID:    "account",
				AccountEmail: "user@example.com",
				FolderID:     "folder",
				FolderPath:   "INBOX",
				UIDValidity:  1,
				UID:          2,
				EMLPath:      "eml/user@example.com/INBOX/message.eml",
			},
		},
	}

	if err := saveBackupIndex(dir, idx); err != nil {
		t.Fatalf("save index: %v", err)
	}

	got, found, err := loadBackupIndex(dir)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	if !found {
		t.Fatal("expected saved backup index to be found")
	}
	if got.Version != backupIndexVersion {
		t.Fatalf("version mismatch: %d", got.Version)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("message count mismatch: %d", len(got.Messages))
	}
	if got.Messages["account:folder:1:2"].EMLPath != "eml/user@example.com/INBOX/message.eml" {
		t.Fatalf("message index was not preserved: %#v", got.Messages)
	}
}

func TestParseBackupTimeAcceptsGoTimeString(t *testing.T) {
	got := parseBackupTime("2025-10-09 10:59:15 +0000 UTC")
	want := time.Date(2025, 10, 9, 10, 59, 15, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("time mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestBackupIndexedFilePathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()

	if _, err := backupIndexedFilePath(dir, "../outside.eml"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, err := backupIndexedFilePath(dir, filepath.Join(dir, "message.eml")); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
	got, err := backupIndexedFilePath(dir, "eml/user@example.com/INBOX/message.eml")
	if err != nil {
		t.Fatalf("expected valid backup path: %v", err)
	}
	if !strings.HasPrefix(got, dir) {
		t.Fatalf("expected path to stay under backup directory, got %s", got)
	}
}

func TestParseBackupViewerEMLExtractsBodyAndAttachments(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: =?UTF-8?B?5rWL6K+V6YKu5Lu2?=",
		"Date: Sat, 04 Jul 2026 12:00:00 +0800",
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"纯文本正文",
		"--mixed",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>HTML 正文</p><script>alert(1)</script>",
		"--mixed",
		"Content-Type: application/pdf; name=\"report.pdf\"",
		"Content-Disposition: attachment; filename=\"report.pdf\"",
		"",
		"PDFDATA",
		"--mixed--",
		"",
	}, "\r\n")

	detail, err := parseBackupViewerEML("k", backupIndexMessage{
		AccountEmail: "backup@example.com",
		FolderPath:   "INBOX",
	}, strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse backup eml: %v", err)
	}
	if detail.Subject != "测试邮件" {
		t.Fatalf("subject mismatch: %q", detail.Subject)
	}
	if detail.BodyText != "纯文本正文" {
		t.Fatalf("plain body mismatch: %q", detail.BodyText)
	}
	if !strings.Contains(detail.BodyHTML, "HTML 正文") || strings.Contains(detail.BodyHTML, "<script") {
		t.Fatalf("html body was not sanitized: %q", detail.BodyHTML)
	}
	if len(detail.Attachments) != 1 || detail.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachment mismatch: %#v", detail.Attachments)
	}
	if detail.Attachments[0].Index != 0 {
		t.Fatalf("attachment index mismatch: %d", detail.Attachments[0].Index)
	}
	if got := strings.Join(detail.To, ", "); got != "Bob <bob@example.com>" {
		t.Fatalf("address mismatch: %q", got)
	}
}

func TestWriteBackupFileFromStreamWritesContent(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("0123456789", 1024)

	written, err := writeBackupFileFromStream(dir, "eml/user@example.com/INBOX/message.eml", func(w io.Writer) (int64, error) {
		return io.Copy(w, strings.NewReader(content))
	})
	if err != nil {
		t.Fatalf("write streamed backup file: %v", err)
	}
	if written != int64(len(content)) {
		t.Fatalf("written size mismatch: got %d want %d", written, len(content))
	}

	path := filepath.Join(dir, "eml", "user@example.com", "INBOX", "message.eml")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read streamed backup file: %v", err)
	}
	if string(got) != content {
		t.Fatal("streamed backup content mismatch")
	}
}

func TestWriteBackupFileFromStreamRemovesPartialOnError(t *testing.T) {
	dir := t.TempDir()
	boom := errors.New("stream failed")

	written, err := writeBackupFileFromStream(dir, "eml/user@example.com/INBOX/message.eml", func(w io.Writer) (int64, error) {
		n, writeErr := io.WriteString(w, "partial")
		if writeErr != nil {
			return int64(n), writeErr
		}
		return int64(n), boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected stream error, got %v", err)
	}
	if written != int64(len("partial")) {
		t.Fatalf("written size mismatch: got %d", written)
	}

	path := filepath.Join(dir, "eml", "user@example.com", "INBOX", "message.eml")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected partial backup file to be removed, stat err: %v", statErr)
	}
	if _, statErr := os.Stat(path + ".tmp"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp backup file to be removed, stat err: %v", statErr)
	}
}
