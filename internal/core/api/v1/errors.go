package v1

import "fmt"

// ErrConflict signals that a mutation lost a race with concurrent state on
// the authoritative side. The Wails layer translates this into an event the UI
// handles by toast + reload.
type ErrConflict struct {
	ContactID string // the record id (UUID) the conflict was on
	Message   string // human-readable detail; safe to display
}

func (e *ErrConflict) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("conflict on contact %s", e.ContactID)
	}
	return fmt.Sprintf("conflict on contact %s: %s", e.ContactID, e.Message)
}
