package message

import (
	"fmt"
	"strings"
)

// UpdateFlags updates only the flags for a message
func (s *Store) UpdateFlags(id string, isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted bool) error {
	query := `
		UPDATE messages SET
			is_read = ?, is_starred = ?, is_answered = ?, is_forwarded = ?,
			is_draft = ?, is_deleted = ?
		WHERE id = ?
		  AND (
			is_read != ? OR is_starred != ? OR is_answered != ? OR is_forwarded != ? OR
			is_draft != ? OR is_deleted != ?
		  )
	`

	_, err := s.db.Exec(query,
		isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted, id,
		isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted,
	)
	if err != nil {
		return fmt.Errorf("failed to update flags: %w", err)
	}

	return nil
}

// UpdateFlagsByUID updates flags for a message by folder ID and UID
func (s *Store) UpdateFlagsByUID(folderID string, uid uint32, isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted bool) error {
	query := `
		UPDATE messages SET
			is_read = ?, is_starred = ?, is_answered = ?, is_forwarded = ?,
			is_draft = ?, is_deleted = ?
		WHERE folder_id = ? AND uid = ?
		  AND (
			is_read != ? OR is_starred != ? OR is_answered != ? OR is_forwarded != ? OR
			is_draft != ? OR is_deleted != ?
		  )
	`

	_, err := s.db.Exec(query,
		isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted, folderID, uid,
		isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted,
	)
	if err != nil {
		return fmt.Errorf("failed to update flags by UID: %w", err)
	}

	return nil
}

// FlagUpdate represents a flag update for a single message by UID
type FlagUpdate struct {
	UID         uint32
	IsRead      bool
	IsStarred   bool
	IsAnswered  bool
	IsForwarded bool
	IsDraft     bool
	IsDeleted   bool
}

// UpdateFlagsByUIDBatch updates flags for multiple messages in a single transaction.
// This is much more efficient than calling UpdateFlagsByUID repeatedly.
func (s *Store) UpdateFlagsByUIDBatch(folderID string, updates []FlagUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		UPDATE messages SET
			is_read = ?, is_starred = ?, is_answered = ?, is_forwarded = ?,
			is_draft = ?, is_deleted = ?
		WHERE folder_id = ? AND uid = ?
		  AND (
			is_read != ? OR is_starred != ? OR is_answered != ? OR is_forwarded != ? OR
			is_draft != ? OR is_deleted != ?
		  )
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		_, err := stmt.Exec(
			u.IsRead, u.IsStarred, u.IsAnswered, u.IsForwarded, u.IsDraft, u.IsDeleted, folderID, u.UID,
			u.IsRead, u.IsStarred, u.IsAnswered, u.IsForwarded, u.IsDraft, u.IsDeleted,
		)
		if err != nil {
			return fmt.Errorf("failed to update flags for UID %d: %w", u.UID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateFlagsBatch updates flags for multiple messages by their IDs
// Pass nil for flags you don't want to update
func (s *Store) UpdateFlagsBatch(ids []string, isRead, isStarred *bool) error {
	if len(ids) == 0 {
		return nil
	}

	// Build dynamic SET clause based on what's being updated
	var setClauses []string
	var args []interface{}

	if isRead != nil {
		setClauses = append(setClauses, "is_read = ?")
		args = append(args, *isRead)
	}
	if isStarred != nil {
		setClauses = append(setClauses, "is_starred = ?")
		args = append(args, *isStarred)
	}

	if len(setClauses) == 0 {
		return nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(
		"UPDATE messages SET %s WHERE id IN (%s)",
		strings.Join(setClauses, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update flags batch: %w", err)
	}
	return nil
}

// MarkBodyFailed flags messages whose body fetch+parse produced no usable content
// so they are excluded from future body-fetch queries (GetMessagesWithoutBody and
// friends). Idempotent: re-flagging an already-flagged row is a no-op. The flag
// survives across sessions; clear it via a one-off migration if a future parser
// improvement should retry these messages.
//
// Without this persistent flag, an unparseable message stays empty in the local
// DB, GetMessagesWithoutBody picks it up next sync, IMAP FETCH runs again, parse
// fails again — forever. See migration v39.
func (s *Store) MarkBodyFailed(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"UPDATE messages SET body_failed = 1 WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	if _, err := s.db.Exec(query, args...); err != nil {
		return fmt.Errorf("failed to mark messages body-failed: %w", err)
	}
	return nil
}
