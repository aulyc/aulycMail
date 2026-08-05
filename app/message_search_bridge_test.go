package app

import (
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func TestMessageConversationSearchAndAttachmentBridges(t *testing.T) {
	a, accountFixture, _ := newContactOwnEmailsTestApp(t)
	a.folderStore = folder.NewStore(a.db)
	a.messageStore = message.NewStore(a.db)
	a.attachmentStore = message.NewAttachmentStore(a.db)
	a.ftsIndexer = message.NewFTSIndexer(a.db.DB)
	inbox := &folder.Folder{AccountID: accountFixture.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := a.folderStore.Create(inbox); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	messages := []*message.Message{
		{
			ID:             "message-1",
			AccountID:      accountFixture.ID,
			FolderID:       inbox.ID,
			UID:            1,
			MessageID:      "first@example.com",
			ThreadID:       "thread-quarterly",
			Subject:        "Quarterly Report",
			FromName:       "Alice",
			FromEmail:      "alice@example.com",
			ToList:         `[{"name":"Owner","email":"owner@example.com"}]`,
			Date:           now.Add(-time.Hour),
			Snippet:        "Quarterly performance details",
			BodyText:       "The complete quarterly performance report",
			BodyFetched:    true,
			HasAttachments: true,
			ReadReceiptTo:  "Alice <alice@example.com>",
		},
		{
			ID:          "message-2",
			AccountID:   accountFixture.ID,
			FolderID:    inbox.ID,
			UID:         2,
			MessageID:   "second@example.com",
			ThreadID:    "thread-quarterly",
			Subject:     "Re: Quarterly Report",
			FromName:    "Bob",
			FromEmail:   "bob@example.com",
			ToList:      `[{"name":"Owner","email":"owner@example.com"}]`,
			Date:        now,
			Snippet:     "Follow-up details",
			BodyText:    "Follow-up body",
			BodyFetched: true,
			IsRead:      true,
			IsStarred:   true,
		},
	}
	if err := a.messageStore.UpsertBatch(messages); err != nil {
		t.Fatal(err)
	}
	if err := a.attachmentStore.Create(&message.Attachment{
		ID:          "inline-logo",
		MessageID:   messages[0].ID,
		Filename:    "logo.png",
		ContentType: "image/png",
		Size:        4,
		ContentID:   "logo",
		IsInline:    true,
		Content:     []byte("logo"),
	}); err != nil {
		t.Fatal(err)
	}

	conversations, err := a.GetConversations(accountFixture.ID, inbox.ID, 0, 20, "newest", "")
	if err != nil || len(conversations) != 1 || conversations[0].MessageCount != 2 || conversations[0].UnreadCount != 1 || !conversations[0].HasAttachments {
		t.Fatalf("folder conversations = %#v, %v", conversations, err)
	}
	if count, err := a.GetConversationCount(accountFixture.ID, inbox.ID, ""); err != nil || count != 1 {
		t.Fatalf("folder conversation count = %d, %v", count, err)
	}
	conversation, err := a.GetConversation("thread-quarterly", inbox.ID)
	if err != nil || conversation == nil || len(conversation.Messages) != 2 {
		t.Fatalf("conversation detail = %#v, %v", conversation, err)
	}

	unified, err := a.GetUnifiedInboxConversations(0, 20, "newest", "")
	if err != nil || len(unified) != 1 || unified[0].AccountID != accountFixture.ID || unified[0].FolderID != inbox.ID {
		t.Fatalf("unified conversations = %#v, %v", unified, err)
	}
	if count, err := a.GetUnifiedInboxCount(""); err != nil || count != 1 {
		t.Fatalf("unified count = %d, %v", count, err)
	}

	searchResults, err := a.SearchConversations(accountFixture.ID, inbox.ID, "Quarterly", 0, 20, "")
	if err != nil || len(searchResults) != 1 || !strings.Contains(searchResults[0].HighlightedSubject, "<mark>") {
		t.Fatalf("folder search = %#v, %v", searchResults, err)
	}
	if count, err := a.GetSearchCount(accountFixture.ID, inbox.ID, "Quarterly", ""); err != nil || count != 1 {
		t.Fatalf("folder search count = %d, %v", count, err)
	}
	unifiedSearch, err := a.SearchUnifiedInbox("Quarterly", 0, 20, "")
	if err != nil || len(unifiedSearch) != 1 || unifiedSearch[0].AccountID != accountFixture.ID {
		t.Fatalf("unified search = %#v, %v", unifiedSearch, err)
	}
	if count, err := a.GetSearchCountUnifiedInbox("Quarterly", ""); err != nil || count != 1 {
		t.Fatalf("unified search count = %d, %v", count, err)
	}
	substringResults, err := a.SearchMailInAccount(accountFixture.ID, "formance", 20)
	if err != nil || len(substringResults) != 1 || substringResults[0].ID != messages[0].ID {
		t.Fatalf("substring account search = %#v, %v", substringResults, err)
	}
	if empty, err := (&App{}).SearchMailInAccount(accountFixture.ID, "anything", 20); err != nil || len(empty) != 0 {
		t.Fatalf("uninitialized message search = %#v, %v", empty, err)
	}
	if status, err := a.GetFTSIndexStatus("missing-folder"); err != nil || status != nil {
		t.Fatalf("missing FTS status = %#v, %v", status, err)
	}
	if a.IsFTSIndexing() {
		t.Fatal("IsFTSIndexing() = true without an active index build")
	}

	attachments, err := a.GetAttachments(messages[0].ID)
	if err != nil || len(attachments) != 1 || attachments[0].Filename != "logo.png" {
		t.Fatalf("attachments = %#v, %v", attachments, err)
	}
	inline, err := a.GetInlineAttachments(messages[0].ID)
	if err != nil || inline["logo"] != "data:image/png;base64,bG9nbw==" {
		t.Fatalf("inline attachments = %#v, %v", inline, err)
	}
	if err := a.IgnoreReadReceipt(accountFixture.ID, messages[0].ID); err != nil {
		t.Fatal(err)
	}
	updated, err := a.messageStore.Get(messages[0].ID)
	if err != nil || updated == nil || !updated.ReadReceiptHandled {
		t.Fatalf("ignored read receipt message = %#v, %v", updated, err)
	}
}
