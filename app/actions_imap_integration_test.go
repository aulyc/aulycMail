package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/activitylog"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/platform"
	"aulyc.local/aulycmail/internal/settings"
	mailSync "aulyc.local/aulycmail/internal/sync"
	"aulyc.local/aulycmail/internal/undo"
	goImap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

const (
	actionIMAPUsername            = "actions@example.com"
	actionIMAPPassword            = "actions-secret"
	actionIMAPDestinationUsername = "destination@example.com"
	actionIMAPDestinationPassword = "destination-secret"
)

func runEmailBackupSynchronouslyForTest(a *App, options BackupRunOptions) (*BackupRunResult, error) {
	a.initBackupBridge()
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if !a.BackupBridge.job.start(startedAt) {
		return nil, errEmailBackupAlreadyRunning
	}
	result, err := a.runEmailBackup(options, startedAt)
	if err != nil {
		a.finishBackupProgress(BackupProgress{Phase: "error", Message: err.Error()})
		return nil, err
	}
	a.finishBackupProgress(backupDoneProgress(result))
	return result, nil
}

type actionIMAPHarness struct {
	host string
	port int
}

func startActionIMAPServer(t *testing.T) actionIMAPHarness {
	t.Helper()
	memory := imapmemserver.New()
	for _, credentials := range [][2]string{
		{actionIMAPUsername, actionIMAPPassword},
		{actionIMAPDestinationUsername, actionIMAPDestinationPassword},
	} {
		user := imapmemserver.NewUser(credentials[0], credentials[1])
		for _, mailbox := range []string{"INBOX", "Archive", "Drafts", "Trash"} {
			if err := user.Create(mailbox, nil); err != nil {
				t.Fatalf("create mailbox %s for %s: %v", mailbox, credentials[0], err)
			}
		}
		memory.AddUser(user)
	}

	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memory.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps: goImap.CapSet{
			goImap.CapIMAP4rev1: {},
			goImap.CapUIDPlus:   {},
		},
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for action IMAP server: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close action IMAP server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("serve action IMAP server: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("action IMAP server did not stop")
		}
	})
	addr := listener.Addr().(*net.TCPAddr)
	return actionIMAPHarness{host: addr.IP.String(), port: addr.Port}
}

func (h actionIMAPHarness) config() imapPkg.ClientConfig {
	return h.configFor(actionIMAPUsername, actionIMAPPassword)
}

func (h actionIMAPHarness) configFor(username, password string) imapPkg.ClientConfig {
	config := imapPkg.DefaultConfig()
	config.Host = h.host
	config.Port = h.port
	config.Security = imapPkg.SecurityNone
	config.Username = username
	config.Password = password
	config.ConnectTimeout = 2 * time.Second
	config.ReadTimeout = 2 * time.Second
	config.WriteTimeout = 2 * time.Second
	return config
}

func (h actionIMAPHarness) client(t *testing.T) *imapPkg.Client {
	return h.clientFor(t, actionIMAPUsername, actionIMAPPassword)
}

func (h actionIMAPHarness) clientFor(t *testing.T, username, password string) *imapPkg.Client {
	t.Helper()
	client := imapPkg.NewClient(h.configFor(username, password))
	if err := client.Connect(); err != nil {
		t.Fatalf("connect action fixture client: %v", err)
	}
	if err := client.Login(); err != nil {
		_ = client.ForceClose()
		t.Fatalf("login action fixture client: %v", err)
	}
	return client
}

func (h actionIMAPHarness) append(t *testing.T, raw []byte) uint32 {
	return h.appendFor(t, actionIMAPUsername, actionIMAPPassword, "INBOX", raw)
}

func (h actionIMAPHarness) appendFor(t *testing.T, username, password, mailbox string, raw []byte) uint32 {
	t.Helper()
	client := h.clientFor(t, username, password)
	defer func() { _ = client.Close() }()
	uid, err := client.AppendMessage(mailbox, nil, time.Now().UTC(), raw)
	if err != nil {
		t.Fatalf("append action fixture to %s: %v", mailbox, err)
	}
	return uint32(uid)
}

func (h actionIMAPHarness) status(t *testing.T, mailbox string) *imapPkg.Mailbox {
	return h.statusFor(t, actionIMAPUsername, actionIMAPPassword, mailbox)
}

func (h actionIMAPHarness) statusFor(t *testing.T, username, password, mailbox string) *imapPkg.Mailbox {
	t.Helper()
	client := h.clientFor(t, username, password)
	defer func() { _ = client.Close() }()
	status, err := client.GetMailboxStatus(context.Background(), mailbox)
	if err != nil {
		t.Fatalf("status %s: %v", mailbox, err)
	}
	return status
}

func (h actionIMAPHarness) flags(t *testing.T, mailbox string, uid uint32) []goImap.Flag {
	return h.flagsFor(t, actionIMAPUsername, actionIMAPPassword, mailbox, uid)
}

func (h actionIMAPHarness) flagsFor(t *testing.T, username, password, mailbox string, uid uint32) []goImap.Flag {
	t.Helper()
	client := h.clientFor(t, username, password)
	defer func() { _ = client.Close() }()
	if _, err := client.SelectMailbox(context.Background(), mailbox); err != nil {
		t.Fatalf("select %s for flags: %v", mailbox, err)
	}
	uidSet := goImap.UIDSet{}
	uidSet.AddNum(goImap.UID(uid))
	fetch := client.RawClient().Fetch(uidSet, &goImap.FetchOptions{UID: true, Flags: true})
	msg := fetch.Next()
	if msg == nil {
		_ = fetch.Close()
		t.Fatalf("fetch flags for UID %d returned no message", uid)
	}
	var flags []goImap.Flag
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		if data, ok := item.(imapclient.FetchItemDataFlags); ok {
			flags = append(flags, data.Flags...)
		}
	}
	if err := fetch.Close(); err != nil {
		t.Fatalf("close flags fetch: %v", err)
	}
	return flags
}

func (h actionIMAPHarness) rawFor(t *testing.T, username, password, mailbox string, uid uint32) []byte {
	t.Helper()
	client := h.clientFor(t, username, password)
	defer func() { _ = client.Close() }()
	if _, err := client.SelectMailbox(context.Background(), mailbox); err != nil {
		t.Fatalf("select %s for raw fetch: %v", mailbox, err)
	}
	uidSet := goImap.UIDSet{}
	uidSet.AddNum(goImap.UID(uid))
	fetch := client.RawClient().Fetch(uidSet, &goImap.FetchOptions{
		BodySection: []*goImap.FetchItemBodySection{{Peek: true}},
	})
	msg := fetch.Next()
	if msg == nil {
		_ = fetch.Close()
		t.Fatalf("fetch raw for UID %d returned no message", uid)
	}
	var raw []byte
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		if data, ok := item.(imapclient.FetchItemDataBodySection); ok && data.Literal != nil {
			var err error
			raw, err = io.ReadAll(data.Literal)
			if err != nil {
				t.Fatalf("read raw for UID %d: %v", uid, err)
			}
		}
	}
	if err := fetch.Close(); err != nil {
		t.Fatalf("close raw fetch: %v", err)
	}
	if raw == nil {
		t.Fatalf("fetch raw for UID %d returned no body", uid)
	}
	return raw
}

type actionEventRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *actionEventRecorder) emit(_ context.Context, eventName string, _ ...interface{}) {
	r.mu.Lock()
	r.events = append(r.events, eventName)
	r.mu.Unlock()
}

func (r *actionEventRecorder) has(eventName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, recorded := range r.events {
		if recorded == eventName {
			return true
		}
	}
	return false
}

func (r *actionEventRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type publicActionFixture struct {
	harness  actionIMAPHarness
	app      *App
	account  *account.Account
	folders  map[folder.Type]*folder.Folder
	messages *message.Store
	events   *actionEventRecorder
}

func newPublicActionFixture(t *testing.T) *publicActionFixture {
	t.Helper()
	harness := startActionIMAPServer(t)
	fixtureClient := harness.client(t)
	if err := fixtureClient.RawClient().Create("Spam", nil).Wait(); err != nil {
		_ = fixtureClient.ForceClose()
		t.Fatalf("create Spam mailbox: %v", err)
	}
	if err := fixtureClient.Close(); err != nil {
		t.Fatalf("close fixture mailbox client: %v", err)
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "public-actions.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	folderStore := folder.NewStore(db)
	folders := map[folder.Type]*folder.Folder{
		folder.TypeInbox: {
			ID: "public-actions-inbox", AccountID: acc.ID, Name: "Inbox", Path: "INBOX",
			Type: folder.TypeInbox, Subscribed: true,
		},
		folder.TypeArchive: {
			ID: "public-actions-archive", AccountID: acc.ID, Name: "Archive", Path: "Archive",
			Type: folder.TypeArchive, Subscribed: true,
		},
		folder.TypeDrafts: {
			ID: "public-actions-drafts", AccountID: acc.ID, Name: "Drafts", Path: "Drafts",
			Type: folder.TypeDrafts, Subscribed: true,
		},
		folder.TypeTrash: {
			ID: "public-actions-trash", AccountID: acc.ID, Name: "Trash", Path: "Trash",
			Type: folder.TypeTrash, Subscribed: true,
		},
		folder.TypeSpam: {
			ID: "public-actions-spam", AccountID: acc.ID, Name: "Spam", Path: "Spam",
			Type: folder.TypeSpam, Subscribed: true,
		},
	}
	for _, item := range folders {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 3
	poolConfig.WaiterTimeout = 2 * time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != acc.ID {
			return nil, errors.New("credentials unavailable")
		}
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)

	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	contactStore := contact.NewStore(db.DB)
	engine := mailSync.NewEngine(pool, accountStore, folderStore, messageStore, attachmentStore, contactStore)
	events := &actionEventRecorder{}
	a := &App{
		ctx:              context.Background(),
		eventsEmit:       events.emit,
		db:               db,
		accountStore:     accountStore,
		activityLogStore: activitylog.NewStore(db),
		folderStore:      folderStore,
		messageStore:     messageStore,
		attachmentStore:  attachmentStore,
		contactStore:     contactStore,
		imapPool:         pool,
		syncEngine:       engine,
		undoStack:        undo.NewStack(50, 30*time.Second),
	}
	a.initBackupBridge()
	a.initSyncBridge()
	a.initDraftBridge()

	return &publicActionFixture{
		harness:  harness,
		app:      a,
		account:  acc,
		folders:  folders,
		messages: messageStore,
		events:   events,
	}
}

func (f *publicActionFixture) appendLocalMessage(t *testing.T, folderType folder.Type, id, rfcMessageID string) uint32 {
	t.Helper()
	folderObj := f.folders[folderType]
	raw := []byte(fmt.Sprintf(
		"From: sender@example.com\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <%s>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\n\r\n%s body\r\n",
		actionIMAPUsername, id, rfcMessageID, id,
	))
	uid := f.harness.appendFor(t, actionIMAPUsername, actionIMAPPassword, folderObj.Path, raw)
	if err := f.messages.Create(&message.Message{
		ID: id, AccountID: f.account.ID, FolderID: folderObj.ID, UID: uid,
		MessageID: rfcMessageID, Subject: id, FromEmail: "sender@example.com",
		Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(), Size: len(raw),
	}); err != nil {
		t.Fatalf("create local message %s: %v", id, err)
	}
	f.refreshFolderCounts(t, folderObj)
	return uid
}

func (f *publicActionFixture) refreshFolderCounts(t *testing.T, folderObj *folder.Folder) {
	t.Helper()
	total, err := f.messages.CountByFolder(folderObj.ID)
	if err != nil {
		t.Fatalf("count folder %s: %v", folderObj.ID, err)
	}
	unread, err := f.messages.CountUnreadByFolder(folderObj.ID)
	if err != nil {
		t.Fatalf("count unread folder %s: %v", folderObj.ID, err)
	}
	if err := f.app.folderStore.UpdateCounts(folderObj.ID, total, unread); err != nil {
		t.Fatalf("update folder %s counts: %v", folderObj.ID, err)
	}
	folderObj.TotalCount = total
	folderObj.UnreadCount = unread
}

func waitForActionCondition(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

func containsActionFlag(flags []goImap.Flag, want goImap.Flag) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

func assertStoredMessageFlags(t *testing.T, store *message.Store, id string, isRead, isStarred bool) {
	t.Helper()
	stored, err := store.Get(id)
	if err != nil || stored == nil {
		t.Fatalf("message %s = (%#v, %v)", id, stored, err)
	}
	if stored.IsRead != isRead || stored.IsStarred != isStarred {
		t.Fatalf("message %s flags = read:%v starred:%v, want read:%v starred:%v", id, stored.IsRead, stored.IsStarred, isRead, isStarred)
	}
}

func TestPublicMessageActionsSynchronizeLocalAndRemoteState(t *testing.T) {
	fixture := newPublicActionFixture(t)
	inbox := fixture.folders[folder.TypeInbox]
	archive := fixture.folders[folder.TypeArchive]
	trash := fixture.folders[folder.TypeTrash]
	spam := fixture.folders[folder.TypeSpam]

	firstUID := fixture.appendLocalMessage(t, folder.TypeInbox, "public-flag-1", "public-flag-1@example.com")
	secondUID := fixture.appendLocalMessage(t, folder.TypeInbox, "public-flag-2", "public-flag-2@example.com")
	if err := fixture.app.MarkAsRead([]string{"public-flag-1"}); err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}
	assertStoredMessageFlags(t, fixture.messages, "public-flag-1", true, false)
	waitForActionCondition(t, "read flag on IMAP", func() bool {
		return containsActionFlag(fixture.harness.flags(t, "INBOX", firstUID), goImap.FlagSeen)
	})

	if err := fixture.app.MarkAllFolderMessagesAsRead(inbox.ID); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsRead: %v", err)
	}
	assertStoredMessageFlags(t, fixture.messages, "public-flag-2", true, false)
	waitForActionCondition(t, "bulk read flag on IMAP", func() bool {
		return containsActionFlag(fixture.harness.flags(t, "INBOX", secondUID), goImap.FlagSeen)
	})
	if err := fixture.app.MarkAllFolderMessagesAsRead(inbox.ID); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsRead(no-op): %v", err)
	}

	if err := fixture.app.MarkAllFolderMessagesAsUnread(inbox.ID); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsUnread: %v", err)
	}
	assertStoredMessageFlags(t, fixture.messages, "public-flag-1", false, false)
	assertStoredMessageFlags(t, fixture.messages, "public-flag-2", false, false)
	waitForActionCondition(t, "unread flags removed on IMAP", func() bool {
		return !containsActionFlag(fixture.harness.flags(t, "INBOX", firstUID), goImap.FlagSeen) &&
			!containsActionFlag(fixture.harness.flags(t, "INBOX", secondUID), goImap.FlagSeen)
	})
	if err := fixture.app.MarkAllFolderMessagesAsUnread(inbox.ID); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsUnread(no-op): %v", err)
	}

	if err := fixture.app.Star([]string{"public-flag-1", "public-flag-2"}); err != nil {
		t.Fatalf("Star: %v", err)
	}
	assertStoredMessageFlags(t, fixture.messages, "public-flag-1", false, true)
	waitForActionCondition(t, "star flags on IMAP", func() bool {
		return containsActionFlag(fixture.harness.flags(t, "INBOX", firstUID), goImap.FlagFlagged) &&
			containsActionFlag(fixture.harness.flags(t, "INBOX", secondUID), goImap.FlagFlagged)
	})
	if err := fixture.app.Unstar([]string{"public-flag-1", "public-flag-2"}); err != nil {
		t.Fatalf("Unstar: %v", err)
	}
	assertStoredMessageFlags(t, fixture.messages, "public-flag-2", false, false)
	waitForActionCondition(t, "star flags removed on IMAP", func() bool {
		return !containsActionFlag(fixture.harness.flags(t, "INBOX", firstUID), goImap.FlagFlagged) &&
			!containsActionFlag(fixture.harness.flags(t, "INBOX", secondUID), goImap.FlagFlagged)
	})
	if !fixture.events.has("messages:readChanged") || !fixture.events.has("folders:countsChanged") {
		t.Fatalf("flag actions did not emit observable events: %#v", fixture.events.snapshot())
	}

	fixture.appendLocalMessage(t, folder.TypeInbox, "public-copy", "public-copy@example.com")
	if err := fixture.app.CopyToFolder([]string{"public-copy"}, archive.ID); err != nil {
		t.Fatalf("CopyToFolder: %v", err)
	}
	waitForActionCondition(t, "copied message reconciliation", func() bool {
		remote := fixture.harness.status(t, "Archive")
		local, err := fixture.messages.CountByFolder(archive.ID)
		return err == nil && remote.Messages == 1 && local == 1
	})
	if original, err := fixture.messages.Get("public-copy"); err != nil || original == nil || original.FolderID != inbox.ID {
		t.Fatalf("copied source message = (%#v, %v), want original in inbox", original, err)
	}

	fixture.appendLocalMessage(t, folder.TypeInbox, "public-move", "public-move@example.com")
	if err := fixture.app.MoveToFolder([]string{"public-move"}, archive.ID); err != nil {
		t.Fatalf("MoveToFolder: %v", err)
	}
	if fixture.app.undoStack.Peek() == nil {
		t.Fatal("MoveToFolder did not create an undo command")
	}
	waitForActionCondition(t, "moved message reconciliation", func() bool {
		remoteInbox := fixture.harness.status(t, "INBOX")
		remoteArchive := fixture.harness.status(t, "Archive")
		local, err := fixture.messages.CountByFolder(archive.ID)
		return err == nil && remoteInbox.Messages == 3 && remoteArchive.Messages == 2 && local == 2
	})
	if movedSource, err := fixture.messages.Get("public-move"); err != nil || movedSource != nil {
		t.Fatalf("temporary moved source = (%#v, %v), want reconciled row", movedSource, err)
	}

	fixture.appendLocalMessage(t, folder.TypeInbox, "public-archive", "public-archive@example.com")
	if err := fixture.app.Archive([]string{"public-archive"}); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	waitForActionCondition(t, "archive action reconciliation", func() bool {
		remote := fixture.harness.status(t, "Archive")
		local, err := fixture.messages.CountByFolder(archive.ID)
		return err == nil && remote.Messages == 3 && local == 3
	})

	fixture.appendLocalMessage(t, folder.TypeInbox, "public-trash", "public-trash@example.com")
	moved, err := fixture.app.Trash([]string{"public-trash"})
	if err != nil || !moved {
		t.Fatalf("Trash = (%v, %v), want moved", moved, err)
	}
	waitForActionCondition(t, "trash action reconciliation", func() bool {
		remote := fixture.harness.status(t, "Trash")
		local, countErr := fixture.messages.CountByFolder(trash.ID)
		return countErr == nil && remote.Messages == 1 && local == 1
	})

	fixture.appendLocalMessage(t, folder.TypeInbox, "public-spam", "public-spam@example.com")
	moved, err = fixture.app.MarkAsSpam([]string{"public-spam"})
	if err != nil || !moved {
		t.Fatalf("MarkAsSpam = (%v, %v), want moved", moved, err)
	}
	var spamIDs []string
	waitForActionCondition(t, "spam action reconciliation", func() bool {
		var findErr error
		spamIDs, findErr = fixture.messages.GetIDsByMessageIDs(fixture.account.ID, spam.ID, []string{"public-spam@example.com"})
		return findErr == nil && len(spamIDs) == 1 && fixture.harness.status(t, "Spam").Messages == 1
	})
	if err := fixture.app.MarkAsNotSpam(spamIDs); err != nil {
		t.Fatalf("MarkAsNotSpam: %v", err)
	}
	waitForActionCondition(t, "not-spam action reconciliation", func() bool {
		return fixture.harness.status(t, "Spam").Messages == 0
	})

	trashIDs, err := fixture.messages.GetIDsByMessageIDs(fixture.account.ID, trash.ID, []string{"public-trash@example.com"})
	if err != nil || len(trashIDs) != 1 {
		t.Fatalf("trash message IDs = %v, %v", trashIDs, err)
	}
	if err := fixture.app.DeletePermanently(trashIDs); err != nil {
		t.Fatalf("DeletePermanently: %v", err)
	}
	if deleted, err := fixture.messages.Get(trashIDs[0]); err != nil || deleted != nil {
		t.Fatalf("permanently deleted local message = (%#v, %v)", deleted, err)
	}
	waitForActionCondition(t, "permanent delete on IMAP", func() bool {
		return fixture.harness.status(t, "Trash").Messages == 0
	})

	fixture.appendLocalMessage(t, folder.TypeTrash, "public-empty-trash-1", "public-empty-trash-1@example.com")
	fixture.appendLocalMessage(t, folder.TypeTrash, "public-empty-trash-2", "public-empty-trash-2@example.com")
	if err := fixture.app.EmptyTrash("wrong-account", trash.ID); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("EmptyTrash(cross-account) error = %v", err)
	}
	if count, err := fixture.messages.CountByFolder(trash.ID); err != nil || count != 2 {
		t.Fatalf("trash count after rejected empty = %d, %v", count, err)
	}
	if err := fixture.app.EmptyTrash(fixture.account.ID, trash.ID); err != nil {
		t.Fatalf("EmptyTrash: %v", err)
	}
	waitForActionCondition(t, "empty trash on IMAP", func() bool {
		local, countErr := fixture.messages.CountByFolder(trash.ID)
		return countErr == nil && local == 0 && fixture.harness.status(t, "Trash").Messages == 0
	})
	if err := fixture.app.EmptyTrash(fixture.account.ID, trash.ID); err != nil {
		t.Fatalf("EmptyTrash(no-op): %v", err)
	}

	remoteOnly := []byte("From: drafts@example.com\r\nTo: actions@example.com\r\nSubject: remote draft\r\nMessage-ID: <public-remote-draft@example.com>\r\n\r\ndraft body\r\n")
	fixture.harness.appendFor(t, actionIMAPUsername, actionIMAPPassword, "Drafts", remoteOnly)
	if err := fixture.app.SyncFolder(fixture.account.ID, fixture.folders[folder.TypeDrafts].ID); err != nil {
		t.Fatalf("SyncFolder: %v", err)
	}
	if count, err := fixture.messages.CountByFolder(fixture.folders[folder.TypeDrafts].ID); err != nil || count != 1 {
		t.Fatalf("draft count after SyncFolder = %d, %v", count, err)
	}
	if !fixture.events.has("messages:moved") || !fixture.events.has("messages:deleted") ||
		!fixture.events.has("folder:synced") || !fixture.events.has("activity-log:created") {
		t.Fatalf("public actions did not emit complete observable events: %#v", fixture.events.snapshot())
	}
}

func TestActionIMAPHelpersAgainstMemoryServer(t *testing.T) {
	harness := startActionIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "actions.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	folderStore := folder.NewStore(db)
	source := &folder.Folder{ID: "actions-inbox", AccountID: acc.ID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox, Subscribed: true}
	archive := &folder.Folder{ID: "actions-archive", AccountID: acc.ID, Name: "Archive", Path: "Archive", Type: folder.TypeArchive, Subscribed: true}
	trash := &folder.Folder{ID: "actions-trash", AccountID: acc.ID, Name: "Trash", Path: "Trash", Type: folder.TypeTrash, Subscribed: true}
	for _, item := range []*folder.Folder{source, archive, trash} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 1
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != acc.ID {
			return nil, errors.New("credentials unavailable")
		}
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)
	messageStore := message.NewStore(db)
	a := &App{
		ctx:          context.Background(),
		accountStore: accountStore,
		folderStore:  folderStore,
		messageStore: messageStore,
		imapPool:     pool,
	}

	raw := []byte("From: sender@example.com\r\nTo: actions@example.com\r\nSubject: action message\r\nMessage-ID: <action-1@example.com>\r\n\r\nbody\r\n")
	uid1 := harness.append(t, raw)
	local := &message.Message{
		ID: "action-message-1", AccountID: acc.ID, FolderID: source.ID, UID: uid1,
		MessageID: "action-1@example.com", Subject: "action message", Date: time.Now().UTC(), Size: len(raw),
	}
	if err := messageStore.Create(local); err != nil {
		t.Fatalf("create local message: %v", err)
	}

	var calls int
	if err := a.withIMAPRetry(acc.ID, func(conn *imapPkg.Client) error {
		calls++
		_, err := conn.ListMailboxes()
		return err
	}); err != nil || calls != 1 {
		t.Fatalf("withIMAPRetry(success) calls = %d, err = %v", calls, err)
	}
	wantErr := errors.New("operation rejected")
	if err := a.withIMAPRetry(acc.ID, func(*imapPkg.Client) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("withIMAPRetry(non-connection error) = %v", err)
	}
	calls = 0
	if err := a.withIMAPRetry(acc.ID, func(conn *imapPkg.Client) error {
		calls++
		if calls == 1 {
			return errors.New("EOF")
		}
		_, err := conn.ListMailboxes()
		return err
	}); err != nil || calls != 2 {
		t.Fatalf("withIMAPRetry(retry) calls = %d, err = %v", calls, err)
	}
	if err := a.withIMAPRetry("missing-account", func(*imapPkg.Client) error { return nil }); err == nil || !strings.Contains(err.Error(), "failed to get IMAP connection") {
		t.Fatalf("withIMAPRetry(missing account) error = %v", err)
	}

	if err := a.syncFlagsToIMAP([]*message.Message{local}, source.ID, "read", true); err != nil {
		t.Fatalf("sync read flag: %v", err)
	}
	if got := harness.flags(t, "INBOX", uid1); !reflect.DeepEqual(got, []goImap.Flag{goImap.FlagSeen}) {
		t.Fatalf("read flags = %v", got)
	}
	if err := a.syncFlagsToIMAP([]*message.Message{local}, source.ID, "read", false); err != nil {
		t.Fatalf("remove read flag: %v", err)
	}
	if got := harness.flags(t, "INBOX", uid1); len(got) != 0 {
		t.Fatalf("flags after unread = %v", got)
	}
	if err := a.syncFlagsToIMAP([]*message.Message{local}, source.ID, "starred", true); err != nil {
		t.Fatalf("sync starred flag: %v", err)
	}
	if got := harness.flags(t, "INBOX", uid1); !reflect.DeepEqual(got, []goImap.Flag{goImap.FlagFlagged}) {
		t.Fatalf("starred flags = %v", got)
	}

	if err := a.copyMessagesToIMAP([]*message.Message{local}, source.ID, archive); err != nil {
		t.Fatalf("copyMessagesToIMAP() error = %v", err)
	}
	if inbox, copied := harness.status(t, "INBOX"), harness.status(t, "Archive"); inbox.Messages != 1 || copied.Messages != 1 {
		t.Fatalf("counts after copy: INBOX=%+v Archive=%+v", inbox, copied)
	}
	if err := a.moveMessagesToIMAP([]*message.Message{local}, source.ID, trash); err != nil {
		t.Fatalf("moveMessagesToIMAP() error = %v", err)
	}
	if inbox, moved := harness.status(t, "INBOX"), harness.status(t, "Trash"); inbox.Messages != 0 || moved.Messages != 1 {
		t.Fatalf("counts after move: INBOX=%+v Trash=%+v", inbox, moved)
	}

	uid2 := harness.append(t, []byte("Subject: permanent delete\r\n\r\nbody\r\n"))
	toDelete := &message.Message{ID: "action-message-2", AccountID: acc.ID, FolderID: source.ID, UID: uid2, Date: time.Now().UTC()}
	if err := messageStore.Create(toDelete); err != nil {
		t.Fatalf("create delete fixture: %v", err)
	}
	ignoredZero := &message.Message{AccountID: acc.ID, FolderID: source.ID, UID: 0}
	ignoredTemp := &message.Message{AccountID: acc.ID, FolderID: source.ID, UID: ^uint32(0)}
	if err := a.deleteMessagesFromIMAP([]*message.Message{ignoredZero, ignoredTemp, toDelete}, source.ID); err != nil {
		t.Fatalf("deleteMessagesFromIMAP() error = %v", err)
	}
	if status := harness.status(t, "INBOX"); status.Messages != 0 {
		t.Fatalf("INBOX after permanent delete = %+v", status)
	}

	missingFolderMessage := &message.Message{AccountID: acc.ID, FolderID: "missing", UID: 1}
	if err := a.copyMessagesToIMAP([]*message.Message{missingFolderMessage}, "missing", archive); err == nil || !strings.Contains(err.Error(), "source folder not found") {
		t.Fatalf("copy from missing folder error = %v", err)
	}
	if err := a.moveMessagesToIMAP([]*message.Message{missingFolderMessage}, "missing", trash); err == nil || !strings.Contains(err.Error(), "source folder not found") {
		t.Fatalf("move from missing folder error = %v", err)
	}
	if err := a.deleteMessagesFromIMAP([]*message.Message{missingFolderMessage}, "missing"); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("delete from missing folder error = %v", err)
	}
	noSelect := &folder.Folder{ID: "actions-group", AccountID: acc.ID, Name: "Group", Path: "Group", Type: folder.TypeFolder, NoSelect: true}
	if err := folderStore.Create(noSelect); err != nil {
		t.Fatalf("create no-select folder: %v", err)
	}
	if err := a.syncFlagsToIMAP([]*message.Message{{AccountID: acc.ID, FolderID: noSelect.ID, UID: 1}}, noSelect.ID, "read", true); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("sync flags for no-select folder error = %v", err)
	}
}

func TestGmailTrashRemotePrimitivesAgainstMemoryServer(t *testing.T) {
	harness := startActionIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "gmail-trash-actions.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	if _, err := db.Exec(`UPDATE accounts SET imap_host = 'imap.gmail.com' WHERE id = ?`, acc.ID); err != nil {
		t.Fatalf("mark fixture as Gmail: %v", err)
	}
	folderStore := folder.NewStore(db)
	inbox := &folder.Folder{
		ID: "gmail-inbox", AccountID: acc.ID, Name: "Inbox", Path: "INBOX",
		Type: folder.TypeInbox, Subscribed: true, TotalCount: 2, UnreadCount: 2,
	}
	archive := &folder.Folder{
		ID: "gmail-archive", AccountID: acc.ID, Name: "Archive", Path: "Archive",
		Type: folder.TypeArchive, Subscribed: true, TotalCount: 1, UnreadCount: 1,
	}
	trash := &folder.Folder{
		ID: "gmail-trash", AccountID: acc.ID, Name: "Trash", Path: "Trash",
		Type: folder.TypeTrash, Subscribed: true,
	}
	for _, item := range []*folder.Folder{inbox, archive, trash} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	copyRaw := []byte("From: sender@example.com\r\nTo: actions@example.com\r\nSubject: labeled copy\r\nMessage-ID: <gmail-copy@example.com>\r\n\r\ncopy body\r\n")
	soleRaw := []byte("From: sender@example.com\r\nTo: actions@example.com\r\nSubject: sole copy\r\n\r\nsole body\r\n")
	copyInboxUID := harness.append(t, copyRaw)
	soleInboxUID := harness.append(t, soleRaw)
	copyArchiveUID := harness.appendFor(t, actionIMAPUsername, actionIMAPPassword, "Archive", copyRaw)

	messageStore := message.NewStore(db)
	fixtures := []*message.Message{
		{
			ID: "gmail-copy-inbox", AccountID: acc.ID, FolderID: inbox.ID,
			UID: copyInboxUID, MessageID: "gmail-copy@example.com", Subject: "labeled copy", Date: time.Now().UTC(),
		},
		{
			ID: "gmail-sole-inbox", AccountID: acc.ID, FolderID: inbox.ID,
			UID: soleInboxUID, Subject: "sole copy", Date: time.Now().UTC(),
		},
		{
			ID: "gmail-copy-archive", AccountID: acc.ID, FolderID: archive.ID,
			UID: copyArchiveUID, MessageID: "gmail-copy@example.com", Subject: "labeled copy", Date: time.Now().UTC(),
		},
	}
	for _, item := range fixtures {
		if err := messageStore.Create(item); err != nil {
			t.Fatalf("create message %s: %v", item.ID, err)
		}
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 1
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != acc.ID {
			return nil, errors.New("credentials unavailable")
		}
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)
	a := &App{
		ctx: context.Background(), db: db, accountStore: accountStore, folderStore: folderStore,
		messageStore: messageStore, imapPool: pool,
	}

	// Force the high-level dispatcher down its real Gmail partitioning path,
	// but stop before Wails event emission (which requires an application
	// lifecycle context unavailable to Go unit tests). This verifies that a
	// mixed selection reports a pending move and surfaces local-delete failure.
	if _, err := db.Exec(`
		CREATE TRIGGER block_gmail_action_delete
		BEFORE DELETE ON messages
		BEGIN
			SELECT RAISE(ABORT, 'blocked Gmail action delete');
		END
	`); err != nil {
		t.Fatalf("create delete guard: %v", err)
	}
	moved, err := a.gmailTrashOrSpam([]string{"gmail-copy-inbox", "gmail-sole-inbox"}, trash)
	if !moved || err == nil || !strings.Contains(err.Error(), "failed to delete messages locally") {
		t.Fatalf("guarded mixed gmailTrashOrSpam = (%v, %v)", moved, err)
	}
	if _, err := db.Exec(`DROP TRIGGER block_gmail_action_delete`); err != nil {
		t.Fatalf("drop delete guard: %v", err)
	}

	// Exercise the two protocol operations selected above against a real IMAP
	// server: copies lose only the current label, while sole messages are
	// copied to Trash and expunged from the source mailbox.
	if err := a.removeFromIMAPFolder([]*message.Message{fixtures[0]}, inbox.ID); err != nil {
		t.Fatalf("remove Gmail label over IMAP: %v", err)
	}
	if err := a.moveMessagesToIMAP([]*message.Message{fixtures[1]}, inbox.ID, trash); err != nil {
		t.Fatalf("move sole Gmail copy over IMAP: %v", err)
	}
	if inboxStatus, archiveStatus, trashStatus := harness.status(t, "INBOX"), harness.status(t, "Archive"), harness.status(t, "Trash"); inboxStatus.Messages != 0 || archiveStatus.Messages != 1 || trashStatus.Messages != 1 {
		t.Fatalf("remote counts after Gmail actions: INBOX=%+v Archive=%+v Trash=%+v", inboxStatus, archiveStatus, trashStatus)
	}
	if got, err := messageStore.Get("gmail-copy-inbox"); err != nil || got == nil {
		t.Fatalf("protocol helper unexpectedly mutated local copy = (%#v, %v)", got, err)
	}
	if got, err := messageStore.Get("gmail-copy-archive"); err != nil || got == nil {
		t.Fatalf("archive copy = (%#v, %v), want preserved", got, err)
	}
}

func TestCrossAccountAppendStreamsRawMessagesAgainstMemoryServer(t *testing.T) {
	harness := startActionIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "cross-account-actions.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	sourceAccount := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	destinationAccount := createOfflineCacheTestAccount(t, accountStore, actionIMAPDestinationUsername)
	folderStore := folder.NewStore(db)
	sourceFolder := &folder.Folder{
		ID: "cross-account-inbox", AccountID: sourceAccount.ID,
		Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox, Subscribed: true,
	}
	destinationFolder := &folder.Folder{
		ID: "cross-account-archive", AccountID: destinationAccount.ID,
		Name: "Archive", Path: "Archive", Type: folder.TypeArchive, Subscribed: true,
	}
	for _, item := range []*folder.Folder{sourceFolder, destinationFolder} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 2
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		var config imapPkg.ClientConfig
		switch accountID {
		case sourceAccount.ID:
			config = harness.configFor(actionIMAPUsername, actionIMAPPassword)
		case destinationAccount.ID:
			config = harness.configFor(actionIMAPDestinationUsername, actionIMAPDestinationPassword)
		default:
			return nil, errors.New("credentials unavailable")
		}
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)
	messageStore := message.NewStore(db)
	engine := mailSync.NewEngine(pool, accountStore, folderStore, messageStore, nil, nil)
	a := &App{
		ctx: context.Background(), accountStore: accountStore, folderStore: folderStore,
		messageStore: messageStore, imapPool: pool, syncEngine: engine,
	}

	streamedRaw := []byte("From: source@example.com\r\nTo: destination@example.com\r\nSubject: streamed copy\r\nMessage-ID: <cross-stream@example.com>\r\n\r\nstreamed body\r\n")
	tempRaw := []byte("From: source@example.com\r\nTo: destination@example.com\r\nSubject: temp copy\r\nMessage-ID: <cross-temp@example.com>\r\n\r\ntemp body\r\n")
	streamedUID := harness.append(t, streamedRaw)
	tempUID := harness.append(t, tempRaw)
	date := time.Now().UTC().Truncate(time.Second)
	streamedMessage := &message.Message{
		ID: "cross-stream", AccountID: sourceAccount.ID, FolderID: sourceFolder.ID,
		UID: streamedUID, Size: len(streamedRaw), Date: date,
		IsRead: true, IsStarred: true, IsAnswered: true, IsDraft: true,
	}
	tempMessage := &message.Message{
		ID: "cross-temp", AccountID: sourceAccount.ID, FolderID: sourceFolder.ID,
		UID: tempUID, Size: 0, Date: date,
	}

	if err := a.copyMessagesAcrossAccounts([]*message.Message{streamedMessage, tempMessage}, destinationFolder); err != nil {
		t.Fatalf("copyMessagesAcrossAccounts() error = %v", err)
	}
	if status := harness.statusFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword, "Archive"); status.Messages != 2 {
		t.Fatalf("destination status after copy = %+v", status)
	}
	if got := harness.rawFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword, "Archive", 1); !bytes.Equal(got, streamedRaw) {
		t.Fatalf("streamed destination raw differs:\n got %q\nwant %q", got, streamedRaw)
	}
	if got := harness.rawFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword, "Archive", 2); !bytes.Equal(got, tempRaw) {
		t.Fatalf("temp destination raw differs:\n got %q\nwant %q", got, tempRaw)
	}
	wantFlags := map[goImap.Flag]bool{
		goImap.FlagSeen: true, goImap.FlagFlagged: true,
		goImap.FlagAnswered: true, goImap.FlagDraft: true,
	}
	gotFlags := harness.flagsFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword, "Archive", 1)
	if len(gotFlags) != len(wantFlags) {
		t.Fatalf("destination flags = %v", gotFlags)
	}
	for _, flag := range gotFlags {
		if !wantFlags[flag] {
			t.Fatalf("unexpected destination flag %q in %v", flag, gotFlags)
		}
	}

	if err := a.copyMessagesAcrossAccounts(nil, destinationFolder); err != nil {
		t.Fatalf("empty cross-account copy error = %v", err)
	}
	noSelect := *destinationFolder
	noSelect.NoSelect = true
	if err := a.copyMessagesAcrossAccounts([]*message.Message{streamedMessage}, &noSelect); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("no-select destination error = %v", err)
	}
	missingDestination := *destinationFolder
	missingDestination.AccountID = "missing-destination"
	if err := a.copyMessagesAcrossAccounts([]*message.Message{streamedMessage}, &missingDestination); err == nil || !strings.Contains(err.Error(), "failed to get IMAP connection") {
		t.Fatalf("missing destination account error = %v", err)
	}
	missingSource := *streamedMessage
	missingSource.AccountID = "missing-source"
	destinationClient := harness.clientFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword)
	if err := a.appendMessageAcrossAccounts(destinationClient, &missingSource, destinationFolder); err == nil || !strings.Contains(err.Error(), "credentials unavailable") {
		t.Fatalf("missing source account error = %v", err)
	}
	_ = destinationClient.ForceClose()
	missingTempSource := *tempMessage
	missingTempSource.UID = 999
	destinationClient = harness.clientFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword)
	if err := a.appendMessageAcrossAccounts(destinationClient, &missingTempSource, destinationFolder); err == nil || !strings.Contains(err.Error(), "failed to stream raw message from source") {
		t.Fatalf("missing temp-source UID error = %v", err)
	}
	_ = destinationClient.ForceClose()
	wrongSize := *streamedMessage
	wrongSize.Size++
	destinationClient = harness.clientFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword)
	if err := a.appendMessageAcrossAccounts(destinationClient, &wrongSize, destinationFolder); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("source size mismatch error = %v", err)
	}
	_ = destinationClient.ForceClose()
	if status := harness.statusFor(t, actionIMAPDestinationUsername, actionIMAPDestinationPassword, "Archive"); status.Messages != 2 {
		t.Fatalf("failed copies changed destination = %+v", status)
	}
}

func TestCompleteSyncMessageSourceBodyAndAttachmentFlowsAgainstMemoryIMAP(t *testing.T) {
	fixture := newPublicActionFixture(t)
	fixture.app.settingsStore = settings.NewStore(fixture.app.db)
	fixture.app.paths = &platform.Paths{Data: t.TempDir()}

	remoteOnly := []byte("From: remote@example.com\r\nTo: actions@example.com\r\nSubject: remote only\r\nMessage-ID: <remote-only@example.com>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\n\r\nremote body\r\n")
	fixture.harness.appendFor(t, actionIMAPUsername, actionIMAPPassword, "Drafts", remoteOnly)
	if err := fixture.app.SyncAccountComplete(fixture.account.ID); err != nil {
		t.Fatalf("SyncAccountComplete: %v", err)
	}
	drafts, err := fixture.messages.ListByFolder(fixture.folders[folder.TypeDrafts].ID, 0, 20)
	if err != nil || len(drafts) != 1 || drafts[0].Subject != "remote only" {
		t.Fatalf("synced drafts = (%#v, %v)", drafts, err)
	}
	if !fixture.events.has("folder:synced") || !fixture.events.has("folders:countsChanged") {
		t.Fatalf("complete-sync events = %v", fixture.events.snapshot())
	}
	if err := fixture.app.SyncAllComplete(); err != nil {
		t.Fatalf("SyncAllComplete: %v", err)
	}
	if err := fixture.app.SyncAccountComplete("missing-account"); err == nil || !strings.Contains(err.Error(), "folder sync failed") {
		t.Fatalf("missing-account complete sync error = %v", err)
	}

	raw := []byte("From: sender@example.com\r\nTo: actions@example.com\r\nSubject: attachment message\r\nMessage-ID: <attachment-message@example.com>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=fixture\r\n\r\n--fixture\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nmail body\r\n--fixture\r\nContent-Type: text/plain; name=report.txt\r\nContent-Disposition: attachment; filename=report.txt\r\nContent-Transfer-Encoding: base64\r\n\r\nYXR0YWNobWVudCBjb250ZW50\r\n--fixture--\r\n")
	uid := fixture.harness.append(t, raw)
	msg := &message.Message{
		ID: "attachment-message", AccountID: fixture.account.ID,
		FolderID: fixture.folders[folder.TypeInbox].ID, UID: uid,
		MessageID: "attachment-message@example.com", Subject: "attachment message",
		FromEmail: "sender@example.com", Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(), Size: len(raw),
	}
	if err := fixture.messages.Create(msg); err != nil {
		t.Fatalf("create attachment message: %v", err)
	}
	attachment := &message.Attachment{
		ID: "download-attachment", MessageID: msg.ID, Filename: "report.txt",
		ContentType: "text/plain", Size: len("attachment content"),
	}
	if err := fixture.app.attachmentStore.Create(attachment); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	source, err := fixture.app.GetMessageSource(msg.ID)
	if err != nil || source.TooLarge || !strings.Contains(source.Content, "attachment message") || source.Size != int64(len(raw)) {
		t.Fatalf("GetMessageSource = (%#v, %v)", source, err)
	}
	if _, err := fixture.app.GetMessageSource("missing-message"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("missing message source error = %v", err)
	}

	updated, err := fixture.app.FetchMessageBody(msg.ID)
	if err != nil || updated == nil || !updated.BodyFetched || !strings.Contains(updated.BodyText, "mail body") {
		t.Fatalf("FetchMessageBody = (%#v, %v)", updated, err)
	}
	if cached, err := fixture.app.FetchMessageBody(msg.ID); err != nil || cached == nil || !cached.BodyFetched {
		t.Fatalf("cached FetchMessageBody = (%#v, %v)", cached, err)
	}
	if _, err := fixture.app.FetchMessageBody("missing-message"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("missing message body error = %v", err)
	}
	storedAttachments, err := fixture.app.attachmentStore.GetByMessage(msg.ID)
	if err != nil {
		t.Fatalf("list extracted attachments: %v", err)
	}
	downloadAttachmentID := ""
	for _, stored := range storedAttachments {
		if stored.Filename == "report.txt" {
			downloadAttachmentID = stored.ID
			break
		}
	}
	if downloadAttachmentID == "" {
		t.Fatalf("extracted attachments = %#v, missing report.txt", storedAttachments)
	}

	customPath := filepath.Join(t.TempDir(), "custom-report.txt")
	downloaded, err := fixture.app.downloadAttachment(downloadAttachmentID, customPath)
	if err != nil || downloaded != customPath {
		t.Fatalf("custom DownloadAttachment = (%q, %v)", downloaded, err)
	}
	content, err := os.ReadFile(customPath)
	if err != nil || string(content) != "attachment content" {
		t.Fatalf("downloaded attachment = (%q, %v)", content, err)
	}
	defaultPath, err := fixture.app.downloadAttachment(downloadAttachmentID, "")
	if err != nil || defaultPath == "" {
		t.Fatalf("default DownloadAttachment = (%q, %v)", defaultPath, err)
	}
	if cachedPath, err := fixture.app.downloadAttachment(downloadAttachmentID, ""); err != nil || cachedPath != defaultPath {
		t.Fatalf("cached DownloadAttachment = (%q, %v), want %q", cachedPath, err, defaultPath)
	}

	var openedPath string
	var revealed bool
	fixture.app.openSystemPath = func(path string, reveal bool) error {
		openedPath = path
		revealed = reveal
		return nil
	}
	if err := fixture.app.OpenAttachment(downloadAttachmentID); err != nil || openedPath != defaultPath || revealed {
		t.Fatalf("OpenAttachment = (%q, reveal %v, %v), want cached attachment", openedPath, revealed, err)
	}
	if err := fixture.app.OpenFile(defaultPath); err != nil || openedPath != defaultPath || revealed {
		t.Fatalf("OpenFile = (%q, reveal %v, %v)", openedPath, revealed, err)
	}
	if err := fixture.app.OpenFolder(defaultPath); err != nil || openedPath != defaultPath || !revealed {
		t.Fatalf("OpenFolder = (%q, reveal %v, %v)", openedPath, revealed, err)
	}
	if err := fixture.app.OpenFile(filepath.Join(t.TempDir(), "outside.txt")); err == nil {
		t.Fatal("OpenFile accepted a path outside application and download roots")
	}

	fixture.app.saveFileDialog = func(defaultDirectory, defaultFilename, title string) (string, error) {
		if defaultFilename != "report.txt" || title != "Save Attachment" || defaultDirectory == "" {
			t.Fatalf("save dialog options = %q, %q, %q", defaultDirectory, defaultFilename, title)
		}
		return "", nil
	}
	if saved, err := fixture.app.SaveAttachmentAs(downloadAttachmentID); err != nil || saved != "" {
		t.Fatalf("cancelled SaveAttachmentAs = (%q, %v)", saved, err)
	}
	saveAsPath := filepath.Join(t.TempDir(), "saved-as.txt")
	fixture.app.saveFileDialog = func(_, _, _ string) (string, error) { return saveAsPath, nil }
	if saved, err := fixture.app.SaveAttachmentAs(downloadAttachmentID); err != nil || saved != saveAsPath {
		t.Fatalf("SaveAttachmentAs = (%q, %v)", saved, err)
	}
	if content, err := os.ReadFile(saveAsPath); err != nil || string(content) != "attachment content" {
		t.Fatalf("saved-as attachment = (%q, %v)", content, err)
	}
	fixture.app.saveFileDialog = func(_, _, _ string) (string, error) {
		return "", errors.New("synthetic save dialog failure")
	}
	if _, err := fixture.app.SaveAttachmentAs(downloadAttachmentID); err == nil || !strings.Contains(err.Error(), "save dialog") {
		t.Fatalf("SaveAttachmentAs dialog error = %v", err)
	}

	fixture.app.openDirectoryDialog = func(defaultDirectory, title string) (string, error) {
		if defaultDirectory == "" || title != "Save All Attachments" {
			t.Fatalf("directory dialog options = %q, %q", defaultDirectory, title)
		}
		return "", nil
	}
	if saved, err := fixture.app.SaveAllAttachments(msg.ID); err != nil || saved != "" {
		t.Fatalf("cancelled SaveAllAttachments = (%q, %v)", saved, err)
	}
	allDirectory := t.TempDir()
	fixture.app.openDirectoryDialog = func(_, _ string) (string, error) { return allDirectory, nil }
	if saved, err := fixture.app.SaveAllAttachments(msg.ID); err != nil || saved != allDirectory {
		t.Fatalf("SaveAllAttachments = (%q, %v)", saved, err)
	}
	if matches, err := filepath.Glob(filepath.Join(allDirectory, "report*.txt")); err != nil || len(matches) == 0 {
		t.Fatalf("saved attachment files = (%v, %v)", matches, err)
	}
	fixture.app.openDirectoryDialog = func(_, _ string) (string, error) {
		return "", errors.New("synthetic directory dialog failure")
	}
	if _, err := fixture.app.SaveAllAttachments(msg.ID); err == nil || !strings.Contains(err.Error(), "folder dialog") {
		t.Fatalf("SaveAllAttachments dialog error = %v", err)
	}
	if _, err := fixture.app.downloadAttachment("missing-attachment", ""); err == nil || !strings.Contains(err.Error(), "attachment not found") {
		t.Fatalf("missing attachment error = %v", err)
	}

	if _, err := fixture.app.db.Exec(`UPDATE messages SET size = ? WHERE id = ?`, maxInlineMessageSourceBytes+1, msg.ID); err != nil {
		t.Fatalf("mark source large: %v", err)
	}
	large, err := fixture.app.GetMessageSource(msg.ID)
	if err != nil || !large.TooLarge || large.FilePath == "" || large.Size != int64(len(raw)) {
		t.Fatalf("large GetMessageSource = (%#v, %v)", large, err)
	}
	if _, err := os.Stat(large.FilePath); err != nil {
		t.Fatalf("large source path: %v", err)
	}
}

func TestCompleteSyncKeepsBackgroundBodyFetchAliveAfterCoordinatorReturns(t *testing.T) {
	fixture := newPublicActionFixture(t)
	fixture.app.syncCoordinator = mailSync.NewCoordinator()
	if _, err := fixture.app.db.Exec(`UPDATE accounts SET body_download_policy = 'all' WHERE id = ?`, fixture.account.ID); err != nil {
		t.Fatalf("enable background body download: %v", err)
	}

	raw := []byte("From: remote@example.com\r\nTo: actions@example.com\r\nSubject: detached body fetch\r\nMessage-ID: <detached-body-fetch@example.com>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\n\r\nbody fetched after header coordinator returns\r\n")
	uid := fixture.harness.append(t, raw)
	if err := fixture.app.SyncAccountComplete(fixture.account.ID); err != nil {
		t.Fatalf("SyncAccountComplete() error = %v", err)
	}

	waitForActionCondition(t, "detached background body fetch", func() bool {
		stored, err := fixture.messages.GetByUID(fixture.folders[folder.TypeInbox].ID, uid)
		return err == nil && stored != nil && stored.BodyFetched && strings.Contains(stored.BodyText, "coordinator returns")
	})
	if !fixture.events.has("folder:synced") {
		t.Fatalf("background body completion events = %v", fixture.events.snapshot())
	}
}

func TestEmailBackupExportsSkipsAndClassifiesUnavailableMemoryIMAPMessages(t *testing.T) {
	fixture := newPublicActionFixture(t)
	fixture.app.settingsStore = settings.NewStore(fixture.app.db)

	raw := []byte("From: backup@example.com\r\nTo: actions@example.com\r\nSubject: backed up message\r\nMessage-ID: <backed-up@example.com>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\n\r\nbackup body\r\n")
	uid := fixture.harness.append(t, raw)
	if err := fixture.messages.Create(&message.Message{
		ID: "backup-available", AccountID: fixture.account.ID,
		FolderID: fixture.folders[folder.TypeInbox].ID, UID: uid,
		MessageID: "backed-up@example.com", Subject: "backed up message",
		FromEmail: "backup@example.com", Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(), Size: len(raw),
	}); err != nil {
		t.Fatalf("create available backup message: %v", err)
	}
	if err := fixture.messages.Create(&message.Message{
		ID: "backup-missing", AccountID: fixture.account.ID,
		FolderID: fixture.folders[folder.TypeInbox].ID, UID: 999,
		MessageID: "missing-backup@example.com", Subject: "missing remote message",
		FromEmail: "backup@example.com", Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create missing backup message: %v", err)
	}
	hierarchy := &folder.Folder{
		ID: "backup-hierarchy", AccountID: fixture.account.ID,
		Name: "Hierarchy", Path: "Hierarchy", Type: folder.TypeFolder, NoSelect: true,
	}
	if err := fixture.app.folderStore.Create(hierarchy); err != nil {
		t.Fatalf("create hierarchy folder: %v", err)
	}
	if err := fixture.messages.Create(&message.Message{
		ID: "backup-unavailable", AccountID: fixture.account.ID, FolderID: hierarchy.ID, UID: 1,
		MessageID: "unavailable-backup@example.com", Subject: "unavailable hierarchy message",
		FromEmail: "backup@example.com", Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create unavailable backup message: %v", err)
	}

	directory := t.TempDir()
	options := BackupRunOptions{
		Directory: directory, Scope: backupScopeSelected,
		SelectedAccountIDs: []string{fixture.account.ID, fixture.account.ID, ""},
	}
	result, err := runEmailBackupSynchronouslyForTest(fixture.app, options)
	if err != nil {
		t.Fatalf("synchronous backup full: %v", err)
	}
	if result.Mode != "full" || result.Total != 3 || result.Exported != 1 || result.Missing != 1 || result.Unavailable != 1 || result.Failed != 0 {
		t.Fatalf("full backup result = %#v", result)
	}
	if result.ReportPath == "" {
		t.Fatal("partial backup did not produce a report")
	}
	if state := fixture.app.GetBackupRunState(); state.Running || state.Progress == nil || state.Progress.Phase != "done" {
		t.Fatalf("backup run state = %#v", state)
	}
	status, err := fixture.app.GetBackupStatus(directory)
	if err != nil || status.Mode != "incremental" || status.MessageCount != 1 || !status.HasIndex {
		t.Fatalf("GetBackupStatus = (%#v, %v)", status, err)
	}

	incremental, err := runEmailBackupSynchronouslyForTest(fixture.app, options)
	if err != nil {
		t.Fatalf("synchronous backup incremental: %v", err)
	}
	if incremental.Mode != "incremental" || incremental.Exported != 0 || incremental.Skipped != 1 || incremental.Missing != 1 || incremental.Unavailable != 1 {
		t.Fatalf("incremental backup result = %#v", incremental)
	}

	backgroundDir := t.TempDir()
	started, err := fixture.app.StartEmailBackup(BackupRunOptions{Directory: backgroundDir})
	if err != nil || !started.Running || started.StartedAt == "" {
		t.Fatalf("StartEmailBackup = (%#v, %v)", started, err)
	}
	waitForActionCondition(t, "background email backup", func() bool {
		return !fixture.app.GetBackupRunState().Running
	})
	if state := fixture.app.GetBackupRunState(); state.Progress == nil || state.Progress.Phase != "done" {
		t.Fatalf("background backup final state = %#v", state)
	}
	if !fixture.events.has("backup:progress") {
		t.Fatalf("backup events = %v", fixture.events.snapshot())
	}

	if _, err := runEmailBackupSynchronouslyForTest(fixture.app, BackupRunOptions{Directory: ""}); err == nil {
		t.Fatal("synchronous backup without directory unexpectedly succeeded")
	}
	if _, err := runEmailBackupSynchronouslyForTest(fixture.app, BackupRunOptions{
		Directory: t.TempDir(), Scope: backupScopeSelected,
		SelectedAccountIDs: []string{"missing-account"},
	}); err == nil || !strings.Contains(err.Error(), "no accounts selected") {
		t.Fatalf("synchronous backup empty selection error = %v", err)
	}
}
