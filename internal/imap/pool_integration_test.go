package imap

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func memoryPoolConfig(host string, port int) ClientConfig {
	config := DefaultConfig()
	config.Host = host
	config.Port = port
	config.Security = SecurityNone
	config.Username = testIMAPUsername
	config.Password = testIMAPPassword
	config.ConnectTimeout = 2 * time.Second
	config.ReadTimeout = 2 * time.Second
	config.WriteTimeout = 2 * time.Second
	return config
}

func TestPoolLifecycleAgainstMemoryServer(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	clientConfig := memoryPoolConfig(host, port)
	poolConfig := DefaultPoolConfig()
	poolConfig.MaxConnections = 1
	poolConfig.IdleTimeout = 5 * time.Millisecond
	poolConfig.WaiterTimeout = 250 * time.Millisecond

	var credentialCalls atomic.Int32
	pool := NewPool(poolConfig, func(accountID string) (*ClientConfig, error) {
		if accountID != "primary" {
			return nil, errors.New("unknown account")
		}
		credentialCalls.Add(1)
		config := clientConfig
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)

	first, err := pool.GetConnection(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetConnection(first) error = %v", err)
	}
	if first.Client() == nil || !first.IsHealthy() {
		t.Fatal("first pooled connection is not healthy")
	}
	if stats := pool.GetStats(); stats.TotalConnections != 1 || stats.ActiveConnections != 1 || stats.IdleConnections != 0 || stats.AccountCount != 1 {
		t.Fatalf("active stats = %+v", stats)
	}

	waiting := make(chan *PooledConnection, 1)
	waitErr := make(chan error, 1)
	go func() {
		conn, getErr := pool.GetConnection(context.Background(), "primary")
		if getErr != nil {
			waitErr <- getErr
			return
		}
		waiting <- conn
	}()
	waitForPoolWaiters(t, pool, "primary", 1)
	pool.Release(first)

	var second *PooledConnection
	select {
	case second = <-waiting:
	case err := <-waitErr:
		t.Fatalf("waiting GetConnection() error = %v", err)
	case <-time.After(time.Second):
		t.Fatal("waiting GetConnection() did not receive released connection")
	}
	if second != first {
		t.Fatal("pool did not hand the released connection to its waiter")
	}
	if credentialCalls.Load() != 1 {
		t.Fatalf("credential provider calls = %d, want 1", credentialCalls.Load())
	}
	pool.Release(second)
	if stats := pool.GetStats(); stats.IdleConnections != 1 || stats.ActiveConnections != 0 {
		t.Fatalf("idle stats = %+v", stats)
	}

	reused, err := pool.GetConnection(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetConnection(reused) error = %v", err)
	}
	if reused != first || credentialCalls.Load() != 1 {
		t.Fatalf("reuse = %p (want %p), credential calls = %d", reused, first, credentialCalls.Load())
	}
	pool.Release(reused)
	time.Sleep(10 * time.Millisecond)
	pool.CleanupIdle()
	if stats := pool.GetStats(); stats.TotalConnections != 0 || stats.AccountCount != 0 {
		t.Fatalf("stats after idle cleanup = %+v", stats)
	}
}

func TestPoolWaitCancellationTimeoutDiscardAndClose(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	clientConfig := memoryPoolConfig(host, port)
	poolConfig := DefaultPoolConfig()
	poolConfig.MaxConnections = 1
	poolConfig.WaiterTimeout = 20 * time.Millisecond

	pool := NewPool(poolConfig, func(accountID string) (*ClientConfig, error) {
		if accountID == "missing" {
			return nil, errors.New("credentials missing")
		}
		config := clientConfig
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)

	if _, err := pool.GetConnection(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "failed to get credentials") {
		t.Fatalf("GetConnection(missing credentials) error = %v", err)
	}

	first, err := pool.GetConnection(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetConnection(first) error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pool.GetConnection(canceled, "primary"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetConnection(canceled) error = %v", err)
	}
	if _, err := pool.GetConnection(context.Background(), "primary"); err == nil || !strings.Contains(err.Error(), "timed out waiting") {
		t.Fatalf("GetConnection(timeout) error = %v", err)
	}

	pool.Discard(first)
	if first.IsHealthy() {
		t.Fatal("discarded connection remains healthy")
	}
	if stats := pool.GetStats(); stats.TotalConnections != 0 {
		t.Fatalf("stats after discard = %+v", stats)
	}

	replacement, err := pool.GetConnection(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetConnection(replacement) error = %v", err)
	}
	pool.CloseAccount("primary")
	if replacement.Client() != nil {
		t.Fatal("CloseAccount did not detach the client")
	}
	pool.Release(replacement)
	pool.CloseAccount("unknown")
	if stats := pool.GetStats(); stats.TotalConnections != 0 || stats.AccountCount != 0 {
		t.Fatalf("stats after CloseAccount = %+v", stats)
	}

	primary, err := pool.GetConnection(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetConnection(primary again) error = %v", err)
	}
	secondary, err := pool.GetConnection(context.Background(), "secondary")
	if err != nil {
		t.Fatalf("GetConnection(secondary) error = %v", err)
	}
	if stats := pool.GetStats(); stats.TotalConnections != 2 || stats.AccountCount != 2 {
		t.Fatalf("two-account stats = %+v", stats)
	}
	pool.CloseAll()
	if primary.Client() != nil || secondary.Client() != nil {
		t.Fatal("CloseAll did not detach all clients")
	}
	if stats := pool.GetStats(); stats.TotalConnections != 0 || stats.AccountCount != 0 {
		t.Fatalf("stats after CloseAll = %+v", stats)
	}
}
