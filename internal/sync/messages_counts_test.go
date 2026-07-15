package sync

import (
	"context"
	"path/filepath"
	"testing"

	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
)

func TestSetFolderCountsFromLocalOverridesInconsistentServerCounts(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	const accountID = "account-1"
	const folderID = "folder-1"
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, "Test", "test@example.com", "imap.example.com", "smtp.example.com", "test@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, total_count, unread_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		folderID, accountID, "Other", "Other", "folder", 0, 1,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages (id, account_id, folder_id, uid, is_read)
		 VALUES
			('message-1', ?, ?, 1, 0),
			('message-2', ?, ?, 2, 1),
			('message-3', ?, ?, 3, 0)`,
		accountID, folderID,
		accountID, folderID,
		accountID, folderID,
	); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	folderStore := folder.NewStore(db)
	messageStore := message.NewStore(db)
	engine := NewEngine(nil, nil, folderStore, messageStore, nil, nil)
	f := &folder.Folder{ID: folderID, Path: "Other", TotalCount: 0, UnreadCount: 1}

	engine.setFolderCountsFromLocal(f, 0, 1)

	if f.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want local count 3", f.TotalCount)
	}
	if f.UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want local unread count 2", f.UnreadCount)
	}

	// A folder-list STATUS refresh can arrive before message sync and report
	// zero for a mailbox whose local records are intentionally retained.
	f.TotalCount = 0
	f.UnreadCount = 1
	engine.preserveLocalCountsForEmptyServerFolder(f)
	if f.TotalCount != 3 {
		t.Fatalf("preserved TotalCount = %d, want local count 3", f.TotalCount)
	}
	if f.UnreadCount != 2 {
		t.Fatalf("preserved UnreadCount = %d, want local unread count 2", f.UnreadCount)
	}

	// A non-empty server count is not an empty-status anomaly and must remain
	// available until message sync performs its normal reconciliation.
	f.TotalCount = 9
	f.UnreadCount = 4
	engine.preserveLocalCountsForEmptyServerFolder(f)
	if f.TotalCount != 9 || f.UnreadCount != 4 {
		t.Fatalf("non-empty server counts changed to total=%d unread=%d", f.TotalCount, f.UnreadCount)
	}
}

func TestNoSelectFolderCountsStayZero(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	f := &folder.Folder{Path: "Other", NoSelect: true, TotalCount: 1318, UnreadCount: 1}

	engine.setFolderCountsFromLocal(f, 1318, 1)
	if f.TotalCount != 0 || f.UnreadCount != 0 {
		t.Fatalf("set counts = total %d unread %d, want zero", f.TotalCount, f.UnreadCount)
	}

	f.TotalCount = 1318
	f.UnreadCount = 1
	engine.preserveLocalCountsForEmptyServerFolder(f)
	if f.TotalCount != 0 || f.UnreadCount != 0 {
		t.Fatalf("preserved counts = total %d unread %d, want zero", f.TotalCount, f.UnreadCount)
	}
}

func TestFetchFolderStatusSkipsNoSelectMailbox(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	mailbox := &imapPkg.Mailbox{Name: "Other", NoSelect: true}

	results := engine.fetchFolderStatusParallel(context.Background(), "account-1", []*imapPkg.Mailbox{mailbox})
	if len(results) != 1 || results[0].mailbox != mailbox {
		t.Fatalf("results = %#v, want skipped mailbox result", results)
	}
	if results[0].status != nil || results[0].err != nil {
		t.Fatalf("no-select STATUS result = status %#v err %v, want both nil", results[0].status, results[0].err)
	}
}
