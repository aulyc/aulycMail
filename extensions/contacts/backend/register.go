package backend

import (
	"fmt"

	"github.com/aulyc/aulycmail/extensions/contacts"
	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

// Extension is the Contacts extension's lifecycle handle. It's intentionally
// tiny — just the manifest plus the Register handshake. The Wails-bound
// surface lives on Bridge (bridge.go), and the actual Contacts logic
// (stores, API, sync) is owned by the API constructed inside Bridge's
// lazy-init path.
//
// This separation is what makes the lightweight-by-default invariant hold:
// the host always calls Register on every known extension at startup (the
// architecture-doc rule for descriptive UI registrations), but doing so
// allocates only a manifest copy + this struct. No stores, no SQLite, no
// API — those are deferred to the first enabled Bridge method call.
type Extension struct {
	manifest coreapi.Manifest
}

// NewExtension constructs the Extension lifecycle handle. Takes no
// arguments because nothing in Register depends on host state beyond
// the Core handle passed in at registration time.
func NewExtension() *Extension {
	return &Extension{manifest: contacts.Manifest()}
}

// Manifest returns the parsed manifest embedded at build time.
func (e *Extension) Manifest() coreapi.Manifest { return e.manifest }

// Register wires the Contacts extension's descriptive UI surface. Runs once per
// aulycmail process lifetime, at App.Startup, regardless of whether the
// extension is currently enabled. The frontend filters rail tabs by enabled
// state at render time.
//
// Returns an Unregister func that tears all registrations down. Called by
// the host on process shutdown.
func (e *Extension) Register(core coreapi.Core) (coreapi.Unregister, error) {
	unregRail, err := core.UI().RegisterRailTab(coreapi.RailTabRequest{
		ExtensionID: e.manifest.ID,
		Label:       e.manifest.Name,
		Icon:        "mdi:account-multiple",
		Component:   "ContactsPane",
		Order:       10,
	})
	if err != nil {
		return nil, fmt.Errorf("contacts: register rail tab: %w", err)
	}

	return func() {
		unregRail()
	}, nil
}

// compile-time check: *Extension satisfies coreapi.Extension
var _ coreapi.Extension = (*Extension)(nil)
