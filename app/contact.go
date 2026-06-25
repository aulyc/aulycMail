package app

import (
	"encoding/json"
	"fmt"

	"github.com/aulyc/aulycmail/internal/contact"
	"github.com/aulyc/aulycmail/internal/folder"
	"github.com/aulyc/aulycmail/internal/logging"
)

// RefreshContactsFromMail re-scans every stored message across all accounts and
// (re)collects its participants into the local address book with roles:
//   - received mail (Inbox / Archive / custom folders — NOT Sent/Drafts/Spam/Trash):
//     From → 发件人 (sender)
//   - Sent mail: To → 收件人 (recipient); Cc/Bcc → 抄送/密送 (ccbcc)
//
// Idempotent — AddOrUpdateWithRole OR-s role flags and bumps counts, so running
// it repeatedly just refreshes the set. Returns the number of messages scanned.
func (a *App) RefreshContactsFromMail() (int, error) {
	log := logging.WithComponent("app")
	if a.contactStore == nil || a.db == nil {
		return 0, fmt.Errorf("contacts not ready")
	}

	rows, err := a.db.DB.Query(`
		SELECT m.from_email, m.from_name,
		       COALESCE(m.to_list, ''), COALESCE(m.cc_list, ''), COALESCE(m.bcc_list, ''),
		       f.folder_type
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
	`)
	if err != nil {
		return 0, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	type addr struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	collectList := func(jsonStr, role string) {
		if jsonStr == "" {
			return
		}
		var list []addr
		if json.Unmarshal([]byte(jsonStr), &list) != nil {
			return
		}
		for _, p := range list {
			if p.Email != "" {
				_ = a.contactStore.AddOrUpdateWithRole(p.Email, p.Name, role)
			}
		}
	}

	count := 0
	for rows.Next() {
		var fromEmail, fromName, toList, ccList, bccList, ftype string
		if err := rows.Scan(&fromEmail, &fromName, &toList, &ccList, &bccList, &ftype); err != nil {
			continue
		}
		count++
		switch folder.Type(ftype) {
		case folder.TypeSent:
			collectList(toList, contact.RoleRecipient)
			collectList(ccList, contact.RoleCcBcc)
			collectList(bccList, contact.RoleCcBcc)
		case folder.TypeDrafts, folder.TypeSpam, folder.TypeTrash:
			// Skip — drafts/spam/trash aren't a source of real contacts.
		default:
			if fromEmail != "" {
				_ = a.contactStore.AddOrUpdateWithRole(fromEmail, fromName, contact.RoleSender)
			}
		}
	}

	log.Info().Int("messages", count).Msg("Refreshed contacts from mail")
	return count, nil
}

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
