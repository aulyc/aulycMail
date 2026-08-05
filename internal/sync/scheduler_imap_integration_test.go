package sync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitForAccountFinished(t *testing.T, finished <-chan bool) bool {
	t.Helper()
	select {
	case succeeded := <-finished:
		return succeeded
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for account sync lifecycle")
		return false
	}
}

func TestSchedulerBlockingAndTriggeredSyncAgainstMemoryIMAP(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	scheduler := NewScheduler(fixture.engine, fixture.engine.accountStore, fixture.folderStore)

	finished := make(chan bool, 8)
	newMail := make(chan NewMailInfo, 8)
	completed := make(chan string, 32)
	scheduler.SetAccountSyncFinishedCallback(func(accountID string, succeeded bool) {
		if accountID != syncAccountID {
			t.Errorf("finished account = %q", accountID)
		}
		finished <- succeeded
	})
	scheduler.SetNewMailCallback(func(info NewMailInfo) { newMail <- info })
	scheduler.SetSyncCompletedResultCallback(func(accountID, folderID string, result MessageSyncResult, err error) {
		if accountID != syncAccountID || err != nil {
			t.Errorf("result callback = account %q folder %q result %+v err %v", accountID, folderID, result, err)
		}
		completed <- folderID
	})
	scheduler.SetSyncCompletedCallback(func(accountID, folderID string, err error) {
		if accountID != syncAccountID || err != nil {
			t.Errorf("legacy callback = account %q folder %q err %v", accountID, folderID, err)
		}
	})

	uid1 := fixture.harness.append(t, syncPlainMessage)
	info, result, err := scheduler.SyncAccountInboxBlockingWithResult(syncAccountID)
	if err != nil {
		t.Fatalf("SyncAccountInboxBlockingWithResult() error = %v", err)
	}
	if info == nil || info.AccountID != syncAccountID || info.FolderID != syncFolderID || info.Count != 1 || result.Added != 1 || !result.Performed {
		t.Fatalf("first blocking sync = info %+v result %+v", info, result)
	}
	if local, err := fixture.messageStore.GetByUID(syncFolderID, uid1); err != nil || local == nil {
		t.Fatalf("first blocking sync local message = %#v, %v", local, err)
	}

	uid2 := fixture.harness.append(t, syncMultipartMessage)
	info, err = scheduler.SyncAccountInboxBlocking(syncAccountID)
	if err != nil || info == nil || info.Count != 1 {
		t.Fatalf("SyncAccountInboxBlocking() = %+v, %v", info, err)
	}
	if local, err := fixture.messageStore.GetByUID(syncFolderID, uid2); err != nil || local == nil {
		t.Fatalf("second blocking sync local message = %#v, %v", local, err)
	}
	if _, _, err := scheduler.SyncAccountInboxBlockingWithResult("missing-account"); err == nil {
		t.Fatal("blocking sync for missing account returned nil error")
	}

	uid3 := fixture.harness.append(t, syncSearchOnlyMessage)
	scheduler.TriggerSync(syncAccountID)
	if !waitForAccountFinished(t, finished) {
		t.Fatal("manual account sync was reported as unsuccessful")
	}
	select {
	case info := <-newMail:
		if info.Count != 1 || info.AccountID != syncAccountID || info.FolderID != syncFolderID {
			t.Fatalf("manual new-mail callback = %+v", info)
		}
	case <-time.After(time.Second):
		t.Fatal("manual account sync did not report new mail")
	}
	if local, err := fixture.messageStore.GetByUID(syncFolderID, uid3); err != nil || local == nil {
		t.Fatalf("manual sync local message = %#v, %v", local, err)
	}
	if len(scheduler.remoteSnapshots[syncAccountID]) != 8 {
		t.Fatalf("stored remote snapshots = %#v", scheduler.remoteSnapshots[syncAccountID])
	}

	completedFolders := make(map[string]bool)
	for len(completed) > 0 {
		completedFolders[<-completed] = true
	}
	for _, path := range []string{"INBOX", "Drafts", "Sent"} {
		folderRecord, err := fixture.folderStore.GetByPath(syncAccountID, path)
		if err != nil || folderRecord == nil || !completedFolders[folderRecord.ID] {
			t.Fatalf("completion for %s = folder %#v, callbacks %#v, err %v", path, folderRecord, completedFolders, err)
		}
	}

	scheduler.TriggerSync("missing-account")
	scheduler.CancelSync(syncAccountID)
	scheduler.TriggerSyncAll()
	if !waitForAccountFinished(t, finished) {
		t.Fatal("all-account sync was reported as unsuccessful")
	}

	if _, err := fixture.db.Exec(`UPDATE accounts SET sync_interval = 30 WHERE id = ?`, syncAccountID); err != nil {
		t.Fatalf("enable scheduled sync: %v", err)
	}
	scheduler.SetConnectivityCheck(func() bool { return false })
	scheduler.syncDueAccounts()
	select {
	case <-finished:
		t.Fatal("offline scheduler unexpectedly started an account sync")
	case <-time.After(30 * time.Millisecond):
	}
	scheduler.SetConnectivityCheck(func() bool { return true })
	if _, err := fixture.db.Exec(
		`UPDATE folders SET last_sync = ? WHERE id = ?`,
		time.Now().Add(-31*time.Minute),
		syncFolderID,
	); err != nil {
		t.Fatalf("age inbox sync timestamp: %v", err)
	}
	scheduler.lastAttempts[syncAccountID] = time.Now().Add(-31 * time.Minute)
	scheduler.syncDueAccounts()
	if !waitForAccountFinished(t, finished) {
		t.Fatal("scheduled account sync was reported as unsuccessful")
	}
}

func TestSchedulerBlockingSyncDiscoversMissingInboxAndHonorsCanceledContext(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	if err := fixture.folderStore.Delete(syncFolderID); err != nil {
		t.Fatalf("delete seeded inbox: %v", err)
	}
	uid := fixture.harness.append(t, syncPlainMessage)
	scheduler := NewScheduler(fixture.engine, fixture.engine.accountStore, fixture.folderStore)

	info, result, err := scheduler.SyncAccountInboxBlockingWithResult(syncAccountID)
	if err != nil || result.Added != 1 || !result.Performed {
		t.Fatalf("discovery blocking sync = info %+v result %+v err %v", info, result, err)
	}
	discovered, err := fixture.folderStore.GetByType(syncAccountID, "inbox")
	if err != nil || discovered == nil || discovered.ID == syncFolderID {
		t.Fatalf("discovered inbox = %#v, %v", discovered, err)
	}
	if local, err := fixture.messageStore.GetByUID(discovered.ID, uid); err != nil || local == nil {
		t.Fatalf("message in discovered inbox = %#v, %v", local, err)
	}
	if len(scheduler.remoteSnapshots[syncAccountID]) != 8 {
		t.Fatalf("blocking discovery snapshots = %#v", scheduler.remoteSnapshots[syncAccountID])
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler.ctx = canceled
	if _, _, err := scheduler.SyncAccountInboxBlockingWithResult(syncAccountID); !errors.Is(err, context.Canceled) {
		t.Fatalf("blocking sync with canceled scheduler context error = %v", err)
	}

	acc, err := fixture.engine.accountStore.Get(syncAccountID)
	if err != nil {
		t.Fatalf("get fixture account: %v", err)
	}
	acc.SyncAllFolders = true
	all, err := scheduler.getAccountSyncFolders(acc)
	if err != nil || len(all) != 8 {
		t.Fatalf("all selectable sync folders = %d, %v", len(all), err)
	}
	acc.SyncAllFolders = false
	acc.SyncFoldersEnabled = true
	subscribed, err := scheduler.getAccountSyncFolders(acc)
	if err != nil || len(subscribed) != 5 {
		t.Fatalf("subscribed sync folders = %d, %v", len(subscribed), err)
	}
	acc.SyncFoldersEnabled = false
}
