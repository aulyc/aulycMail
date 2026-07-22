package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoordinatorRunsFirstIdleImmediatelyAndCoalescesFollowUp(t *testing.T) {
	coordinator := NewCoordinator()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	var runs atomic.Int32

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- coordinator.Do(context.Background(), "account-1", TriggerIdle, func(context.Context) error {
			if runs.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst
			}
			return nil
		})
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first IDLE trigger did not start immediately")
	}

	followUpDone := make(chan error, 1)
	go func() {
		followUpDone <- coordinator.Do(context.Background(), "account-1", TriggerIdle, func(context.Context) error {
			runs.Add(1)
			close(secondStarted)
			return nil
		})
	}()
	deadline := time.Now().Add(time.Second)
	for {
		coordinator.mu.Lock()
		state := coordinator.accounts["account-1"]
		pending := state != nil && state.pending != nil
		coordinator.mu.Unlock()
		if pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("follow-up trigger was not queued")
		}
		time.Sleep(time.Millisecond)
	}

	thirdErr := coordinator.Do(context.Background(), "account-1", TriggerIdle, func(context.Context) error {
		runs.Add(1)
		return nil
	})
	if !errors.Is(thirdErr, ErrCoalesced) {
		t.Fatalf("third trigger error = %v, want ErrCoalesced", thirdErr)
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	if err := <-followUpDone; err != nil {
		t.Fatalf("follow-up trigger: %v", err)
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("coalesced follow-up did not run")
	}
	if got := runs.Load(); got != 2 {
		t.Fatalf("runs = %d, want 2", got)
	}
}

func TestCoordinatorManualPreemptsScheduledWork(t *testing.T) {
	coordinator := NewCoordinator()
	scheduledStarted := make(chan struct{})
	scheduledCancelled := make(chan struct{})
	manualStarted := make(chan struct{})

	scheduledDone := make(chan error, 1)
	go func() {
		scheduledDone <- coordinator.Do(context.Background(), "account-1", TriggerScheduled, func(ctx context.Context) error {
			close(scheduledStarted)
			<-ctx.Done()
			close(scheduledCancelled)
			return ctx.Err()
		})
	}()
	<-scheduledStarted

	manualDone := make(chan error, 1)
	go func() {
		manualDone <- coordinator.Do(context.Background(), "account-1", TriggerManual, func(context.Context) error {
			close(manualStarted)
			return nil
		})
	}()

	select {
	case <-scheduledCancelled:
	case <-time.After(time.Second):
		t.Fatal("manual trigger did not cancel scheduled work")
	}
	if err := <-scheduledDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("scheduled trigger error = %v, want context.Canceled", err)
	}
	if err := <-manualDone; err != nil {
		t.Fatalf("manual trigger: %v", err)
	}
	select {
	case <-manualStarted:
	case <-time.After(time.Second):
		t.Fatal("manual trigger did not run after preemption")
	}
}

func TestCoordinatorNeverDropsQueuedManualWork(t *testing.T) {
	coordinator := NewCoordinator()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var runs atomic.Int32

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- coordinator.Do(context.Background(), "account-1", TriggerManual, func(context.Context) error {
			runs.Add(1)
			close(firstStarted)
			<-releaseFirst
			return nil
		})
	}()
	<-firstStarted

	manualDone := make(chan error, 2)
	for range 2 {
		go func() {
			manualDone <- coordinator.Do(context.Background(), "account-1", TriggerManual, func(context.Context) error {
				runs.Add(1)
				return nil
			})
		}()
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first manual trigger: %v", err)
	}
	for range 2 {
		if err := <-manualDone; err != nil {
			t.Fatalf("queued manual trigger: %v", err)
		}
	}
	if got := runs.Load(); got != 3 {
		t.Fatalf("manual runs = %d, want 3", got)
	}
}
