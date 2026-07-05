package sync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAcquireMessageSyncSerializesSameFolder(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)

	releaseFirst, err := engine.acquireMessageSync(context.Background(), "account-1", "folder-1")
	if err != nil {
		t.Fatalf("acquire first sync: %v", err)
	}
	defer releaseFirst()

	acquiredSecond := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		releaseSecond, err := engine.acquireMessageSync(context.Background(), "account-1", "folder-1")
		if err != nil {
			secondDone <- err
			return
		}
		close(acquiredSecond)
		releaseSecond()
		secondDone <- nil
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("second sync acquired the same folder before first released")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()

	select {
	case <-acquiredSecond:
	case <-time.After(time.Second):
		t.Fatal("second sync did not acquire after first released")
	}

	if err := <-secondDone; err != nil {
		t.Fatalf("second sync returned error: %v", err)
	}
}

func TestAcquireMessageSyncAllowsDifferentFolders(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)

	releaseFirst, err := engine.acquireMessageSync(context.Background(), "account-1", "folder-1")
	if err != nil {
		t.Fatalf("acquire first sync: %v", err)
	}
	defer releaseFirst()

	releaseSecond, err := engine.acquireMessageSync(context.Background(), "account-1", "folder-2")
	if err != nil {
		t.Fatalf("different folder should acquire immediately: %v", err)
	}
	releaseSecond()
}

func TestAcquireMessageSyncWaitHonorsContextCancellation(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)

	releaseFirst, err := engine.acquireMessageSync(context.Background(), "account-1", "folder-1")
	if err != nil {
		t.Fatalf("acquire first sync: %v", err)
	}
	defer releaseFirst()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	releaseSecond, err := engine.acquireMessageSync(ctx, "account-1", "folder-1")
	if releaseSecond != nil {
		releaseSecond()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("second sync error = %v, want context.Canceled", err)
	}
}
