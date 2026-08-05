package sync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
)

func TestRunAccountSyncLifecycleFinishesAfterNoChangeWork(t *testing.T) {
	scheduler := &Scheduler{}
	workFinished := false
	var finishedAccount string
	var succeeded bool

	scheduler.SetAccountSyncFinishedCallback(func(accountID string, syncSucceeded bool) {
		if !workFinished {
			t.Fatal("account sync finished callback ran before the work returned")
		}
		finishedAccount = accountID
		succeeded = syncSucceeded
	})

	scheduler.runAccountSyncLifecycle("account-1", func() bool {
		// A scheduled remote probe with no changed folders has no folder-level
		// completion callback, but the account-level lifecycle must still finish.
		workFinished = true
		return true
	})

	if finishedAccount != "account-1" {
		t.Fatalf("finished account = %q, want account-1", finishedAccount)
	}
	if !succeeded {
		t.Fatal("successful account sync was reported as unsuccessful")
	}
}

func TestIncludeRequiredFolderPathsAlwaysChecksScheduledCoreFolders(t *testing.T) {
	changed := map[string]bool{
		"Projects": true,
	}
	configured := []*folder.Folder{
		{Path: "INBOX", Type: folder.TypeInbox},
		{Path: "Drafts", Type: folder.TypeDrafts},
		{Path: "Sent Messages", Type: folder.TypeSent},
		{Path: "Archive", Type: folder.TypeArchive},
	}

	includeRequiredFolderPaths(changed, configured, false)

	for _, path := range []string{"INBOX", "Drafts", "Sent Messages", "Projects"} {
		if !changed[path] {
			t.Fatalf("required folder %q was not selected for scheduled sync", path)
		}
	}
	if changed["Archive"] {
		t.Fatal("unchanged non-core folder should still be skipped during scheduled sync")
	}
}

func TestIncludeRequiredFolderPathsForcesEveryConfiguredFolderOnManualSync(t *testing.T) {
	changed := map[string]bool{}
	configured := []*folder.Folder{
		{Path: "INBOX", Type: folder.TypeInbox},
		{Path: "Archive", Type: folder.TypeArchive},
	}

	includeRequiredFolderPaths(changed, configured, true)

	for _, path := range []string{"INBOX", "Archive"} {
		if !changed[path] {
			t.Fatalf("manual sync did not select configured folder %q", path)
		}
	}
}

func TestSchedulerCallbacksLifecycleAndCancellation(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	originalCoordinator := scheduler.coordinator
	scheduler.SetCoordinator(nil)
	if scheduler.coordinator != originalCoordinator {
		t.Fatal("SetCoordinator(nil) replaced the coordinator")
	}
	customCoordinator := NewCoordinator()
	scheduler.SetCoordinator(customCoordinator)
	if scheduler.coordinator != customCoordinator {
		t.Fatal("SetCoordinator(custom) did not replace the coordinator")
	}

	var resultCalls, legacyCalls int
	var resultErr, legacyErr error
	scheduler.SetNewMailCallback(func(NewMailInfo) {})
	scheduler.SetSyncCompletedResultCallback(func(_ string, _ string, _ MessageSyncResult, err error) {
		resultCalls++
		resultErr = err
	})
	scheduler.SetSyncCompletedCallback(func(_ string, _ string, err error) {
		legacyCalls++
		legacyErr = err
	})
	scheduler.notifySyncCompleted("account-1", "folder-1", MessageSyncResult{Added: 2}, context.Canceled)
	if resultCalls != 1 || !errors.Is(resultErr, context.Canceled) {
		t.Fatalf("result callback = %d calls, err %v", resultCalls, resultErr)
	}
	if legacyCalls != 1 || legacyErr != nil {
		t.Fatalf("legacy canceled callback = %d calls, err %v; want nil err", legacyCalls, legacyErr)
	}

	wantErr := errors.New("sync failed")
	scheduler.notifySyncCompleted("account-1", "folder-1", MessageSyncResult{}, wantErr)
	if resultCalls != 2 || legacyCalls != 2 || !errors.Is(resultErr, wantErr) || !errors.Is(legacyErr, wantErr) {
		t.Fatalf("ordinary error callbacks = result %d/%v legacy %d/%v", resultCalls, resultErr, legacyCalls, legacyErr)
	}

	scheduler.SetConnectivityCheck(func() bool { return false })
	if scheduler.isConnected == nil || scheduler.isConnected() {
		t.Fatal("SetConnectivityCheck() did not retain offline check")
	}
	scheduler.Start(context.Background())
	scheduler.Start(context.Background())
	scheduler.Stop()
	scheduler.Stop()
	if scheduler.running {
		t.Fatal("scheduler remained running after Stop()")
	}
}

func TestSchedulerChangedPathsSnapshotsAndFolderSelection(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username, enabled, sync_interval)
		VALUES ('account-1', 'Test', 'test@example.com', 'imap.example.com', 'smtp.example.com', 'test', 1, 30)`); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	folderStore := folder.NewStore(db)
	for _, item := range []*folder.Folder{
		{ID: "inbox", AccountID: "account-1", Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox},
		{ID: "sent", AccountID: "account-1", Name: "Sent", Path: "Sent", Type: folder.TypeSent},
		{ID: "projects", AccountID: "account-1", Name: "Projects", Path: "Projects", Type: folder.TypeFolder, Subscribed: true},
		{ID: "directory", AccountID: "account-1", Name: "Other", Path: "Other", Type: folder.TypeFolder, Subscribed: true, NoSelect: true},
	} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("Create folder %s: %v", item.ID, err)
		}
	}
	scheduler := NewScheduler(nil, account.NewStore(db), folderStore)

	initial := map[string]RemoteFolderSnapshot{
		"INBOX": {UIDNext: 2},
		"Other": {UIDNext: 2, NoSelect: true},
	}
	changed := scheduler.changedFolderPaths("account-1", FolderSyncResult{Snapshots: initial}, false)
	if !changed["INBOX"] || changed["Other"] {
		t.Fatalf("initial changed paths = %v", changed)
	}
	scheduler.storeRemoteSnapshot("account-1", initial)
	initial["INBOX"] = RemoteFolderSnapshot{UIDNext: 99}
	if scheduler.remoteSnapshots["account-1"]["INBOX"].UIDNext != 2 {
		t.Fatal("storeRemoteSnapshot() retained caller-owned map")
	}

	next := map[string]RemoteFolderSnapshot{
		"INBOX":    {UIDNext: 3},
		"Projects": {UIDNext: 1},
		"Other":    {UIDNext: 3, NoSelect: true},
	}
	changed = scheduler.changedFolderPaths("account-1", FolderSyncResult{Snapshots: next}, false)
	if !changed["INBOX"] || !changed["Projects"] || changed["Other"] {
		t.Fatalf("incremental changed paths = %v", changed)
	}
	forced := scheduler.changedFolderPaths("account-1", FolderSyncResult{Snapshots: next}, true)
	if !forced["INBOX"] || !forced["Projects"] || forced["Other"] {
		t.Fatalf("forced changed paths = %v", forced)
	}

	acc := &account.Account{ID: "account-1"}
	core, err := scheduler.getAccountSyncFolders(acc)
	if err != nil || len(core) != 2 {
		t.Fatalf("core folders = %#v, %v; want inbox and sent", core, err)
	}
	acc.SyncFoldersEnabled = true
	subscribed, err := scheduler.getAccountSyncFolders(acc)
	if err != nil || len(subscribed) != 3 {
		t.Fatalf("subscribed folders = %#v, %v; want 3 selectable folders", subscribed, err)
	}
	acc.SyncAllFolders = true
	all, err := scheduler.getAccountSyncFolders(acc)
	if err != nil || len(all) != 3 {
		t.Fatalf("all folders = %#v, %v; want 3 selectable folders", all, err)
	}

	dueAccount := &account.Account{ID: "account-1", SyncInterval: 30}
	if !scheduler.isSyncDue(dueAccount) {
		t.Fatal("never-attempted account was not due")
	}
	scheduler.lastAttempts[dueAccount.ID] = time.Now()
	if scheduler.isSyncDue(dueAccount) {
		t.Fatal("recently-attempted account was due")
	}
	scheduler.lastAttempts[dueAccount.ID] = time.Now().Add(-31 * time.Minute)
	if !scheduler.isSyncDue(dueAccount) {
		t.Fatal("account beyond interval was not due")
	}
}
