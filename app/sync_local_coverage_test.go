package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	syncengine "aulyc.local/aulycmail/internal/sync"
)

func TestSyncSelectionCancellationAndOwnershipGuards(t *testing.T) {
	a, acc, _ := newContactOwnEmailsTestApp(t)
	a.folderStore = folder.NewStore(a.db)
	a.messageStore = message.NewStore(a.db)
	a.attachmentStore = message.NewAttachmentStore(a.db)
	a.ctx = context.Background()
	a.SyncBridge.lastRequest = make(map[string]time.Time)
	a.SyncBridge.contexts = make(map[string]context.CancelFunc)

	folders := []*folder.Folder{
		{ID: "sync-inbox", AccountID: acc.ID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox, Subscribed: true},
		{ID: "sync-drafts", AccountID: acc.ID, Name: "Drafts", Path: "Drafts", Type: folder.TypeDrafts},
		{ID: "sync-sent", AccountID: acc.ID, Name: "Sent", Path: "Sent", Type: folder.TypeSent, Subscribed: true},
		{ID: "sync-archive", AccountID: acc.ID, Name: "Archive", Path: "Archive", Type: folder.TypeArchive, Subscribed: true},
		{ID: "sync-other", AccountID: acc.ID, Name: "Other", Path: "Other", Type: folder.TypeFolder},
		{ID: "sync-group", AccountID: acc.ID, Name: "Group", Path: "Group", Type: folder.TypeFolder, Subscribed: true, NoSelect: true},
	}
	for _, item := range folders {
		if err := a.folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	selected, err := a.getSyncFolders(acc.ID)
	if err != nil {
		t.Fatalf("getSyncFolders(default): %v", err)
	}
	assertSyncFolderIDs(t, selected, []string{"sync-inbox", "sync-drafts", "sync-sent"})

	if _, err := a.db.Exec(`UPDATE accounts SET sync_all_folders = 1 WHERE id = ?`, acc.ID); err != nil {
		t.Fatalf("enable all-folder sync: %v", err)
	}
	selected, err = a.getSyncFolders(acc.ID)
	if err != nil {
		t.Fatalf("getSyncFolders(all): %v", err)
	}
	assertSyncFolderIDs(t, selected, []string{"sync-inbox", "sync-drafts", "sync-sent", "sync-archive", "sync-other"})

	if _, err := a.db.Exec(`UPDATE accounts SET sync_all_folders = 0, sync_folders_enabled = 1 WHERE id = ?`, acc.ID); err != nil {
		t.Fatalf("enable subscribed-folder sync: %v", err)
	}
	selected, err = a.getSyncFolders(acc.ID)
	if err != nil {
		t.Fatalf("getSyncFolders(subscribed): %v", err)
	}
	assertSyncFolderIDs(t, selected, []string{"sync-inbox", "sync-drafts", "sync-sent", "sync-archive"})
	if _, err := a.getSyncFolders("missing-account"); err == nil {
		t.Fatal("getSyncFolders(missing) unexpectedly succeeded")
	}

	sentinel := errors.New("synthetic coordinated failure")
	if err := a.coordinateAccountSync(acc.ID, syncengine.TriggerManual, func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("coordinateAccountSync without coordinator = %v", err)
	}
	a.syncCoordinator = syncengine.NewCoordinator()
	ran := false
	if err := a.coordinateAccountSync(acc.ID, syncengine.TriggerManual, func() error {
		ran = true
		return nil
	}); err != nil || !ran {
		t.Fatalf("coordinateAccountSync = (%v, ran %v)", err, ran)
	}

	if err := a.SyncFolder("wrong-account", "sync-inbox"); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("SyncFolder(cross-account) error = %v", err)
	}
	if err := a.SyncFolder(acc.ID, "missing-folder"); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("SyncFolder(missing) error = %v", err)
	}

	if err := a.messageStore.Create(&message.Message{
		ID: "force-sync-message", AccountID: acc.ID, FolderID: "sync-inbox", UID: 1,
		Subject: "Preserve on rejected force sync", BodyText: "cached body", BodyHTML: "<p>cached body</p>",
		BodyFetched: true, Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create force-sync message: %v", err)
	}
	if err := a.attachmentStore.Create(&message.Attachment{
		ID: "force-sync-attachment", MessageID: "force-sync-message", Filename: "cached.txt", ContentType: "text/plain", Size: 6,
	}); err != nil {
		t.Fatalf("create force-sync attachment: %v", err)
	}
	if err := a.ForceSyncFolder("wrong-account", "sync-inbox"); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("ForceSyncFolder(cross-account) error = %v", err)
	}
	preserved, err := a.messageStore.Get("force-sync-message")
	if err != nil || preserved == nil || preserved.BodyText != "cached body" || !preserved.BodyFetched {
		t.Fatalf("message after rejected force sync = (%#v, %v), want cached body preserved", preserved, err)
	}
	if got, err := a.attachmentStore.Get("force-sync-attachment"); err != nil || got == nil {
		t.Fatalf("attachment after rejected force sync = (%#v, %v), want preserved", got, err)
	}

	firstCtx, firstCancel := context.WithCancel(context.Background())
	secondCtx, secondCancel := context.WithCancel(context.Background())
	a.SyncBridge.contexts[acc.ID+":sync-inbox"] = firstCancel
	a.SyncBridge.contexts[acc.ID+":sync-sent"] = secondCancel
	a.CancelFolderSync(acc.ID, "sync-inbox")
	if firstCtx.Err() != context.Canceled {
		t.Fatalf("CancelFolderSync context error = %v", firstCtx.Err())
	}
	if _, exists := a.SyncBridge.contexts[acc.ID+":sync-inbox"]; exists {
		t.Fatal("CancelFolderSync left context registered")
	}
	a.CancelFolderSync(acc.ID, "missing")
	a.CancelAllSyncs()
	if secondCtx.Err() != context.Canceled || !a.SyncBridge.cancelled || len(a.SyncBridge.contexts) != 0 {
		t.Fatalf("CancelAllSyncs state = secondErr %v cancelled %v contexts %#v", secondCtx.Err(), a.SyncBridge.cancelled, a.SyncBridge.contexts)
	}
	if err := a.SyncAccountComplete(acc.ID); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("SyncAccountComplete after cancellation error = %v", err)
	}

	if err := a.accountStore.SetEnabled(acc.ID, false); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	if err := a.SyncAllComplete(); err != nil {
		t.Fatalf("SyncAllComplete with disabled account: %v", err)
	}
	if a.SyncBridge.cancelled {
		t.Fatal("SyncAllComplete did not reset cancellation state")
	}
}

func assertSyncFolderIDs(t *testing.T, got []*folder.Folder, want []string) {
	t.Helper()
	gotSet := make(map[string]bool, len(got))
	for _, item := range got {
		if item.NoSelect {
			t.Fatalf("sync selection included no-select folder %#v", item)
		}
		gotSet[item.ID] = true
	}
	if len(gotSet) != len(want) {
		t.Fatalf("sync folder IDs = %#v, want %v", gotSet, want)
	}
	for _, id := range want {
		if !gotSet[id] {
			t.Fatalf("sync folder IDs = %#v, missing %s", gotSet, id)
		}
	}
}
