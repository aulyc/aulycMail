package imap

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fastIdleConfig() IdleConfig {
	return IdleConfig{
		IdleTimeout:          80 * time.Millisecond,
		ReconnectBackoff:     5 * time.Millisecond,
		MaxReconnectBackoff:  10 * time.Millisecond,
		MaxReconnectAttempts: 2,
		EventSendTimeout:     20 * time.Millisecond,
		HealthCheckEnabled:   true,
		ShutdownTimeout:      500 * time.Millisecond,
	}
}

func idleCredentials(host string, port int, password string) *ClientConfig {
	config := DefaultConfig()
	config.Host = host
	config.Port = port
	config.Security = SecurityNone
	config.Username = testIMAPUsername
	config.Password = password
	return &config
}

func waitForIdleStatus(t *testing.T, manager *IdleManager, accountID string, want IdleState) IdleStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if status, ok := manager.AccountStatus(accountID); ok && status.State == want {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	status, ok := manager.AccountStatus(accountID)
	t.Fatalf("account %q did not reach %q: status=%#v exists=%t", accountID, want, status, ok)
	return IdleStatus{}
}

func waitForIdleEvent(t *testing.T, events <-chan MailEvent, want EventType) MailEvent {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Type == want {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for IDLE event %s", want.String())
		}
	}
}

func TestIdleManagerReceivesMailboxEventsAndRestartsAccount(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	manager := NewIdleManager(fastIdleConfig(), func(accountID string) (*ClientConfig, error) {
		if accountID != "idle-account" {
			return nil, errors.New("unknown account")
		}
		return idleCredentials(host, port, testIMAPPassword), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	manager.Start(ctx)
	t.Cleanup(manager.Stop)

	manager.StartAccount("idle-account", "Idle Account")
	connected := waitForIdleStatus(t, manager, "idle-account", IdleStateIdling)
	if connected.LastConnectedAt.IsZero() || connected.ConsecutiveFailures != 0 || connected.LastErrorKind != ConnectionErrorNone {
		t.Fatalf("connected status = %#v", connected)
	}

	manager.mu.Lock()
	firstConnection := manager.connections["idle-account"]
	manager.mu.Unlock()
	manager.StartAccount("idle-account", "Idle Account")
	manager.mu.Lock()
	if manager.connections["idle-account"] != firstConnection {
		manager.mu.Unlock()
		t.Fatal("idempotent StartAccount replaced a running connection")
	}
	manager.mu.Unlock()

	client := newMemoryIMAPClient(t, host, port, testIMAPPassword)
	if err := client.Login(); err != nil {
		t.Fatalf("Login() for IDLE event source = %v", err)
	}
	uid, err := client.AppendMessage("INBOX", nil, time.Now().UTC(), []byte("Subject: idle event\r\n\r\nbody\r\n"))
	if err != nil {
		t.Fatalf("AppendMessage() for IDLE event = %v", err)
	}
	newMail := waitForIdleEvent(t, manager.Events(), EventNewMail)
	if newMail.AccountID != "idle-account" || newMail.Folder != "INBOX" || newMail.Count != 1 {
		t.Fatalf("new-mail event = %#v", newMail)
	}

	if _, err := client.SelectMailbox(context.Background(), "INBOX"); err != nil {
		t.Fatalf("SelectMailbox() for expunge = %v", err)
	}
	if err := client.DeleteMessageByUID(uid); err != nil {
		t.Fatalf("DeleteMessageByUID() for expunge = %v", err)
	}
	expunged := waitForIdleEvent(t, manager.Events(), EventExpunge)
	if expunged.AccountID != "idle-account" || expunged.Folder != "INBOX" || expunged.SeqNum != 1 {
		t.Fatalf("expunge event = %#v", expunged)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() event source = %v", err)
	}

	manager.RestartAccount("idle-account", "Idle Account")
	waitForIdleStatus(t, manager, "idle-account", IdleStateIdling)
	manager.mu.Lock()
	restartedConnection := manager.connections["idle-account"]
	manager.mu.Unlock()
	if restartedConnection == firstConnection {
		t.Fatal("RestartAccount retained the previous connection")
	}

	manager.StopAccount("idle-account")
	if _, ok := manager.AccountStatus("idle-account"); ok {
		t.Fatal("StopAccount retained account status")
	}
	manager.StopAccount("missing-account")
}

func TestIdleConnectionRejectsBadCredentials(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	connection := newIdleConnection("bad-auth", "Bad Auth", fastIdleConfig(), func(string) (*ClientConfig, error) {
		return idleCredentials(host, port, "wrong-password"), nil
	})
	connection.stopCh = make(chan struct{})
	connection.events = make(chan MailEvent, 1)
	if err := connection.ensureConnected(context.Background()); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("bad credentials error = %v", err)
	}
	if connection.client != nil {
		t.Fatal("authentication failure retained a client")
	}
}

func TestIdleConnectionStopsOfflineAndDuringBackoff(t *testing.T) {
	var credentialCalls atomic.Int32
	offline := newIdleConnection("offline", "Offline", fastIdleConfig(), func(string) (*ClientConfig, error) {
		credentialCalls.Add(1)
		return nil, errors.New("credentials should not be requested")
	})
	offline.isConnected = func() bool { return false }
	offline.Start(context.Background(), make(chan MailEvent, 1))
	select {
	case <-offline.doneCh:
	case <-time.After(time.Second):
		t.Fatal("offline IDLE connection did not stop")
	}
	if credentialCalls.Load() != 0 {
		t.Fatalf("offline connection requested credentials %d times", credentialCalls.Load())
	}
	if status := offline.Status(); status.State != IdleStateStopped {
		t.Fatalf("offline status = %#v", status)
	}
	offline.Stop()

	backoff := newIdleConnection("backoff", "Backoff", fastIdleConfig(), func(string) (*ClientConfig, error) {
		credentialCalls.Add(1)
		return nil, errors.New("temporary credential lookup failure")
	})
	backoff.Start(context.Background(), make(chan MailEvent, 1))
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := backoff.Status()
		if status.State == IdleStateBackoff && status.ConsecutiveFailures > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	failed := backoff.Status()
	if failed.State != IdleStateBackoff || failed.ConsecutiveFailures == 0 || failed.LastErrorKind != ConnectionErrorOther {
		t.Fatalf("backoff status = %#v", failed)
	}
	backoff.Stop()
	if status := backoff.Status(); status.State != IdleStateStopped {
		t.Fatalf("stopped backoff status = %#v", status)
	}
}

func TestIdleConnectionEventDeliveryNeverBlocksShutdown(t *testing.T) {
	config := fastIdleConfig()
	connection := newIdleConnection("events", "Events", config, nil)
	connection.events = make(chan MailEvent)
	connection.stopCh = make(chan struct{})

	started := time.Now()
	connection.sendEvent(MailEvent{Type: EventNewMail})
	if elapsed := time.Since(started); elapsed < config.EventSendTimeout || elapsed > 250*time.Millisecond {
		t.Fatalf("full event channel returned after %v", elapsed)
	}

	close(connection.stopCh)
	started = time.Now()
	connection.sendEvent(MailEvent{Type: EventExpunge})
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("stopping event delivery took %v", elapsed)
	}

	connection.client = nil
	if err := connection.idleCycle(context.Background()); err != nil {
		t.Fatalf("idleCycle(nil client) = %v", err)
	}
}
