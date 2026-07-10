package message

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ContactMessage is a compact mail summary for the Contacts detail view —
// one row per message involving a given contact address.
type ContactMessage struct {
	ID           string    `json:"id"`
	ThreadID     string    `json:"threadId"`
	AccountID    string    `json:"accountId"`
	AccountName  string    `json:"accountName,omitempty"`
	AccountEmail string    `json:"accountEmail,omitempty"`
	FolderID     string    `json:"folderId"`
	Subject      string    `json:"subject"`
	FromName     string    `json:"fromName"`
	FromEmail    string    `json:"fromEmail"`
	Date         time.Time `json:"date" ts_type:"string"`
	IsRead       bool      `json:"isRead"`
	Incoming     bool      `json:"incoming"` // true when the contact is the sender
	Snippet      string    `json:"snippet"`  // short body preview (search overlay)
}

// ListByParticipant returns recent messages where the given email address is a
// participant (sender, To, Cc, or Bcc), across all folders and accounts, newest
// first. Drafts are excluded (not real correspondence). Used by the Contacts
// detail "related mail" list.
func (s *Store) ListByParticipant(email string, limit int) ([]*ContactMessage, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return []*ContactMessage{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	jsonAddressMatch := `
		EXISTS (
			SELECT 1
			FROM json_each(CASE WHEN json_valid(COALESCE(%s, '')) THEN %s ELSE '[]' END) addr
			WHERE LOWER(COALESCE(json_extract(addr.value, '$.email'), json_extract(addr.value, '$.address'), '')) = ?
		)`

	rows, err := s.db.Query(`
		SELECT m.id, COALESCE(m.thread_id, m.id), m.subject, m.from_name, m.from_email,
		       m.date, m.folder_id, f.account_id, COALESCE(a.name, ''), COALESCE(a.email, ''), m.is_read
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
		JOIN accounts a ON f.account_id = a.id
		WHERE f.folder_type != 'drafts'
		  AND (
		    LOWER(m.from_email) = ?
		    OR `+fmt.Sprintf(jsonAddressMatch, "m.to_list", "m.to_list")+`
		    OR `+fmt.Sprintf(jsonAddressMatch, "m.cc_list", "m.cc_list")+`
		    OR `+fmt.Sprintf(jsonAddressMatch, "m.bcc_list", "m.bcc_list")+`
		  )
		ORDER BY m.date DESC
		LIMIT ?
	`, email, email, email, email, limit)
	if err != nil {
		return nil, fmt.Errorf("query contact messages: %w", err)
	}
	defer rows.Close()

	var out []*ContactMessage
	for rows.Next() {
		m := &ContactMessage{}
		var dateStr sql.NullString
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Subject, &m.FromName, &m.FromEmail,
			&dateStr, &m.FolderID, &m.AccountID, &m.AccountName, &m.AccountEmail, &m.IsRead); err != nil {
			return nil, fmt.Errorf("scan contact message: %w", err)
		}
		if dateStr.Valid {
			m.Date = parseTimeString(dateStr.String)
		}
		m.Incoming = strings.EqualFold(m.FromEmail, email)
		out = append(out, m)
	}
	return out, nil
}

// SearchMessagesInFolder returns messages in a folder whose subject, sender, or
// recipients contain the query as a SUBSTRING (case-insensitive), newest first.
// Unlike the FTS-backed conversation search, this matches mid-word and suffix
// fragments (e.g. "亚军" inside "廖亚军", "yajun" inside "liaoyajun"), which is
// what the `/` search overlay needs. Powers the overlay's mail results.
func (s *Store) SearchMessagesInFolder(folderID, query string, limit int) ([]*ContactMessage, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if folderID == "" || q == "" {
		return []*ContactMessage{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	like := "%" + q + "%"

	rows, err := s.db.Query(`
		SELECT m.id, COALESCE(m.thread_id, m.id), m.subject, m.from_name, m.from_email,
		       m.date, m.folder_id, f.account_id, m.is_read,
		       COALESCE(m.snippet, ''), COALESCE(a.name, ''), COALESCE(a.email, '')
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
		JOIN accounts a ON f.account_id = a.id
		WHERE m.folder_id = ?
		  AND (
		    LOWER(COALESCE(m.subject, '')) LIKE ?
		    OR LOWER(COALESCE(m.from_name, '')) LIKE ?
		    OR LOWER(COALESCE(m.from_email, '')) LIKE ?
		    OR LOWER(COALESCE(m.to_list, '')) LIKE ?
		    OR LOWER(COALESCE(m.cc_list, '')) LIKE ?
		    OR LOWER(COALESCE(m.snippet, '')) LIKE ?
		  )
		ORDER BY m.date DESC
		LIMIT ?
	`, folderID, like, like, like, like, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages in folder: %w", err)
	}
	defer rows.Close()

	var out []*ContactMessage
	for rows.Next() {
		m := &ContactMessage{}
		var dateStr sql.NullString
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Subject, &m.FromName, &m.FromEmail,
			&dateStr, &m.FolderID, &m.AccountID, &m.IsRead, &m.Snippet, &m.AccountName, &m.AccountEmail); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if dateStr.Valid {
			m.Date = parseTimeString(dateStr.String)
		}
		// Direction: received unless the sender is this account's own address
		// (mirrors the Contacts "related mail" arrow icons).
		m.Incoming = !strings.EqualFold(strings.TrimSpace(m.FromEmail), strings.TrimSpace(m.AccountEmail))
		out = append(out, m)
	}
	return out, nil
}

// SearchMessagesInAccount returns messages matching query across every folder
// in an account. When accountID is empty, it searches every account.
func (s *Store) SearchMessagesInAccount(accountID, query string, limit int) ([]*ContactMessage, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return []*ContactMessage{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	like := "%" + q + "%"

	rows, err := s.db.Query(`
		SELECT m.id, COALESCE(m.thread_id, m.id), m.subject, m.from_name, m.from_email,
		       m.date, m.folder_id, f.account_id, m.is_read,
		       COALESCE(m.snippet, ''), COALESCE(a.name, ''), COALESCE(a.email, '')
		FROM messages m
		JOIN folders f ON m.folder_id = f.id
		JOIN accounts a ON f.account_id = a.id
		WHERE (? = '' OR f.account_id = ?)
		  AND (
		    LOWER(COALESCE(m.subject, '')) LIKE ?
		    OR LOWER(COALESCE(m.from_name, '')) LIKE ?
		    OR LOWER(COALESCE(m.from_email, '')) LIKE ?
		    OR LOWER(COALESCE(m.to_list, '')) LIKE ?
		    OR LOWER(COALESCE(m.cc_list, '')) LIKE ?
		    OR LOWER(COALESCE(m.snippet, '')) LIKE ?
		  )
		ORDER BY m.date DESC
		LIMIT ?
	`, accountID, accountID, like, like, like, like, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages in account: %w", err)
	}
	defer rows.Close()

	var out []*ContactMessage
	for rows.Next() {
		m := &ContactMessage{}
		var dateStr sql.NullString
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Subject, &m.FromName, &m.FromEmail,
			&dateStr, &m.FolderID, &m.AccountID, &m.IsRead, &m.Snippet, &m.AccountName, &m.AccountEmail); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if dateStr.Valid {
			m.Date = parseTimeString(dateStr.String)
		}
		m.Incoming = !strings.EqualFold(strings.TrimSpace(m.FromEmail), strings.TrimSpace(m.AccountEmail))
		out = append(out, m)
	}
	return out, nil
}
