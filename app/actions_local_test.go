package app

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	goImap "github.com/emersion/go-imap/v2"
)

func TestLocalActionHelpersPartitionAccountsAndMapFlags(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	accountStore := account.NewStore(db)
	folderStore := folder.NewStore(db)
	messageStore := message.NewStore(db)
	first := createOfflineCacheTestAccount(t, accountStore, "first@example.com")
	second := createOfflineCacheTestAccount(t, accountStore, "second@example.com")
	if _, err := db.Exec(`UPDATE accounts SET imap_host = 'imap.gmail.com' WHERE id = ?`, second.ID); err != nil {
		t.Fatalf("mark Gmail account: %v", err)
	}
	for _, item := range []*folder.Folder{
		{ID: "first-inbox", AccountID: first.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox},
		{ID: "empty-folder", AccountID: first.ID, Name: "Empty", Path: "Empty", Type: folder.TypeFolder},
		{ID: "second-inbox", AccountID: second.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox},
	} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("Create folder %s: %v", item.ID, err)
		}
	}
	for _, item := range []*message.Message{
		{ID: "first-1", AccountID: first.ID, FolderID: "first-inbox", UID: 1, Date: time.Now().UTC()},
		{ID: "first-2", AccountID: first.ID, FolderID: "first-inbox", UID: 2, Date: time.Now().UTC()},
		{ID: "second-1", AccountID: second.ID, FolderID: "second-inbox", UID: 1, Date: time.Now().UTC()},
	} {
		if err := messageStore.Create(item); err != nil {
			t.Fatalf("Create message %s: %v", item.ID, err)
		}
	}
	a := &App{accountStore: accountStore, folderStore: folderStore, messageStore: messageStore}

	partitioned, err := a.partitionByAccount([]string{"first-1", "missing", "second-1", "first-2"})
	if err != nil {
		t.Fatalf("partitionByAccount() error = %v", err)
	}
	wantPartitions := []accountPartition{
		{accountID: first.ID, messageIDs: []string{"first-1", "first-2"}},
		{accountID: second.ID, messageIDs: []string{"second-1"}},
	}
	if !reflect.DeepEqual(partitioned, wantPartitions) {
		t.Fatalf("partitioned IDs = %#v, want %#v", partitioned, wantPartitions)
	}
	if a.isGmailAccount(first.ID) || !a.isGmailAccount(second.ID) || a.isGmailAccount("missing") {
		t.Fatalf("Gmail detection = first %v second %v missing %v", a.isGmailAccount(first.ID), a.isGmailAccount(second.ID), a.isGmailAccount("missing"))
	}

	if got := flagsForAppend(&message.Message{}); len(got) != 0 {
		t.Fatalf("empty flags = %v", got)
	}
	allFlags := flagsForAppend(&message.Message{IsRead: true, IsStarred: true, IsAnswered: true, IsDraft: true})
	wantFlags := []goImap.Flag{goImap.FlagSeen, goImap.FlagFlagged, goImap.FlagAnswered, goImap.FlagDraft}
	if !reflect.DeepEqual(allFlags, wantFlags) {
		t.Fatalf("flagsForAppend() = %v, want %v", allFlags, wantFlags)
	}

	for name, action := range map[string]func() error{
		"mark read":          func() error { return a.MarkAsRead(nil) },
		"mark unread":        func() error { return a.MarkAsUnread(nil) },
		"star":               func() error { return a.Star(nil) },
		"unstar":             func() error { return a.Unstar(nil) },
		"move":               func() error { return a.MoveToFolder(nil, "") },
		"copy":               func() error { return a.CopyToFolder(nil, "") },
		"archive":            func() error { return a.Archive(nil) },
		"mark not spam":      func() error { return a.MarkAsNotSpam(nil) },
		"delete permanently": func() error { return a.DeletePermanently(nil) },
	} {
		if err := action(); err != nil {
			t.Fatalf("%s empty action error = %v", name, err)
		}
	}
	if moved, err := a.Trash(nil); err != nil || moved {
		t.Fatalf("Trash(nil) = %v, %v", moved, err)
	}
	if moved, err := a.MarkAsSpam(nil); err != nil || moved {
		t.Fatalf("MarkAsSpam(nil) = %v, %v", moved, err)
	}
	if err := a.syncFlagsToIMAP(nil, "", "read", true); err != nil {
		t.Fatalf("syncFlagsToIMAP(nil) error = %v", err)
	}
	if err := a.gmailRemoveLabel(nil); err != nil {
		t.Fatalf("gmailRemoveLabel(nil) error = %v", err)
	}
	if err := a.removeFromIMAPFolder(nil, ""); err != nil {
		t.Fatalf("removeFromIMAPFolder(nil) error = %v", err)
	}

	if err := a.MarkAllFolderMessagesAsRead("empty-folder"); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsRead(empty folder) error = %v", err)
	}
	if err := a.MarkAllFolderMessagesAsUnread("empty-folder"); err != nil {
		t.Fatalf("MarkAllFolderMessagesAsUnread(empty folder) error = %v", err)
	}
	if err := a.EmptyTrash(first.ID, "empty-folder"); err != nil {
		t.Fatalf("EmptyTrash(empty folder) error = %v", err)
	}
	for name, action := range map[string]func() error{
		"archive missing":  func() error { return a.Archive([]string{"missing"}) },
		"not-spam missing": func() error { return a.MarkAsNotSpam([]string{"missing"}) },
	} {
		if err := action(); err == nil || !strings.Contains(err.Error(), "failed to get message") {
			t.Fatalf("%s error = %v", name, err)
		}
	}
	if _, err := a.Trash([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "failed to get message") {
		t.Fatalf("Trash(missing) error = %v", err)
	}
	if _, err := a.MarkAsSpam([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "failed to get message") {
		t.Fatalf("MarkAsSpam(missing) error = %v", err)
	}
}

func TestCrossAccountBulkActionsIsolatePartitionFailuresWithoutMutation(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cross-account-actions.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	first := createOfflineCacheTestAccount(t, accountStore, "cross-first@example.com")
	second := createOfflineCacheTestAccount(t, accountStore, "cross-second@example.com")
	folderStore := folder.NewStore(db)
	firstSource := &folder.Folder{
		ID: "cross-first-source", AccountID: first.ID, Name: "Hierarchy only", Path: "Hierarchy only",
		Type: folder.TypeFolder, NoSelect: true,
	}
	secondSource := &folder.Folder{
		ID: "cross-second-source", AccountID: second.ID, Name: "Source", Path: "Source",
		Type: folder.TypeFolder,
	}
	moveDestination := &folder.Folder{
		ID: "cross-move-destination", AccountID: first.ID, Name: "Destination", Path: "Destination",
		Type: folder.TypeFolder, NoSelect: true,
	}
	for _, item := range []*folder.Folder{
		firstSource,
		secondSource,
		moveDestination,
		{ID: "cross-first-inbox", AccountID: first.ID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox},
		{ID: "cross-first-archive", AccountID: first.ID, Name: "Archive", Path: "Archive", Type: folder.TypeArchive},
		{ID: "cross-first-trash", AccountID: first.ID, Name: "Trash", Path: "Trash", Type: folder.TypeTrash},
		{ID: "cross-first-spam", AccountID: first.ID, Name: "Spam", Path: "Spam", Type: folder.TypeSpam},
	} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	messageStore := message.NewStore(db)
	for _, item := range []*message.Message{
		{ID: "cross-first-message", AccountID: first.ID, FolderID: firstSource.ID, UID: 1, Date: time.Now().UTC()},
		{ID: "cross-second-message", AccountID: second.ID, FolderID: secondSource.ID, UID: 1, Date: time.Now().UTC()},
	} {
		if err := messageStore.Create(item); err != nil {
			t.Fatalf("create message %s: %v", item.ID, err)
		}
	}
	a := &App{accountStore: accountStore, folderStore: folderStore, messageStore: messageStore}
	mixedIDs := []string{"cross-first-message", "missing-message", "cross-second-message"}

	moved, err := a.Trash(mixedIDs)
	if !moved || err == nil {
		t.Fatalf("Trash(mixed) = (%v, %v), want moved with partition error", moved, err)
	}
	if !errors.Is(err, folder.ErrNotSelectable) && !strings.Contains(err.Error(), "no trash folder") {
		t.Fatalf("Trash(mixed) error = %v", err)
	}
	moved, err = a.MarkAsSpam(mixedIDs)
	if !moved || err == nil {
		t.Fatalf("MarkAsSpam(mixed) = (%v, %v), want moved with partition error", moved, err)
	}
	if err := a.Archive(mixedIDs); err == nil {
		t.Fatal("Archive(mixed) unexpectedly succeeded")
	}
	if err := a.MarkAsNotSpam(mixedIDs); err == nil {
		t.Fatal("MarkAsNotSpam(mixed) unexpectedly succeeded")
	}
	if err := a.MoveToFolder(mixedIDs, moveDestination.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("MoveToFolder(mixed, no-select) error = %v", err)
	}
	if err := a.CopyToFolder(mixedIDs, moveDestination.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("CopyToFolder(mixed, no-select) error = %v", err)
	}

	for messageID, wantFolderID := range map[string]string{
		"cross-first-message":  firstSource.ID,
		"cross-second-message": secondSource.ID,
	} {
		got, err := messageStore.Get(messageID)
		if err != nil || got == nil || got.FolderID != wantFolderID {
			t.Fatalf("message %s after failed partitions = (%#v, %v), want folder %s", messageID, got, err, wantFolderID)
		}
	}
}
