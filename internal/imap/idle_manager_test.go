package imap

import (
	"context"
	"sync"
	"testing"
)

func TestIdleManagerStartIsIdempotent(t *testing.T) {
	manager := NewIdleManager(DefaultIdleConfig(), nil)

	firstParent, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	secondParent, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()

	manager.Start(firstParent)
	firstCtx := manager.ctx
	if firstCtx == nil || manager.cancel == nil {
		t.Fatal("manager did not initialize context")
	}

	manager.Start(secondParent)
	if manager.ctx != firstCtx {
		t.Fatal("Start replaced an active manager context")
	}

	manager.Stop()
}

func TestIdleManagerStopClearsContextAndAllowsRestart(t *testing.T) {
	manager := NewIdleManager(DefaultIdleConfig(), nil)

	manager.Start(context.Background())
	manager.Stop()

	if manager.ctx != nil || manager.cancel != nil {
		t.Fatal("Stop did not clear manager context")
	}

	manager.Start(context.Background())
	if manager.ctx == nil || manager.cancel == nil {
		t.Fatal("manager did not restart after Stop")
	}
	manager.Stop()
}

func TestIdleManagerStartAccountSkippedWhenStopped(t *testing.T) {
	manager := NewIdleManager(DefaultIdleConfig(), nil)

	manager.StartAccount("account-1", "Account 1")

	manager.mu.Lock()
	_, exists := manager.connections["account-1"]
	manager.mu.Unlock()
	if exists {
		t.Fatal("StartAccount created a connection while manager was stopped")
	}
}

func TestIdleManagerConcurrentStartStop(t *testing.T) {
	manager := NewIdleManager(DefaultIdleConfig(), nil)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			manager.Start(parent)
		}()
		go func() {
			defer wg.Done()
			manager.Stop()
		}()
	}
	wg.Wait()
	manager.Stop()
}
