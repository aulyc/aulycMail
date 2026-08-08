package app

import (
	"context"
	"sync"
	"time"
)

// SyncBridge owns the Wails sync surface and all process-local synchronization
// state for one App instance.
type SyncBridge struct {
	app *App

	mu          sync.Mutex
	contexts    map[string]context.CancelFunc
	lastRequest map[string]time.Time
	cancelled   bool
	wakeSyncing bool

	// accountSyncTimeout is configurable only for deterministic tests. A zero
	// value uses the production default.
	accountSyncTimeout time.Duration
}

func (a *App) initSyncBridge() {
	if a.SyncBridge == nil {
		a.SyncBridge = &SyncBridge{
			app:         a,
			contexts:    make(map[string]context.CancelFunc),
			lastRequest: make(map[string]time.Time),
		}
	}
}

// DraftBridge owns the Wails draft surface, draft operations, and cancellation
// state for one App instance.
type DraftBridge struct {
	app *App
	ops draftOps

	mu           sync.Mutex
	syncContexts map[string]context.CancelFunc
	syncDone     map[string]chan struct{}
}

func (a *App) initDraftBridge() {
	if a.DraftBridge == nil {
		a.DraftBridge = &DraftBridge{
			app:          a,
			syncContexts: make(map[string]context.CancelFunc),
			syncDone:     make(map[string]chan struct{}),
		}
	}
}
