package sync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
	goImap "github.com/emersion/go-imap/v2"
)

func TestEngineFolderDiscoveryAndReconciliationAgainstMemoryIMAP(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	stale := &folder.Folder{
		ID: "stale-folder", AccountID: syncAccountID, Name: "Stale", Path: "Stale", Type: folder.TypeFolder, Subscribed: true,
	}
	if err := fixture.folderStore.Create(stale); err != nil {
		t.Fatalf("create stale folder: %v", err)
	}

	var progress []SyncProgress
	fixture.engine.SetProgressCallback(func(update SyncProgress) { progress = append(progress, update) })
	result, err := fixture.engine.SyncFoldersWithResult(context.Background(), syncAccountID)
	if err != nil {
		t.Fatalf("SyncFoldersWithResult() error = %v", err)
	}
	if !result.Complete || len(result.Snapshots) != 8 {
		t.Fatalf("folder sync result = complete %v, snapshots %#v", result.Complete, result.Snapshots)
	}
	if len(progress) != 9 || progress[0].Phase != "folders" || progress[0].Fetched != 0 || progress[len(progress)-1].Fetched != 8 {
		t.Fatalf("folder progress = %#v", progress)
	}
	if removed, err := fixture.folderStore.Get(stale.ID); err != nil || removed != nil {
		t.Fatalf("stale folder after sync = %#v, %v", removed, err)
	}

	folders, err := fixture.folderStore.List(syncAccountID)
	if err != nil || len(folders) != 8 {
		t.Fatalf("local folders = %d, %v", len(folders), err)
	}
	byPath := make(map[string]*folder.Folder, len(folders))
	for _, item := range folders {
		byPath[item.Path] = item
	}
	for path, wantType := range map[string]folder.Type{
		"INBOX": folder.TypeInbox, "Archive": folder.TypeArchive, "Drafts": folder.TypeDrafts,
		"Sent": folder.TypeSent, "Trash": folder.TypeTrash, "Junk": folder.TypeSpam,
		"Projects": folder.TypeFolder, "Projects/2026": folder.TypeFolder,
	} {
		if item := byPath[path]; item == nil || item.Type != wantType || item.NoSelect {
			t.Fatalf("folder %s = %#v, want type %s", path, item, wantType)
		}
	}
	if !byPath["INBOX"].Subscribed || !byPath["Archive"].Subscribed || !byPath["Projects"].Subscribed || byPath["Sent"].Subscribed {
		t.Fatalf("folder subscriptions: INBOX=%v Archive=%v Projects=%v Sent=%v",
			byPath["INBOX"].Subscribed, byPath["Archive"].Subscribed, byPath["Projects"].Subscribed, byPath["Sent"].Subscribed)
	}
	if byPath["Projects/2026"].Name != "2026" || byPath["Projects/2026"].ParentID != byPath["Projects"].ID {
		t.Fatalf("nested folder = %#v, parent = %#v", byPath["Projects/2026"], byPath["Projects"])
	}
	for path, snapshot := range result.Snapshots {
		if snapshot.NoSelect || snapshot.UIDValidity == 0 || snapshot.UIDNext == 0 {
			t.Fatalf("snapshot %s = %+v", path, snapshot)
		}
	}

	acc, err := fixture.engine.accountStore.Get(syncAccountID)
	if err != nil {
		t.Fatalf("get account mappings: %v", err)
	}
	if acc.ArchiveFolderPath != "Archive" || acc.DraftsFolderPath != "Drafts" || acc.SentFolderPath != "Sent" || acc.TrashFolderPath != "Trash" || acc.SpamFolderPath != "Junk" {
		t.Fatalf("auto-detected mappings = archive %q drafts %q sent %q trash %q spam %q",
			acc.ArchiveFolderPath, acc.DraftsFolderPath, acc.SentFolderPath, acc.TrashFolderPath, acc.SpamFolderPath)
	}

	if err := fixture.harness.user.Delete("Projects/2026"); err != nil {
		t.Fatalf("delete remote child folder: %v", err)
	}
	if err := fixture.harness.user.Create("Later", nil); err != nil {
		t.Fatalf("create remote folder: %v", err)
	}
	if err := fixture.engine.SyncFolders(context.Background(), syncAccountID); err != nil {
		t.Fatalf("SyncFolders() reconciliation error = %v", err)
	}
	if child, err := fixture.folderStore.GetByPath(syncAccountID, "Projects/2026"); err != nil || child != nil {
		t.Fatalf("removed remote child locally = %#v, %v", child, err)
	}
	if added, err := fixture.folderStore.GetByPath(syncAccountID, "Later"); err != nil || added == nil || added.Type != folder.TypeFolder {
		t.Fatalf("new remote folder locally = %#v, %v", added, err)
	}

	if result, err := fixture.engine.SyncFoldersWithResult(context.Background(), "missing-account"); err == nil || !strings.Contains(err.Error(), "failed to get connection") || len(result.Snapshots) != 0 {
		t.Fatalf("missing-account folder sync = %+v, %v", result, err)
	}
}

var syncSearchOnlyMessage = []byte(strings.Join([]string{
	"Date: Sun, 02 Aug 2026 10:00:00 +0000",
	"From: Carol Search <carol@example.com>",
	"To: Sync User <sync@example.com>",
	"Subject: Search-only server result",
	"Message-ID: <search-3@example.com>",
	"References: <plain-1@example.com>",
	"Content-Type: text/plain; charset=utf-8",
	"",
	"This message exists only on the IMAP server at first.",
	"",
}, "\r\n"))

func TestEngineServerSearchAndFetchAgainstMemoryIMAP(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	uid1 := fixture.harness.append(t, syncPlainMessage)
	uid2 := fixture.harness.append(t, syncMultipartMessage)
	if err := fixture.engine.SyncMessagesWithOptions(context.Background(), syncAccountID, syncFolderID, MessageSyncOptions{Strategy: account.SyncStrategyFull}); err != nil {
		t.Fatalf("initial message sync: %v", err)
	}
	uid3 := fixture.harness.append(t, syncSearchOnlyMessage, goImap.FlagSeen, goImap.FlagFlagged)
	if uid1 != 1 || uid2 != 2 || uid3 != 3 {
		t.Fatalf("fixture UIDs = %d, %d, %d", uid1, uid2, uid3)
	}

	response, err := fixture.engine.IMAPSearch(context.Background(), syncAccountID, syncFolderID, "example.com", 2)
	if err != nil {
		t.Fatalf("IMAPSearch() error = %v", err)
	}
	if response.TotalCount != 3 || len(response.Results) != 2 {
		t.Fatalf("limited search response = %+v", response)
	}
	byUID := make(map[uint32]*IMAPSearchResult, len(response.Results))
	for _, item := range response.Results {
		byUID[item.UID] = item
	}
	if local := byUID[uid2]; local == nil || !local.IsLocal || local.MessageID == "" || local.Subject != "Multipart integration message" {
		t.Fatalf("local search result = %#v", local)
	}
	if remote := byUID[uid3]; remote == nil || remote.IsLocal || remote.MessageID != "" || remote.Subject != "Search-only server result" || remote.FromEmail != "carol@example.com" || !remote.IsRead || !remote.IsStarred {
		t.Fatalf("remote search result = %#v", remote)
	}
	if !response.Results[0].Date.After(response.Results[1].Date) {
		t.Fatalf("search results not newest-first: %#v", response.Results)
	}

	fetched, err := fixture.engine.FetchServerMessage(context.Background(), syncAccountID, syncFolderID, uid3)
	if err != nil {
		t.Fatalf("FetchServerMessage() error = %v", err)
	}
	if fetched.UID != uid3 || fetched.MessageID != "search-3@example.com" || fetched.Subject != "Search-only server result" || !fetched.BodyFetched || !fetched.IsRead || !fetched.IsStarred || !strings.Contains(fetched.BodyText, "exists only on the IMAP server") || fetched.Snippet == "" || fetched.ThreadID == "" {
		t.Fatalf("fetched server message = %+v", fetched)
	}
	again, err := fixture.engine.FetchServerMessage(context.Background(), syncAccountID, syncFolderID, uid3)
	if err != nil || again == nil || again.ID != fetched.ID {
		t.Fatalf("FetchServerMessage(existing) = %#v, %v", again, err)
	}

	all, err := fixture.engine.IMAPSearch(context.Background(), syncAccountID, syncFolderID, "example.com", 0)
	if err != nil || all.TotalCount != 3 || len(all.Results) != 3 {
		t.Fatalf("unlimited search = %+v, %v", all, err)
	}
	for _, item := range all.Results {
		if !item.IsLocal || item.MessageID == "" {
			t.Fatalf("result remained non-local after fetch: %#v", item)
		}
	}
	empty, err := fixture.engine.IMAPSearch(context.Background(), syncAccountID, syncFolderID, "definitely-no-match", 10)
	if err != nil || empty.TotalCount != 0 || len(empty.Results) != 0 {
		t.Fatalf("empty search = %+v, %v", empty, err)
	}
	if _, err := fixture.engine.FetchServerMessage(context.Background(), syncAccountID, syncFolderID, 999); err == nil || !strings.Contains(err.Error(), "message not found on server") {
		t.Fatalf("FetchServerMessage(missing UID) error = %v", err)
	}
	if _, err := fixture.engine.IMAPSearch(context.Background(), syncAccountID, "missing-folder", "x", 1); err == nil || !strings.Contains(err.Error(), "folder not found") {
		t.Fatalf("IMAPSearch(missing folder) error = %v", err)
	}
	noSelect := &folder.Folder{ID: "search-no-select", AccountID: syncAccountID, Name: "Group", Path: "Group", Type: folder.TypeFolder, NoSelect: true}
	if err := fixture.folderStore.Create(noSelect); err != nil {
		t.Fatalf("create no-select search folder: %v", err)
	}
	if _, err := fixture.engine.IMAPSearch(context.Background(), syncAccountID, noSelect.ID, "x", 1); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("IMAPSearch(no-select) error = %v", err)
	}
	if _, err := fixture.engine.FetchServerMessage(context.Background(), syncAccountID, noSelect.ID, uid1); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("FetchServerMessage(no-select) error = %v", err)
	}

	if response.Results[0].Date.Before(time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("search result date was not parsed as UTC: %v", response.Results[0].Date)
	}
}
