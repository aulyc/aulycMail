package backend

import (
	"fmt"

	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

const (
	// PaneID is the stable rail id used by the built-in Contacts pane.
	PaneID = "contacts"

	// PaneLabel is the default label shown in the rail.
	PaneLabel = "Contacts"
)

// RegisterRailTab wires the Contacts rail tab. It runs once per process
// lifetime at App.Startup and returns a teardown function for shutdown.
func RegisterRailTab(ui coreapi.UI) (coreapi.Unregister, error) {
	unregRail, err := ui.RegisterRailTab(coreapi.RailTabRequest{
		ExtensionID: PaneID,
		Label:       PaneLabel,
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
