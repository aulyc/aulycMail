package v1

import "time"

// Contact is the shared representation used by the built-in Contacts pane and
// Wails bindings. It carries the full multi-field shape from the unified
// contact_records schema. Single-value fields (Org/Title/Note/Bday/Nickname)
// are surfaced directly; multi-value fields are slices of small sub-types.
//
// Empty/zero-valued sub-fields are omitted from JSON so the frontend can
// cleanly hide UI sections that have no data.
type Contact struct {
	ID                 string                     `json:"id"`
	Name               string                     `json:"name"`
	Emails             []string                   `json:"emails"`
	EmailItems         []ContactEmail             `json:"emailItems,omitempty"` // richer per-email metadata (type, isPrimary)
	AssociatedAccounts []ContactAssociatedAccount `json:"associatedAccounts,omitempty"`
	Phones             []ContactPhone             `json:"phones,omitempty"`
	Addresses          []ContactAddress           `json:"addresses,omitempty"`
	URLs               []ContactURL               `json:"urls,omitempty"`
	IMPPs              []ContactIMPP              `json:"impps,omitempty"`
	Org                string                     `json:"org,omitempty"`
	Title              string                     `json:"title,omitempty"`
	Note               string                     `json:"note,omitempty"`
	Bday               string                     `json:"bday,omitempty"`
	Nickname           string                     `json:"nickname,omitempty"`
	Categories         []string                   `json:"categories,omitempty"`
	// Photo fields (Phase 2b.2.b.2). Flat-scalar pattern matching Org/Title/Note.
	// At most one of {PhotoData + PhotoMediaType} OR PhotoURL is populated:
	//   - PhotoData (base64) + PhotoMediaType (e.g. "image/jpeg") = inline embed
	//   - PhotoURL = vCard URL-ref (PHOTO;VALUE=URI). Avatar falls back to initials
	//     in this phase; fetching is its own track.
	PhotoData      string    `json:"photoData,omitempty"`
	PhotoMediaType string    `json:"photoMediaType,omitempty"`
	PhotoURL       string    `json:"photoUrl,omitempty"`
	SourceID       string    `json:"sourceId,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt" ts_type:"string"`
}

// ContactAssociatedAccount is one mail account where a contact appears in
// related messages. It is distinct from the contact's own email addresses.
type ContactAssociatedAccount struct {
	AccountID string `json:"accountId"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email"`
}

// ContactAccountGroup backs the Contacts sidebar tree:
// All contacts -> mail account -> sender/recipient/cc/bcc roles.
type ContactAccountGroup struct {
	AccountID      string `json:"accountId"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email"`
	Count          int    `json:"count"`
	SenderCount    int    `json:"senderCount"`
	RecipientCount int    `json:"recipientCount"`
	CcCount        int    `json:"ccCount"`
	BccCount       int    `json:"bccCount"`
}

// ContactBrowseResult is the paged Contacts-pane list result. Items contains
// the current page; Total is the full count for the same source/search filter.
type ContactBrowseResult struct {
	Items []Contact `json:"items"`
	Total int       `json:"total"`
}

// ContactEmail is one email on a Contact, with its TYPE and primary flag.
type ContactEmail struct {
	Email     string `json:"email"`
	Type      string `json:"type,omitempty"`
	IsPrimary bool   `json:"isPrimary,omitempty"`
}

// ContactPhone is one phone number on a Contact.
type ContactPhone struct {
	Number    string `json:"number"`
	Type      string `json:"type,omitempty"`
	IsPrimary bool   `json:"isPrimary,omitempty"`
}

// ContactAddress is a structured postal address.
type ContactAddress struct {
	Type     string `json:"type,omitempty"`
	Street   string `json:"street,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Country  string `json:"country,omitempty"`
}

// ContactURL is a URL associated with a Contact.
type ContactURL struct {
	URL  string `json:"url"`
	Type string `json:"type,omitempty"`
}

// ContactIMPP is an instant-messaging handle on a Contact.
type ContactIMPP struct {
	Handle string `json:"handle"`
	Type   string `json:"type,omitempty"`
}

// ContactFilter is the input to Contacts.ListContacts.
//
// SourceID accepts these values (in addition to the empty-string default):
//   - "local"            → all local contacts (both manual and collected)
//   - "local:manual"     → only user-added local contacts
//   - "local:collected"  → only auto-collected (from sent-mail) local contacts
type ContactFilter struct {
	Query    string `json:"query,omitempty"`
	SourceID string `json:"sourceId,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}
