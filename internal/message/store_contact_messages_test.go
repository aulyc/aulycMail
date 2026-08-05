package message

import (
	"testing"
	"time"
)

func TestContactMessageQueriesMatchParticipantsAndFolderSubstrings(t *testing.T) {
	store, accountID, inboxID := newBodyFailedTestStore(t)
	if _, err := store.db.Exec(`
		INSERT INTO folders (id, account_id, name, path, folder_type, selectable)
		VALUES
			('drafts', ?, 'Drafts', 'Drafts', 'drafts', 1),
			('hidden', ?, 'Other', 'Other', 'folder', 0)
	`, accountID, accountID); err != nil {
		t.Fatalf("seed folders: %v", err)
	}
	now := time.Now().UTC()
	for _, item := range []*Message{
		{
			ID: "incoming", AccountID: accountID, FolderID: inboxID, UID: 1,
			ThreadID: "thread-incoming", Subject: "Quarterly report", FromName: "Target Person", FromEmail: "target@example.com",
			ToList: `[{"email":"test@example.com"}]`, Date: now, Snippet: "Revenue grew", IsRead: false,
		},
		{
			ID: "outgoing", AccountID: accountID, FolderID: inboxID, UID: 2,
			Subject: "Follow up", FromName: "Owner", FromEmail: "test@example.com",
			ToList: `[{"name":"Target Person","email":"TARGET@example.com"}]`, Date: now.Add(-time.Hour), Snippet: "Next quarter", IsRead: true,
		},
		{
			ID: "cc-match", AccountID: accountID, FolderID: inboxID, UID: 3,
			Subject: "Status", FromName: "Colleague", FromEmail: "colleague@example.com",
			CcList: `[{"address":"target@example.com"}]`, Date: now.Add(-2 * time.Hour), Snippet: "project alpha",
		},
		{ID: "draft-match", AccountID: accountID, FolderID: "drafts", UID: 4, Subject: "Draft target", FromEmail: "target@example.com", Date: now.Add(time.Hour)},
		{ID: "hidden-match", AccountID: accountID, FolderID: "hidden", UID: 5, Subject: "Hidden target", FromEmail: "target@example.com", Date: now.Add(2 * time.Hour)},
		{ID: "unrelated", AccountID: accountID, FolderID: inboxID, UID: 6, Subject: "Other", FromEmail: "other@example.com", Date: now.Add(-3 * time.Hour)},
	} {
		if err := store.Create(item); err != nil {
			t.Fatalf("Create(%s) error = %v", item.ID, err)
		}
	}

	if empty, err := store.ListByParticipant(" ", 10); err != nil || len(empty) != 0 {
		t.Fatalf("ListByParticipant(blank) = %#v, %v", empty, err)
	}
	related, err := store.ListByParticipant(" TARGET@example.com ", 0)
	if err != nil {
		t.Fatalf("ListByParticipant() error = %v", err)
	}
	if len(related) != 3 || related[0].ID != "incoming" || related[1].ID != "outgoing" || related[2].ID != "cc-match" {
		t.Fatalf("related messages = %#v", related)
	}
	if !related[0].Incoming || related[1].Incoming || related[0].ThreadID != "thread-incoming" || related[1].ThreadID != "outgoing" {
		t.Fatalf("related directions/thread fallbacks = %#v", related)
	}
	if related[0].AccountName != "Test" || related[0].AccountEmail != "test@example.com" || related[0].Date.IsZero() {
		t.Fatalf("related account/date metadata = %#v", related[0])
	}

	for _, test := range []struct {
		name  string
		query string
		want  string
	}{
		{name: "subject suffix", query: "report", want: "incoming"},
		{name: "sender name", query: "target person", want: "incoming"},
		{name: "recipient JSON", query: "TARGET@EXAMPLE", want: "outgoing"},
		{name: "snippet", query: "alpha", want: "cc-match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			results, err := store.SearchMessagesInFolder(inboxID, test.query, 0)
			if err != nil {
				t.Fatalf("SearchMessagesInFolder() error = %v", err)
			}
			found := false
			for _, result := range results {
				if result.ID == test.want {
					found = true
					if result.AccountName != "Test" || result.AccountID != accountID || result.Date.IsZero() {
						t.Fatalf("result metadata = %#v", result)
					}
				}
			}
			if !found {
				t.Fatalf("query %q results = %#v, want %s", test.query, results, test.want)
			}
		})
	}
	if empty, err := store.SearchMessagesInFolder("", "query", 10); err != nil || len(empty) != 0 {
		t.Fatalf("empty folder search = %#v, %v", empty, err)
	}
	if empty, err := store.SearchMessagesInFolder(inboxID, " ", 10); err != nil || len(empty) != 0 {
		t.Fatalf("empty query search = %#v, %v", empty, err)
	}

	limited, err := store.SearchMessagesInFolder(inboxID, "quarter", 1)
	if err != nil || len(limited) != 1 || limited[0].ID != "incoming" {
		t.Fatalf("limited search = %#v, %v", limited, err)
	}
}
