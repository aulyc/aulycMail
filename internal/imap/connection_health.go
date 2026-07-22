package imap

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

type ConnectionErrorKind string

const (
	ConnectionErrorNone           ConnectionErrorKind = "none"
	ConnectionErrorCancelled      ConnectionErrorKind = "cancelled"
	ConnectionErrorTimeout        ConnectionErrorKind = "timeout"
	ConnectionErrorEOF            ConnectionErrorKind = "eof"
	ConnectionErrorAuthentication ConnectionErrorKind = "authentication"
	ConnectionErrorServerLimit    ConnectionErrorKind = "server_limit"
	ConnectionErrorProtocol       ConnectionErrorKind = "protocol"
	ConnectionErrorOther          ConnectionErrorKind = "other"
)

type IdleState string

const (
	IdleStateDisconnected IdleState = "disconnected"
	IdleStateConnecting   IdleState = "connecting"
	IdleStateIdling       IdleState = "idling"
	IdleStateBackoff      IdleState = "backoff"
	IdleStateStopped      IdleState = "stopped"
)

type IdleStatus struct {
	State               IdleState
	ConsecutiveFailures int
	LastErrorKind       ConnectionErrorKind
	LastConnectedAt     time.Time
	LastFailureAt       time.Time
}

func ClassifyConnectionError(err error) ConnectionErrorKind {
	if err == nil {
		return ConnectionErrorNone
	}
	if errors.Is(err, context.Canceled) {
		return ConnectionErrorCancelled
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ConnectionErrorEOF
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ConnectionErrorTimeout
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authentication"), strings.Contains(message, "bad credentials"), strings.Contains(message, "login failed"):
		return ConnectionErrorAuthentication
	case strings.Contains(message, "maximum number of connections"), strings.Contains(message, "too many connections"):
		return ConnectionErrorServerLimit
	case strings.Contains(message, "does not support idle"), strings.Contains(message, "protocol"):
		return ConnectionErrorProtocol
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return ConnectionErrorTimeout
	case strings.Contains(message, "unexpected eof"):
		return ConnectionErrorEOF
	default:
		return ConnectionErrorOther
	}
}
