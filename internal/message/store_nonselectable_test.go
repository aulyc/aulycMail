package message

import (
	"testing"
	"time"
)

func TestNoSelectFoldersAreExcludedFromGlobalMailSurfaces(t *testing.T) {
	store, accountID, selectableFolderID := newBodyFailedTestStore(t)

	if _, err := store.db.Exec(
		`UPDATE folders SET folder_type = 'folder', unread_count = 2 WHERE id = ?`,
		selectableFolderID,
	); err != nil {
		t.Fatalf("update selectable folder: %v", err)
	}
	const directoryID = "folder-directory"
	if _, err := store.db.Exec(
		`INSERT INTO folders
		 (id, account_id, name, path, folder_type, unread_count, selectable)
		 VALUES (?, ?, 'Other', 'Other', 'folder', 5, 0)`,
		directoryID, accountID,
	); err != nil {
		t.Fatalf("seed no-select folder: %v", err)
	}

	now := time.Now().UTC()
	for _, msg := range []*Message{
		{
			ID: "visible-message", AccountID: accountID, FolderID: selectableFolderID,
			UID: 1, Subject: "recovery needle visible", FromEmail: "sender@example.com", Date: now,
		},
		{
			ID: "stale-directory-message", AccountID: accountID, FolderID: directoryID,
			UID: 1, Subject: "recovery needle stale", FromEmail: "sender@example.com", Date: now.Add(time.Minute),
		},
	} {
		if err := store.Create(msg); err != nil {
			t.Fatalf("Create(%s): %v", msg.ID, err)
		}
	}

	results, err := store.SearchMessagesInAccount(accountID, "recovery needle", 20)
	if err != nil {
		t.Fatalf("SearchMessagesInAccount: %v", err)
	}
	if len(results) != 1 || results[0].ID != "visible-message" {
		t.Fatalf("search results = %#v, want only selectable-folder message", results)
	}

	badge, err := store.GetBadgeUnreadCount()
	if err != nil {
		t.Fatalf("GetBadgeUnreadCount: %v", err)
	}
	if badge != 2 {
		t.Fatalf("badge count = %d, want selectable-folder count 2", badge)
	}
}
