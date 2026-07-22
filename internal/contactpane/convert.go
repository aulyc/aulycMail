package contactpane

import (
	"aulyc.local/aulycmail/internal/contact"
	contactdto "aulyc.local/aulycmail/internal/contactdto"
)

// fromLocal converts a core contact.Contact into the API-surface Contact.
//
// Core contacts are keyed by email, so we use the email itself as the ID. The
// local Source field becomes SourceID for autocomplete-style per-email row
// paths; multi-field fromRecord is the path the Contacts pane uses for its list
// + detail views.
func fromLocal(c *contact.Contact) contactdto.Contact {
	updated := c.LastUsed
	if updated.IsZero() {
		updated = c.CreatedAt
	}
	return contactdto.Contact{
		ID:        c.Email,
		Name:      c.DisplayName,
		Emails:    []string{c.Email},
		SourceID:  c.Source,
		UpdatedAt: updated,
	}
}

// fromRecord converts a local contact.Record into the API-surface
// contactdto.Contact. One Contact per record, with all sub-tables surfaced through
// the rich Emails/Phones/Addresses/URLs/IMPPs/Categories slices.
//
// SourceID semantics:
//   - For local records: returns the legacy-mapped Source value ("aulycmail") so
//     the ContactDetail.svelte gate `sourceId === 'aulycmail'` keeps working for
//     Edit/Delete on local contacts.
func fromRecord(rec *contact.Record) contactdto.Contact {
	if rec == nil {
		return contactdto.Contact{Emails: []string{}}
	}
	// Initialize Emails as empty slice (not nil) so the JSON payload always has
	// `"emails": []` rather than `"emails": null`. Frontend `{#each contact.emails}`
	// blocks iterate empty arrays fine; iterating null throws.
	out := contactdto.Contact{
		ID:             rec.ID,
		Name:           rec.Fn,
		Emails:         []string{},
		Org:            rec.Org,
		Title:          rec.Title,
		Note:           rec.Note,
		Bday:           rec.Bday,
		Nickname:       rec.Nickname,
		PhotoData:      rec.PhotoData,
		PhotoMediaType: rec.PhotoMediaType,
		PhotoURL:       rec.PhotoURL,
		UpdatedAt:      rec.UpdatedAt,
	}

	// Source mapping: 'local' → 'aulycmail' for the detail-pane edit/delete gate.
	out.SourceID = rec.Source
	if rec.Source == "local" {
		out.SourceID = "aulycmail"
	}

	// Flat email list (legacy autocomplete shape) + structured email items.
	for _, e := range rec.Emails {
		out.Emails = append(out.Emails, e.Email)
		out.EmailItems = append(out.EmailItems, contactdto.ContactEmail{
			Email:     e.Email,
			Type:      e.EmailType,
			IsPrimary: e.IsPrimary,
		})
	}
	for _, p := range rec.Phones {
		out.Phones = append(out.Phones, contactdto.ContactPhone{
			Number:    p.Number,
			Type:      p.PhoneType,
			IsPrimary: p.IsPrimary,
		})
	}
	for _, a := range rec.Addresses {
		out.Addresses = append(out.Addresses, contactdto.ContactAddress{
			Type:     a.AddrType,
			Street:   a.Street,
			City:     a.City,
			Region:   a.Region,
			Postcode: a.Postcode,
			Country:  a.Country,
		})
	}
	for _, u := range rec.URLs {
		out.URLs = append(out.URLs, contactdto.ContactURL{URL: u.URL, Type: u.URLType})
	}
	for _, i := range rec.IMPPs {
		out.IMPPs = append(out.IMPPs, contactdto.ContactIMPP{Handle: i.Handle, Type: i.IMPPType})
	}
	out.Categories = append(out.Categories, rec.Categories...)
	return out
}

func fromRecordSummary(summary *contact.RecordSummary) contactdto.ContactListItem {
	if summary == nil {
		return contactdto.ContactListItem{Emails: []string{}}
	}
	emails := []string{}
	if summary.PrimaryEmail != "" {
		emails = append(emails, summary.PrimaryEmail)
	}
	sourceID := summary.Source
	if sourceID == "local" {
		sourceID = "aulycmail"
	}
	return contactdto.ContactListItem{
		ID:        summary.ID,
		Name:      summary.Fn,
		Emails:    emails,
		SourceID:  sourceID,
		UpdatedAt: summary.UpdatedAt,
	}
}

func contactFromRecordSummary(summary *contact.RecordSummary) contactdto.Contact {
	item := fromRecordSummary(summary)
	return contactdto.Contact{
		ID:        item.ID,
		Name:      item.Name,
		Emails:    item.Emails,
		SourceID:  item.SourceID,
		UpdatedAt: item.UpdatedAt,
	}
}
