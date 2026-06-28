package compose

import (
	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

// API implements coreapi.Composer. Opening a composer programmatically from an
// extension is not currently supported — the detached composer window was
// removed, and the in-app composer is driven entirely from the frontend. The
// surface is kept so the core API contract stays stable for future use.
type API struct{}

// NewAPI constructs the Composer API wrapper.
func NewAPI() *API {
	return &API{}
}

// OpenComposer is unimplemented; the in-app composer is opened from the
// frontend, not via the core API.
func (a *API) OpenComposer(req coreapi.ComposeRequest) error {
	return coreapi.ErrUnimplemented
}
