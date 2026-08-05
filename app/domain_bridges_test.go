package app

import (
	"context"
	"testing"
	"time"
)

func TestSyncAndDraftBridgeStateIsIsolatedPerAppInstance(t *testing.T) {
	first := NewApp(nil)
	second := NewApp(nil)

	if first.SyncBridge == nil || second.SyncBridge == nil {
		t.Fatal("NewApp must initialize an independent SyncBridge")
	}
	if first.DraftBridge == nil || second.DraftBridge == nil {
		t.Fatal("NewApp must initialize an independent DraftBridge")
	}

	first.SyncBridge.lastRequest["account:folder"] = time.Now()
	if _, leaked := second.SyncBridge.lastRequest["account:folder"]; leaked {
		t.Fatal("sync debounce state leaked between App instances")
	}

	_, cancel := context.WithCancel(context.Background())
	first.DraftBridge.syncContexts["draft"] = cancel
	if _, leaked := second.DraftBridge.syncContexts["draft"]; leaked {
		t.Fatal("draft sync cancellation state leaked between App instances")
	}
}
