package message

import (
	"testing"
	"time"
)

func TestThreadDiscoveryAndReconciliationUseNormalizedMessageIDs(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()
	for _, item := range []*Message{
		{ID: "root", AccountID: accountID, FolderID: folderID, UID: 1, MessageID: "<root@example.com>", ThreadID: "<root-thread>", Date: now},
		{ID: "new-reply", AccountID: accountID, FolderID: folderID, UID: 2, MessageID: "new@example.com", InReplyTo: "<root@example.com>", ThreadID: "wrong-thread", Date: now},
		{ID: "waiting-reply", AccountID: accountID, FolderID: folderID, UID: 3, MessageID: "waiting@example.com", InReplyTo: "<new@example.com>", ThreadID: "waiting-thread", Date: now},
	} {
		if err := store.Create(item); err != nil {
			t.Fatalf("Create(%s) error = %v", item.ID, err)
		}
	}

	if got, err := store.FindThreadID(accountID, " <standalone@example.com> ", "", nil); err != nil || got != "standalone@example.com" {
		t.Fatalf("standalone FindThreadID() = %q, %v", got, err)
	}
	if got, err := store.FindThreadID(accountID, "reply@example.com", " <root@example.com> ", []string{"", "<ignored@example.com>"}); err != nil || got != "root-thread" {
		t.Fatalf("existing FindThreadID() = %q, %v; want root-thread", got, err)
	}
	if got, err := store.FindThreadID(accountID, "reply@example.com", "", []string{" <missing-root@example.com> ", "<missing-parent@example.com>"}); err != nil || got != "missing-root@example.com" {
		t.Fatalf("reference fallback FindThreadID() = %q, %v", got, err)
	}
	if got, err := store.FindThreadID(accountID, "reply@example.com", "<missing-parent@example.com>", nil); err != nil || got != "missing-parent@example.com" {
		t.Fatalf("in-reply-to fallback FindThreadID() = %q, %v", got, err)
	}

	if err := store.UpdateThreadID("new-reply", "temporary-thread"); err != nil {
		t.Fatalf("UpdateThreadID() error = %v", err)
	}
	updated, err := store.Get("new-reply")
	if err != nil || updated == nil || updated.ThreadID != "temporary-thread" {
		t.Fatalf("updated thread = %#v, %v", updated, err)
	}
	if affected, err := store.ReconcileThreads(accountID, " ", "unused"); err != nil || affected != 0 {
		t.Fatalf("empty ReconcileThreads() = %d, %v", affected, err)
	}

	if err := store.ReconcileThreadsForNewMessage(accountID, "new-reply", "<new@example.com>", "<temporary-thread>", "<root@example.com>"); err != nil {
		t.Fatalf("ReconcileThreadsForNewMessage() error = %v", err)
	}
	newReply, err := store.Get("new-reply")
	if err != nil || newReply == nil || newReply.ThreadID != "root-thread" {
		t.Fatalf("new reply thread = %#v, %v; want root-thread", newReply, err)
	}
	waiting, err := store.Get("waiting-reply")
	if err != nil || waiting == nil || waiting.ThreadID != "root-thread" {
		t.Fatalf("waiting reply thread = %#v, %v; want root-thread", waiting, err)
	}

	if affected, err := store.ReconcileThreads(accountID, "new@example.com", "root-thread"); err != nil || affected != 0 {
		t.Fatalf("idempotent ReconcileThreads() = %d, %v; want 0", affected, err)
	}
}

func TestThreadStoreReturnsErrorsAfterDatabaseClose(t *testing.T) {
	store, accountID, _ := newBodyFailedTestStore(t)
	if err := store.db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := store.FindThreadID(accountID, "child@example.com", "parent@example.com", nil); err != nil {
		// FindThreadID intentionally falls back to the supplied parent when lookup fails.
		t.Fatalf("FindThreadID() fallback error = %v", err)
	}
	if err := store.UpdateThreadID("missing", "thread"); err == nil {
		t.Fatal("UpdateThreadID() error = nil after database close")
	}
	if _, err := store.ReconcileThreads(accountID, "message@example.com", "thread"); err == nil {
		t.Fatal("ReconcileThreads() error = nil after database close")
	}
}
