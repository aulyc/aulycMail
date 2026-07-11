package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/activitylog"
	mailBackup "aulyc.local/aulycmail/internal/backup"
)

func TestBackupMessageKeyIncludesUIDValidity(t *testing.T) {
	first := mailBackup.MessageRow{
		AccountID:   "account-1",
		FolderID:    "folder-1",
		UIDValidity: 100,
		UID:         42,
	}
	second := first
	second.UIDValidity = 200

	if mailBackup.MessageKey(first) == mailBackup.MessageKey(second) {
		t.Fatal("expected backup keys to differ when UIDVALIDITY changes")
	}
}

func TestBackupMessageRelativePathIncludesStableMetadata(t *testing.T) {
	row := mailBackup.MessageRow{
		AccountEmail: "user@example.com",
		FolderPath:   "INBOX/Reports",
		UIDValidity:  42,
		UID:          99,
		Subject:      "Q2/Risk:Plan",
		Date:         time.Date(2026, 7, 4, 12, 13, 14, 0, time.UTC),
	}

	got := mailBackup.MessageRelativePathForRow(row)
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

func TestGroupBackupMessageRowsPreservesGroupAndRowOrder(t *testing.T) {
	rows := []mailBackup.MessageRow{
		{AccountID: "a1", FolderID: "inbox", UID: 1},
		{AccountID: "a1", FolderID: "sent", UID: 2},
		{AccountID: "a1", FolderID: "inbox", UID: 3},
		{AccountID: "a2", FolderID: "inbox", UID: 4},
	}

	groups := mailBackup.GroupMessageRows(rows)
	if len(groups) != 3 {
		t.Fatalf("group count mismatch: %d", len(groups))
	}
	if groups[0].AccountID != "a1" || groups[0].FolderID != "inbox" {
		t.Fatalf("first group mismatch: %#v", groups[0])
	}
	if got := mailBackup.RowUIDs(groups[0].Rows); !reflect.DeepEqual(got, []uint32{1, 3}) {
		t.Fatalf("first group UIDs mismatch: %#v", got)
	}
	if groups[1].AccountID != "a1" || groups[1].FolderID != "sent" {
		t.Fatalf("second group mismatch: %#v", groups[1])
	}
	if groups[2].AccountID != "a2" || groups[2].FolderID != "inbox" {
		t.Fatalf("third group mismatch: %#v", groups[2])
	}
}

func TestBackupRowsByUIDIgnoresZeroUID(t *testing.T) {
	rows := []mailBackup.MessageRow{
		{UID: 0, Subject: "missing"},
		{UID: 10, Subject: "ten"},
		{UID: 11, Subject: "eleven"},
	}

	got := mailBackup.RowsByUID(rows)
	if len(got) != 2 {
		t.Fatalf("UID map size mismatch: %d", len(got))
	}
	if got[10].Subject != "ten" || got[11].Subject != "eleven" {
		t.Fatalf("UID map contents mismatch: %#v", got)
	}
}

func TestBackupRunTrackerBlocksConcurrentStarts(t *testing.T) {
	var tracker backupRunTracker
	if !tracker.start("2026-07-08T00:00:00Z") {
		t.Fatal("expected first backup start to succeed")
	}
	if tracker.start("2026-07-08T00:00:01Z") {
		t.Fatal("expected second backup start to be rejected while running")
	}

	state := tracker.snapshot()
	if !state.Running {
		t.Fatal("expected tracker to report running")
	}
	if state.StartedAt != "2026-07-08T00:00:00Z" {
		t.Fatalf("startedAt changed unexpectedly: %q", state.StartedAt)
	}
	if state.Progress == nil || state.Progress.Phase != "running" {
		t.Fatalf("running progress missing: %#v", state.Progress)
	}
}

func TestBackupRunTrackerFinishPreservesLastProgress(t *testing.T) {
	var tracker backupRunTracker
	if !tracker.start("2026-07-08T00:00:00Z") {
		t.Fatal("expected backup start to succeed")
	}
	tracker.update(BackupProgress{Phase: "running", Current: 3, Total: 10})
	tracker.finish(BackupProgress{Phase: "done", Current: 10, Total: 10, Exported: 7, Skipped: 3})

	state := tracker.snapshot()
	if state.Running {
		t.Fatal("expected tracker to stop running after finish")
	}
	if state.Progress == nil || state.Progress.Phase != "done" {
		t.Fatalf("done progress missing: %#v", state.Progress)
	}
	if state.Progress.Exported != 7 || state.Progress.Skipped != 3 {
		t.Fatalf("finish counts were not preserved: %#v", state.Progress)
	}
}

func TestBackupDoneProgressIncludesMissingCount(t *testing.T) {
	progress := backupDoneProgress(&BackupRunResult{
		Total:    10,
		Exported: 7,
		Skipped:  2,
		Missing:  1,
	})
	if progress.Missing != 1 {
		t.Fatalf("missing count was not preserved: %#v", progress)
	}
}

func TestFormatBackupRunResultIncludesMissingOnlyWhenPresent(t *testing.T) {
	withMissing := mailBackup.FormatRunResult(mailBackup.IndexRun{Exported: 7, Skipped: 2, Missing: 1})
	if !strings.Contains(withMissing, "1 missing") {
		t.Fatalf("expected missing count in result: %s", withMissing)
	}
	withoutMissing := mailBackup.FormatRunResult(mailBackup.IndexRun{Exported: 7, Skipped: 3})
	if strings.Contains(withoutMissing, "missing") {
		t.Fatalf("did not expect missing count in result: %s", withoutMissing)
	}
}

func TestBackupIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	hasAttachments := true
	idx := &mailBackup.Index{
		Version:   mailBackup.IndexVersion,
		CreatedAt: "2026-07-04T00:00:00Z",
		UpdatedAt: "2026-07-04T00:00:00Z",
		Messages: map[string]mailBackup.IndexMessage{
			"account:folder:1:2": {
				AccountID:      "account",
				AccountEmail:   "user@example.com",
				FolderID:       "folder",
				FolderPath:     "INBOX",
				UIDValidity:    1,
				UID:            2,
				EMLPath:        "eml/user@example.com/INBOX/message.eml",
				HasAttachments: &hasAttachments,
			},
		},
	}

	if err := mailBackup.SaveIndex(dir, idx); err != nil {
		t.Fatalf("save index: %v", err)
	}

	got, found, err := mailBackup.LoadIndex(dir)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	if !found {
		t.Fatal("expected saved backup index to be found")
	}
	if got.Version != mailBackup.IndexVersion {
		t.Fatalf("version mismatch: %d", got.Version)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("message count mismatch: %d", len(got.Messages))
	}
	if got.Messages["account:folder:1:2"].EMLPath != "eml/user@example.com/INBOX/message.eml" {
		t.Fatalf("message index was not preserved: %#v", got.Messages)
	}
	if got.Messages["account:folder:1:2"].HasAttachments == nil || !*got.Messages["account:folder:1:2"].HasAttachments {
		t.Fatalf("attachment flag was not preserved: %#v", got.Messages["account:folder:1:2"].HasAttachments)
	}
}

func TestParseBackupTimeAcceptsGoTimeString(t *testing.T) {
	got := mailBackup.ParseMessageTime("2025-10-09 10:59:15 +0000 UTC")
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

	detail, err := parseBackupViewerEML("k", mailBackup.IndexMessage{
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

func TestBackupViewerMessagesIncludesAttachmentCount(t *testing.T) {
	hasAttachments := true
	idx := &mailBackup.Index{Messages: map[string]mailBackup.IndexMessage{
		"k": {
			AccountEmail:   "user@example.com",
			FolderPath:     "INBOX",
			Subject:        "With attachment",
			Date:           "Sat, 04 Jul 2026 12:00:00 +0800",
			EMLPath:        "missing/message.eml",
			Size:           1024,
			HasAttachments: &hasAttachments,
		},
	}}

	messages := backupViewerMessages(idx, "", "", 0)
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	if messages[0].AttachmentCount != 1 {
		t.Fatalf("attachment count mismatch: got %d", messages[0].AttachmentCount)
	}
}

func TestWriteBackupFileFromReaderWritesContent(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("0123456789", 1024)

	written, err := mailBackup.WriteFileFromReader(dir, "eml/user@example.com/INBOX/message.eml", strings.NewReader(content))
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

func TestWriteBackupFileFromReaderRemovesPartialOnError(t *testing.T) {
	dir := t.TempDir()
	boom := errors.New("stream failed")

	written, err := mailBackup.WriteFileFromReader(
		dir,
		"eml/user@example.com/INBOX/message.eml",
		io.MultiReader(strings.NewReader("partial"), errorReader{err: boom}),
	)
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

func TestWriteBackupFileFromReaderRejectsEmptyContent(t *testing.T) {
	dir := t.TempDir()

	written, err := mailBackup.WriteFileFromReader(dir, "eml/user@example.com/INBOX/message.eml", strings.NewReader(""))
	if err == nil {
		t.Fatal("expected empty reader to fail")
	}
	if written != 0 {
		t.Fatalf("written size mismatch: got %d", written)
	}

	path := filepath.Join(dir, "eml", "user@example.com", "INBOX", "message.eml")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected empty backup file to be removed, stat err: %v", statErr)
	}
}

type errorReader struct {
	err error
}

func TestBackupActivityStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *BackupRunResult
		err    error
		want   string
	}{
		{name: "success", result: &BackupRunResult{Exported: 2, Skipped: 3}, want: activitylog.StatusSuccess},
		{name: "partial missing", result: &BackupRunResult{Skipped: 3, Missing: 1}, want: activitylog.StatusPartial},
		{name: "partial failed", result: &BackupRunResult{Exported: 2, Failed: 1}, want: activitylog.StatusPartial},
		{name: "failed no completed", result: &BackupRunResult{Missing: 1, Failed: 1}, want: activitylog.StatusFailed},
		{name: "fatal", err: errors.New("boom"), want: activitylog.StatusFailed},
		{name: "cancelled", err: context.Canceled, want: activitylog.StatusCancelled},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := backupActivityStatus(tc.result, tc.err); got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
