package message

import "fmt"

func (s *Store) MarkReadReceiptHandled(id string) error {
	_, err := s.db.Exec("UPDATE messages SET read_receipt_handled = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to mark read receipt handled: %w", err)
	}
	return nil
}

// MarkComposeDraft records that a source message currently has a reply/forward draft.
// A sent status is kept if the message was already successfully handled.
func (s *Store) MarkComposeDraft(sourceMessageID, actionType, draftID string) error {
	actionType = NormalizeComposeAction(actionType)
	if sourceMessageID == "" || actionType == "" || draftID == "" {
		return nil
	}

	query := `
		INSERT INTO message_compose_status (source_message_id, action_type, status, draft_id, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source_message_id) DO UPDATE SET
			action_type = CASE
				WHEN message_compose_status.status = ? THEN message_compose_status.action_type
				ELSE excluded.action_type
			END,
			status = CASE
				WHEN message_compose_status.status = ? THEN message_compose_status.status
				ELSE excluded.status
			END,
			draft_id = CASE
				WHEN message_compose_status.status = ? THEN message_compose_status.draft_id
				ELSE excluded.draft_id
			END,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query,
		sourceMessageID, actionType, ComposeStatusDraft, draftID,
		ComposeStatusSent, ComposeStatusSent, ComposeStatusSent,
	)
	if err != nil {
		return fmt.Errorf("failed to mark compose draft: %w", err)
	}
	return nil
}

// MarkComposeSent records that a reply/reply-all/forward for a source message
// has been sent successfully.
func (s *Store) MarkComposeSent(sourceMessageID, actionType string) error {
	actionType = NormalizeComposeAction(actionType)
	if sourceMessageID == "" || actionType == "" {
		return nil
	}

	query := `
		INSERT INTO message_compose_status (source_message_id, action_type, status, draft_id, updated_at)
		VALUES (?, ?, ?, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(source_message_id) DO UPDATE SET
			action_type = excluded.action_type,
			status = excluded.status,
			draft_id = NULL,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, sourceMessageID, actionType, ComposeStatusSent)
	if err != nil {
		return fmt.Errorf("failed to mark compose sent: %w", err)
	}
	return nil
}

// ClearComposeDraft removes a draft marker if the marker still points at the
// deleted draft. Sent markers are intentionally preserved.
func (s *Store) ClearComposeDraft(sourceMessageID, draftID string) error {
	if sourceMessageID == "" || draftID == "" {
		return nil
	}

	_, err := s.db.Exec(
		`DELETE FROM message_compose_status WHERE source_message_id = ? AND draft_id = ? AND status = ?`,
		sourceMessageID,
		draftID,
		ComposeStatusDraft,
	)
	if err != nil {
		return fmt.Errorf("failed to clear compose draft: %w", err)
	}
	return nil
}
