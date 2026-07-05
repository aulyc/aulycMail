// Package platform provides platform-specific functionality
package platform

import (
	"context"
	"time"
)

// SleepWakeEvent represents a system sleep or wake event
type SleepWakeEvent struct {
	IsSleeping bool      // true = going to sleep, false = waking up
	Timestamp  time.Time // When the event occurred
}

// SleepWakeMonitor monitors system sleep/wake events
type SleepWakeMonitor interface {
	// Start begins monitoring for sleep/wake events
	Start(ctx context.Context) error

	// Events returns a channel that receives sleep/wake events
	Events() <-chan SleepWakeEvent

	// Stop stops the monitor and cleans up resources
	Stop() error
}

// NetworkEvent represents a network connectivity change
type NetworkEvent struct {
	Connected bool      // true = network available, false = offline
	Timestamp time.Time // When the event occurred
}

// NetworkMonitor monitors network connectivity changes.
// Implementations should be event-driven (zero polling) using OS APIs.
type NetworkMonitor interface {
	// Start begins monitoring for network connectivity changes
	Start(ctx context.Context) error

	// Events returns a channel that receives network connectivity change events
	Events() <-chan NetworkEvent

	// IsConnected returns the current connectivity state
	IsConnected() bool

	// WaitForConnection blocks until network is available or context is cancelled.
	// Returns true if connected, false if context was cancelled.
	WaitForConnection(ctx context.Context) bool

	// Invalidate resets the cached connectivity state to disconnected.
	// Call this when connections are known to be dead (e.g. system sleep)
	// so that WaitForConnection will wait for a fresh signal.
	Invalidate()

	// Stop stops the monitor and cleans up resources
	Stop() error
}
