package message

import (
	"database/sql"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	draftPkg "aulyc.local/aulycmail/internal/draft"
)

func sortedMessageIDs(ids []string) []string {
	result := append([]string(nil), ids...)
	sort.Strings(result)
	return result
}

func TestMessageListingPaginationAndUnreadCounts(t *testing.T) {
	store, accountID, inboxID := newBodyFailedTestStore(t)
	if _, err := store.db.Exec(`
		INSERT INTO folders (id, account_id, name, path, folder_type, unread_count, selectable)
		VALUES
			('custom-folder', ?, 'Projects', 'Projects', 'folder', 3, 1),
			('trash-folder', ?, 'Trash', 'Trash', 'trash', 100, 1),
			('hidden-inbox', ?, 'Hidden', 'Hidden', 'inbox', 200, 0)
	`, accountID, accountID, accountID); err != nil {
		t.Fatalf("seed count folders: %v", err)
	}
	if _, err := store.db.Exec("UPDATE folders SET unread_count = 5 WHERE id = ?", inboxID); err != nil {
		t.Fatalf("update inbox count: %v", err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO accounts (id, name, email, imap_host, smtp_host, username, enabled)
		VALUES ('disabled-account', 'Disabled', 'disabled@example.com', 'imap.example.com', 'smtp.example.com', 'disabled@example.com', 0);
		INSERT INTO folders (id, account_id, name, path, folder_type, unread_count, selectable)
		VALUES ('disabled-inbox', 'disabled-account', 'INBOX', 'INBOX', 'inbox', 50, 1)
	`); err != nil {
		t.Fatalf("seed disabled account: %v", err)
	}

	now := time.Now().UTC()
	if err := store.UpsertBatch([]*Message{
		{ID: "oldest", AccountID: accountID, FolderID: inboxID, UID: 1, Subject: "Oldest", Date: now.Add(-2 * time.Hour), IsRead: false},
		{ID: "middle", AccountID: accountID, FolderID: inboxID, UID: 2, Subject: "Middle", Date: now.Add(-time.Hour), IsRead: true},
		{ID: "newest", AccountID: accountID, FolderID: inboxID, UID: 3, Subject: "Newest", Date: now, IsRead: false, IsStarred: true, HasAttachments: true},
	}); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	page, err := store.ListByFolder(inboxID, 0, 2)
	if err != nil {
		t.Fatalf("ListByFolder first page: %v", err)
	}
	if len(page) != 2 || page[0].ID != "newest" || page[1].ID != "middle" || !page[0].HasAttachments {
		t.Fatalf("first page = %#v", page)
	}
	secondPage, err := store.ListByFolder(inboxID, 2, 2)
	if err != nil || len(secondPage) != 1 || secondPage[0].ID != "oldest" {
		t.Fatalf("second page = (%#v, %v)", secondPage, err)
	}

	if got, err := store.CountUnreadByFolder(inboxID); err != nil || got != 2 {
		t.Fatalf("CountUnreadByFolder = (%d, %v), want 2", got, err)
	}
	unread, err := store.GetUnreadMessageIDsByFolder(inboxID)
	if err != nil || !reflect.DeepEqual(sortedMessageIDs(unread), []string{"newest", "oldest"}) {
		t.Fatalf("unread IDs = (%v, %v)", unread, err)
	}
	read, err := store.GetReadMessageIDsByFolder(inboxID)
	if err != nil || !reflect.DeepEqual(read, []string{"middle"}) {
		t.Fatalf("read IDs = (%v, %v)", read, err)
	}
	all, err := store.GetAllIDsByFolder(inboxID)
	if err != nil || !reflect.DeepEqual(sortedMessageIDs(all), []string{"middle", "newest", "oldest"}) {
		t.Fatalf("all IDs = (%v, %v)", all, err)
	}
	if got, err := store.GetUnifiedInboxUnreadCount(); err != nil || got != 5 {
		t.Fatalf("GetUnifiedInboxUnreadCount = (%d, %v), want 5", got, err)
	}
	if got, err := store.GetBadgeUnreadCount(); err != nil || got != 8 {
		t.Fatalf("GetBadgeUnreadCount = (%d, %v), want 8", got, err)
	}
}

func TestMessageLifecycleQueriesMovesAndDeletes(t *testing.T) {
	store, accountID, inboxID := newBodyFailedTestStore(t)
	const trashID = "trash-folder"
	if _, err := store.db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES (?, ?, ?, ?, ?)`,
		trashID, accountID, "Trash", "Trash", "trash",
	); err != nil {
		t.Fatalf("seed trash: %v", err)
	}
	now := time.Now().UTC()
	if err := store.UpsertBatch([]*Message{
		{ID: "old-inbox", AccountID: accountID, FolderID: inboxID, UID: 1, MessageID: "shared@example.com", References: `["<root@example.com>"]`, Date: now.Add(-90 * 24 * time.Hour)},
		{ID: "new-inbox", AccountID: accountID, FolderID: inboxID, UID: 3, MessageID: "new@example.com", Date: now},
		{ID: "local-inbox", AccountID: accountID, FolderID: inboxID, UID: 0, MessageID: "local@example.com", Date: now},
		{ID: "trash-copy", AccountID: accountID, FolderID: trashID, UID: 2, MessageID: "shared@example.com", Date: now},
	}); err != nil {
		t.Fatalf("seed lifecycle messages: %v", err)
	}

	if got, err := store.ExistsInFolder("shared@example.com", "trash", accountID); err != nil || !got {
		t.Fatalf("ExistsInFolder = (%v, %v), want true", got, err)
	}
	if got, err := store.ExistsInFolder("missing@example.com", "trash", accountID); err != nil || got {
		t.Fatalf("missing ExistsInFolder = (%v, %v), want false", got, err)
	}
	if got, err := store.HasCopiesInOtherFolders("shared@example.com", inboxID, accountID); err != nil || !got {
		t.Fatalf("HasCopiesInOtherFolders = (%v, %v), want true", got, err)
	}
	uids, err := store.GetAllUIDs(inboxID)
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	if err != nil || !reflect.DeepEqual(uids, []uint32{1, 3}) {
		t.Fatalf("GetAllUIDs = (%v, %v)", uids, err)
	}
	if highest, err := store.GetHighestUID(inboxID); err != nil || highest != 3 {
		t.Fatalf("GetHighestUID = (%d, %v)", highest, err)
	}
	if count, err := store.CountByFolder(inboxID); err != nil || count != 3 {
		t.Fatalf("CountByFolder = (%d, %v)", count, err)
	}
	if uid, folderID, err := store.GetMessageUIDAndFolder("new-inbox"); err != nil || uid != 3 || folderID != inboxID {
		t.Fatalf("GetMessageUIDAndFolder = (%d, %q, %v)", uid, folderID, err)
	}
	if _, _, err := store.GetMessageUIDAndFolder("missing"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("missing GetMessageUIDAndFolder error = %v", err)
	}
	if empty, err := store.GetMessageUIDsAndFolder(nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty GetMessageUIDsAndFolder = (%v, %v)", empty, err)
	}
	ids, err := store.GetIDsByMessageIDs(accountID, inboxID, []string{"shared@example.com", "local@example.com"})
	if err != nil || !reflect.DeepEqual(ids, []string{"old-inbox"}) {
		t.Fatalf("GetIDsByMessageIDs = (%v, %v)", ids, err)
	}
	if ids, err := store.GetIDsByMessageIDs(accountID, inboxID, nil); err != nil || ids != nil {
		t.Fatalf("empty GetIDsByMessageIDs = (%v, %v)", ids, err)
	}
	messages, err := store.GetByIDs([]string{"old-inbox", "new-inbox"})
	if err != nil || len(messages) != 2 {
		t.Fatalf("GetByIDs = (%#v, %v)", messages, err)
	}
	foundReference := false
	for _, msg := range messages {
		if msg.ID == "old-inbox" && msg.References == `["<root@example.com>"]` {
			foundReference = true
		}
	}
	if !foundReference {
		t.Fatalf("GetByIDs lost references: %#v", messages)
	}
	if messages, err := store.GetByIDs(nil); err != nil || messages != nil {
		t.Fatalf("empty GetByIDs = (%#v, %v)", messages, err)
	}

	deleted, err := store.DeleteOlderThan(accountID, now.Add(-30*24*time.Hour))
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteOlderThan = (%d, %v), want 1", deleted, err)
	}
	if err := store.MoveMessages([]string{"new-inbox", "new-inbox"}, trashID); err != nil {
		t.Fatalf("MoveMessages: %v", err)
	}
	if err := store.MoveMessages(nil, trashID); err != nil {
		t.Fatalf("empty MoveMessages: %v", err)
	}
	var movedFolder string
	var movedUID int64
	if err := store.db.QueryRow("SELECT folder_id, uid FROM messages WHERE id = 'new-inbox'").Scan(&movedFolder, &movedUID); err != nil {
		t.Fatalf("read moved message: %v", err)
	}
	if movedFolder != trashID || movedUID >= 0 {
		t.Fatalf("moved message = folder:%q uid:%d", movedFolder, movedUID)
	}
	if err := store.DeleteTempUIDs(trashID); err != nil {
		t.Fatalf("DeleteTempUIDs: %v", err)
	}
	if got, err := store.Get("new-inbox"); err != nil || got != nil {
		t.Fatalf("temp message after delete = (%#v, %v)", got, err)
	}

	if err := store.DeleteBatch(nil); err != nil {
		t.Fatalf("empty DeleteBatch: %v", err)
	}
	if err := store.DeleteBatch([]string{"local-inbox", "trash-copy"}); err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	if count, err := store.CountByFolder(trashID); err != nil || count != 0 {
		t.Fatalf("trash count after batch delete = (%d, %v)", count, err)
	}
}

func TestMessageLifecycleDeleteVariantsAndAccountSpan(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	if _, err := store.db.Exec(`
		INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		VALUES ('second-account', 'Second', 'second@example.com', 'imap.example.com', 'smtp.example.com', 'second@example.com');
		INSERT INTO folders (id, account_id, name, path, folder_type)
		VALUES ('second-inbox', 'second-account', 'INBOX', 'INBOX', 'inbox')
	`); err != nil {
		t.Fatalf("seed second account: %v", err)
	}
	now := time.Now().UTC()
	if err := store.UpsertBatch([]*Message{
		{ID: "delete-id", AccountID: accountID, FolderID: folderID, UID: 10, Date: now},
		{ID: "delete-uid", AccountID: accountID, FolderID: folderID, UID: 11, Date: now},
		{ID: "delete-folder", AccountID: accountID, FolderID: folderID, UID: 12, Date: now},
		{ID: "second-message", AccountID: "second-account", FolderID: "second-inbox", UID: 13, Date: now},
	}); err != nil {
		t.Fatalf("seed delete messages: %v", err)
	}
	if spans, err := store.SpansMultipleAccounts([]string{"delete-id"}); err != nil || spans {
		t.Fatalf("single SpansMultipleAccounts = (%v, %v)", spans, err)
	}
	if spans, err := store.SpansMultipleAccounts([]string{"delete-id", "delete-uid"}); err != nil || spans {
		t.Fatalf("same-account SpansMultipleAccounts = (%v, %v)", spans, err)
	}
	if spans, err := store.SpansMultipleAccounts([]string{"delete-id", "second-message"}); err != nil || !spans {
		t.Fatalf("cross-account SpansMultipleAccounts = (%v, %v)", spans, err)
	}
	if err := store.Delete("delete-id"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.DeleteByUID(folderID, 11); err != nil {
		t.Fatalf("DeleteByUID: %v", err)
	}
	if err := store.DeleteByFolder(folderID); err != nil {
		t.Fatalf("DeleteByFolder: %v", err)
	}
	if count, err := store.CountByFolder(folderID); err != nil || count != 0 {
		t.Fatalf("folder count after deletes = (%d, %v)", count, err)
	}
}

func TestComposeStatusTransitionsPreserveSentState(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()
	for index, id := range []string{"source-sent", "source-draft"} {
		if err := store.Create(&Message{ID: id, AccountID: accountID, FolderID: folderID, UID: uint32(20 + index), Date: now}); err != nil {
			t.Fatalf("Create source %s: %v", id, err)
		}
	}
	draftStore := draftPkg.NewStore(store.db)
	for _, draft := range []*draftPkg.Draft{
		{ID: "draft-1", AccountID: accountID, SourceMessageID: "source-sent"},
		{ID: "draft-after-send", AccountID: accountID, SourceMessageID: "source-sent"},
		{ID: "draft-2", AccountID: accountID, SourceMessageID: "source-draft"},
	} {
		if err := draftStore.Create(draft); err != nil {
			t.Fatalf("Create draft %s: %v", draft.ID, err)
		}
	}
	if err := store.MarkComposeDraft("", ComposeActionReply, "draft"); err != nil {
		t.Fatalf("invalid MarkComposeDraft: %v", err)
	}
	if err := store.MarkComposeSent("source-sent", "unsupported"); err != nil {
		t.Fatalf("invalid MarkComposeSent: %v", err)
	}

	if err := store.MarkComposeDraft("source-sent", ComposeActionReply, "draft-1"); err != nil {
		t.Fatalf("MarkComposeDraft: %v", err)
	}
	assertComposeStatus(t, store, "source-sent", ComposeActionReply, ComposeStatusDraft, sql.NullString{String: "draft-1", Valid: true})
	if err := store.MarkComposeSent("source-sent", ComposeActionReplyAll); err != nil {
		t.Fatalf("MarkComposeSent: %v", err)
	}
	assertComposeStatus(t, store, "source-sent", ComposeActionReplyAll, ComposeStatusSent, sql.NullString{})
	if err := store.MarkComposeDraft("source-sent", ComposeActionForward, "draft-after-send"); err != nil {
		t.Fatalf("draft after send: %v", err)
	}
	assertComposeStatus(t, store, "source-sent", ComposeActionReplyAll, ComposeStatusSent, sql.NullString{})
	if err := store.ClearComposeDraft("source-sent", "draft-after-send"); err != nil {
		t.Fatalf("ClearComposeDraft sent: %v", err)
	}
	assertComposeStatus(t, store, "source-sent", ComposeActionReplyAll, ComposeStatusSent, sql.NullString{})

	if err := store.MarkComposeDraft("source-draft", ComposeActionForward, "draft-2"); err != nil {
		t.Fatalf("second MarkComposeDraft: %v", err)
	}
	if err := store.ClearComposeDraft("source-draft", "wrong-draft"); err != nil {
		t.Fatalf("ClearComposeDraft mismatch: %v", err)
	}
	assertComposeStatus(t, store, "source-draft", ComposeActionForward, ComposeStatusDraft, sql.NullString{String: "draft-2", Valid: true})
	if err := store.ClearComposeDraft("source-draft", "draft-2"); err != nil {
		t.Fatalf("ClearComposeDraft: %v", err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM message_compose_status WHERE source_message_id = ?", "source-draft").Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft status after clear = (%d, %v)", count, err)
	}

	if err := store.MarkReadReceiptHandled("source-sent"); err != nil {
		t.Fatalf("MarkReadReceiptHandled: %v", err)
	}
	updated, err := store.Get("source-sent")
	if err != nil || updated == nil || !updated.ReadReceiptHandled {
		t.Fatalf("read receipt state = (%#v, %v)", updated, err)
	}
}

func assertComposeStatus(t *testing.T, store *Store, sourceID, wantAction, wantStatus string, wantDraft sql.NullString) {
	t.Helper()
	var action, status string
	var draft sql.NullString
	if err := store.db.QueryRow(`
		SELECT action_type, status, draft_id FROM message_compose_status WHERE source_message_id = ?
	`, sourceID).Scan(&action, &status, &draft); err != nil {
		t.Fatalf("read compose status %s: %v", sourceID, err)
	}
	if action != wantAction || status != wantStatus || draft != wantDraft {
		t.Fatalf("compose status %s = (%q, %q, %#v), want (%q, %q, %#v)", sourceID, action, status, draft, wantAction, wantStatus, wantDraft)
	}
}
