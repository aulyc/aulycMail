package app

import (
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
