package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/message"
	goImap "github.com/emersion/go-imap/v2"
)

func TestCondStoreFlagSyncUsesFullFallbackAndGuardsInvalidRequests(t *testing.T) {
	fixture := newSyncIntegrationFixture(t)
	uid := fixture.harness.append(t, syncPlainMessage, goImap.FlagSeen, goImap.FlagFlagged, goImap.FlagAnswered, goImap.FlagDraft, goImap.FlagDeleted, "$Forwarded")
	createBodyFetchMessage(t, fixture, &message.Message{
		ID: "condstore-message", UID: uid, MessageID: "plain-1@example.com", Date: time.Now().UTC(),
	})

	conn, err := fixture.pool.GetConnection(context.Background(), syncAccountID)
	if err != nil {
		t.Fatalf("get CONDSTORE fixture connection: %v", err)
	}
	defer fixture.pool.Release(conn)
	if _, err := conn.Client().SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatalf("select CONDSTORE fixture mailbox: %v", err)
	}
	rawClient := conn.Client().RawClient()

	if changed, err := fixture.engine.syncMessageFlagsChangedSince(context.Background(), rawClient, syncFolderID, 0); err == nil || changed != 0 {
		t.Fatalf("zero baseline changed flags = %d, %v", changed, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if changed, err := fixture.engine.syncMessageFlagsChangedSince(canceled, rawClient, syncFolderID, 1); !errors.Is(err, context.Canceled) || changed != 0 {
		t.Fatalf("canceled changed flags = %d, %v", changed, err)
	}

	if ok := fixture.engine.runFlagSync(context.Background(), rawClient, syncFolderID, []uint32{uid}, false, 0, 2, false); !ok {
		t.Fatal("full flag fallback failed")
	}
	stored, err := fixture.messageStore.Get("condstore-message")
	if err != nil || stored == nil || !stored.IsRead || !stored.IsStarred || !stored.IsAnswered || !stored.IsForwarded || !stored.IsDraft || !stored.IsDeleted {
		t.Fatalf("full fallback flags = (%#v, %v)", stored, err)
	}

	// The in-memory server deliberately does not advertise CONDSTORE. Forcing
	// this branch verifies that an unsupported CHANGEDSINCE command falls back
	// to the full flag fetch without advancing an untrusted baseline.
	if ok := fixture.engine.runFlagSync(context.Background(), rawClient, syncFolderID, []uint32{uid}, false, 1, 2, true); !ok {
		t.Fatal("CONDSTORE error did not recover through full flag fallback")
	}
}
