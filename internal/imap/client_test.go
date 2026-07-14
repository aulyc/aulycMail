package imap

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/rs/zerolog"
)

func TestSelectMailboxCancellationClosesConnection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	rawClient := imapclient.New(clientConn, nil)
	command := make(chan string, 1)
	go func() {
		if _, err := serverConn.Write([]byte("* OK test server ready\r\n")); err != nil {
			return
		}
		reader := bufio.NewReader(serverConn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(line, " CAPABILITY") {
				fields := strings.Fields(line)
				if len(fields) == 0 {
					return
				}
				response := "* CAPABILITY IMAP4rev1\r\n" + fields[0] + " OK CAPABILITY completed\r\n"
				if _, err := serverConn.Write([]byte(response)); err != nil {
					return
				}
				continue
			}
			if strings.Contains(line, " SELECT ") {
				command <- line
				// Deliberately withhold the tagged SELECT response until the
				// client cancels and closes this connection.
			}
		}
	}()
	if err := rawClient.WaitGreeting(); err != nil {
		t.Fatalf("wait for greeting: %v", err)
	}

	client := &Client{client: rawClient, log: zerolog.Nop()}
	ctx, cancel := context.WithCancel(context.Background())
	selectErr := make(chan error, 1)
	go func() {
		_, err := client.SelectMailbox(ctx, "INBOX")
		selectErr <- err
	}()

	select {
	case line := <-command:
		if !strings.Contains(line, " SELECT ") {
			t.Fatalf("expected SELECT command, got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SELECT command")
	}

	cancel()
	select {
	case err := <-selectErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled SELECT did not return")
	}

	noopErr := make(chan error, 1)
	go func() {
		noopErr <- rawClient.Noop().Wait()
	}()
	select {
	case err := <-noopErr:
		if err == nil {
			t.Fatal("expected cancelled SELECT connection to be unusable")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closed connection did not reject NOOP")
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "random error",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "connection reset",
			err:  errors.New("connection reset by peer"),
			want: true,
		},
		{
			name: "EOF",
			err:  errors.New("unexpected EOF"),
			want: true,
		},
		{
			name: "broken pipe",
			err:  errors.New("write: broken pipe"),
			want: true,
		},
		{
			name: "i/o timeout",
			err:  errors.New("read: i/o timeout"),
			want: true,
		},
		{
			name: "connection refused",
			err:  errors.New("dial tcp: connection refused"),
			want: true,
		},
		{
			name: "no such host",
			err:  errors.New("dial tcp: lookup mail.example.com: no such host"),
			want: true,
		},
		{
			name: "network is unreachable",
			err:  errors.New("dial tcp: network is unreachable"),
			want: true,
		},
		{
			name: "use of closed network connection",
			err:  errors.New("use of closed network connection"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsConnectionError(tt.err)
			if got != tt.want {
				t.Errorf("IsConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 993 {
		t.Errorf("DefaultConfig().Port = %d, want 993", cfg.Port)
	}
	if cfg.Security != SecurityTLS {
		t.Errorf("DefaultConfig().Security = %q, want %q", cfg.Security, SecurityTLS)
	}
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("DefaultConfig().ConnectTimeout = %v, want %v", cfg.ConnectTimeout, 30*time.Second)
	}
	if cfg.ReadTimeout != 3*time.Minute {
		t.Errorf("DefaultConfig().ReadTimeout = %v, want %v", cfg.ReadTimeout, 3*time.Minute)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("DefaultConfig().WriteTimeout = %v, want %v", cfg.WriteTimeout, 30*time.Second)
	}
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxConnections != 3 {
		t.Errorf("DefaultPoolConfig().MaxConnections = %d, want 3", cfg.MaxConnections)
	}
	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("DefaultPoolConfig().IdleTimeout = %v, want %v", cfg.IdleTimeout, 5*time.Minute)
	}
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("DefaultPoolConfig().ConnectTimeout = %v, want %v", cfg.ConnectTimeout, 30*time.Second)
	}
	if cfg.WaiterTimeout != 2*time.Minute {
		t.Errorf("DefaultPoolConfig().WaiterTimeout = %v, want %v", cfg.WaiterTimeout, 2*time.Minute)
	}
}

func TestDefaultIdleConfig(t *testing.T) {
	cfg := DefaultIdleConfig()

	if cfg.IdleTimeout != 10*time.Minute {
		t.Errorf("DefaultIdleConfig().IdleTimeout = %v, want %v", cfg.IdleTimeout, 10*time.Minute)
	}
	if cfg.ReconnectBackoff != 1*time.Second {
		t.Errorf("DefaultIdleConfig().ReconnectBackoff = %v, want %v", cfg.ReconnectBackoff, 1*time.Second)
	}
	if cfg.MaxReconnectBackoff != 5*time.Minute {
		t.Errorf("DefaultIdleConfig().MaxReconnectBackoff = %v, want %v", cfg.MaxReconnectBackoff, 5*time.Minute)
	}
	if cfg.MaxReconnectAttempts != 10 {
		t.Errorf("DefaultIdleConfig().MaxReconnectAttempts = %d, want 10", cfg.MaxReconnectAttempts)
	}
}
