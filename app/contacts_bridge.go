package app

import (
	"aulyc.local/aulycmail/internal/contactpane"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// initContactsBridge wires the built-in Contacts bridge into App during
// Startup. The bridge stays lazy: it constructs the contact store/API only
// when a Contacts_* Wails method is first called.
func (a *App) initContactsBridge() {
	// Single emitter for all contacts:* frontend events (conflict, changed).
	emit := func(eventName string, payload any) {
		wailsRuntime.EventsEmit(a.ctx, eventName, payload)
	}

	a.ContactsBridge = contactpane.NewContactsBridge(contactpane.ContactsBridgeDeps{
		DB:      a.db,
		Emitter: emit,
	})
}
