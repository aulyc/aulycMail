package imap

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
)

func TestClassifyConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ConnectionErrorKind
	}{
		{name: "cancelled", err: context.Canceled, want: ConnectionErrorCancelled},
		{name: "eof", err: io.ErrUnexpectedEOF, want: ConnectionErrorEOF},
		{name: "timeout", err: &net.DNSError{IsTimeout: true}, want: ConnectionErrorTimeout},
		{name: "authentication", err: errors.New("authentication failed: bad credentials"), want: ConnectionErrorAuthentication},
		{name: "connection limit", err: errors.New("Maximum number of connections exceeded"), want: ConnectionErrorServerLimit},
		{name: "protocol", err: errors.New("server does not support IDLE"), want: ConnectionErrorProtocol},
		{name: "other", err: errors.New("unexpected response"), want: ConnectionErrorOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyConnectionError(tt.err); got != tt.want {
				t.Fatalf("ClassifyConnectionError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestIdleStatusTracksSanitizedFailureAndRecovery(t *testing.T) {
	connection := newIdleConnection("account-1", "Account", DefaultIdleConfig(), nil)
	if got := connection.Status(); got.State != IdleStateDisconnected || got.LastErrorKind != ConnectionErrorNone {
		t.Fatalf("initial status = %#v", got)
	}

	connection.recordFailure(io.ErrUnexpectedEOF, IdleStateBackoff)
	failed := connection.Status()
	if failed.State != IdleStateBackoff || failed.ConsecutiveFailures != 1 || failed.LastErrorKind != ConnectionErrorEOF || failed.LastFailureAt.IsZero() {
		t.Fatalf("failed status = %#v", failed)
	}

	connection.recordConnected()
	recovered := connection.Status()
	if recovered.State != IdleStateIdling || recovered.ConsecutiveFailures != 0 || recovered.LastErrorKind != ConnectionErrorNone || recovered.LastConnectedAt.IsZero() {
		t.Fatalf("recovered status = %#v", recovered)
	}
}
