package app

import (
	"encoding/json"
	"fmt"

	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/message"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

	// Keep the contact store's own-address set current, then purge mailbox and
	// sender-identity addresses collected before this exclusion existed. New
	// collection below already skips all of them via the store.
	if _, err := a.refreshContactOwnEmails(); err != nil {
		return 0, fmt.Errorf("refresh contact own-address exclusions: %w", err)
	}

	total := 0
	if err := a.db.DB.QueryRow(`
		SELECT COUNT(*)
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
	`).Scan(&total); err != nil {
		log.Warn().Err(err).Msg("Failed to count messages for contact refresh progress")
	}
	a.emitContactRefreshProgress("scanning", 0, total)

	rows, err := a.db.DB.Query(`
		SELECT m.from_email, m.from_name,
		       COALESCE(m.to_list, ''), COALESCE(m.cc_list, ''), COALESCE(m.bcc_list, ''),
		       f.folder_type
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
	`)
	if err != nil {
		a.emitContactRefreshProgress("error", 0, total)
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
			collectList(ccList, contact.RoleCc)
			collectList(bccList, contact.RoleBcc)
		case folder.TypeDrafts, folder.TypeSpam, folder.TypeTrash:
			// Skip — drafts/spam/trash aren't a source of real contacts.
		default:
			if fromEmail != "" {
				_ = a.contactStore.AddOrUpdateWithRole(fromEmail, fromName, contact.RoleSender)
			}
		}
		if count%50 == 0 {
			a.emitContactRefreshProgress("scanning", count, total)
		}
	}
	if err := rows.Err(); err != nil {
		a.emitContactRefreshProgress("error", count, total)
		return count, fmt.Errorf("scan messages: %w", err)
	}

	a.emitContactRefreshProgress("complete", count, total)
	log.Info().Int("messages", count).Msg("Refreshed contacts from mail")
	return count, nil
}

func (a *App) emitContactRefreshProgress(phase string, scanned, total int) {
	if a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "contacts:refreshProgress", map[string]interface{}{
		"phase":   phase,
		"scanned": scanned,
		"total":   total,
	})
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

// GetContactMessages returns recent mail involving the given contact address
// (as sender, To, Cc, or Bcc), newest first. Powers the Contacts detail's
// "related mail" list; clicking a row navigates to that conversation in mail.
func (a *App) GetContactMessages(email string, limit int) ([]*message.ContactMessage, error) {
	if a.messageStore == nil {
		return []*message.ContactMessage{}, nil
	}
	return a.messageStore.ListByParticipant(email, limit)
}
