package message

import (
	"database/sql"
	"fmt"
)

// ListByFolder returns message headers for a folder with pagination
func (s *Store) ListByFolder(folderID string, offset, limit int) ([]*MessageHeader, error) {
	query := `
		SELECT id, account_id, folder_id, uid, subject, from_name, from_email,
		       date, snippet, is_read, is_starred, has_attachments
		FROM messages
		WHERE folder_id = ?
		ORDER BY date DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, folderID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*MessageHeader
	for rows.Next() {
		m := &MessageHeader{}
		var dateStr sql.NullString
		var snippet sql.NullString
		var uidI64 int64

		err := rows.Scan(
			&m.ID, &m.AccountID, &m.FolderID, &uidI64,
			&m.Subject, &m.FromName, &m.FromEmail,
			&dateStr, &snippet,
			&m.IsRead, &m.IsStarred, &m.HasAttachments,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		m.UID = uint32(uidI64)

		if dateStr.Valid && dateStr.String != "" {
			m.Date = parseTimeString(dateStr.String)
		}
		if snippet.Valid {
			m.Snippet = snippet.String
		}

		messages = append(messages, m)
	}

	return messages, nil
}

// GetUnifiedInboxUnreadCount returns the total unread message count across all inbox folders
// Uses the cached folder.unread_count values to stay consistent with sidebar folder counts
func (s *Store) GetUnifiedInboxUnreadCount() (int, error) {
	// First, log individual inbox folders for debugging
	debugQuery := `
		SELECT f.id, f.name, f.folder_type, f.unread_count, a.name as account_name, a.enabled
		FROM folders f
		INNER JOIN accounts a ON f.account_id = a.id
		WHERE f.folder_type = 'inbox' AND f.selectable = 1
	`
	rows, err := s.db.Query(debugQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var folderID, folderName, folderType, accountName string
			var unreadCount int
			var enabled bool
			if err := rows.Scan(&folderID, &folderName, &folderType, &unreadCount, &accountName, &enabled); err == nil {
				s.log.Debug().
					Str("folderID", folderID).
					Str("folderName", folderName).
					Str("folderType", folderType).
					Int("unreadCount", unreadCount).
					Str("accountName", accountName).
					Bool("enabled", enabled).
					Msg("Inbox folder for unified count")
			}
		}
	}

	query := `
		SELECT COALESCE(SUM(f.unread_count), 0)
		FROM folders f
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		WHERE f.folder_type = 'inbox' AND f.selectable = 1
	`

	var count int
	err = s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unified inbox unread: %w", err)
	}

	s.log.Debug().Int("unreadCount", count).Msg("GetUnifiedInboxUnreadCount (sum of folder counts)")
	return count, nil
}

// GetBadgeUnreadCount returns the total unread across all enabled accounts that
// should drive the Dock badge: the inbox plus user folders, but NOT the noise
// folders (sent, drafts, trash, spam) or the virtual folders (all-mail,
// starred, archive) that would double-count or surface junk. For IMAP accounts
// a message lives in exactly one folder, so inbox + custom folders never
// overlap.
func (s *Store) GetBadgeUnreadCount() (int, error) {
	query := `
		SELECT COALESCE(SUM(f.unread_count), 0)
		FROM folders f
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		WHERE f.folder_type IN ('inbox', 'folder') AND f.selectable = 1
	`
	var count int
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count badge unread: %w", err)
	}
	return count, nil
}

// CountUnreadByFolder returns the unread message count for a folder
func (s *Store) CountUnreadByFolder(folderID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM messages WHERE folder_id = ? AND is_read = 0", folderID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread messages: %w", err)
	}
	return count, nil
}

// GetUnreadMessageIDsByFolder returns the IDs of all unread messages in a folder
func (s *Store) GetUnreadMessageIDsByFolder(folderID string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM messages WHERE folder_id = ? AND is_read = 0", folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unread messages: %w", err)
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

// GetReadMessageIDsByFolder returns the IDs of all read messages in a folder
func (s *Store) GetReadMessageIDsByFolder(folderID string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM messages WHERE folder_id = ? AND is_read = 1", folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query read messages: %w", err)
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

// GetAllIDsByFolder returns the IDs of all messages in a folder
func (s *Store) GetAllIDsByFolder(folderID string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM messages WHERE folder_id = ?", folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
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
