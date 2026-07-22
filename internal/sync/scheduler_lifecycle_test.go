package sync

import "testing"

func TestRunAccountSyncLifecycleFinishesAfterNoChangeWork(t *testing.T) {
	scheduler := &Scheduler{}
	workFinished := false
	var finishedAccount string

	scheduler.SetAccountSyncFinishedCallback(func(accountID string) {
		if !workFinished {
			t.Fatal("account sync finished callback ran before the work returned")
		}
		finishedAccount = accountID
	})

	scheduler.runAccountSyncLifecycle("account-1", func() {
		// A scheduled remote probe with no changed folders has no folder-level
		// completion callback, but the account-level lifecycle must still finish.
		workFinished = true
	})

	if finishedAccount != "account-1" {
		t.Fatalf("finished account = %q, want account-1", finishedAccount)
	}
}
