package message

import (
	"database/sql"
	"fmt"
	"strings"
)

// normalizeMessageID strips angle brackets from Message-IDs for consistent comparison
func normalizeMessageID(msgID string) string {
	msgID = strings.TrimSpace(msgID)
	msgID = strings.TrimPrefix(msgID, "<")
	msgID = strings.TrimSuffix(msgID, ">")
	return msgID
}

// FindThreadID finds the thread ID for a message based on References and In-Reply-To headers
func (s *Store) FindThreadID(accountID, messageID, inReplyTo string, references []string) (string, error) {
	// Normalize the message ID
	messageID = normalizeMessageID(messageID)
	inReplyTo = normalizeMessageID(inReplyTo)

	// Normalize all references
	normalizedRefs := make([]string, 0, len(references))
	for _, ref := range references {
		if normalized := normalizeMessageID(ref); normalized != "" {
			normalizedRefs = append(normalizedRefs, normalized)
		}
	}

	// Build list of all potential thread roots to check
	allRefs := make([]string, 0)
	if inReplyTo != "" {
		allRefs = append(allRefs, inReplyTo)
	}
	allRefs = append(allRefs, normalizedRefs...)

	if len(allRefs) == 0 {
		// No threading info - this message starts its own thread
		return messageID, nil
	}

	// Check if any of the references match existing messages
	for _, ref := range allRefs {
		// Check with and without angle brackets since DB might have either format
		var existingThreadID sql.NullString

		// Try exact match first
		err := s.db.QueryRow(
			"SELECT COALESCE(thread_id, id) FROM messages WHERE account_id = ? AND (message_id = ? OR message_id = ? OR message_id = ?) LIMIT 1",
			accountID, ref, "<"+ref+">", strings.TrimPrefix(strings.TrimSuffix(ref, ">"), "<"),
		).Scan(&existingThreadID)

		if err == nil && existingThreadID.Valid && existingThreadID.String != "" {
			// Normalize the returned thread ID too
			return normalizeMessageID(existingThreadID.String), nil
		}
	}

	// No existing thread found - use the first reference as thread ID (root message)
	// This is the original message that started the thread
	if len(normalizedRefs) > 0 {
		return normalizedRefs[0], nil
	}
	if inReplyTo != "" {
		return inReplyTo, nil
	}

	return messageID, nil
}

// UpdateThreadID updates the thread_id for a message
func (s *Store) UpdateThreadID(id, threadID string) error {
	_, err := s.db.Exec("UPDATE messages SET thread_id = ? WHERE id = ?", threadID, id)
	if err != nil {
		return fmt.Errorf("failed to update thread_id: %w", err)
	}
	return nil
}

// ReconcileThreads updates thread_ids for messages that reference a newly synced message.
// This ensures that when a new message arrives, any existing messages that reference it
// (via In-Reply-To) get linked to the same thread.
// Returns the number of messages updated.
func (s *Store) ReconcileThreads(accountID, messageID, threadID string) (int, error) {
	// Normalize the message ID for comparison
	normalizedMsgID := normalizeMessageID(messageID)
	if normalizedMsgID == "" {
		return 0, nil
	}

	// Find messages that have in_reply_to pointing to this message's ID
	// and update their thread_id to match
	// We check multiple formats of the message ID (with/without angle brackets)
	query := `
		UPDATE messages
		SET thread_id = ?
		WHERE account_id = ?
		AND thread_id != ?
		AND (
			REPLACE(REPLACE(in_reply_to, '<', ''), '>', '') = ?
			OR in_reply_to = ?
			OR in_reply_to = ?
		)
	`

	result, err := s.db.Exec(query,
		threadID,
		accountID,
		threadID,
		normalizedMsgID,
		normalizedMsgID,
		"<"+normalizedMsgID+">",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to reconcile threads: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		s.log.Info().
			Str("accountID", accountID).
			Str("messageID", messageID).
			Str("threadID", threadID).
			Int64("updated", affected).
			Msg("Reconciled thread IDs for related messages")
	}

	return int(affected), nil
}

// ReconcileThreadsForNewMessage is called after syncing a new message.
// It checks both directions:
// 1. If this message references other messages, update this message's thread_id to match
// 2. If other messages reference this message's ID, update their thread_ids to match this one
func (s *Store) ReconcileThreadsForNewMessage(accountID, messageUUID, messageID, threadID, inReplyTo string) error {
	normalizedMsgID := normalizeMessageID(messageID)
	normalizedThreadID := normalizeMessageID(threadID)
	normalizedInReplyTo := normalizeMessageID(inReplyTo)

	// Direction 1: This message replies to another - find the original and adopt its thread_id
	if normalizedInReplyTo != "" {
		var existingThreadID sql.NullString
		err := s.db.QueryRow(`
			SELECT COALESCE(thread_id, id) FROM messages
			WHERE account_id = ?
			AND (
				REPLACE(REPLACE(message_id, '<', ''), '>', '') = ?
				OR message_id = ?
				OR message_id = ?
			)
			LIMIT 1
		`, accountID, normalizedInReplyTo, normalizedInReplyTo, "<"+normalizedInReplyTo+">").Scan(&existingThreadID)

		if err == nil && existingThreadID.Valid && existingThreadID.String != "" {
			existingNormalized := normalizeMessageID(existingThreadID.String)
			if existingNormalized != normalizedThreadID {
				// Update this message's thread_id to match the existing thread
				if err := s.UpdateThreadID(messageUUID, existingNormalized); err != nil {
					s.log.Warn().Err(err).Msg("Failed to update thread_id for reply")
				} else {
					s.log.Debug().
						Str("messageUUID", messageUUID).
						Str("oldThreadID", normalizedThreadID).
						Str("newThreadID", existingNormalized).
						Msg("Updated reply message to join existing thread")
					normalizedThreadID = existingNormalized
				}
			}
		}
	}

	// Direction 2: Other messages may have replied to this one - update their thread_ids
	if normalizedMsgID != "" {
		_, err := s.ReconcileThreads(accountID, normalizedMsgID, normalizedThreadID)
		if err != nil {
			s.log.Warn().Err(err).Msg("Failed to reconcile threads for replies to this message")
		}
	}

	return nil
}
