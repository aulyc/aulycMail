package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	goImap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

const (
	syncIMAPUsername = "sync@example.com"
	syncIMAPPassword = "sync-secret"
	syncAccountID    = "sync-account"
	syncFolderID     = "sync-inbox"
)

type syncIMAPHarness struct {
	host string
	port int
	user *imapmemserver.User
}

func startSyncIMAPServer(t *testing.T) syncIMAPHarness {
	t.Helper()
	memory := imapmemserver.New()
	user := imapmemserver.NewUser(syncIMAPUsername, syncIMAPPassword)
	mailboxes := []struct {
		name       string
		specialUse goImap.MailboxAttr
	}{
		{name: "INBOX"},
		{name: "Archive", specialUse: goImap.MailboxAttrArchive},
		{name: "Drafts", specialUse: goImap.MailboxAttrDrafts},
		{name: "Sent", specialUse: goImap.MailboxAttrSent},
		{name: "Trash", specialUse: goImap.MailboxAttrTrash},
		{name: "Junk", specialUse: goImap.MailboxAttrJunk},
		{name: "Projects"},
		{name: "Projects/2026"},
	}
	for _, mailbox := range mailboxes {
		var options *goImap.CreateOptions
		if mailbox.specialUse != "" {
			options = &goImap.CreateOptions{SpecialUse: []goImap.MailboxAttr{mailbox.specialUse}}
		}
		if err := user.Create(mailbox.name, options); err != nil {
			t.Fatalf("create mailbox %s: %v", mailbox.name, err)
		}
	}
	for _, mailbox := range []string{"INBOX", "Archive", "Projects"} {
		if err := user.Subscribe(mailbox); err != nil {
			t.Fatalf("subscribe mailbox %s: %v", mailbox, err)
		}
	}
	memory.AddUser(user)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memory.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps: goImap.CapSet{
			goImap.CapIMAP4rev1:    {},
			goImap.CapListExtended: {},
			goImap.CapSpecialUse:   {},
			goImap.CapUIDPlus:      {},
		},
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for sync IMAP server: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close sync IMAP server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("serve sync IMAP server: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("sync IMAP server did not stop")
		}
	})

	addr := listener.Addr().(*net.TCPAddr)
	return syncIMAPHarness{host: addr.IP.String(), port: addr.Port, user: user}
}

func (h syncIMAPHarness) clientConfig() imapPkg.ClientConfig {
	config := imapPkg.DefaultConfig()
	config.Host = h.host
	config.Port = h.port
	config.Security = imapPkg.SecurityNone
	config.Username = syncIMAPUsername
	config.Password = syncIMAPPassword
	config.ConnectTimeout = 2 * time.Second
	config.ReadTimeout = 2 * time.Second
	config.WriteTimeout = 2 * time.Second
	return config
}

func (h syncIMAPHarness) client(t *testing.T) *imapPkg.Client {
	t.Helper()
	client := imapPkg.NewClient(h.clientConfig())
	if err := client.Connect(); err != nil {
		t.Fatalf("connect fixture client: %v", err)
	}
	if err := client.Login(); err != nil {
		_ = client.ForceClose()
		t.Fatalf("login fixture client: %v", err)
	}
	return client
}

func (h syncIMAPHarness) append(t *testing.T, raw []byte, flags ...goImap.Flag) uint32 {
	return h.appendAt(t, time.Now().UTC(), raw, flags...)
}

func (h syncIMAPHarness) appendAt(t *testing.T, internalDate time.Time, raw []byte, flags ...goImap.Flag) uint32 {
	t.Helper()
	client := h.client(t)
	defer func() { _ = client.Close() }()
	uid, err := client.AppendMessage("INBOX", flags, internalDate, raw)
	if err != nil {
		t.Fatalf("append fixture message: %v", err)
	}
	return uint32(uid)
}

type syncIntegrationFixture struct {
	db              *database.DB
	harness         syncIMAPHarness
	pool            *imapPkg.Pool
	engine          *Engine
	folderStore     *folder.Store
	messageStore    *message.Store
	attachmentStore *message.AttachmentStore
	contactStore    *contact.Store
}

func newSyncIntegrationFixture(t *testing.T) *syncIntegrationFixture {
	t.Helper()
	harness := startSyncIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO accounts (
			id, name, email, imap_host, imap_port, imap_security,
			smtp_host, smtp_port, smtp_security, auth_type, username, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, syncAccountID, "Sync Test", syncIMAPUsername, harness.host, harness.port, account.SecurityNone,
		"smtp.example.com", 587, account.SecurityStartTLS, account.AuthPassword, syncIMAPUsername); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	folderStore := folder.NewStore(db)
	if err := folderStore.Create(&folder.Folder{
		ID: syncFolderID, AccountID: syncAccountID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox, Subscribed: true,
	}); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 2
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != syncAccountID {
			return nil, fmt.Errorf("unknown account %q", accountID)
		}
		config := harness.clientConfig()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)

	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	contactStore := contact.NewStore(db.DB)
	contactStore.SetOwnEmails([]string{syncIMAPUsername})
	engine := NewEngine(pool, account.NewStore(db), folderStore, messageStore, attachmentStore, contactStore)

	return &syncIntegrationFixture{
		db:              db,
		harness:         harness,
		pool:            pool,
		engine:          engine,
		folderStore:     folderStore,
		messageStore:    messageStore,
		attachmentStore: attachmentStore,
		contactStore:    contactStore,
	}
}

var syncPlainMessage = []byte(strings.Join([]string{
	"Date: Sun, 02 Aug 2026 08:30:00 +0000",
	"From: Alice Example <alice@example.com>",
	"To: Sync User <sync@example.com>",
	"Subject: Plain integration message",
	"Message-ID: <plain-1@example.com>",
	"Disposition-Notification-To: receipts@example.com",
	"Content-Type: text/plain; charset=utf-8",
	"Content-Transfer-Encoding: 8bit",
	"",
	"Hello from the local IMAP integration test.",
	"",
}, "\r\n"))

var syncMultipartMessage = []byte(strings.Join([]string{
	"Date: Sun, 02 Aug 2026 09:00:00 +0000",
	"From: Bob Example <bob@example.com>",
	"To: Sync User <sync@example.com>",
	"Subject: Multipart integration message",
	"Message-ID: <multipart-2@example.com>",
	"MIME-Version: 1.0",
	`Content-Type: multipart/mixed; boundary="sync-boundary"`,
	"",
	"--sync-boundary",
	"Content-Type: text/plain; charset=utf-8",
	"",
	"Multipart body text.",
	"--sync-boundary",
	`Content-Type: application/octet-stream; name="report.txt"`,
	"Content-Disposition: attachment; filename=\"report.txt\"",
	"Content-Transfer-Encoding: base64",
	"",
	"cmVwb3J0IGNvbnRlbnQ=",
	"--sync-boundary--",
	"",
}, "\r\n"))

func TestEngineSyncFetchAndRawExportAgainstMemoryIMAP(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	uid1 := fixture.harness.append(t, syncPlainMessage, goImap.FlagSeen, goImap.FlagFlagged)
	uid2 := fixture.harness.append(t, syncMultipartMessage)
	if uid1 != 1 || uid2 != 2 {
		t.Fatalf("fixture UIDs = %d, %d", uid1, uid2)
	}

	var progress []SyncProgress
	fixture.engine.SetProgressCallback(func(update SyncProgress) { progress = append(progress, update) })
	result, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{
		Strategy: account.SyncStrategyFull,
	})
	if err != nil {
		t.Fatalf("SyncMessagesWithOptionsResult() error = %v", err)
	}
	if result.Added != 2 || result.Removed != 0 || !result.Performed {
		t.Fatalf("sync result = %+v", result)
	}
	if len(progress) < 3 || progress[0].Phase != "messages" || progress[len(progress)-1].Phase != "headers" {
		t.Fatalf("header progress = %#v", progress)
	}

	plain, err := fixture.messageStore.GetByUID(syncFolderID, uid1)
	if err != nil || plain == nil {
		t.Fatalf("GetByUID(plain) = %#v, %v", plain, err)
	}
	if plain.Subject != "Plain integration message" || plain.FromEmail != "alice@example.com" || !plain.IsRead || !plain.IsStarred || plain.BodyFetched {
		t.Fatalf("plain header = %+v", plain)
	}
	if plain.MessageID != "plain-1@example.com" || plain.ReadReceiptTo != "receipts@example.com" {
		t.Fatalf("plain identity headers = %+v", plain)
	}
	multipart, err := fixture.messageStore.GetByUID(syncFolderID, uid2)
	if err != nil || multipart == nil || multipart.Subject != "Multipart integration message" || multipart.IsRead {
		t.Fatalf("GetByUID(multipart) = %#v, %v", multipart, err)
	}
	f, err := fixture.folderStore.Get(syncFolderID)
	if err != nil || f == nil || f.TotalCount != 2 || f.UnreadCount != 1 || f.UIDValidity == 0 || f.UIDNext != 3 || f.LastSync == nil || f.LastFullSync == nil {
		t.Fatalf("synced folder = %#v, %v", f, err)
	}
	contacts, err := fixture.contactStore.Search("alice", 10)
	if err != nil || len(contacts) != 1 || contacts[0].Email != "alice@example.com" {
		t.Fatalf("auto-collected contacts = %#v, %v", contacts, err)
	}

	fetchedPlain, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, plain.ID)
	if err != nil {
		t.Fatalf("FetchMessageBody() error = %v", err)
	}
	if !fetchedPlain.BodyFetched || !strings.Contains(fetchedPlain.BodyText, "Hello from the local IMAP integration test") || fetchedPlain.Snippet == "" {
		t.Fatalf("fetched plain body = %+v", fetchedPlain)
	}
	if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, syncFolderID, 0); err != nil {
		t.Fatalf("FetchBodiesInBackground() error = %v", err)
	}
	multipart, err = fixture.messageStore.GetByUID(syncFolderID, uid2)
	if err != nil || multipart == nil || !multipart.BodyFetched || !strings.Contains(multipart.BodyText, "Multipart body text") || !multipart.HasAttachments {
		t.Fatalf("background-fetched multipart = %#v, %v", multipart, err)
	}
	attachments, err := fixture.attachmentStore.GetByMessage(multipart.ID)
	if err != nil || len(attachments) != 1 || attachments[0].Filename != "report.txt" || attachments[0].Size != len("report content") {
		t.Fatalf("multipart attachments = %#v, %v", attachments, err)
	}

	raw, err := fixture.engine.FetchRawMessage(context.Background(), syncAccountID, syncFolderID, uid1)
	if err != nil || !bytes.Equal(raw, syncPlainMessage) {
		t.Fatalf("FetchRawMessage() len = %d, equal = %t, err = %v", len(raw), bytes.Equal(raw, syncPlainMessage), err)
	}
	var streamed bytes.Buffer
	streamResult, err := fixture.engine.StreamRawMessage(context.Background(), syncAccountID, syncFolderID, uid2, &streamed)
	if err != nil || !bytes.Equal(streamed.Bytes(), syncMultipartMessage) || streamResult.BytesWritten != int64(len(syncMultipartMessage)) || streamResult.ReportedSize != int64(len(syncMultipartMessage)) {
		t.Fatalf("StreamRawMessage() = %+v, equal = %t, err = %v", streamResult, bytes.Equal(streamed.Bytes(), syncMultipartMessage), err)
	}

	streamedBatch := make(map[uint32][]byte)
	results, failures, err := fixture.engine.StreamRawMessages(context.Background(), syncAccountID, syncFolderID, []uint32{uid1, uid2, 999, 0, uid1}, func(uid uint32, body io.Reader) (int64, error) {
		var dst bytes.Buffer
		n, copyErr := io.Copy(&dst, body)
		streamedBatch[uid] = dst.Bytes()
		if copyErr != nil {
			return n, copyErr
		}
		if uid == uid2 {
			return n, errors.New("synthetic destination failure")
		}
		return n, nil
	})
	if err != nil || len(results) != 1 || results[uid1] == nil || !bytes.Equal(streamedBatch[uid1], syncPlainMessage) {
		t.Fatalf("StreamRawMessages results = %#v, err = %v", results, err)
	}
	if len(failures) != 2 || failures[uid2] == nil || !strings.Contains(failures[uid2].Error(), "synthetic destination failure") {
		t.Fatalf("StreamRawMessages failures = %#v", failures)
	}
	var notFound RawMessageNotFoundError
	if !errors.As(failures[999], &notFound) || notFound.UID != 999 {
		t.Fatalf("missing UID failure = %v", failures[999])
	}
}

func TestEngineIncrementalFlagsDeletionAndErrorsAgainstMemoryIMAP(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	uid1 := fixture.harness.append(t, syncPlainMessage)
	if err := fixture.engine.SyncMessages(context.Background(), syncAccountID, syncFolderID, 0); err != nil {
		t.Fatalf("initial SyncMessages() error = %v", err)
	}

	uid2 := fixture.harness.append(t, syncMultipartMessage)
	result, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{
		Strategy:              account.SyncStrategyIncremental,
		FullCheckIntervalDays: 0,
	})
	if err != nil || result.Added != 1 || result.Removed != 0 {
		t.Fatalf("incremental sync = %+v, %v", result, err)
	}
	if uid2 != 2 {
		t.Fatalf("incremental fixture UID = %d", uid2)
	}

	remote := fixture.harness.client(t)
	if _, err := remote.SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatalf("select fixture INBOX: %v", err)
	}
	if err := remote.AddMessageFlags([]goImap.UID{goImap.UID(uid1)}, []goImap.Flag{goImap.FlagSeen, goImap.FlagAnswered, "$Forwarded"}); err != nil {
		t.Fatalf("set remote flags: %v", err)
	}
	if err := remote.DeleteMessageByUID(goImap.UID(uid2)); err != nil {
		t.Fatalf("delete remote message: %v", err)
	}
	if err := remote.Close(); err != nil {
		t.Fatalf("close remote mutator: %v", err)
	}

	result, err = fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{
		Strategy: account.SyncStrategyFull,
	})
	if err != nil || result.Added != 0 || result.Removed != 1 {
		t.Fatalf("full reconciliation = %+v, %v", result, err)
	}
	updated, err := fixture.messageStore.GetByUID(syncFolderID, uid1)
	if err != nil || updated == nil || !updated.IsRead || !updated.IsAnswered || !updated.IsForwarded {
		t.Fatalf("updated flags = %#v, %v", updated, err)
	}
	deleted, err := fixture.messageStore.GetByUID(syncFolderID, uid2)
	if err != nil || deleted != nil {
		t.Fatalf("deleted local message = %#v, %v", deleted, err)
	}

	if _, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, "missing-message"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("FetchMessageBody(missing) error = %v", err)
	}
	if _, err := fixture.engine.FetchRawMessage(context.Background(), syncAccountID, syncFolderID, 999); err == nil {
		t.Fatal("FetchRawMessage(missing UID) returned nil error")
	}
	if _, err := fixture.engine.StreamRawMessage(context.Background(), syncAccountID, syncFolderID, 999, io.Discard); err == nil {
		t.Fatal("StreamRawMessage(missing UID) returned nil error")
	}
	if _, _, err := fixture.engine.StreamRawMessages(context.Background(), syncAccountID, syncFolderID, []uint32{uid1}, nil); err == nil || !strings.Contains(err.Error(), "handler is nil") {
		t.Fatalf("StreamRawMessages(nil handler) error = %v", err)
	}
	results, failures, err := fixture.engine.StreamRawMessages(context.Background(), syncAccountID, syncFolderID, nil, nil)
	if err != nil || len(results) != 0 || len(failures) != 0 {
		t.Fatalf("StreamRawMessages(empty) = %#v, %#v, %v", results, failures, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := fixture.engine.StreamRawMessages(canceled, syncAccountID, syncFolderID, []uint32{uid1}, func(uint32, io.Reader) (int64, error) { return 0, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("StreamRawMessages(canceled) error = %v", err)
	}
	if result, err := fixture.engine.SyncMessagesWithOptionsResult(canceled, syncAccountID, syncFolderID, MessageSyncOptions{}); !errors.Is(err, context.Canceled) || result.Performed {
		t.Fatalf("canceled sync = %+v, %v", result, err)
	}
	if _, err := fixture.engine.FetchRawMessage(context.Background(), syncAccountID, "missing-folder", uid1); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("FetchRawMessage(missing folder) error = %v", err)
	}
	if _, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, "missing-folder", MessageSyncOptions{}); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("sync missing folder error = %v", err)
	}

	noSelect := &folder.Folder{ID: "sync-no-select", AccountID: syncAccountID, Name: "Group", Path: "Group", Type: folder.TypeFolder, NoSelect: true}
	if err := fixture.folderStore.Create(noSelect); err != nil {
		t.Fatalf("create no-select folder: %v", err)
	}
	if _, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, noSelect.ID, MessageSyncOptions{}); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("sync no-select folder error = %v", err)
	}
	if _, err := fixture.engine.FetchRawMessage(context.Background(), syncAccountID, noSelect.ID, uid1); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("FetchRawMessage(no-select) error = %v", err)
	}
}

func TestEngineRetentionSearchAndEmptyMailboxSafeguardAgainstMemoryIMAP(t *testing.T) {
	t.Run("retention search excludes old server mail and removes old local rows", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		oldDate := time.Now().UTC().AddDate(0, 0, -90)
		oldUID := fixture.harness.appendAt(t, oldDate, syncPlainMessage)
		recentUID := fixture.harness.appendAt(t, time.Now().UTC(), syncMultipartMessage)

		if err := fixture.messageStore.Create(&message.Message{
			ID: "locally-old", AccountID: syncAccountID, FolderID: syncFolderID,
			UID: 900, Subject: "expired local row", Date: oldDate, ReceivedAt: oldDate,
		}); err != nil {
			t.Fatalf("seed old local message: %v", err)
		}

		result, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{
			RetentionDays: 30,
			Strategy:      account.SyncStrategyFull,
		})
		if err != nil {
			t.Fatalf("retention sync error = %v", err)
		}
		if result.Added != 1 || result.Removed != 1 || !result.Performed {
			t.Fatalf("retention sync result = %+v", result)
		}
		if got, err := fixture.messageStore.GetByUID(syncFolderID, oldUID); err != nil || got != nil {
			t.Fatalf("old remote message = %#v, %v; want absent", got, err)
		}
		if got, err := fixture.messageStore.GetByUID(syncFolderID, recentUID); err != nil || got == nil || got.Subject != "Multipart integration message" {
			t.Fatalf("recent remote message = %#v, %v", got, err)
		}
		if got, err := fixture.messageStore.Get("locally-old"); err != nil || got != nil {
			t.Fatalf("old local message = %#v, %v; want deleted", got, err)
		}
	})

	t.Run("empty server result preserves local messages and counts", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		now := time.Now().UTC()
		if err := fixture.messageStore.Create(&message.Message{
			ID: "preserved", AccountID: syncAccountID, FolderID: syncFolderID,
			UID: 77, Subject: "keep me", Date: now, ReceivedAt: now,
		}); err != nil {
			t.Fatalf("seed preserved message: %v", err)
		}

		result, err := fixture.engine.SyncMessagesWithOptionsResult(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{
			Strategy: account.SyncStrategyFull,
		})
		if err != nil || result.Added != 0 || result.Removed != 0 || !result.Performed {
			t.Fatalf("empty mailbox sync = %+v, %v", result, err)
		}
		if got, err := fixture.messageStore.Get("preserved"); err != nil || got == nil {
			t.Fatalf("preserved local message = %#v, %v", got, err)
		}
		gotFolder, err := fixture.folderStore.Get(syncFolderID)
		if err != nil || gotFolder == nil || gotFolder.TotalCount != 1 || gotFolder.UnreadCount != 1 || gotFolder.LastSync == nil {
			t.Fatalf("preserved folder counts = %#v, %v", gotFolder, err)
		}
	})
}

func TestEngineBodyFailureClassificationAndFetchEdgeCases(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	now := time.Now().UTC()
	for index, id := range []string{"resolved", "deferred", "failed"} {
		if err := fixture.messageStore.Create(&message.Message{
			ID: id, AccountID: syncAccountID, FolderID: syncFolderID,
			UID: uint32(100 + index), Subject: id, Date: now, ReceivedAt: now,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	fixture.engine.markUnresolvedAsFailed(
		[]string{"resolved", "deferred", "failed"},
		[]message.BodyUpdate{{MessageID: "resolved", BodyText: "usable"}},
		map[string]fetchedSize{"deferred": {received: 5, reported: 10}},
	)
	fixture.engine.markUnresolvedAsFailed(nil, nil, nil)
	fixture.engine.markUnresolvedAsFailed([]string{"resolved"}, []message.BodyUpdate{{MessageID: "resolved", BodyHTML: "<p>ok</p>"}}, nil)

	for id, want := range map[string]int{"resolved": 0, "deferred": 0, "failed": 1} {
		var got int
		if err := fixture.db.QueryRow(`SELECT body_failed FROM messages WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatalf("query body_failed for %s: %v", id, err)
		}
		if got != want {
			t.Fatalf("body_failed[%s] = %d, want %d", id, got, want)
		}
	}

	if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, "missing-folder", 0); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("background fetch missing folder error = %v", err)
	}
	noSelect := &folder.Folder{ID: "body-no-select", AccountID: syncAccountID, Name: "Group", Path: "Group", Type: folder.TypeFolder, NoSelect: true}
	if err := fixture.folderStore.Create(noSelect); err != nil {
		t.Fatalf("create no-select folder: %v", err)
	}
	if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, noSelect.ID, 0); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("background fetch no-select error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := fixture.engine.FetchBodiesInBackground(canceled, syncAccountID, syncFolderID, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("background fetch canceled error = %v", err)
	}

	noSelectMessage := &message.Message{
		ID: "no-select-message", AccountID: syncAccountID, FolderID: noSelect.ID,
		UID: 1, Subject: "no-select folder", Date: now, ReceivedAt: now,
	}
	if err := fixture.messageStore.Create(noSelectMessage); err != nil {
		t.Fatalf("seed no-select message: %v", err)
	}
	if _, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, noSelectMessage.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("on-demand no-select error = %v", err)
	}
}
