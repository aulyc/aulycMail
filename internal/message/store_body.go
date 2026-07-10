package message

import (
	"database/sql"
	"fmt"
	"time"
)

// UpdateBody updates the body content of a message and marks it as fetched
func (s *Store) UpdateBody(messageID, bodyHTML, bodyText, snippet string, hasAttachments bool) error {
	query := `
		UPDATE messages
		SET body_html = ?, body_text = ?, snippet = ?, body_fetched = 1, has_attachments = ?
		WHERE id = ?
	`
	_, err := s.db.Exec(query, nullString(bodyHTML), nullString(bodyText), nullString(snippet), hasAttachments, messageID)
	if err != nil {
		return fmt.Errorf("failed to update body: %w", err)
	}
	return nil
}

// GetMessagesWithoutBody returns message IDs that don't have their body fetched yet
// GetMessagesWithoutBody returns message IDs that don't have their body fetched yet,
// or have body_fetched=1 but empty body content (self-healing for failed parses).
// If sinceDate is not zero, only returns messages dated on or after that date.
func (s *Store) GetMessagesWithoutBody(folderID string, limit int, sinceDate time.Time) ([]string, error) {
	var query string
	var rows *sql.Rows
	var err error

	// Include messages where body_fetched=0 OR body was fetched but is empty (needs re-fetch)
	// Exclude encrypted messages which intentionally have empty body (decrypted on-view)
	if sinceDate.IsZero() {
		query = `
			SELECT id FROM messages
			WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0
			ORDER BY date DESC
			LIMIT ?
		`
		rows, err = s.db.Query(query, folderID, limit)
	} else {
		query = `
			SELECT id FROM messages
			WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0 AND (date >= ? OR date < '1970-01-01')
			ORDER BY date DESC
			LIMIT ?
		`
		rows, err = s.db.Query(query, folderID, sinceDate, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query messages without body: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan message id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// MessageWithSize holds a message ID and its RFC822 size for batch planning
type MessageWithSize struct {
	ID   string
	Size int
}

// GetMessagesWithoutBodyAndSize returns message IDs and sizes that don't have their body fetched yet,
// ordered by date descending (newest first). Used for byte-aware batch planning.
// If sinceDate is not zero, only returns messages dated on or after that date.
func (s *Store) GetMessagesWithoutBodyAndSize(folderID string, limit int, sinceDate time.Time) ([]MessageWithSize, error) {
	var query string
	var rows *sql.Rows
	var err error

	// Include messages where body_fetched=0 OR body was fetched but is empty (needs re-fetch)
	// Exclude encrypted messages which intentionally have empty body (decrypted on-view)
	if sinceDate.IsZero() {
		query = `
			SELECT id, size FROM messages
			WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0
			ORDER BY date DESC
			LIMIT ?
		`
		rows, err = s.db.Query(query, folderID, limit)
	} else {
		query = `
			SELECT id, size FROM messages
			WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0 AND (date >= ? OR date < '1970-01-01')
			ORDER BY date DESC
			LIMIT ?
		`
		rows, err = s.db.Query(query, folderID, sinceDate, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query messages without body: %w", err)
	}
	defer rows.Close()

	var messages []MessageWithSize
	for rows.Next() {
		var msg MessageWithSize
		if err := rows.Scan(&msg.ID, &msg.Size); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// CountMessagesWithoutBody returns the count of messages that don't have their body fetched,
// or have body_fetched=1 but empty body content (self-healing for failed parses).
// If sinceDate is not zero, only counts messages dated on or after that date.
func (s *Store) CountMessagesWithoutBody(folderID string, sinceDate time.Time) (int, error) {
	var count int
	var err error

	// Include messages where body_fetched=0 OR body was fetched but is empty (needs re-fetch)
	// Exclude encrypted messages which intentionally have empty body (decrypted on-view)
	if sinceDate.IsZero() {
		err = s.db.QueryRow(
			`SELECT COUNT(*) FROM messages WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0`,
			folderID,
		).Scan(&count)
	} else {
		err = s.db.QueryRow(
			`SELECT COUNT(*) FROM messages WHERE folder_id = ? AND (
				body_fetched = 0 OR
				(body_fetched = 1 AND smime_encrypted = 0 AND pgp_encrypted = 0 AND (body_text IS NULL OR body_text = '') AND (body_html IS NULL OR body_html = ''))
			) AND body_failed = 0 AND (date >= ? OR date < '1970-01-01')`,
			folderID, sinceDate,
		).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to count messages without body: %w", err)
	}
	return count, nil
}
