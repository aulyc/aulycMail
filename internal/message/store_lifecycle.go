package message

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// DeleteByUID deletes a message by folder ID and UID
func (s *Store) DeleteByUID(folderID string, uid uint32) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE folder_id = ? AND uid = ?", folderID, uid)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// DeleteByFolder deletes all messages in a folder
func (s *Store) DeleteByFolder(folderID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE folder_id = ?", folderID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}
	return nil
}

// ExistsInFolder checks if a message with the given RFC 822 Message-ID exists
// in a folder of the specified type (e.g., "trash", "spam") for the account.
func (s *Store) ExistsInFolder(messageID string, folderType string, accountID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM messages m
		JOIN folders f ON m.folder_id = f.id
		WHERE m.message_id = ? AND f.folder_type = ? AND m.account_id = ?
	`, messageID, folderType, accountID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check message in folder type: %w", err)
	}
	return count > 0, nil
}

// HasCopiesInOtherFolders checks if a message with the same RFC 822 Message-ID
// exists in any other folder (excluding the specified folder) for the account.
// Used by Gmail-aware permanent delete to avoid destroying the underlying message
// when it's still visible in other labels.
func (s *Store) HasCopiesInOtherFolders(messageIDHeader string, excludeFolderID string, accountID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM messages
		WHERE message_id = ? AND folder_id != ? AND account_id = ? AND uid > 0
	`, messageIDHeader, excludeFolderID, accountID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for copies: %w", err)
	}
	return count > 0, nil
}

// GetAllUIDs returns all UIDs for a folder
func (s *Store) GetAllUIDs(folderID string) ([]uint32, error) {
	rows, err := s.db.Query("SELECT uid FROM messages WHERE folder_id = ? AND uid > 0", folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query UIDs: %w", err)
	}
	defer rows.Close()

	var uids []uint32
	for rows.Next() {
		var uid uint32
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("failed to scan UID: %w", err)
		}
		uids = append(uids, uid)
	}

	return uids, nil
}

// GetHighestUID returns the highest UID in a folder
func (s *Store) GetHighestUID(folderID string) (uint32, error) {
	var uid sql.NullInt64
	err := s.db.QueryRow("SELECT MAX(uid) FROM messages WHERE folder_id = ?", folderID).Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("failed to get highest UID: %w", err)
	}
	if uid.Valid {
		return uint32(uid.Int64), nil
	}
	return 0, nil
}

// CountByFolder returns the total message count for a folder
func (s *Store) CountByFolder(folderID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM messages WHERE folder_id = ?", folderID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return count, nil
}

// DeleteOlderThan deletes messages older than the specified time for an account
// Returns the number of messages deleted
func (s *Store) DeleteOlderThan(accountID string, before time.Time) (int, error) {
	result, err := s.db.Exec(
		"DELETE FROM messages WHERE account_id = ? AND date < ?",
		accountID, before,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old messages: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected > 0 {
		s.log.Info().
			Str("accountID", accountID).
			Time("before", before).
			Int64("deleted", affected).
			Msg("Deleted old messages based on sync period")
	}

	return int(affected), nil
}

// GetMessageUIDAndFolder returns the UID and folder_id for a message
func (s *Store) GetMessageUIDAndFolder(messageID string) (uint32, string, error) {
	var uidI64 int64
	var folderID string
	err := s.db.QueryRow(
		"SELECT uid, folder_id FROM messages WHERE id = ?",
		messageID,
	).Scan(&uidI64, &folderID)
	if err == sql.ErrNoRows {
		return 0, "", fmt.Errorf("message not found: %s", messageID)
	}
	if err != nil {
		return 0, "", fmt.Errorf("failed to get message: %w", err)
	}
	return uint32(uidI64), folderID, nil
}

// UIDInfo holds UID and folder information for a message
type UIDInfo struct {
	UID      uint32
	FolderID string
}

// GetMessageUIDsAndFolder returns UIDs and folder_ids for multiple messages in one query
func (s *Store) GetMessageUIDsAndFolder(messageIDs []string) (map[string]UIDInfo, error) {
	if len(messageIDs) == 0 {
		return make(map[string]UIDInfo), nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT id, uid, folder_id FROM messages WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query message UIDs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]UIDInfo)
	for rows.Next() {
		var id string
		var uidI64 int64
		var folderID string
		if err := rows.Scan(&id, &uidI64, &folderID); err != nil {
			return nil, fmt.Errorf("failed to scan message UID: %w", err)
		}
		result[id] = UIDInfo{UID: uint32(uidI64), FolderID: folderID}
	}

	return result, nil
}

// MoveMessages updates the folder_id for multiple messages.
//
// Design note — temp UIDs:
// When a message is moved locally, it lives in the destination folder before
// the IMAP server has assigned a real UID for it there. To bridge that gap
// without using NULL or colliding with real UIDs, this function writes
// `uid = -rowid` — a guaranteed-unique negative value derived from SQLite's
// auto-increment row id. Real IMAP UIDs are positive uint32, so the sign
// distinguishes "temp" from "synced" cleanly:
//
//   - WHERE uid > 0   → only real, server-assigned UIDs
//   - WHERE uid < 0   → only locally-moved rows awaiting reconciliation
//   - DeleteTempUIDs cleans up uid < 0 rows
//   - app/actions.go skips rows where int32(m.UID) < 0 before sending to IMAP
//
// The Go layer reads uid into uint32 (the IMAP type), so a stored int64(-37727)
// becomes uint32(0xFFFF6CE1) = 4_294_929_569 in memory. The int32() recast in
// the skip check correctly identifies these. Scan sites that read the uid
// column must use an int64 intermediary first — scanning a negative int64
// directly into uint32 fails on modernc.org/sqlite's converter (the lower 32
// bits of the int64 are preserved by the explicit uint32 cast).
//
// Potential future refactor: split this into a dedicated `pending_move BOOLEAN`
// (or `sync_state TEXT`) column. The current design works fine; the
// motivation for refactoring would be onboarding clarity — making the
// temp-marker concept explicit in the schema rather than implicit in the sign
// of the uid column.
func (s *Store) MoveMessages(ids []string, newFolderID string) error {
	if len(ids) == 0 {
		return nil
	}

	// Deduplicate message IDs to prevent constraint violations
	seen := make(map[string]bool)
	uniqueIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	placeholders := make([]string, len(uniqueIDs))
	args := []interface{}{newFolderID}
	for i, id := range uniqueIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(
		"UPDATE messages SET folder_id = ?, uid = -rowid WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to move messages: %w", err)
	}
	return nil
}

// DeleteTempUIDs removes messages with temporary negative UIDs in a folder.
// These are left over after MoveMessages assigns -rowid as a placeholder UID.
func (s *Store) DeleteTempUIDs(folderID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE folder_id = ? AND uid < 0", folderID)
	if err != nil {
		return fmt.Errorf("failed to delete temp UID messages: %w", err)
	}
	return nil
}

// GetIDsByMessageIDs finds local DB message IDs by RFC822 Message-ID header and folder.
// Only returns messages with positive UIDs (excludes temp rows from in-flight moves).
func (s *Store) GetIDsByMessageIDs(accountID, folderID string, rfc822MessageIDs []string) ([]string, error) {
	if len(rfc822MessageIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(rfc822MessageIDs))
	args := []interface{}{accountID, folderID}
	for i, mid := range rfc822MessageIDs {
		placeholders[i] = "?"
		args = append(args, mid)
	}

	query := fmt.Sprintf(
		"SELECT id FROM messages WHERE account_id = ? AND folder_id = ? AND uid > 0 AND message_id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan message ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteBatch deletes multiple messages by their IDs
func (s *Store) DeleteBatch(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM messages WHERE id IN (%s)", strings.Join(placeholders, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages batch: %w", err)
	}
	return nil
}

// GetByIDs retrieves multiple messages by their IDs
// SpansMultipleAccounts returns true when the given message IDs belong
// to two or more different accounts. Cheap (single SELECT COUNT
// DISTINCT against the indexed account_id column) — used by bulk-action
// callers in app/actions.go to decide between the single-account fast
// path and the cross-account partition+dispatch path. A nil/short input
// returns false without hitting the DB.
func (s *Store) SpansMultipleAccounts(ids []string) (bool, error) {
	if len(ids) < 2 {
		return false, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		"SELECT COUNT(DISTINCT account_id) FROM messages WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		return false, fmt.Errorf("failed to check account span: %w", err)
	}
	return n > 1, nil
}

func (s *Store) GetByIDs(ids []string) ([]*Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, account_id, folder_id, uid, message_id, in_reply_to, references_list, thread_id,
		       subject, from_name, from_email, to_list, cc_list, bcc_list, reply_to, date,
		       snippet, is_read, is_starred, is_answered, is_forwarded, is_draft, is_deleted,
		       size, has_attachments, body_text, body_html, body_fetched,
		       read_receipt_to, read_receipt_handled,
		       received_at
		FROM messages WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		m := &Message{}
		var messageID, inReplyTo, references, threadID, toList, ccList, bccList, replyTo, snippet, bodyText, bodyHTML, readReceiptTo sql.NullString
		var dateStr, receivedAtStr sql.NullString
		var uidI64 int64

		err := rows.Scan(
			&m.ID, &m.AccountID, &m.FolderID, &uidI64, &messageID, &inReplyTo, &references, &threadID,
			&m.Subject, &m.FromName, &m.FromEmail, &toList, &ccList, &bccList, &replyTo, &dateStr,
			&snippet, &m.IsRead, &m.IsStarred, &m.IsAnswered, &m.IsForwarded, &m.IsDraft, &m.IsDeleted,
			&m.Size, &m.HasAttachments, &bodyText, &bodyHTML, &m.BodyFetched,
			&readReceiptTo, &m.ReadReceiptHandled,
			&receivedAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		m.UID = uint32(uidI64)

		if messageID.Valid {
			m.MessageID = messageID.String
		}
		if inReplyTo.Valid {
			m.InReplyTo = inReplyTo.String
		}
		if references.Valid {
			m.References = references.String
		}
		if threadID.Valid {
			m.ThreadID = threadID.String
		}
		if toList.Valid {
			m.ToList = toList.String
		}
		if ccList.Valid {
			m.CcList = ccList.String
		}
		if bccList.Valid {
			m.BccList = bccList.String
		}
		if replyTo.Valid {
			m.ReplyTo = replyTo.String
		}
		if dateStr.Valid && dateStr.String != "" {
			m.Date = parseTimeString(dateStr.String)
		}
		if snippet.Valid {
			m.Snippet = snippet.String
		}
		if bodyText.Valid {
			m.BodyText = bodyText.String
		}
		if bodyHTML.Valid {
			m.BodyHTML = bodyHTML.String
		}
		if readReceiptTo.Valid {
			m.ReadReceiptTo = readReceiptTo.String
		}
		if receivedAtStr.Valid && receivedAtStr.String != "" {
			m.ReceivedAt = parseTimeString(receivedAtStr.String)
		}

		messages = append(messages, m)
	}

	return messages, nil
}
