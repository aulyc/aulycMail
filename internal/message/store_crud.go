package message

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

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
