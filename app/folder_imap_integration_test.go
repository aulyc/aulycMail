package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

func TestFolderBridgeDiscoversAndManagesSubscriptionsAgainstMemoryServer(t *testing.T) {
	harness := startActionIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "folder-imap.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc := createOfflineCacheTestAccount(t, accountStore, actionIMAPUsername)
	other := createOfflineCacheTestAccount(t, accountStore, "other-folder-account@example.com")
	folderStore := folder.NewStore(db)
	messageStore := message.NewStore(db)
	poolConfig := imapPkg.DefaultPoolConfig()
	poolConfig.MaxConnections = 5
	poolConfig.WaiterTimeout = time.Second
	pool := imapPkg.NewPool(poolConfig, func(accountID string) (*imapPkg.ClientConfig, error) {
		if accountID != acc.ID {
			return nil, errors.New("credentials unavailable")
		}
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)
	engine := mailSync.NewEngine(pool, accountStore, folderStore, messageStore, nil, nil)
	a := &App{
		ctx: context.Background(), db: db, accountStore: accountStore,
		folderStore: folderStore, messageStore: messageStore, imapPool: pool, syncEngine: engine,
	}
	a.initSyncBridge()

	folders, err := a.GetAccountFoldersForMapping(acc.ID)
	if err != nil {
		t.Fatalf("GetAccountFoldersForMapping: %v", err)
	}
	if len(folders) != 4 {
		t.Fatalf("discovered folders = %d, want 4: %#v", len(folders), folders)
	}
	if folders[0].Type != folder.TypeInbox {
		t.Fatalf("first discovered folder = %#v, want inbox sorted first", folders[0])
	}
	byPath := make(map[string]*folder.Folder, len(folders))
	for _, item := range folders {
		if item.NoSelect {
			t.Fatalf("discovered selectable mailbox marked no-select: %#v", item)
		}
		byPath[item.Path] = item
	}
	archive := byPath["Archive"]
	if archive == nil {
		t.Fatalf("Archive missing from discovered folders: %#v", byPath)
	}

	if err := a.SubscribeFolder(acc.ID, archive.ID); err != nil {
		t.Fatalf("SubscribeFolder: %v", err)
	}
	assertLocalFolderSubscribed(t, folderStore, archive.ID, true)
	assertRemoteSubscriptions(t, harness, []string{"Archive"})

	if err := a.UnsubscribeFolder(acc.ID, archive.ID); err != nil {
		t.Fatalf("UnsubscribeFolder: %v", err)
	}
	assertLocalFolderSubscribed(t, folderStore, archive.ID, false)
	assertRemoteSubscriptions(t, harness, nil)

	if err := a.SubscribeAllFolders(acc.ID); err != nil {
		t.Fatalf("SubscribeAllFolders: %v", err)
	}
	for _, item := range folders {
		assertLocalFolderSubscribed(t, folderStore, item.ID, true)
	}
	assertRemoteSubscriptions(t, harness, []string{"Archive", "Drafts", "INBOX", "Trash"})

	noSelect := &folder.Folder{
		ID: "folder-group", AccountID: acc.ID, Name: "Group", Path: "Group",
		Type: folder.TypeFolder, NoSelect: true,
	}
	if err := folderStore.Create(noSelect); err != nil {
		t.Fatalf("create hierarchy-only folder: %v", err)
	}
	if err := a.SubscribeFolder(acc.ID, noSelect.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("SubscribeFolder(no-select) error = %v", err)
	}
	if err := a.UnsubscribeFolder(acc.ID, noSelect.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("UnsubscribeFolder(no-select) error = %v", err)
	}
	if err := a.SubscribeFolder(acc.ID, "missing-folder"); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("SubscribeFolder(missing) error = %v", err)
	}
	if err := a.UnsubscribeFolder(acc.ID, "missing-folder"); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("UnsubscribeFolder(missing) error = %v", err)
	}
	if err := a.SubscribeFolder(other.ID, archive.ID); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("SubscribeFolder(cross-account) error = %v", err)
	}
	if err := a.UnsubscribeFolder(other.ID, archive.ID); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("UnsubscribeFolder(cross-account) error = %v", err)
	}
}

func assertLocalFolderSubscribed(t *testing.T, store *folder.Store, folderID string, want bool) {
	t.Helper()
	got, err := store.Get(folderID)
	if err != nil || got == nil || got.Subscribed != want {
		t.Fatalf("local folder %s subscribed = (%#v, %v), want %v", folderID, got, err, want)
	}
}

func assertRemoteSubscriptions(t *testing.T, harness actionIMAPHarness, want []string) {
	t.Helper()
	client := harness.client(t)
	defer func() { _ = client.Close() }()
	mailboxes, err := client.ListSubscribedMailboxes()
	if err != nil {
		t.Fatalf("ListSubscribedMailboxes: %v", err)
	}
	got := mailboxes
	if len(got) != len(want) {
		t.Fatalf("remote subscriptions = %#v, want %v", got, want)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("remote subscriptions = %#v, missing %q", got, name)
		}
	}
}
