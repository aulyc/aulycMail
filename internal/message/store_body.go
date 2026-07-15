package message

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// UpdateBody updates the body content of a message and marks it as fetched
func (s *Store) UpdateBody(messageID, bodyHTML, bodyText, snippet string, hasAttachments bool) error {
	query := `
		UPDATE messages
		SET body_html = ?, body_text = ?, snippet = ?, body_fetched = 1,
		    body_failed = 0, has_attachments = ?
		WHERE id = ?
	`
	_, err := s.db.Exec(query, nullString(bodyHTML), nullString(bodyText), nullString(snippet), hasAttachments, messageID)
	if err != nil {
		return fmt.Errorf("failed to update body: %w", err)
	}
	return nil
}

// RestoreBody replaces a message body and its parsed attachment metadata in a
// single transaction. The raw source must be parsed and identity-checked by the
// caller before this persistence boundary.
func (s *Store) RestoreBody(messageID, bodyHTML, bodyText, snippet string, hasAttachments bool, attachments []*Attachment) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin body restore: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(`
		UPDATE messages
		SET body_html = ?, body_text = ?, snippet = ?, body_fetched = 1,
		    body_failed = 0, has_attachments = ?
		WHERE id = ?
	`, nullString(bodyHTML), nullString(bodyText), nullString(snippet), hasAttachments, messageID)
	if err != nil {
		return fmt.Errorf("failed to restore message body: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify restored message body: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("message not found during body restore: %s", messageID)
	}

	if _, err := tx.Exec(`DELETE FROM attachments WHERE message_id = ?`, messageID); err != nil {
		return fmt.Errorf("failed to replace attachment metadata: %w", err)
	}
	if len(attachments) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO attachments (id, message_id, filename, content_type, size, content_id, is_inline, local_path, content)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare attachment restore: %w", err)
		}
		defer stmt.Close()
		for _, attachment := range attachments {
			if attachment == nil {
				return errors.New("cannot restore nil attachment metadata")
			}
			var content []byte
			if attachment.IsInline && len(attachment.Content) > 0 {
				content = attachment.Content
			}
			if _, err := stmt.Exec(
				attachment.ID, messageID, attachment.Filename, attachment.ContentType,
				attachment.Size, nullString(attachment.ContentID), boolToInt(attachment.IsInline),
				nullString(attachment.LocalPath), content,
			); err != nil {
				return fmt.Errorf("failed to restore attachment metadata: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit body restore: %w", err)
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

// BodyUpdate holds body content for batch updates
type BodyUpdate struct {
	MessageID      string
	BodyHTML       string
	BodyText       string
	Snippet        string
	HasAttachments bool
}

// UpdateBodiesBatch updates body content for multiple messages in a single transaction
func (s *Store) UpdateBodiesBatch(updates []BodyUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		UPDATE messages
		SET body_html = ?, body_text = ?, snippet = ?, body_fetched = 1,
		    body_failed = 0, has_attachments = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		_, err := stmt.Exec(
			nullString(u.BodyHTML), nullString(u.BodyText), nullString(u.Snippet),
			u.HasAttachments,
			u.MessageID,
		)
		if err != nil {
			s.log.Warn().Err(err).Str("messageID", u.MessageID).Msg("Failed to update body in batch")
			// Continue with other updates
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ClearBodiesForFolder clears body content for all messages in a folder.
// This resets body_html, body_text, snippet to NULL and body_fetched to 0,
// allowing the messages to be re-fetched and re-parsed during the next body sync.
func (s *Store) ClearBodiesForFolder(folderID string) (int64, error) {
	query := `
		UPDATE messages
		SET body_html = NULL, body_text = NULL, snippet = NULL, body_fetched = 0
		WHERE folder_id = ?
	`
	result, err := s.db.Exec(query, folderID)
	if err != nil {
		return 0, fmt.Errorf("failed to clear bodies for folder: %w", err)
	}

	affected, _ := result.RowsAffected()
	s.log.Info().Str("folderID", folderID).Int64("affected", affected).Msg("Cleared bodies for folder")
	return affected, nil
}
