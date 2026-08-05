package message

import (
	"testing"
	"time"
)

func TestBodyQueueHonorsSizeDateEncryptionAndBatchUpdates(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	messages := []*Message{
		{ID: "recent", AccountID: accountID, FolderID: folderID, UID: 1, Date: now, Size: 11},
		{ID: "empty", AccountID: accountID, FolderID: folderID, UID: 2, Date: now.Add(-time.Hour), Size: 22, BodyFetched: true},
		{ID: "old", AccountID: accountID, FolderID: folderID, UID: 3, Date: now.Add(-48 * time.Hour), Size: 33},
		{ID: "legacy-date", AccountID: accountID, FolderID: folderID, UID: 4, Date: time.Time{}, Size: 44},
		{ID: "encrypted", AccountID: accountID, FolderID: folderID, UID: 5, Date: now, Size: 55, BodyFetched: true},
		{ID: "failed", AccountID: accountID, FolderID: folderID, UID: 6, Date: now, Size: 66},
	}
	for _, item := range messages {
		if err := store.Create(item); err != nil {
			t.Fatalf("Create(%s) error = %v", item.ID, err)
		}
	}
	if _, err := store.db.Exec(`UPDATE messages SET smime_encrypted = 1 WHERE id = 'encrypted'`); err != nil {
		t.Fatalf("mark encrypted fixture: %v", err)
	}
	if err := store.MarkBodyFailed([]string{"failed"}); err != nil {
		t.Fatalf("MarkBodyFailed() error = %v", err)
	}

	all, err := store.GetMessagesWithoutBodyAndSize(folderID, 20, time.Time{})
	if err != nil {
		t.Fatalf("GetMessagesWithoutBodyAndSize(all) error = %v", err)
	}
	allSizes := make(map[string]int, len(all))
	for _, item := range all {
		allSizes[item.ID] = item.Size
	}
	for id, size := range map[string]int{"recent": 11, "empty": 22, "old": 33, "legacy-date": 44} {
		if allSizes[id] != size {
			t.Fatalf("allSizes[%q] = %d, want %d; all=%v", id, allSizes[id], size, all)
		}
	}
	if _, exists := allSizes["encrypted"]; exists {
		t.Fatal("encrypted empty body entered the refetch queue")
	}
	if _, exists := allSizes["failed"]; exists {
		t.Fatal("failed body entered the refetch queue")
	}

	since := now.Add(-24 * time.Hour)
	recent, err := store.GetMessagesWithoutBodyAndSize(folderID, 20, since)
	if err != nil {
		t.Fatalf("GetMessagesWithoutBodyAndSize(since) error = %v", err)
	}
	recentIDs := make(map[string]bool, len(recent))
	for _, item := range recent {
		recentIDs[item.ID] = true
	}
	if !recentIDs["recent"] || !recentIDs["empty"] || !recentIDs["legacy-date"] || recentIDs["old"] {
		t.Fatalf("recent queue IDs = %v", recentIDs)
	}
	if count, err := store.CountMessagesWithoutBody(folderID, since); err != nil || count != 3 {
		t.Fatalf("CountMessagesWithoutBody(since) = %d, %v; want 3", count, err)
	}
	if count, err := store.CountMessagesWithoutBody(folderID, time.Time{}); err != nil || count != 4 {
		t.Fatalf("CountMessagesWithoutBody(all) = %d, %v; want 4", count, err)
	}

	if err := store.UpdateBody("old", "<p>old</p>", "old", "old snippet", true); err != nil {
		t.Fatalf("UpdateBody() error = %v", err)
	}
	if err := store.UpdateBodiesBatch(nil); err != nil {
		t.Fatalf("UpdateBodiesBatch(nil) error = %v", err)
	}
	if err := store.UpdateBodiesBatch([]BodyUpdate{
		{MessageID: "recent", BodyText: "recent body", Snippet: "recent snippet"},
		{MessageID: "empty", BodyHTML: "<p>filled</p>", Snippet: "filled", HasAttachments: true},
		{MessageID: "legacy-date", BodyText: "legacy body"},
	}); err != nil {
		t.Fatalf("UpdateBodiesBatch() error = %v", err)
	}
	if count, err := store.CountMessagesWithoutBody(folderID, time.Time{}); err != nil || count != 0 {
		t.Fatalf("queue after updates = %d, %v; want empty", count, err)
	}

	var bodyText, bodyHTML, snippet string
	var fetched, hasAttachments int
	if err := store.db.QueryRow(`SELECT COALESCE(body_text, ''), COALESCE(body_html, ''), COALESCE(snippet, ''), body_fetched, has_attachments FROM messages WHERE id = 'empty'`).
		Scan(&bodyText, &bodyHTML, &snippet, &fetched, &hasAttachments); err != nil {
		t.Fatalf("query updated body: %v", err)
	}
	if bodyText != "" || bodyHTML != "<p>filled</p>" || snippet != "filled" || fetched != 1 || hasAttachments != 1 {
		t.Fatalf("updated empty body = text %q html %q snippet %q fetched %d attachments %d", bodyText, bodyHTML, snippet, fetched, hasAttachments)
	}
}

func TestBodyStoreReturnsDatabaseErrorsAfterClose(t *testing.T) {
	store, _, folderID := newBodyFailedTestStore(t)
	if err := store.db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := store.UpdateBody("missing", "", "body", "", false); err == nil {
		t.Fatal("UpdateBody() error = nil after database close")
	}
	if _, err := store.GetMessagesWithoutBodyAndSize(folderID, 10, time.Time{}); err == nil {
		t.Fatal("GetMessagesWithoutBodyAndSize() error = nil after database close")
	}
	if _, err := store.CountMessagesWithoutBody(folderID, time.Now()); err == nil {
		t.Fatal("CountMessagesWithoutBody() error = nil after database close")
	}
	if err := store.UpdateBodiesBatch([]BodyUpdate{{MessageID: "missing", BodyText: "body"}}); err == nil {
		t.Fatal("UpdateBodiesBatch() error = nil after database close")
	}
	if _, err := store.ClearBodiesForFolder(folderID); err == nil {
		t.Fatal("ClearBodiesForFolder() error = nil after database close")
	}
}
