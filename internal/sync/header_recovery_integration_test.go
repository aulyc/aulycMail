package sync

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

func TestRecoverFailedHeaderBatchFetchesParsesAndPersistsHeaders(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	internalDate := time.Date(2026, time.August, 5, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	raw := []byte(strings.Join([]string{
		"From: =?UTF-8?B?5byg5LiJ?= <sender@example.com>",
		"To: Receiver <receiver@example.com>",
		"Cc: Copy <copy@example.com>",
		"Reply-To: reply@example.com",
		"Subject: =?UTF-8?B?5oGi5aSN5rWL6K+V?=",
		"Message-ID: <recovered@example.com>",
		"In-Reply-To: <parent@example.com>",
		"References: <root@example.com> <parent@example.com>",
		"Disposition-Notification-To: receipts@example.com",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
		"",
	}, "\r\n"))
	uid := fixture.harness.appendAt(t, internalDate, raw, imap.FlagSeen, imap.FlagFlagged, imap.FlagAnswered, imap.FlagDraft, imap.FlagDeleted)

	connection, err := fixture.pool.GetConnection(context.Background(), syncAccountID)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.pool.Release(connection)
	if _, err := connection.Client().SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatal(err)
	}
	recovered, err := fixture.engine.recoverFailedHeaderBatch(
		context.Background(), connection.Client().RawClient(), syncAccountID, syncFolderID, []uint32{uid},
	)
	if err != nil {
		t.Fatalf("recoverFailedHeaderBatch() error = %v", err)
	}
	if len(recovered) != 1 {
		t.Fatalf("recovered = %#v", recovered)
	}
	message := recovered[0]
	if message.UID != uid || message.Subject != "恢复测试" || message.FromName != "张三" || message.FromEmail != "sender@example.com" {
		t.Fatalf("recovered identity = %#v", message)
	}
	if !message.Date.Equal(internalDate.UTC()) || message.Size != len(raw) {
		t.Fatalf("recovered metadata = %#v", message)
	}
	if !message.IsRead || !message.IsStarred || !message.IsAnswered || !message.IsDraft || !message.IsDeleted || message.BodyFetched {
		t.Fatalf("recovered flags = %#v", message)
	}
	if message.ReadReceiptTo != "receipts@example.com" || message.ReplyTo != "reply@example.com" {
		t.Fatalf("recovered notification headers = %#v", message)
	}
	var references []string
	if err := json.Unmarshal([]byte(message.References), &references); err != nil || len(references) != 2 {
		t.Fatalf("references = %q, %v", message.References, err)
	}
	persisted, err := fixture.messageStore.GetByUID(syncFolderID, uid)
	if err != nil || persisted == nil || persisted.Subject != message.Subject || persisted.ID == "" {
		t.Fatalf("persisted recovery = %#v, %v", persisted, err)
	}
}

func TestRecoverFailedHeaderBatchHandlesEmptyAndCancelledRequests(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	if recovered, err := fixture.engine.recoverFailedHeaderBatch(context.Background(), nil, syncAccountID, syncFolderID, nil); err != nil || recovered != nil {
		t.Fatalf("empty recovery = %#v, %v", recovered, err)
	}

	uid := fixture.harness.append(t, syncPlainMessage)
	connection, err := fixture.pool.GetConnection(context.Background(), syncAccountID)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.pool.Release(connection)
	if _, err := connection.Client().SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recovered, err := fixture.engine.recoverFailedHeaderBatch(ctx, connection.Client().RawClient(), syncAccountID, syncFolderID, []uint32{uid})
	if !errors.Is(err, context.Canceled) || len(recovered) != 0 {
		t.Fatalf("cancelled recovery = %#v, %v", recovered, err)
	}
}

func TestRecoverFailedHeaderBatchDropsMessagesWhenPersistenceIsUnavailable(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	uid := fixture.harness.append(t, syncPlainMessage)
	connection, err := fixture.pool.GetConnection(context.Background(), syncAccountID)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.pool.Release(connection)
	if _, err := connection.Client().SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := fixture.engine.recoverFailedHeaderBatch(
		context.Background(), connection.Client().RawClient(), syncAccountID, syncFolderID, []uint32{uid},
	)
	if err != nil {
		t.Fatalf("recovery should degrade after persistence failure: %v", err)
	}
	if len(recovered) != 0 {
		t.Fatalf("unpersisted messages were reported as recovered: %#v", recovered)
	}
}
