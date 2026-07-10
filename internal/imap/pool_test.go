package imap

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPoolCreationFailureWakesWaiters(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.MaxConnections = 1
	cfg.WaiterTimeout = 5 * time.Second

	createStarted := make(chan struct{})
	releaseCreate := make(chan struct{})
	createErr := errors.New("credentials unavailable")
	var once sync.Once

	p := NewPool(cfg, func(accountID string) (*ClientConfig, error) {
		once.Do(func() { close(createStarted) })
		<-releaseCreate
		return nil, createErr
	})

	firstErr := make(chan error, 1)
	go func() {
		_, err := p.GetConnection(context.Background(), "acct")
		firstErr <- err
	}()

	select {
	case <-createStarted:
	case <-time.After(time.Second):
		t.Fatal("first connection attempt did not start")
	}

	secondErr := make(chan error, 1)
	go func() {
		_, err := p.GetConnection(context.Background(), "acct")
		secondErr <- err
	}()

	waitForPoolWaiters(t, p, "acct", 1)
	close(releaseCreate)

	select {
	case err := <-secondErr:
		if err == nil || !strings.Contains(err.Error(), createErr.Error()) {
			t.Fatalf("waiter error = %v, want wrapped create error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not wake after connection creation failed")
	}

	select {
	case err := <-firstErr:
		if err == nil || !strings.Contains(err.Error(), createErr.Error()) {
			t.Fatalf("creator error = %v, want wrapped create error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("creator did not return after connection creation failed")
	}
}

func TestPoolReleaseUnhealthyConnectionRemovesIt(t *testing.T) {
	p := NewPool(DefaultPoolConfig(), nil)
	conn := &PooledConnection{accountID: "acct", inUse: true}

	p.mu.Lock()
	p.connections["acct"] = []*PooledConnection{conn}
	p.mu.Unlock()

	p.Release(conn)

	p.mu.Lock()
	defer p.mu.Unlock()
	if got := len(p.connections["acct"]); got != 0 {
		t.Fatalf("pool retained unhealthy connection, got %d entries", got)
	}
}

func waitForPoolWaiters(t *testing.T, p *Pool, accountID string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		got := len(p.waiters[accountID])
		p.mu.Unlock()
		if got >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d pool waiter(s)", want)
}
