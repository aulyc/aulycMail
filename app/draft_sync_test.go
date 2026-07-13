package app

import (
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/draft"
	"aulyc.local/aulycmail/internal/message"
)

func TestDeleteReplacedDraftMessageRemovesOnlyOldUID(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const (
		accountID = "account-1"
		folderID  = "drafts-1"
		oldUID    = uint32(41)
		newUID    = uint32(42)
	)
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type)
		 VALUES (?, ?, ?, ?, ?)`,
		folderID, accountID, "Drafts", "Drafts", "drafts",
	); err != nil {
		t.Fatalf("seed drafts folder: %v", err)
	}

	messageStore := message.NewStore(db)
	now := time.Now().UTC()
	for _, msg := range []*message.Message{
		{ID: "old-draft", AccountID: accountID, FolderID: folderID, UID: oldUID, Date: now},
		{ID: "new-draft", AccountID: accountID, FolderID: folderID, UID: newUID, Date: now},
	} {
		if err := messageStore.Create(msg); err != nil {
			t.Fatalf("seed message %s: %v", msg.ID, err)
		}
	}

	ops := &draftOps{messageStore: messageStore}
	localDraft := &draft.Draft{FolderID: folderID, IMAPUID: oldUID}
	if err := ops.deleteReplacedDraftMessage(localDraft, newUID); err != nil {
		t.Fatalf("deleteReplacedDraftMessage: %v", err)
	}

	uids, err := messageStore.GetAllUIDs(folderID)
	if err != nil {
		t.Fatalf("GetAllUIDs: %v", err)
	}
	if len(uids) != 1 || uids[0] != newUID {
		t.Fatalf("remaining UIDs = %v, want [%d]", uids, newUID)
	}
}

func TestDeleteReplacedDraftMessageKeepsCurrentUID(t *testing.T) {
	ops := &draftOps{}
	for _, tc := range []struct {
		name   string
		draft  *draft.Draft
		newUID uint32
	}{
		{name: "nil draft", draft: nil, newUID: 42},
		{name: "unsynced draft", draft: &draft.Draft{FolderID: "drafts-1"}, newUID: 42},
		{name: "missing folder", draft: &draft.Draft{IMAPUID: 41}, newUID: 42},
		{name: "same UID", draft: &draft.Draft{FolderID: "drafts-1", IMAPUID: 42}, newUID: 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ops.deleteReplacedDraftMessage(tc.draft, tc.newUID); err != nil {
				t.Fatalf("deleteReplacedDraftMessage: %v", err)
			}
		})
	}
}

func TestDeleteReplacedDraftMessageReturnsStoreError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	messageStore := message.NewStore(db)
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ops := &draftOps{messageStore: messageStore}
	localDraft := &draft.Draft{FolderID: "drafts-1", IMAPUID: 41}
	if err := ops.deleteReplacedDraftMessage(localDraft, 42); err == nil {
		t.Fatal("deleteReplacedDraftMessage error = nil, want closed-store error")
	}
}

func TestResolveDraftReferenceAcceptsDraftAndMessageIDs(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const (
		accountID = "account-1"
		folderID  = "drafts-1"
		messageID = "draft-message-1"
		draftID   = "local-draft-1"
		draftUID  = uint32(42)
	)
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type)
		 VALUES (?, ?, ?, ?, ?)`,
		folderID, accountID, "Drafts", "Drafts", "drafts",
	); err != nil {
		t.Fatalf("seed drafts folder: %v", err)
	}

	messageStore := message.NewStore(db)
	if err := messageStore.Create(&message.Message{
		ID: messageID, AccountID: accountID, FolderID: folderID, UID: draftUID, Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed draft message: %v", err)
	}
	draftStore := draft.NewStore(db)
	if err := draftStore.Create(&draft.Draft{
		ID: draftID, AccountID: accountID, FolderID: folderID, IMAPUID: draftUID, SyncStatus: draft.SyncStatusSynced,
	}); err != nil {
		t.Fatalf("seed local draft: %v", err)
	}

	ops := &draftOps{messageStore: messageStore, draftStore: draftStore}
	for _, ref := range []string{draftID, messageID} {
		resolved, err := ops.resolveDraftReference(ref)
		if err != nil {
			t.Fatalf("resolveDraftReference(%q): %v", ref, err)
		}
		if resolved == nil || resolved.ID != draftID {
			t.Fatalf("resolveDraftReference(%q) = %#v, want draft %q", ref, resolved, draftID)
		}
	}

	resolved, err := ops.resolveDraftReference("missing")
	if err != nil {
		t.Fatalf("resolveDraftReference(missing): %v", err)
	}
	if resolved != nil {
		t.Fatalf("resolveDraftReference(missing) = %#v, want nil", resolved)
	}
}

func TestDraftHasRemoteCopyIncludesPendingReplacement(t *testing.T) {
	tests := []struct {
		name  string
		draft *draft.Draft
		want  bool
	}{
		{name: "synced UID", draft: &draft.Draft{SyncStatus: draft.SyncStatusSynced, IMAPUID: 42, FolderID: "drafts-1"}, want: true},
		{name: "pending replacement keeps old UID", draft: &draft.Draft{SyncStatus: draft.SyncStatusPending, IMAPUID: 42, FolderID: "drafts-1"}, want: true},
		{name: "missing UID", draft: &draft.Draft{SyncStatus: draft.SyncStatusSynced, FolderID: "drafts-1"}, want: false},
		{name: "missing folder", draft: &draft.Draft{SyncStatus: draft.SyncStatusSynced, IMAPUID: 42}, want: false},
		{name: "nil draft", draft: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := draftHasRemoteCopy(tt.draft); got != tt.want {
				t.Fatalf("draftHasRemoteCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}
