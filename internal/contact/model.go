// Package contact provides contact management for email autocomplete
package contact

import "time"

// Contact represents a contact for email autocomplete. One *Contact corresponds
// to a (record, email) pair in the contact_records schema — autocomplete is
// per-email.
// Multi-field data (phones/addresses/etc.) is exposed via Record.
type Contact struct {
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Source      string    `json:"source"`         // legacy API value: "aulycmail" for local contacts
	Kind        string    `json:"kind,omitempty"` // "manual" | "collected" — only set on local contacts
	AvatarURL   string    `json:"avatar_url,omitempty"`
	SendCount   int       `json:"send_count"`
	LastUsed    time.Time `json:"last_used" ts_type:"string"`
	CreatedAt   time.Time `json:"created_at" ts_type:"string"`
}

// Record is the rich, multi-field contact_records row + its sub-table data.
// Returned by GetRecord/ListRecords, consumed by the built-in Contacts pane.
type Record struct {
	ID       string `json:"id"`
	Source   string `json:"source"`         // always 'local'
	Kind     string `json:"kind,omitempty"` // 'manual' | 'collected'
	Fn       string `json:"fn"`             // display name
	NGiven   string `json:"n_given,omitempty"`
	NFamily  string `json:"n_family,omitempty"`
	Org      string `json:"org,omitempty"`
	Title    string `json:"title,omitempty"`
	Note     string `json:"note,omitempty"`
	Bday     string `json:"bday,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	// Photo fields. Flat-scalar pattern matching Org/Title/Note.
	// At most one of {PhotoData + PhotoMediaType} OR PhotoURL is populated:
	//   - PhotoData (base64) + PhotoMediaType (e.g. "image/jpeg") = inline embed
	//   - PhotoURL = URL-ref photo. Avatar falls back to initials.
	// All-empty = no photo.
	PhotoData      string    `json:"photo_data,omitempty"`
	PhotoMediaType string    `json:"photo_media_type,omitempty"`
	PhotoURL       string    `json:"photo_url,omitempty"`
	CreatedAt      time.Time `json:"created_at" ts_type:"string"`
	UpdatedAt      time.Time `json:"updated_at" ts_type:"string"`

	// Sub-table data — populated by GetRecord and (optionally) ListRecords.
	Emails     []RecordEmail   `json:"emails,omitempty"`
	Phones     []RecordPhone   `json:"phones,omitempty"`
	Addresses  []RecordAddress `json:"addresses,omitempty"`
	URLs       []RecordURL     `json:"urls,omitempty"`
	IMPPs      []RecordIMPP    `json:"impps,omitempty"`
	Categories []string        `json:"categories,omitempty"`
}

// RecordSummary is the lightweight shape used by contact-list and global-search
// reads. Rich sub-table fields stay on Record and are loaded only when the user
// opens a contact detail.
type RecordSummary struct {
	ID           string
	Source       string
	Fn           string
	PrimaryEmail string
	UpdatedAt    time.Time
}

// RecordEmail is a single email belonging to a Record. Carries the per-email
// autocomplete metadata that lived on the legacy `contacts` table.
type RecordEmail struct {
	Email          string    `json:"email"`
	EmailType      string    `json:"email_type,omitempty"`
	IsPrimary      bool      `json:"is_primary,omitempty"`
	SendCount      int       `json:"send_count"`
	LastUsed       time.Time `json:"last_used" ts_type:"string"`
	NameOverridden bool      `json:"name_overridden,omitempty"`
}

// RecordPhone is a phone number belonging to a Record.
type RecordPhone struct {
	Number    string `json:"number"`
	PhoneType string `json:"phone_type,omitempty"`
	IsPrimary bool   `json:"is_primary,omitempty"`
}

// RecordAddress is a structured address belonging to a Record.
type RecordAddress struct {
	AddrType string `json:"addr_type,omitempty"`
	Street   string `json:"street,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Country  string `json:"country,omitempty"`
}

// RecordURL is a URL belonging to a Record.
type RecordURL struct {
	URL     string `json:"url"`
	URLType string `json:"url_type,omitempty"`
}

// RecordIMPP is an instant-messaging handle belonging to a Record (vCard IMPP).
type RecordIMPP struct {
	Handle   string `json:"handle"`
	IMPPType string `json:"impp_type,omitempty"`
}

// RecordFilter parameterizes ListRecords queries.
type RecordFilter struct {
	Source    string // 'local' | '' for local contacts
	Kind      string // for local: 'manual' | 'collected' | ''
	Role      string // for collected: 'sender' | 'recipient' | 'ccbcc' | '' for any
	AccountID string // optional mail account scope for dynamic message-role grouping
	Query     string // optional case-insensitive fn/email substring
	Limit     int
	Offset    int
}

// AccountAssociation summarizes which mail account(s) a contact record is
// associated with, and how many local contacts are visible under each role for
// the sidebar tree.
type AccountAssociation struct {
	AccountID      string
	Name           string
	Email          string
	Count          int
	SenderCount    int
	RecipientCount int
	CcCount        int
	BccCount       int
}
