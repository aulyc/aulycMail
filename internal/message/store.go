package message

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/aulyc/aulycmail/internal/database"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Store provides message persistence operations
type Store struct {
	db  *database.DB
	log zerolog.Logger
}

// NewStore creates a new message store
func NewStore(db *database.DB) *Store {
	return &Store{
		db:  db,
		log: logging.WithComponent("message-store"),
	}
}

// filterHavingClause returns a HAVING clause for conversation-level filtering.
// prefix should be "" for single-table queries or "m." for joined queries.
func filterHavingClause(filter, prefix string) string {
	switch filter {
	case "unread":
		return fmt.Sprintf(" HAVING SUM(CASE WHEN %sis_read = 0 THEN 1 ELSE 0 END) > 0", prefix)
	case "starred":
		return fmt.Sprintf(" HAVING MAX(CASE WHEN %sis_starred = 1 THEN 1 ELSE 0 END) = 1", prefix)
	case "attachments":
		return fmt.Sprintf(" HAVING MAX(CASE WHEN %shas_attachments = 1 THEN 1 ELSE 0 END) = 1", prefix)
	default:
		return ""
	}
}

// filterWhereClause returns a WHERE condition for count queries.
// prefix should be "" for single-table queries or "m." for joined queries.
func filterWhereClause(filter, prefix string) string {
	switch filter {
	case "unread":
		return fmt.Sprintf(" AND %sis_read = 0", prefix)
	case "starred":
		return fmt.Sprintf(" AND %sis_starred = 1", prefix)
	case "attachments":
		return fmt.Sprintf(" AND %shas_attachments = 1", prefix)
	default:
		return ""
	}
}

func applyComposeStatusRank(c *Conversation, rank int) {
	switch {
	case rank >= 20:
		c.ComposeStatus = ComposeStatusSent
	case rank >= 10:
		c.ComposeStatus = ComposeStatusDraft
	default:
		return
	}

	switch rank % 10 {
	case 3:
		c.ComposeAction = ComposeActionReplyAll
	case 2:
		c.ComposeAction = ComposeActionReply
	case 1:
		c.ComposeAction = ComposeActionForward
	}
}

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
		WHERE f.folder_type = 'inbox'
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
		WHERE f.folder_type = 'inbox'
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
		WHERE f.folder_type IN ('inbox', 'folder')
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

// Get returns a full message by ID
func (s *Store) Get(id string) (*Message, error) {
	query := `
		SELECT id, account_id, folder_id, uid, message_id, in_reply_to, thread_id,
		       subject, from_name, from_email, to_list, cc_list, bcc_list, reply_to, date,
		       snippet, is_read, is_starred, is_answered, is_forwarded, is_draft, is_deleted,
		       size, has_attachments, body_text, body_html, body_fetched,
		       read_receipt_to, read_receipt_handled,
		       received_at
		FROM messages
		WHERE id = ?
	`

	m := &Message{}
	var messageID, inReplyTo, threadID, toList, ccList, bccList, replyTo, snippet, bodyText, bodyHTML, readReceiptTo sql.NullString
	var dateStr, receivedAtStr sql.NullString
	var uidI64 int64

	err := s.db.QueryRow(query, id).Scan(
		&m.ID, &m.AccountID, &m.FolderID, &uidI64, &messageID, &inReplyTo, &threadID,
		&m.Subject, &m.FromName, &m.FromEmail, &toList, &ccList, &bccList, &replyTo, &dateStr,
		&snippet, &m.IsRead, &m.IsStarred, &m.IsAnswered, &m.IsForwarded, &m.IsDraft, &m.IsDeleted,
		&m.Size, &m.HasAttachments, &bodyText, &bodyHTML, &m.BodyFetched,
		&readReceiptTo, &m.ReadReceiptHandled,
		&receivedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	m.UID = uint32(uidI64)

	if messageID.Valid {
		m.MessageID = messageID.String
	}
	if inReplyTo.Valid {
		m.InReplyTo = inReplyTo.String
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

	return m, nil
}

// GetByUID returns a message by folder ID and UID
func (s *Store) GetByUID(folderID string, uid uint32) (*Message, error) {
	query := `
		SELECT id, account_id, folder_id, uid, message_id, in_reply_to, thread_id,
		       subject, from_name, from_email, to_list, cc_list, bcc_list, reply_to, date,
		       snippet, is_read, is_starred, is_answered, is_forwarded, is_draft, is_deleted,
		       size, has_attachments, body_text, body_html, body_fetched,
		       read_receipt_to, read_receipt_handled,
		       received_at
		FROM messages
		WHERE folder_id = ? AND uid = ?
	`

	m := &Message{}
	var messageID, inReplyTo, threadID, toList, ccList, bccList, replyTo, snippet, bodyText, bodyHTML, readReceiptTo sql.NullString
	var dateStr, receivedAtStr sql.NullString
	var uidI64 int64

	err := s.db.QueryRow(query, folderID, uid).Scan(
		&m.ID, &m.AccountID, &m.FolderID, &uidI64, &messageID, &inReplyTo, &threadID,
		&m.Subject, &m.FromName, &m.FromEmail, &toList, &ccList, &bccList, &replyTo, &dateStr,
		&snippet, &m.IsRead, &m.IsStarred, &m.IsAnswered, &m.IsForwarded, &m.IsDraft, &m.IsDeleted,
		&m.Size, &m.HasAttachments, &bodyText, &bodyHTML, &m.BodyFetched,
		&readReceiptTo, &m.ReadReceiptHandled,
		&receivedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	m.UID = uint32(uidI64)

	// Populate optional fields
	if messageID.Valid {
		m.MessageID = messageID.String
	}
	if inReplyTo.Valid {
		m.InReplyTo = inReplyTo.String
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

	return m, nil
}

// Create creates a new message
func (s *Store) Create(m *Message) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.ReceivedAt.IsZero() {
		m.ReceivedAt = time.Now().UTC()
	}

	s.log.Debug().
		Str("id", m.ID).
		Str("subject", m.Subject).
		Str("messageID", m.MessageID).
		Str("threadID", m.ThreadID).
		Int("bodyTextLen", len(m.BodyText)).
		Int("bodyHTMLLen", len(m.BodyHTML)).
		Uint32("uid", m.UID).
		Msg("Creating message in store")

	query := `
		INSERT INTO messages (
			id, account_id, folder_id, uid, message_id, in_reply_to, references_list, thread_id,
			subject, from_name, from_email, to_list, cc_list, bcc_list, reply_to, date,
			snippet, is_read, is_starred, is_answered, is_forwarded, is_draft, is_deleted,
			size, has_attachments, body_text, body_html, body_fetched,
			read_receipt_to, read_receipt_handled, received_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		m.ID, m.AccountID, m.FolderID, m.UID,
		nullString(m.MessageID), nullString(m.InReplyTo), nullString(m.References), nullString(m.ThreadID),
		m.Subject, m.FromName, m.FromEmail,
		nullString(m.ToList), nullString(m.CcList), nullString(m.BccList), nullString(m.ReplyTo),
		m.Date, nullString(m.Snippet),
		m.IsRead, m.IsStarred, m.IsAnswered, m.IsForwarded, m.IsDraft, m.IsDeleted,
		m.Size, m.HasAttachments,
		nullString(m.BodyText), nullString(m.BodyHTML), m.BodyFetched,
		nullString(m.ReadReceiptTo), m.ReadReceiptHandled,
		m.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

// Upsert inserts a message or updates it if a row with the same (folder_id, uid) already exists.
// This handles cases where a previous copy was deleted but the stale row remains, or where
// the IMAP server reuses UIDs after EXPUNGE.
func (s *Store) Upsert(m *Message) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.ReceivedAt.IsZero() {
		m.ReceivedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO messages (
			id, account_id, folder_id, uid, message_id, in_reply_to, references_list, thread_id,
			subject, from_name, from_email, to_list, cc_list, bcc_list, reply_to, date,
			snippet, is_read, is_starred, is_answered, is_forwarded, is_draft, is_deleted,
			size, has_attachments, body_text, body_html, body_fetched,
			read_receipt_to, read_receipt_handled, received_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(folder_id, uid) DO UPDATE SET
			id=excluded.id, account_id=excluded.account_id,
			message_id=excluded.message_id, in_reply_to=excluded.in_reply_to,
			references_list=excluded.references_list, thread_id=excluded.thread_id,
			subject=excluded.subject, from_name=excluded.from_name, from_email=excluded.from_email,
			to_list=excluded.to_list, cc_list=excluded.cc_list, bcc_list=excluded.bcc_list,
			reply_to=excluded.reply_to, date=excluded.date,
			snippet=excluded.snippet, is_read=excluded.is_read, is_starred=excluded.is_starred,
			is_answered=excluded.is_answered, is_forwarded=excluded.is_forwarded,
			is_draft=excluded.is_draft, is_deleted=excluded.is_deleted,
			size=excluded.size, has_attachments=excluded.has_attachments,
			body_text=excluded.body_text, body_html=excluded.body_html,
			body_fetched=excluded.body_fetched,
			read_receipt_to=excluded.read_receipt_to, read_receipt_handled=excluded.read_receipt_handled,
			received_at=excluded.received_at
	`

	_, err := s.db.Exec(query,
		m.ID, m.AccountID, m.FolderID, m.UID,
		nullString(m.MessageID), nullString(m.InReplyTo), nullString(m.References), nullString(m.ThreadID),
		m.Subject, m.FromName, m.FromEmail,
		nullString(m.ToList), nullString(m.CcList), nullString(m.BccList), nullString(m.ReplyTo),
		m.Date, nullString(m.Snippet),
		m.IsRead, m.IsStarred, m.IsAnswered, m.IsForwarded, m.IsDraft, m.IsDeleted,
		m.Size, m.HasAttachments,
		nullString(m.BodyText), nullString(m.BodyHTML), m.BodyFetched,
		nullString(m.ReadReceiptTo), m.ReadReceiptHandled,
		m.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert message: %w", err)
	}

	return nil
}

// Update updates an existing message
func (s *Store) Update(m *Message) error {
	query := `
		UPDATE messages SET
			message_id = ?, in_reply_to = ?, references_list = ?, thread_id = ?,
			subject = ?, from_name = ?, from_email = ?,
			to_list = ?, cc_list = ?, bcc_list = ?, reply_to = ?, date = ?,
			snippet = ?, is_read = ?, is_starred = ?, is_answered = ?, is_forwarded = ?,
			is_draft = ?, is_deleted = ?, size = ?, has_attachments = ?,
			body_text = ?, body_html = ?, read_receipt_to = ?, read_receipt_handled = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query,
		nullString(m.MessageID), nullString(m.InReplyTo), nullString(m.References), nullString(m.ThreadID),
		m.Subject, m.FromName, m.FromEmail,
		nullString(m.ToList), nullString(m.CcList), nullString(m.BccList), nullString(m.ReplyTo),
		m.Date, nullString(m.Snippet),
		m.IsRead, m.IsStarred, m.IsAnswered, m.IsForwarded,
		m.IsDraft, m.IsDeleted, m.Size, m.HasAttachments,
		nullString(m.BodyText), nullString(m.BodyHTML),
		nullString(m.ReadReceiptTo), m.ReadReceiptHandled,
		m.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return nil
}

// MarkReadReceiptHandled marks a message's read receipt as handled (sent or ignored)
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

// Delete deletes a message
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
		    has_attachments = ?
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

// helper to convert empty string to NULL
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// parseTimeString parses a time string in various formats
func parseTimeString(s string) time.Time {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05 -0700 MST", // Format used by Go's time.Time.String() when stored in SQLite
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
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
