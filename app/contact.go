package app

import (
	"github.com/aulyc/aulycmail/internal/contact"
)

// ============================================================================
// Contact API - Exposed to frontend via Wails bindings
// ============================================================================

// SearchContacts searches the single local address book for contacts matching
// the query (used by composer autocomplete).
func (a *App) SearchContacts(query string, limit int) ([]*contact.Contact, error) {
	contacts, err := a.contactStore.Search(query, limit)
	if err != nil {
		return nil, err
	}
	if len(contacts) > limit {
		contacts = contacts[:limit]
	}
	return contacts, nil
}

// GetContact returns a single contact by ID
func (a *App) GetContact(id string) (*contact.Contact, error) {
	return a.contactStore.Get(id)
}

// AddContact adds or updates a contact
func (a *App) AddContact(email, displayName string) error {
	return a.contactStore.AddOrUpdate(email, displayName)
}

// DeleteContact deletes a contact
func (a *App) DeleteContact(id string) error {
	return a.contactStore.Delete(id)
}

// ListContacts returns all contacts
func (a *App) ListContacts(limit int) ([]*contact.Contact, error) {
	return a.contactStore.List(limit)
}
