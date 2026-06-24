package backend

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/aulyc/aulycmail/extensions/contacts/backend/imaging"
	"github.com/aulyc/aulycmail/internal/contact"
	coreapi "github.com/aulyc/aulycmail/internal/core/api/v1"
)

// Source IDs for aulycmail's single local contact store. There are no remote
// sources — every contact lives in the local unified contact-record schema.
//
// The local store distinguishes entries by `contacts.kind`:
//   - manual    → entries the user added via the Add Contact UI
//   - collected → auto-collected from mail (senders/recipients/cc-bcc)
//
// The "Local" parent (SourceIDLocal) returns both. Sub-source values select
// one kind only.
const (
	SourceIDLocal          = "local"
	SourceIDLocalManual    = "local:manual"
	SourceIDLocalCollected = "local:collected"
)

// localKindFromSourceID returns the `contacts.kind` filter value for a local
// sub-source ID, or "" for the parent "local" / empty (= no filter, return both).
func localKindFromSourceID(id string) string {
	switch id {
	case SourceIDLocalManual:
		return "manual"
	case SourceIDLocalCollected:
		return "collected"
	default:
		return ""
	}
}

// API implements the contacts surface over the single local contact.Store.
// All remote sources (CardDAV / Google / Microsoft) were removed — this is a
// local-only address book.
type API struct {
	localStore *contact.Store
}

// NewAPI constructs the Contacts API wrapper over the local contact store.
// localStore may be nil — the wrapper degrades gracefully (search returns no
// results; writes error with a clear message rather than panicking).
func NewAPI(localStore *contact.Store) *API {
	return &API{localStore: localStore}
}

// SearchContacts delegates to the local contact store's search (email + display
// name match; ranking by send count + recency).
func (a *API) SearchContacts(query string, limit int) ([]coreapi.Contact, error) {
	if a.localStore == nil {
		return nil, nil
	}
	results, err := a.localStore.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("contacts.SearchContacts: %w", err)
	}
	out := make([]coreapi.Contact, 0, len(results))
	for _, c := range results {
		out = append(out, fromLocal(c))
	}
	return out, nil
}

// GetContact looks up a contact by record id or, when the argument contains
// '@', by email. Returns (nil, nil) when not found.
func (a *API) GetContact(emailOrID string) (*coreapi.Contact, error) {
	if emailOrID == "" || a.localStore == nil {
		return nil, nil
	}

	rec, err := a.localStore.GetRecord(emailOrID)
	if err != nil {
		return nil, fmt.Errorf("contacts.GetContact: %w", err)
	}
	if rec != nil {
		out := fromRecord(rec)
		return &out, nil
	}

	if strings.Contains(emailOrID, "@") {
		rec, err := a.localStore.GetRecordByEmail(emailOrID)
		if err != nil {
			return nil, fmt.Errorf("contacts.GetContact: %w", err)
		}
		if rec != nil {
			out := fromRecord(rec)
			return &out, nil
		}
	}
	return nil, nil
}

// ListContacts returns local contacts filtered by SourceID:
//   - "" / SourceIDLocal       → all local contacts (manual + collected)
//   - SourceIDLocalManual      → user-added local contacts only
//   - SourceIDLocalCollected   → auto-collected local contacts only
func (a *API) ListContacts(filter coreapi.ContactFilter) ([]coreapi.Contact, error) {
	if a.localStore == nil {
		return nil, nil
	}
	kind := localKindFromSourceID(filter.SourceID)
	records, err := a.localStore.ListRecords(contact.RecordFilter{
		Source: "local",
		Kind:   kind,
		Query:  filter.Query,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("contacts.ListContacts: %w", err)
	}
	out := make([]coreapi.Contact, 0, len(records))
	for _, rec := range records {
		out = append(out, fromRecord(rec))
	}
	return out, nil
}

// CreateContact creates a new local contact and returns its id.
//
//   - "", "local", "local:manual" → local manual entry (kind='manual'). The
//     legacy email+name shortcut is preserved for the mail auto-collection path.
//     Returns the email (minimal create) or a fresh UUID (rich create).
//   - "local:collected"            → rejected (reserved for auto-collection).
//
// Email is normalized (trim + lowercase) before storage.
func (a *API) CreateContact(input coreapi.ContactCreateInput) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return "", fmt.Errorf("contacts.CreateContact: email is required")
	}
	if !strings.Contains(email, "@") {
		return "", fmt.Errorf("contacts.CreateContact: email is not valid")
	}

	switch input.SourceID {
	case "", SourceIDLocal, SourceIDLocalManual:
		if a.localStore == nil {
			return "", fmt.Errorf("contacts.CreateContact: local store unavailable")
		}
		// Legacy minimal-create shortcut: email+name only.
		if !hasRichFields(input) {
			if err := a.localStore.Create(email, strings.TrimSpace(input.Name)); err != nil {
				return "", err
			}
			return email, nil
		}
		// Rich-create path: build a full Record and UpsertRecord into the
		// local store. NameOverridden=true on every email so future
		// auto-collection doesn't clobber the user's chosen FN.
		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = email
		}
		rec := recordFromCreateInput(input, email, name)
		rec.ID = uuid.New().String()
		rec.Source = "local"
		rec.Kind = "manual"
		for i := range rec.Emails {
			rec.Emails[i].NameOverridden = true
		}
		if err := a.localStore.UpsertRecord(rec); err != nil {
			if errors.Is(err, contact.ErrContactExists) {
				return "", err
			}
			return "", fmt.Errorf("contacts.CreateContact: upsert local record: %w", err)
		}
		return rec.ID, nil
	case SourceIDLocalCollected:
		return "", fmt.Errorf("contacts.CreateContact: cannot manually create a Collected contact (auto-derived from mail)")
	}
	return "", fmt.Errorf("contacts.CreateContact: unknown source %q", input.SourceID)
}

// UpdateContact applies a ContactPatch to a local contact. Resolves the id to a
// record (by record id, or by email when the arg contains '@'), applies every
// non-nil patch field, then writes via UpsertRecord. Empty/nil patch is a
// no-op success.
func (a *API) UpdateContact(id string, patch coreapi.ContactPatch) error {
	if id == "" {
		return fmt.Errorf("contacts.UpdateContact: id is required")
	}
	if a.localStore == nil {
		return fmt.Errorf("contacts.UpdateContact: local store unavailable")
	}

	rec, err := a.localStore.GetRecord(id)
	if err != nil {
		return fmt.Errorf("contacts.UpdateContact: %w", err)
	}
	if rec == nil && strings.Contains(id, "@") {
		rec, err = a.localStore.GetRecordByEmail(id)
		if err != nil {
			return fmt.Errorf("contacts.UpdateContact: %w", err)
		}
	}
	if rec == nil {
		// Idempotent miss.
		return nil
	}

	if !applyContactPatchToRecord(rec, patch) {
		// All patch fields nil — no-op success.
		return nil
	}
	return a.localStore.UpsertRecord(rec)
}

// applyContactPatchToRecord copies every non-nil patch field onto the record.
// Returns true if any field was applied; false if the patch was entirely nil
// (caller can short-circuit with a no-op success).
//
// Scalar fields are simple string assignments (with TrimSpace for user input).
// Multi-value fields use the pointer-to-slice contract: non-nil empty slice
// = clear, non-nil populated slice = replace. Photo uses pointer-to-struct
// with the same semantics: non-nil with empty Data+URL = clear.
func applyContactPatchToRecord(rec *contact.Record, patch coreapi.ContactPatch) bool {
	applied := false
	if patch.Name != nil {
		rec.Fn = strings.TrimSpace(*patch.Name)
		// Mark all emails as name_overridden so future auto-collection from
		// mail doesn't clobber the user edit.
		for i := range rec.Emails {
			rec.Emails[i].NameOverridden = true
		}
		applied = true
	}
	if patch.Nickname != nil {
		rec.Nickname = strings.TrimSpace(*patch.Nickname)
		applied = true
	}
	if patch.Org != nil {
		rec.Org = strings.TrimSpace(*patch.Org)
		applied = true
	}
	if patch.Title != nil {
		rec.Title = strings.TrimSpace(*patch.Title)
		applied = true
	}
	if patch.Note != nil {
		rec.Note = strings.TrimSpace(*patch.Note)
		applied = true
	}
	if patch.Bday != nil {
		rec.Bday = strings.TrimSpace(*patch.Bday)
		applied = true
	}
	if patch.Emails != nil {
		rec.Emails = nil
		for _, e := range *patch.Emails {
			rec.Emails = append(rec.Emails, contact.RecordEmail{
				Email:     strings.ToLower(strings.TrimSpace(e.Email)),
				EmailType: e.Type,
				IsPrimary: e.IsPrimary,
			})
		}
		applied = true
	}
	if patch.Phones != nil {
		rec.Phones = nil
		for _, p := range *patch.Phones {
			rec.Phones = append(rec.Phones, contact.RecordPhone{
				Number:    strings.TrimSpace(p.Number),
				PhoneType: p.Type,
				IsPrimary: p.IsPrimary,
			})
		}
		applied = true
	}
	if patch.Addresses != nil {
		rec.Addresses = nil
		for _, a := range *patch.Addresses {
			rec.Addresses = append(rec.Addresses, contact.RecordAddress{
				AddrType: a.Type,
				Street:   strings.TrimSpace(a.Street),
				City:     strings.TrimSpace(a.City),
				Region:   strings.TrimSpace(a.Region),
				Postcode: strings.TrimSpace(a.Postcode),
				Country:  strings.TrimSpace(a.Country),
			})
		}
		applied = true
	}
	if patch.URLs != nil {
		rec.URLs = nil
		for _, u := range *patch.URLs {
			rec.URLs = append(rec.URLs, contact.RecordURL{
				URL:     strings.TrimSpace(u.URL),
				URLType: u.Type,
			})
		}
		applied = true
	}
	if patch.IMPPs != nil {
		rec.IMPPs = nil
		for _, i := range *patch.IMPPs {
			rec.IMPPs = append(rec.IMPPs, contact.RecordIMPP{
				Handle:   strings.TrimSpace(i.Handle),
				IMPPType: i.Type,
			})
		}
		applied = true
	}
	if patch.Categories != nil {
		rec.Categories = append([]string{}, *patch.Categories...)
		applied = true
	}
	if patch.Photo != nil {
		rec.PhotoData = strings.TrimSpace(patch.Photo.Data)
		rec.PhotoMediaType = strings.TrimSpace(patch.Photo.MediaType)
		rec.PhotoURL = strings.TrimSpace(patch.Photo.URL)
		applied = true
	}
	return applied
}

// recordFromCreateInput builds a contact.Record from a (rich) ContactCreateInput.
// `email` is the resolved primary email (already lowercased+trimmed by the
// caller); `name` is the resolved display name.
//
// When input.Emails is non-empty it REPLACES the implicit single-primary email
// constructed from `email`. Same for the other repeating slices.
func recordFromCreateInput(input coreapi.ContactCreateInput, email, name string) *contact.Record {
	rec := &contact.Record{
		Fn:       strings.TrimSpace(name),
		Nickname: strings.TrimSpace(input.Nickname),
		Org:      strings.TrimSpace(input.Org),
		Title:    strings.TrimSpace(input.Title),
		Note:     strings.TrimSpace(input.Note),
		Bday:     strings.TrimSpace(input.Bday),
	}

	if len(input.Emails) > 0 {
		for _, e := range input.Emails {
			rec.Emails = append(rec.Emails, contact.RecordEmail{
				Email:     strings.ToLower(strings.TrimSpace(e.Email)),
				EmailType: e.Type,
				IsPrimary: e.IsPrimary,
			})
		}
	} else if email != "" {
		rec.Emails = []contact.RecordEmail{{
			Email:     email,
			IsPrimary: true,
		}}
	}

	for _, p := range input.Phones {
		rec.Phones = append(rec.Phones, contact.RecordPhone{
			Number:    strings.TrimSpace(p.Number),
			PhoneType: p.Type,
			IsPrimary: p.IsPrimary,
		})
	}
	for _, a := range input.Addresses {
		rec.Addresses = append(rec.Addresses, contact.RecordAddress{
			AddrType: a.Type,
			Street:   strings.TrimSpace(a.Street),
			City:     strings.TrimSpace(a.City),
			Region:   strings.TrimSpace(a.Region),
			Postcode: strings.TrimSpace(a.Postcode),
			Country:  strings.TrimSpace(a.Country),
		})
	}
	for _, u := range input.URLs {
		rec.URLs = append(rec.URLs, contact.RecordURL{
			URL:     strings.TrimSpace(u.URL),
			URLType: u.Type,
		})
	}
	for _, i := range input.IMPPs {
		rec.IMPPs = append(rec.IMPPs, contact.RecordIMPP{
			Handle:   strings.TrimSpace(i.Handle),
			IMPPType: i.Type,
		})
	}
	if len(input.Categories) > 0 {
		rec.Categories = append([]string{}, input.Categories...)
	}
	if input.Photo != nil {
		rec.PhotoData = strings.TrimSpace(input.Photo.Data)
		rec.PhotoMediaType = strings.TrimSpace(input.Photo.MediaType)
		rec.PhotoURL = strings.TrimSpace(input.Photo.URL)
	}
	return rec
}

// hasRichFields reports whether any non-legacy field on the input is set.
// Used by CreateContact to pick between the legacy minimal-create shortcut
// (email + name only) and the full recordFromCreateInput path.
func hasRichFields(input coreapi.ContactCreateInput) bool {
	if input.Nickname != "" || input.Org != "" || input.Title != "" || input.Note != "" || input.Bday != "" {
		return true
	}
	if len(input.Categories) > 0 || len(input.Emails) > 0 || len(input.Phones) > 0 {
		return true
	}
	if len(input.Addresses) > 0 || len(input.URLs) > 0 || len(input.IMPPs) > 0 {
		return true
	}
	if input.Photo != nil && (input.Photo.Data != "" || input.Photo.URL != "") {
		return true
	}
	return false
}

// DeleteContact removes a local contact by id (or email when the arg contains
// '@'). Cascade-deletes the record and its sub-tables. Idempotent on a miss.
func (a *API) DeleteContact(id string) error {
	if id == "" {
		return fmt.Errorf("contacts.DeleteContact: id is required")
	}
	if a.localStore == nil {
		return fmt.Errorf("contacts.DeleteContact: local store unavailable")
	}

	rec, err := a.localStore.GetRecord(id)
	if err != nil {
		return fmt.Errorf("contacts.DeleteContact: %w", err)
	}
	if rec == nil && strings.Contains(id, "@") {
		rec, err = a.localStore.GetRecordByEmail(id)
		if err != nil {
			return fmt.Errorf("contacts.DeleteContact: %w", err)
		}
	}
	if rec == nil {
		// Idempotent miss.
		return nil
	}
	return a.localStore.DeleteRecord(rec.ID)
}

// ResizeContactPhoto takes a base64-encoded image (PNG / JPEG / WEBP / GIF),
// rescales it to a max edge of 256px preserving aspect ratio, and re-encodes
// as JPEG at quality 85. Returns the resized base64 + "image/jpeg" ready to
// drop into a coreapi.ContactPatch.Photo.
func (a *API) ResizeContactPhoto(b64In string) (b64Out string, mediaType string, err error) {
	b64In = strings.TrimSpace(b64In)
	if b64In == "" {
		return "", "", fmt.Errorf("contacts.ResizeContactPhoto: empty input")
	}
	raw, err := base64.StdEncoding.DecodeString(b64In)
	if err != nil {
		return "", "", fmt.Errorf("contacts.ResizeContactPhoto: decode base64: %w", err)
	}
	jpegBytes, mt, err := imaging.ResizeToJPEG(raw, imaging.ResizeOptions{
		MaxEdge: 256,
		Quality: 85,
	})
	if err != nil {
		return "", "", fmt.Errorf("contacts.ResizeContactPhoto: %w", err)
	}
	return base64.StdEncoding.EncodeToString(jpegBytes), mt, nil
}
