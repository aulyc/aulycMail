package app

import (
	extcontactsbe "github.com/aulyc/aulycmail/extensions/contacts/backend"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// initContactsExtension wires the built-in Contacts bridge into App during
// Startup. The bridge stays lazy: it constructs the contact store/API only
// when a Contacts_* Wails method is first called.
func (a *App) initContactsExtension() {
	// Single emitter for all contacts:* frontend events (conflict, changed).
	emit := func(eventName string, payload any) {
		wailsRuntime.EventsEmit(a.ctx, eventName, payload)
	}

	a.ContactsBridge = extcontactsbe.NewContactsBridge(extcontactsbe.ContactsBridgeDeps{
		DB:      a.db,
		Emitter: emit,
	})
}
