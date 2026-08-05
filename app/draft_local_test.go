package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/draft"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/smtp"
)

const (
	draftTestAccountID = "draft-account"
	draftTestFolderID  = "drafts-folder"
)

func newDraftLocalFixture(t *testing.T) (*database.DB, *draftOps) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		draftTestAccountID, "Draft Test", "draft@example.com", "imap.example.com", "smtp.example.com", "draft@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		draftTestFolderID, draftTestAccountID, "Drafts", "Drafts", "drafts", 9, 1,
	); err != nil {
		t.Fatalf("seed Drafts folder: %v", err)
	}
	return db, &draftOps{
		accountStore: account.NewStore(db),
		folderStore:  folder.NewStore(db),
		messageStore: message.NewStore(db),
		draftStore:   draft.NewStore(db),
	}
}

func TestDraftOpsPrepareSaveUpdateAndRestoreComposeMessage(t *testing.T) {
	_, ops := newDraftLocalFixture(t)
	if err := ops.messageStore.Create(&message.Message{
		ID: "source-message", AccountID: draftTestAccountID, FolderID: draftTestFolderID,
		UID: 100, Subject: "Source", Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed source message: %v", err)
	}
	msg := smtp.ComposeMessage{
		To:              []smtp.Address{{Name: "Alice", Address: "alice@example.com"}},
		Cc:              []smtp.Address{{Address: "copy@example.com"}},
		Bcc:             []smtp.Address{{Address: "hidden@example.com"}},
		Subject:         "Original subject",
		HTMLBody:        "<p>Hello</p>",
		TextBody:        "Hello",
		InReplyTo:       "<parent@example.com>",
		References:      []string{"<root@example.com>", "<parent@example.com>"},
		SourceMessageID: "source-message",
		ReplyType:       message.ComposeActionReply,
		Attachments: []smtp.Attachment{
			{Filename: "base64.txt", ContentType: "text/plain", ContentBase64: "cGF5bG9hZA=="},
			{Filename: "bytes.bin", ContentType: "application/octet-stream", Content: []byte{0, 1, 2}},
		},
	}

	body, err := ops.prepareDraftBody(msg)
	if err != nil {
		t.Fatalf("prepareDraftBody: %v", err)
	}
	if body.bodyHTML != msg.HTMLBody || body.bodyText != msg.TextBody || len(body.attachmentsData) == 0 {
		t.Fatalf("unexpected prepared body: %#v", body)
	}
	var storedAttachments []smtp.Attachment
	if err := json.Unmarshal(body.attachmentsData, &storedAttachments); err != nil {
		t.Fatalf("decode stored attachments: %v", err)
	}
	if len(storedAttachments) != 2 || string(storedAttachments[0].Content) != "payload" || storedAttachments[0].ContentBase64 != "" {
		t.Fatalf("attachments were not normalized: %#v", storedAttachments)
	}

	created, err := ops.saveDraftToDB(draftTestAccountID, nil, msg, body)
	if err != nil {
		t.Fatalf("saveDraftToDB create: %v", err)
	}
	if created.ID == "" || created.SyncStatus != draft.SyncStatusPending || created.SourceMessageID != "source-message" {
		t.Fatalf("unexpected created draft: %#v", created)
	}

	restored := ops.toComposeMessage(created)
	if len(restored.To) != 1 || restored.To[0].Address != "alice@example.com" || len(restored.Attachments) != 2 || string(restored.Attachments[0].Content) != "payload" {
		t.Fatalf("unexpected restored compose message: %#v", restored)
	}
	if len(restored.References) != 2 || restored.References[1] != "<parent@example.com>" {
		t.Fatalf("unexpected restored references: %#v", restored.References)
	}

	msg.Subject = "Updated subject"
	msg.TextBody = "Updated body"
	msg.References = []string{"<updated@example.com>"}
	msg.Attachments = nil
	body, err = ops.prepareDraftBody(msg)
	if err != nil {
		t.Fatalf("prepare updated draft: %v", err)
	}
	updated, err := ops.saveDraftToDB(draftTestAccountID, created, msg, body)
	if err != nil {
		t.Fatalf("saveDraftToDB update: %v", err)
	}
	if updated.ID != created.ID || updated.Subject != "Updated subject" || len(updated.AttachmentsData) != 0 || updated.SyncStatus != draft.SyncStatusPending {
		t.Fatalf("unexpected updated draft: %#v", updated)
	}
	fresh := ops.latestDraft(&draft.Draft{ID: created.ID, Subject: "stale"})
	if fresh == nil || fresh.Subject != "Updated subject" {
		t.Fatalf("latestDraft = %#v", fresh)
	}
	if ops.latestDraft(nil) != nil {
		t.Fatal("latestDraft(nil) must return nil")
	}

	if _, err := resolveAttachmentContent([]smtp.Attachment{{Filename: "broken.txt", ContentBase64: "%%%"}}); err == nil {
		t.Fatal("invalid attachment base64 must fail")
	}
	broken := ops.toComposeMessage(&draft.Draft{ID: "broken", AttachmentsData: []byte("{")})
	if len(broken.Attachments) != 0 {
		t.Fatalf("malformed stored attachments should be ignored: %#v", broken.Attachments)
	}
}

func TestDraftOpsSpecialFolderUsesMappingThenFallsBackToType(t *testing.T) {
	db, ops := newDraftLocalFixture(t)

	auto, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || auto == nil || auto.ID != draftTestFolderID {
		t.Fatalf("auto-detected Drafts folder = %#v, err=%v", auto, err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, selectable)
		 VALUES ('custom-drafts', ?, 'Custom Drafts', 'Custom Drafts', 'folder', 1)`,
		draftTestAccountID,
	); err != nil {
		t.Fatalf("seed mapped folder: %v", err)
	}
	if _, err := db.Exec(`UPDATE accounts SET drafts_folder_path = 'Custom Drafts' WHERE id = ?`, draftTestAccountID); err != nil {
		t.Fatalf("set Drafts mapping: %v", err)
	}
	mapped, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || mapped == nil || mapped.ID != "custom-drafts" {
		t.Fatalf("mapped Drafts folder = %#v, err=%v", mapped, err)
	}
	if _, err := db.Exec(`UPDATE folders SET selectable = 0 WHERE id = 'custom-drafts'`); err != nil {
		t.Fatalf("mark mapped Drafts folder as non-selectable: %v", err)
	}
	fallback, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || fallback == nil || fallback.ID != draftTestFolderID {
		t.Fatalf("non-selectable mapping fallback = %#v, err=%v", fallback, err)
	}

	if _, err := db.Exec(`UPDATE accounts SET drafts_folder_path = 'Missing Drafts' WHERE id = ?`, draftTestAccountID); err != nil {
		t.Fatalf("set missing mapping: %v", err)
	}
	fallback, err = ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || fallback == nil || fallback.ID != draftTestFolderID {
		t.Fatalf("fallback Drafts folder = %#v, err=%v", fallback, err)
	}
	if _, err := ops.getSpecialFolder("missing-account", folder.TypeDrafts); err == nil {
		t.Fatal("missing account must fail special-folder lookup")
	}
}

func TestDraftOpsLocalDeleteRemovesSyncedMessageAndDraft(t *testing.T) {
	_, ops := newDraftLocalFixture(t)
	now := time.Now().UTC()
	if err := ops.messageStore.Create(&message.Message{
		ID: "remote-draft-message", AccountID: draftTestAccountID, FolderID: draftTestFolderID,
		UID: 42, Subject: "Remote draft", Date: now,
	}); err != nil {
		t.Fatalf("seed draft message: %v", err)
	}
	synced := &draft.Draft{
		ID: "synced-draft", AccountID: draftTestAccountID, FolderID: draftTestFolderID,
		IMAPUID: 42, Subject: "Remote draft", SyncStatus: draft.SyncStatusSynced,
	}
	if err := ops.draftStore.Create(synced); err != nil {
		t.Fatalf("seed synced draft: %v", err)
	}

	draftsFolder, deleted, err := ops.deleteDraftLocalCore(&draft.Draft{ID: synced.ID, AccountID: synced.AccountID})
	if err != nil {
		t.Fatalf("deleteDraftLocalCore: %v", err)
	}
	if draftsFolder == nil || draftsFolder.ID != draftTestFolderID || deleted == nil || deleted.IMAPUID != 42 {
		t.Fatalf("unexpected local delete result: folder=%#v draft=%#v", draftsFolder, deleted)
	}
	if got, err := ops.draftStore.Get(synced.ID); err != nil || got != nil {
		t.Fatalf("draft remains after delete: draft=%#v err=%v", got, err)
	}
	if got, err := ops.messageStore.Get("remote-draft-message"); err != nil || got != nil {
		t.Fatalf("draft message remains after delete: message=%#v err=%v", got, err)
	}

	local := &draft.Draft{ID: "local-only", AccountID: draftTestAccountID, Subject: "Local only"}
	if err := ops.draftStore.Create(local); err != nil {
		t.Fatalf("seed local draft: %v", err)
	}
	if _, err := ops.deleteDraftCore(context.Background(), local); err != nil {
		t.Fatalf("deleteDraftCore local-only: %v", err)
	}
	if got, err := ops.draftStore.Get(local.ID); err != nil || got != nil {
		t.Fatalf("local-only draft remains: draft=%#v err=%v", got, err)
	}
}

func TestAppGetDraftSupportsLocalExternalAndMissingReferences(t *testing.T) {
	_, ops := newDraftLocalFixture(t)
	local := &draft.Draft{
		ID: "local-draft", AccountID: draftTestAccountID, Subject: "Local subject",
		ToList: `[{"name":"Alice","address":"alice@example.com"}]`, SyncStatus: draft.SyncStatusSynced,
	}
	if err := ops.draftStore.Create(local); err != nil {
		t.Fatalf("seed local draft: %v", err)
	}
	external := &message.Message{
		ID: "external-draft", AccountID: draftTestAccountID, FolderID: draftTestFolderID, UID: 99,
		Subject: "External subject", ToList: `[{"name":"Bob","email":"bob@example.com"}]`,
		BodyText: "External body", Date: time.Now().UTC(),
	}
	if err := ops.messageStore.Create(external); err != nil {
		t.Fatalf("seed external draft message: %v", err)
	}
	a := &App{
		ctx:          context.Background(),
		messageStore: ops.messageStore,
		draftStore:   ops.draftStore,
	}
	a.initSyncBridge()
	a.initDraftBridge()
	a.DraftBridge.ops = *ops

	got, err := a.GetDraft(local.ID)
	if err != nil || got == nil || got.Subject != "Local subject" || len(got.To) != 1 || got.To[0].Address != "alice@example.com" {
		t.Fatalf("GetDraft local = %#v, err=%v", got, err)
	}
	got, err = a.GetDraft(external.ID)
	if err != nil || got == nil || got.Subject != "External subject" || got.TextBody != "External body" || len(got.To) != 1 || got.To[0].Address != "bob@example.com" {
		t.Fatalf("GetDraft external = %#v, err=%v", got, err)
	}
	got, err = a.GetDraft("missing")
	if err != nil || got != nil {
		t.Fatalf("GetDraft missing = %#v, err=%v", got, err)
	}
	if err := a.syncPendingDrafts(draftTestAccountID); err != nil {
		t.Fatalf("SyncPendingDrafts with no pending drafts: %v", err)
	}
}

func TestAppDraftSyncCancellationBookkeeping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done)
	a := &App{}
	a.initSyncBridge()
	a.initDraftBridge()
	a.DraftBridge.syncContexts["draft"] = cancel
	a.DraftBridge.syncDone["draft"] = done
	a.cancelDraftSync("missing")
	a.cancelDraftSync("draft")
	if ctx.Err() != context.Canceled {
		t.Fatalf("cancelDraftSync did not cancel context: %v", ctx.Err())
	}
	if err := a.cancelDraftSyncAndWait("draft"); err != nil {
		t.Fatalf("cancelDraftSyncAndWait: %v", err)
	}
	if err := a.cancelDraftSyncAndWait("missing"); err != nil {
		t.Fatalf("cancelDraftSyncAndWait missing: %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	a.DraftBridge.syncContexts["without-done"] = cancel
	if err := a.cancelDraftSyncAndWait("without-done"); err != nil {
		t.Fatalf("cancelDraftSyncAndWait without done channel: %v", err)
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("cancel without done did not cancel context: %v", ctx.Err())
	}
}
