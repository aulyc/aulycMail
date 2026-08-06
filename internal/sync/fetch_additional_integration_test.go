package sync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func createBodyFetchMessage(t *testing.T, fixture *syncIntegrationFixture, item *message.Message) {
	t.Helper()
	if item.AccountID == "" {
		item.AccountID = syncAccountID
	}
	if item.FolderID == "" {
		item.FolderID = syncFolderID
	}
	if item.Date.IsZero() {
		item.Date = time.Now().UTC()
	}
	if item.ReceivedAt.IsZero() {
		item.ReceivedAt = item.Date
	}
	if err := fixture.messageStore.Create(item); err != nil {
		t.Fatalf("create body-fetch message %s: %v", item.ID, err)
	}
}

func bodyFailedValue(t *testing.T, fixture *syncIntegrationFixture, messageID string) int {
	t.Helper()
	var value int
	if err := fixture.db.QueryRow(`SELECT body_failed FROM messages WHERE id = ?`, messageID).Scan(&value); err != nil {
		t.Fatalf("query body_failed for %s: %v", messageID, err)
	}
	return value
}

func TestFetchMessageBodyRejectsMissingRemoteIdentityAndMailbox(t *testing.T) {
	t.Run("missing remote UID", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		createBodyFetchMessage(t, fixture, &message.Message{ID: "missing-remote", UID: 999, Size: 128})

		_, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, "missing-remote")
		var notFound RawMessageNotFoundError
		if !errors.As(err, &notFound) || notFound.UID != 999 {
			t.Fatalf("FetchMessageBody(missing remote) error = %v", err)
		}
	})

	t.Run("message identity mismatch", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		uid := fixture.harness.append(t, syncPlainMessage)
		createBodyFetchMessage(t, fixture, &message.Message{
			ID: "identity-mismatch", UID: uid, Size: len(syncPlainMessage),
			MessageID: "different-message@example.com",
		})

		_, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, "identity-mismatch")
		if !IsMessageIdentityMismatchError(err) {
			t.Fatalf("FetchMessageBody(identity mismatch) error = %v", err)
		}
		stored, getErr := fixture.messageStore.Get("identity-mismatch")
		if getErr != nil || stored == nil || stored.BodyFetched {
			t.Fatalf("identity-mismatched body mutated local row = (%#v, %v)", stored, getErr)
		}
	})

	t.Run("unknown account", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		createBodyFetchMessage(t, fixture, &message.Message{ID: "unknown-account", UID: 1})

		_, err := fixture.engine.FetchMessageBody(context.Background(), "unknown-account", "unknown-account")
		if err == nil || !strings.Contains(err.Error(), "failed to get connection") {
			t.Fatalf("FetchMessageBody(unknown account) error = %v", err)
		}
	})

	t.Run("missing mailbox", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		missingMailbox := &folder.Folder{
			ID: "missing-mailbox", AccountID: syncAccountID, Name: "Missing", Path: "Missing",
			Type: folder.TypeFolder, Subscribed: true,
		}
		if err := fixture.folderStore.Create(missingMailbox); err != nil {
			t.Fatalf("create missing mailbox folder: %v", err)
		}
		createBodyFetchMessage(t, fixture, &message.Message{ID: "missing-mailbox-message", FolderID: missingMailbox.ID, UID: 1})

		_, err := fixture.engine.FetchMessageBody(context.Background(), syncAccountID, "missing-mailbox-message")
		if err == nil || !strings.Contains(err.Error(), "failed to select mailbox") {
			t.Fatalf("FetchMessageBody(missing mailbox) error = %v", err)
		}
	})
}

func TestFetchBodiesInBackgroundBoundsMissingAndOversizedBodies(t *testing.T) {
	t.Run("empty folder completes without fetching", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		var progress []SyncProgress
		fixture.engine.SetProgressCallback(func(update SyncProgress) { progress = append(progress, update) })

		if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, syncFolderID, 30); err != nil {
			t.Fatalf("FetchBodiesInBackground(empty) error = %v", err)
		}
		if len(progress) != 1 || progress[0].Fetched != 1 || progress[0].Total != 1 || progress[0].Phase != "bodies" {
			t.Fatalf("empty body-fetch progress = %#v", progress)
		}
	})

	t.Run("missing remote UID is persisted as failed", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		createBodyFetchMessage(t, fixture, &message.Message{ID: "background-missing", UID: 999, Size: 256})

		if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, syncFolderID, 0); err != nil {
			t.Fatalf("FetchBodiesInBackground(missing UID) error = %v", err)
		}
		if got := bodyFailedValue(t, fixture, "background-missing"); got != 1 {
			t.Fatalf("body_failed for missing UID = %d, want 1", got)
		}
	})

	t.Run("oversized candidate is skipped before IMAP fetch", func(t *testing.T) {
		fixture := newSyncIntegrationFixture(t)
		createBodyFetchMessage(t, fixture, &message.Message{
			ID: "background-oversized", UID: 999, Size: maxBackgroundRawBodyBytes + 1,
		})

		if err := fixture.engine.FetchBodiesInBackground(context.Background(), syncAccountID, syncFolderID, 0); err != nil {
			t.Fatalf("FetchBodiesInBackground(oversized) error = %v", err)
		}
		if got := bodyFailedValue(t, fixture, "background-oversized"); got != 1 {
			t.Fatalf("body_failed for oversized message = %d, want 1", got)
		}
	})
}
