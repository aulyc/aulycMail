package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/activitylog"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/draft"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/smtp"
	mailSync "aulyc.local/aulycmail/internal/sync"
	goImap "github.com/emersion/go-imap/v2"
)

type draftSyncEmission struct {
	status    draft.SyncStatus
	imapUID   uint32
	syncError string
}

type draftIMAPFixture struct {
	db           *database.DB
	harness      actionIMAPHarness
	account      *account.Account
	draftsFolder *folder.Folder
	draftStore   *draft.Store
	messageStore *message.Store
	pool         *imapPkg.Pool
	ops          *draftOps
}

func newDraftIMAPFixture(t *testing.T) *draftIMAPFixture {
	t.Helper()
	harness := startActionIMAPServer(t)
	db, err := database.Open(t.TempDir() + "/draft-imap.db")
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	accountFixture := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	folderStore := folder.NewStore(db)
	draftsFolder := &folder.Folder{
		ID: "draft-imap-folder", AccountID: accountFixture.ID,
		Name: "Drafts", Path: "Drafts", Type: folder.TypeDrafts,
		Subscribed: true,
	}
	if err := folderStore.Create(draftsFolder); err != nil {
		t.Fatalf("create Drafts folder: %v", err)
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 1
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != accountFixture.ID {
			return nil, errors.New("credentials unavailable")
		}
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)

	draftStore := draft.NewStore(db)
	messageStore := message.NewStore(db)
	return &draftIMAPFixture{
		db: db, harness: harness, account: accountFixture,
		draftsFolder: draftsFolder, draftStore: draftStore,
		messageStore: messageStore, pool: pool,
		ops: &draftOps{
			accountStore: accountStore, folderStore: folderStore,
			messageStore: messageStore, draftStore: draftStore, imapPool: pool,
		},
	}
}

func draftIMAPMessage(subject string) smtp.ComposeMessage {
	return smtp.ComposeMessage{
		From:     smtp.Address{Name: "Draft Author", Address: actionIMAPUsername},
		To:       []smtp.Address{{Name: "Recipient", Address: "recipient@example.com"}},
		Subject:  subject,
		TextBody: "Synthetic draft body",
		HTMLBody: "<p>Synthetic draft body</p>",
	}
}

func TestDraftOpsSynchronizesReplacesAndDeletesRemoteDraft(t *testing.T) {
	fixture := newDraftIMAPFixture(t)
	localDraft := &draft.Draft{
		ID: "draft-imap-record", AccountID: fixture.account.ID,
		Subject: "First draft", SyncStatus: draft.SyncStatusPending,
	}
	if err := fixture.draftStore.Create(localDraft); err != nil {
		t.Fatalf("create local draft: %v", err)
	}

	var emissions []draftSyncEmission
	emit := func(status draft.SyncStatus, imapUID uint32, syncError string) {
		emissions = append(emissions, draftSyncEmission{status: status, imapUID: imapUID, syncError: syncError})
	}
	resultFolder := fixture.ops.syncToIMAP(context.Background(), localDraft, draftIMAPMessage("First draft"), emit)
	if resultFolder == nil || resultFolder.ID != fixture.draftsFolder.ID {
		t.Fatalf("first sync folder = %#v", resultFolder)
	}
	if len(emissions) != 1 || emissions[0].status != draft.SyncStatusSynced || emissions[0].imapUID != 1 || emissions[0].syncError != "" {
		t.Fatalf("first sync emissions = %#v", emissions)
	}
	stored, err := fixture.draftStore.Get(localDraft.ID)
	if err != nil || stored == nil || stored.SyncStatus != draft.SyncStatusSynced || stored.IMAPUID != 1 || stored.FolderID != fixture.draftsFolder.ID {
		t.Fatalf("stored first draft = %#v, %v", stored, err)
	}
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 1 {
		t.Fatalf("Drafts after first sync = %+v", status)
	}
	if raw := fixture.harness.rawFor(t, actionIMAPUsername, actionIMAPPassword, "Drafts", 1); !strings.Contains(string(raw), "Subject: First draft") || !strings.Contains(string(raw), "Synthetic draft body") {
		t.Fatalf("first remote draft raw = %q", raw)
	}
	flags := fixture.harness.flags(t, "Drafts", 1)
	wantFlags := map[goImap.Flag]bool{goImap.FlagDraft: true, goImap.FlagSeen: true}
	if len(flags) != len(wantFlags) {
		t.Fatalf("first remote draft flags = %v", flags)
	}
	for _, flag := range flags {
		if !wantFlags[flag] {
			t.Fatalf("unexpected remote draft flag %q in %v", flag, flags)
		}
	}

	if err := fixture.messageStore.Create(&message.Message{
		ID: "old-draft-message", AccountID: fixture.account.ID,
		FolderID: fixture.draftsFolder.ID, UID: stored.IMAPUID,
		Subject: stored.Subject, Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create old draft message: %v", err)
	}
	emissions = nil
	resultFolder = fixture.ops.syncToIMAP(context.Background(), stored, draftIMAPMessage("Replacement draft"), emit)
	if resultFolder == nil || len(emissions) != 1 || emissions[0].status != draft.SyncStatusSynced || emissions[0].imapUID != 2 {
		t.Fatalf("replacement sync = folder %#v emissions %#v", resultFolder, emissions)
	}
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 1 {
		t.Fatalf("Drafts after replacement = %+v", status)
	}
	if raw := fixture.harness.rawFor(t, actionIMAPUsername, actionIMAPPassword, "Drafts", 2); !strings.Contains(string(raw), "Subject: Replacement draft") {
		t.Fatalf("replacement remote raw = %q", raw)
	}
	if old, err := fixture.messageStore.Get("old-draft-message"); err != nil || old != nil {
		t.Fatalf("old local draft message = %#v, %v", old, err)
	}

	stored, err = fixture.draftStore.Get(localDraft.ID)
	if err != nil || stored == nil || stored.IMAPUID != 2 {
		t.Fatalf("stored replacement draft = %#v, %v", stored, err)
	}
	if err := fixture.messageStore.Create(&message.Message{
		ID: "replacement-draft-message", AccountID: fixture.account.ID,
		FolderID: fixture.draftsFolder.ID, UID: stored.IMAPUID,
		Subject: "Replacement draft", Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create replacement draft message: %v", err)
	}
	deletedFolder, err := fixture.ops.deleteDraftCore(context.Background(), stored)
	if err != nil || deletedFolder == nil || deletedFolder.ID != fixture.draftsFolder.ID {
		t.Fatalf("deleteDraftCore() = folder %#v, %v", deletedFolder, err)
	}
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 0 {
		t.Fatalf("Drafts after delete = %+v", status)
	}
	if got, err := fixture.draftStore.Get(localDraft.ID); err != nil || got != nil {
		t.Fatalf("local draft after delete = %#v, %v", got, err)
	}
	if got, err := fixture.messageStore.Get("replacement-draft-message"); err != nil || got != nil {
		t.Fatalf("replacement message after delete = %#v, %v", got, err)
	}
	if err := fixture.ops.deleteDraftFromIMAP(context.Background(), nil, fixture.draftsFolder); err != nil {
		t.Fatalf("deleteDraftFromIMAP(nil) = %v", err)
	}
}

func TestDraftOpsReportsFolderConnectionAndMessageFailures(t *testing.T) {
	fixture := newDraftIMAPFixture(t)
	missingFolderAccount := createOfflineCacheTestAccount(t, fixture.ops.accountStore, "no-drafts@example.com")
	missingFolderDraft := &draft.Draft{
		ID: "missing-folder-draft", AccountID: missingFolderAccount.ID,
		Subject: "No folder", SyncStatus: draft.SyncStatusPending,
	}
	if err := fixture.draftStore.Create(missingFolderDraft); err != nil {
		t.Fatalf("create missing-folder draft: %v", err)
	}
	var emissions []draftSyncEmission
	emit := func(status draft.SyncStatus, imapUID uint32, syncError string) {
		emissions = append(emissions, draftSyncEmission{status: status, imapUID: imapUID, syncError: syncError})
	}
	if got := fixture.ops.syncToIMAP(context.Background(), missingFolderDraft, draftIMAPMessage("No folder"), emit); got != nil {
		t.Fatalf("missing-folder sync returned %#v", got)
	}
	if len(emissions) != 1 || emissions[0].status != draft.SyncStatusFailed || emissions[0].syncError != "no drafts folder found" {
		t.Fatalf("missing-folder emissions = %#v", emissions)
	}
	stored, err := fixture.draftStore.Get(missingFolderDraft.ID)
	if err != nil || stored == nil || stored.SyncStatus != draft.SyncStatusFailed || stored.SyncError != "no drafts folder found" {
		t.Fatalf("missing-folder stored draft = %#v, %v", stored, err)
	}

	connectionDraft := &draft.Draft{
		ID: "connection-failure-draft", AccountID: fixture.account.ID,
		Subject: "Connection failure", SyncStatus: draft.SyncStatusPending,
	}
	if err := fixture.draftStore.Create(connectionDraft); err != nil {
		t.Fatalf("create connection-failure draft: %v", err)
	}
	brokenPool := imapPkg.NewPool(imapPkg.DefaultPoolConfig(), func(string) (*imapPkg.ClientConfig, error) {
		return nil, errors.New("synthetic credential failure")
	})
	t.Cleanup(brokenPool.CloseAll)
	brokenOps := *fixture.ops
	brokenOps.imapPool = brokenPool
	emissions = nil
	if got := brokenOps.syncToIMAP(context.Background(), connectionDraft, draftIMAPMessage("Connection failure"), emit); got != nil {
		t.Fatalf("connection-failure sync returned %#v", got)
	}
	if len(emissions) != 1 || emissions[0].status != draft.SyncStatusFailed || !strings.Contains(emissions[0].syncError, "synthetic credential failure") {
		t.Fatalf("connection-failure emissions = %#v", emissions)
	}

	invalidDraft := &draft.Draft{
		ID: "invalid-rfc822-draft", AccountID: fixture.account.ID,
		Subject: "Invalid RFC822", SyncStatus: draft.SyncStatusPending,
	}
	if err := fixture.draftStore.Create(invalidDraft); err != nil {
		t.Fatalf("create invalid-RFC822 draft: %v", err)
	}
	invalidMessage := draftIMAPMessage("Invalid RFC822")
	invalidMessage.Attachments = []smtp.Attachment{{
		Filename: "broken.txt", ContentType: "text/plain", ContentBase64: "%%%",
	}}
	emissions = nil
	if got := fixture.ops.syncToIMAP(context.Background(), invalidDraft, invalidMessage, emit); got != nil {
		t.Fatalf("invalid-RFC822 sync returned %#v", got)
	}
	if len(emissions) != 1 || emissions[0].status != draft.SyncStatusFailed || emissions[0].syncError == "" {
		t.Fatalf("invalid-RFC822 emissions = %#v", emissions)
	}
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 0 {
		t.Fatalf("failed draft syncs changed remote mailbox = %+v", status)
	}

	deletedBeforeSync := &draft.Draft{
		ID: "already-deleted-draft", AccountID: fixture.account.ID,
		Subject: "Already deleted", SyncStatus: draft.SyncStatusPending,
	}
	emissions = nil
	if got := fixture.ops.syncToIMAP(context.Background(), deletedBeforeSync, draftIMAPMessage("Already deleted"), emit); got != nil {
		t.Fatalf("deleted-before-sync returned %#v", got)
	}
	if len(emissions) != 0 {
		t.Fatalf("deleted-before-sync emitted status %#v", emissions)
	}
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 0 {
		t.Fatalf("deleted-before-sync changed remote mailbox = %+v", status)
	}
}

func TestAppDraftLifecycleSynchronizesUpdatesAndDeletesRemoteDraft(t *testing.T) {
	fixture := newDraftIMAPFixture(t)
	attachmentStore := message.NewAttachmentStore(fixture.db)
	contactStore := contact.NewStore(fixture.db.DB)
	engine := mailSync.NewEngine(
		fixture.pool,
		fixture.ops.accountStore,
		fixture.ops.folderStore,
		fixture.messageStore,
		attachmentStore,
		contactStore,
	)
	events := &actionEventRecorder{}
	a := &App{
		ctx:              context.Background(),
		eventsEmit:       events.emit,
		db:               fixture.db,
		accountStore:     fixture.ops.accountStore,
		activityLogStore: activitylog.NewStore(fixture.db),
		folderStore:      fixture.ops.folderStore,
		messageStore:     fixture.messageStore,
		attachmentStore:  attachmentStore,
		contactStore:     contactStore,
		draftStore:       fixture.draftStore,
		imapPool:         fixture.pool,
		syncEngine:       engine,
	}
	a.initSyncBridge()
	a.initDraftBridge()
	a.DraftBridge.ops = *fixture.ops

	result, err := a.SaveDraft(fixture.account.ID, draftIMAPMessage("Public first draft"), "")
	if err != nil || result == nil || result.Draft == nil {
		t.Fatalf("SaveDraft create = (%#v, %v)", result, err)
	}
	waitForActionCondition(t, "first public draft sync", func() bool {
		stored, getErr := fixture.draftStore.Get(result.Draft.ID)
		return getErr == nil && stored != nil && stored.SyncStatus == draft.SyncStatusSynced && stored.IMAPUID == 1
	})
	if status := fixture.harness.status(t, "Drafts"); status.Messages != 1 {
		t.Fatalf("Drafts after public save = %#v", status)
	}
	compose, err := a.GetDraft(result.Draft.ID)
	if err != nil || compose == nil || compose.Subject != "Public first draft" {
		t.Fatalf("GetDraft after save = (%#v, %v)", compose, err)
	}

	updatedResult, err := a.SaveDraft(fixture.account.ID, draftIMAPMessage("Public replacement draft"), result.Draft.ID)
	if err != nil || updatedResult == nil || updatedResult.Draft.ID != result.Draft.ID {
		t.Fatalf("SaveDraft update = (%#v, %v)", updatedResult, err)
	}
	waitForActionCondition(t, "replacement public draft sync", func() bool {
		stored, getErr := fixture.draftStore.Get(result.Draft.ID)
		return getErr == nil && stored != nil && stored.SyncStatus == draft.SyncStatusSynced && stored.IMAPUID == 2
	})
	if raw := fixture.harness.rawFor(t, actionIMAPUsername, actionIMAPPassword, "Drafts", 2); !strings.Contains(string(raw), "Public replacement draft") {
		t.Fatalf("replacement public draft raw = %q", raw)
	}

	if err := fixture.draftStore.UpdateSyncStatus(result.Draft.ID, draft.SyncStatusPending, 2, fixture.draftsFolder.ID, ""); err != nil {
		t.Fatalf("mark draft pending: %v", err)
	}
	if err := a.syncPendingDrafts(fixture.account.ID); err != nil {
		t.Fatalf("SyncPendingDrafts: %v", err)
	}
	if err := a.syncPendingDrafts("missing-account"); err != nil {
		t.Fatalf("SyncPendingDrafts missing account: %v", err)
	}

	invalid := draftIMAPMessage("Invalid public draft")
	invalid.Attachments = []smtp.Attachment{{Filename: "broken.txt", ContentBase64: "%%%"}}
	if _, err := a.SaveDraft(fixture.account.ID, invalid, ""); err == nil {
		t.Fatal("SaveDraft with invalid attachment unexpectedly succeeded")
	}
	absentReference, err := a.SaveDraft(fixture.account.ID, draftIMAPMessage("Resolve failure"), "missing-reference")
	if err != nil {
		t.Fatalf("SaveDraft with absent optional reference: %v", err)
	}
	waitForActionCondition(t, "absent-reference draft sync", func() bool {
		stored, getErr := fixture.draftStore.Get(absentReference.Draft.ID)
		return getErr == nil && stored != nil && stored.SyncStatus == draft.SyncStatusSynced
	})

	if err := a.DeleteDraft(result.Draft.ID); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	if err := a.DeleteDraft(result.Draft.ID); err != nil {
		t.Fatalf("DeleteDraft idempotent: %v", err)
	}
	if err := a.DeleteDraft(absentReference.Draft.ID); err != nil {
		t.Fatalf("DeleteDraft absent-reference draft: %v", err)
	}
	waitForActionCondition(t, "public draft removal", func() bool {
		stored, getErr := fixture.draftStore.Get(result.Draft.ID)
		return getErr == nil && stored == nil && fixture.harness.status(t, "Drafts").Messages == 0
	})
	if !events.has("draft:syncStatusChanged") || !events.has("messages:updated") || !events.has("folder:synced") {
		t.Fatalf("draft lifecycle events = %v", events.snapshot())
	}
}
