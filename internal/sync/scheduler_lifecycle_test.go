package sync

import (
	"testing"

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
