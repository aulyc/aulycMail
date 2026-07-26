package contactpane

import (
	"errors"
	"sync"

	"aulyc.local/aulycmail/internal/contact"
	contactdto "aulyc.local/aulycmail/internal/contactdto"
	"aulyc.local/aulycmail/internal/database"
)

// ContactsBridge is the Wails-bindable surface for the built-in Contacts pane.
// embedded into the host `*app.App` struct; Go's method-promotion makes
// every ContactsBridge method appear on App so Wails' reflection-based bind
// generator picks them up. All Contacts-specific logic lives here, not
// in the host. Store/API state is lazy-constructed inside ensureInit, so
// startup only allocates this small bridge struct.
type ContactsBridge struct {
	// Dependencies provided by the host at construction time.
	deps ContactsBridgeDeps

	// Lazy-initialized Contacts state. Nil until the first Contacts_* method
	// call kicks ensureInit.
	initOnce sync.Once
	initErr  error
	api      *API
}

// ContactsBridgeDeps bundles the host-provided dependencies the bridge needs.
type ContactsBridgeDeps struct {
	// DB is the shared application database. Used to construct the
	// Contacts-specific stores at init time.
	DB *database.DB

	// Emitter forwards `contacts:*` Wails events to the frontend. Captured
	// here so the bridge doesn't have to reach back into the host for ctx
	// every time it needs to publish a conflict event.
	Emitter EventEmitter
}

// EventEmitter forwards Wails events to the frontend. The host wires this
// to `wailsRuntime.EventsEmit(ctx, ...)` during Startup once the Wails ctx
// is available. Defined as a function type so callers don't have to write
// a one-method struct.
type EventEmitter func(eventName string, payload any)

// NewContactsBridge constructs the bridge with its dependencies. Does not touch
// the DB until ensureInit runs for the first Contacts_* call.
func NewContactsBridge(deps ContactsBridgeDeps) *ContactsBridge {
	return &ContactsBridge{deps: deps}
}

// ensureInit lazily constructs the contact store and API wrapper. sync.Once
// means it runs at most once per process lifetime.
func (b *ContactsBridge) ensureInit() error {
	b.initOnce.Do(func() {
		if b.deps.DB == nil {
			b.initErr = errors.New("contacts.ContactsBridge: missing DB in deps")
			return
		}
		contactStore := contact.NewStore(b.deps.DB.DB)
		b.api = NewAPI(contactStore)
	})
	return b.initErr
}

// emitConflict translates a `*contactdto.ErrConflict` from a write path into
// a `contacts:conflict` event the frontend listens for. Returns true when
// the error was a conflict (and an event was emitted) so the caller can
// short-circuit further error handling — the user's intent was acknowledged,
// just superseded by the server.
func (b *ContactsBridge) emitConflict(err error) bool {
	var conflict *contactdto.ErrConflict
	if !errors.As(err, &conflict) {
		return false
	}
	if b.deps.Emitter != nil {
		b.deps.Emitter("contacts:conflict", map[string]string{
			"contactId": conflict.ContactID,
			"message":   conflict.Message,
		})
	}
	return true
}

// ============================================================================
// Wails-bound surface
//
// The frontend imports these as e.g. `Contacts_UpdateContact` from
// $wailsjs/go/app/App. The Contacts_ prefix preserves the existing Wails API.
// ============================================================================

// Contacts_ListContactsForBrowse returns local contacts filtered by sourceID:
//   - ""                            → all local contacts, search applied
//   - SourceIDLocal                 → all local contacts
//   - SourceIDLocalManual           → user-added local contacts
//   - SourceIDLocalCollected        → auto-collected local contacts
func (b *ContactsBridge) Contacts_ListContactsForBrowse(query, sourceID string, limit, offset int) ([]contactdto.Contact, error) {
	if err := b.ensureInit(); err != nil {
		return nil, err
	}
	return b.api.ListContacts(contactdto.ContactFilter{
		Query:    query,
		SourceID: sourceID,
		Limit:    limit,
		Offset:   offset,
	})
}

// Contacts_BrowseContacts returns a paged list and the full total for the
// current source/search filter. The older Contacts_ListContactsForBrowse method
// stays array-shaped for existing lightweight search callers.
func (b *ContactsBridge) Contacts_BrowseContacts(query, sourceID, sortOrder string, limit, offset int) (contactdto.ContactBrowseResult, error) {
	if err := b.ensureInit(); err != nil {
		return contactdto.ContactBrowseResult{}, err
	}
	return b.api.BrowseContacts(contactdto.ContactFilter{
		Query:     query,
		SourceID:  sourceID,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	})
}

// Contacts_GetContactAccountGroups returns the enabled mail accounts that back
// the Contacts sidebar tree, including per-role counts for each account.
func (b *ContactsBridge) Contacts_GetContactAccountGroups() ([]contactdto.ContactAccountGroup, error) {
	if err := b.ensureInit(); err != nil {
		return nil, err
	}
	return b.api.ListAccountGroups()
}

// Contacts_GetContactDetail returns a single contact by email (if argument
// contains '@') or by local record UUID otherwise.
func (b *ContactsBridge) Contacts_GetContactDetail(emailOrID string) (*contactdto.Contact, error) {
	if err := b.ensureInit(); err != nil {
		return nil, err
	}
	return b.api.GetContact(emailOrID)
}

// Contacts_CreateContact creates a new contact and returns its id.
//
// Dispatch by input.SourceID:
//   - "", "local", "local:manual" → local manual entry. Returns the normalized
//     email as the id.
//   - "local:collected"            → rejected.
//
// The historical `Contacts_CreateLocalContact(email, name)` shape was renamed
// here when the bridge started accepting the full input shape.
func (b *ContactsBridge) Contacts_CreateContact(input contactdto.ContactCreateInput) (string, error) {
	if err := b.ensureInit(); err != nil {
		return "", err
	}
	return b.api.CreateContact(input)
}

// Contacts_UpdateContact applies a ContactPatch to a local contact via
// contact.Store.UpsertRecord.
//
// 412 conflicts surface as a contacts:conflict event the UI listens for;
// the method returns nil on conflict (the user's edit was discarded but
// the local cache now matches the server, so the UI just reloads).
func (b *ContactsBridge) Contacts_UpdateContact(idOrEmail string, patch contactdto.ContactPatch) error {
	if err := b.ensureInit(); err != nil {
		return err
	}
	err := b.api.UpdateContact(idOrEmail, patch)
	if b.emitConflict(err) {
		return nil
	}
	return err
}

// Contacts_DeleteLocalContact removes a contact from the local contact store.
// It is idempotent on missing records.
//
// Note: there's a separate top-level `App.DeleteContact` from the older
// contacts implementation for legacy callers.
func (b *ContactsBridge) Contacts_DeleteLocalContact(idOrEmail string) error {
	if err := b.ensureInit(); err != nil {
		return err
	}
	err := b.api.DeleteContact(idOrEmail)
	if b.emitConflict(err) {
		return nil
	}
	return err
}
