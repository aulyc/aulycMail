package v1

// ContactPatch is the optional-fields shape passed to Contacts_UpdateContact.
// Pointer fields distinguish "leave unchanged" (nil) from "set to empty"
// (non-nil pointer to zero value), so the Edit dialog can patch any subset of
// a contact's data in a single call.
//
// For multi-value fields (Emails, Phones, Addresses, etc.) the pointer-to-slice
// preserves three states: nil = leave unchanged; non-nil empty slice = clear all
// rows; non-nil populated slice = replace existing rows with the new set. The
// backend writes the whole record via UpsertRecordTx, so partial-row updates
// are NOT supported — callers send the full desired list.
//
// For Photo (single-value but structured), the same convention via
// pointer-to-struct: nil = unchanged; non-nil with empty Data + URL = remove
// the photo; non-nil with populated Data + MediaType = set inline.
type ContactPatch struct {
	Name       *string           `json:"name,omitempty"`
	Nickname   *string           `json:"nickname,omitempty"`
	Org        *string           `json:"org,omitempty"`
	Title      *string           `json:"title,omitempty"`
	Note       *string           `json:"note,omitempty"`
	Bday       *string           `json:"bday,omitempty"`
	Emails     *[]ContactEmail   `json:"emails,omitempty"`
	Phones     *[]ContactPhone   `json:"phones,omitempty"`
	Addresses  *[]ContactAddress `json:"addresses,omitempty"`
	URLs       *[]ContactURL     `json:"urls,omitempty"`
	IMPPs      *[]ContactIMPP    `json:"impps,omitempty"`
	Categories *[]string         `json:"categories,omitempty"`
	Photo      *ContactPhoto     `json:"photo,omitempty"`
}

// ContactPhoto is the PATCH-side grouping for photo edits. Pointer-to-struct
// on the patch matches the "nil = unchanged, non-nil = set" semantics that
// *[]ContactEmail uses for collections. This grouping shows up ONLY here —
// the read surface (Contact) keeps the existing flat-scalar pattern with
// PhotoData / PhotoMediaType / PhotoURL fields.
//
// URL is read-only on patch: the backend sets it when the parser sees a
// vCard URL-ref PHOTO. Write path always emits inline base64; callers that
// want to remove a photo send Data + URL both empty.
type ContactPhoto struct {
	Data      string `json:"data,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url,omitempty"`
}

// ContactCreateInput is the shape passed to Contacts_CreateContact.
//
// SourceID selects where the new contact lives:
//   - "" or "local" or "local:manual" → local manual contact (aulycmail's
//     own SQLite store). The kind='manual' designation is set automatically.
//   - "local:collected"               → REJECTED. The 'collected' kind is
//     reserved for the sent-mail collection process to assign; users adding
//     via the Add dialog get kind='manual' regardless of which local sub-view
//     they came from.
//   - anything else                   → rejected. Remote contact sources are
//     not implemented.
//
// Rich-field support: when any of the optional rich fields below is set,
// the create dispatchers route through recordFromCreateInput which mirrors
// ContactPatch's shape onto a new contact.Record. Email + Name remain the
// legacy minimum; when only those are set the create paths take a thin
// "single primary email" shortcut for backward compatibility with the
// sent-mail collection path that still uses email+name.
//
// Slice semantics differ from ContactPatch on purpose: Patch uses *[]T to
// distinguish "unchanged" (nil) from "empty/cleared" ([]); Create has no
// such ambiguity (omitting means empty, providing means set), so plain
// slices are used.
type ContactCreateInput struct {
	SourceID string `json:"sourceId,omitempty"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`

	// Optional rich fields, mirroring ContactPatch's field set. When
	// Emails is supplied (non-empty), it REPLACES the implicit single
	// primary email built from the legacy Email field; otherwise the
	// legacy single-email shortcut applies.
	Nickname   string           `json:"nickname,omitempty"`
	Org        string           `json:"org,omitempty"`
	Title      string           `json:"title,omitempty"`
	Note       string           `json:"note,omitempty"`
	Bday       string           `json:"bday,omitempty"`
	Categories []string         `json:"categories,omitempty"`
	Emails     []ContactEmail   `json:"emails,omitempty"`
	Phones     []ContactPhone   `json:"phones,omitempty"`
	Addresses  []ContactAddress `json:"addresses,omitempty"`
	URLs       []ContactURL     `json:"urls,omitempty"`
	IMPPs      []ContactIMPP    `json:"impps,omitempty"`
	Photo      *ContactPhoto    `json:"photo,omitempty"`
}
