package message

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ListConversationsUnifiedInbox returns conversations from all inbox folders across all accounts
// This is used for the unified inbox view
func (s *Store) ListConversationsUnifiedInbox(offset, limit int, sortOrder, filter string) ([]*Conversation, error) {
	// Determine sort direction
	orderClause := "ORDER BY latest_date DESC"
	if sortOrder == "oldest" {
		orderClause = "ORDER BY latest_date ASC"
	}

	// Query conversations from all inbox folders, joining with accounts for name and color
	query := `
		SELECT
			COALESCE(m.thread_id, m.id) as conv_thread_id,
			MIN(m.subject) as subject,
			MAX(m.snippet) as snippet,
			COUNT(*) as message_count,
			SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) as unread_count,
			MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END) as has_attachments,
			MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END) as is_starred,
			MAX(m.date) as latest_date,
			GROUP_CONCAT(m.id) as message_ids,
			MAX(CASE WHEN m.smime_encrypted = 1 OR m.pgp_encrypted = 1 THEN 1 ELSE 0 END) as is_encrypted,
			a.id as account_id,
			a.name as account_name,
			a.color as account_color,
			f.id as folder_id,
			MAX(CASE
				WHEN mcs.status = 'sent' AND mcs.action_type = 'reply-all' THEN 23
				WHEN mcs.status = 'sent' AND mcs.action_type = 'reply' THEN 22
				WHEN mcs.status = 'sent' AND mcs.action_type = 'forward' THEN 21
				WHEN mcs.status = 'draft' AND mcs.action_type = 'reply-all' THEN 13
				WHEN mcs.status = 'draft' AND mcs.action_type = 'reply' THEN 12
				WHEN mcs.status = 'draft' AND mcs.action_type = 'forward' THEN 11
				ELSE 0
			END) as compose_status_rank,
			json_group_array(DISTINCT json_object('name', m.from_name, 'email', m.from_email)) as participants_json
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'inbox'
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		LEFT JOIN message_compose_status mcs ON mcs.source_message_id = m.id
		GROUP BY COALESCE(m.thread_id, m.id), a.id` +
		filterHavingClause(filter, "m.") + `
		` + orderClause + `
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query unified inbox conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		c := &Conversation{}
		var latestDateStr sql.NullString
		var snippet sql.NullString
		var messageIDsStr sql.NullString
		var participantsJSON sql.NullString
		var composeStatusRank sql.NullInt64

		err := rows.Scan(
			&c.ThreadID,
			&c.Subject,
			&snippet,
			&c.MessageCount,
			&c.UnreadCount,
			&c.HasAttachments,
			&c.IsStarred,
			&latestDateStr,
			&messageIDsStr,
			&c.IsEncrypted,
			&c.AccountID,
			&c.AccountName,
			&c.AccountColor,
			&c.FolderID,
			&composeStatusRank,
			&participantsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unified inbox conversation: %w", err)
		}

		if snippet.Valid {
			c.Snippet = snippet.String
		}
		if latestDateStr.Valid && latestDateStr.String != "" {
			c.LatestDate = parseTimeString(latestDateStr.String)
		}

		// Parse message IDs from comma-separated string
		if messageIDsStr.Valid && messageIDsStr.String != "" {
			c.MessageIDs = strings.Split(messageIDsStr.String, ",")
		}

		if participantsJSON.Valid {
			c.Participants = parseParticipantsJSON(participantsJSON.String)
		}
		if composeStatusRank.Valid {
			applyComposeStatusRank(c, int(composeStatusRank.Int64))
		}

		conversations = append(conversations, c)
	}

	return conversations, nil
}

// CountConversationsUnifiedInbox returns the total count of conversations across all inbox folders
func (s *Store) CountConversationsUnifiedInbox(filter string) (int, error) {
	filterCond := filterWhereClause(filter, "m.")
	wherePart := ""
	if filterCond != "" {
		wherePart = " WHERE" + filterCond[len(" AND"):]
	}

	query := `
		SELECT COUNT(DISTINCT COALESCE(m.thread_id, m.id) || '-' || a.id)
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'inbox'
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
	` + wherePart

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unified inbox conversations: %w", err)
	}
	return count, nil
}

// ListConversationsByFolder returns conversations (grouped by thread) for a folder with pagination
// sortOrder can be "newest" (default) or "oldest"
func (s *Store) ListConversationsByFolder(folderID string, offset, limit int, sortOrder, filter string) ([]*Conversation, error) {
	// Determine sort direction
	orderClause := "ORDER BY latest_date DESC"
	if sortOrder == "oldest" {
		orderClause = "ORDER BY latest_date ASC"
	}

	// Pre-fetch folder type so we can pick the right participants
	// aggregation. Sent and Drafts folders should surface the recipient
	// list ("who you wrote to") instead of the sender (always self).
	// Mirrors the inline folder-type lookup in GetConversation. Lookup
	// errors are intentionally swallowed: an empty folderType falls
	// through to the existing sender-based behavior, so the list never
	// breaks on a metadata hiccup.
	var folderType string
	_ = s.db.QueryRow("SELECT folder_type FROM folders WHERE id = ?", folderID).Scan(&folderType)
	useToList := folderType == "sent" || folderType == "drafts"

	// participantsExpr is byte-identical to the historical query for
	// every folder except sent/drafts. The sent/drafts branch
	// aggregates per-message to_list JSON arrays into a nested array
	// that parseAggregatedToListJSON flattens + dedupes in Go (DISTINCT
	// doesn't work across nested-array values in SQLite).
	participantsExpr := `json_group_array(DISTINCT json_object('name', m.from_name, 'email', m.from_email))`
	if useToList {
		participantsExpr = `json_group_array(json(m.to_list))`
	}

	// Get conversations grouped by thread_id, ordered by date
	// Use GROUP_CONCAT to get all message IDs in a single query
	query := fmt.Sprintf(`
		SELECT
			COALESCE(m.thread_id, m.id) as conv_thread_id,
			MIN(m.subject) as subject,
			MAX(m.snippet) as snippet,
			COUNT(*) as message_count,
			SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) as unread_count,
			MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END) as has_attachments,
			MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END) as is_starred,
			MAX(m.date) as latest_date,
			GROUP_CONCAT(m.id) as message_ids,
			MAX(CASE WHEN m.smime_encrypted = 1 OR m.pgp_encrypted = 1 THEN 1 ELSE 0 END) as is_encrypted,
			MAX(CASE
				WHEN mcs.status = 'sent' AND mcs.action_type = 'reply-all' THEN 23
				WHEN mcs.status = 'sent' AND mcs.action_type = 'reply' THEN 22
				WHEN mcs.status = 'sent' AND mcs.action_type = 'forward' THEN 21
				WHEN mcs.status = 'draft' AND mcs.action_type = 'reply-all' THEN 13
				WHEN mcs.status = 'draft' AND mcs.action_type = 'reply' THEN 12
				WHEN mcs.status = 'draft' AND mcs.action_type = 'forward' THEN 11
				ELSE 0
			END) as compose_status_rank,
			%s as participants_json
		FROM messages m
		LEFT JOIN message_compose_status mcs ON mcs.source_message_id = m.id
		WHERE m.folder_id = ?
		GROUP BY COALESCE(m.thread_id, m.id)`+
		filterHavingClause(filter, "m.")+`
		`+orderClause+`
		LIMIT ? OFFSET ?
	`, participantsExpr)

	rows, err := s.db.Query(query, folderID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		c := &Conversation{}
		var latestDateStr sql.NullString
		var snippet sql.NullString
		var messageIDsStr sql.NullString
		var participantsJSON sql.NullString
		var composeStatusRank sql.NullInt64

		err := rows.Scan(
			&c.ThreadID,
			&c.Subject,
			&snippet,
			&c.MessageCount,
			&c.UnreadCount,
			&c.HasAttachments,
			&c.IsStarred,
			&latestDateStr,
			&messageIDsStr,
			&c.IsEncrypted,
			&composeStatusRank,
			&participantsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}

		if snippet.Valid {
			c.Snippet = snippet.String
		}
		if latestDateStr.Valid && latestDateStr.String != "" {
			c.LatestDate = parseTimeString(latestDateStr.String)
		}

		// Parse message IDs from comma-separated string
		if messageIDsStr.Valid && messageIDsStr.String != "" {
			c.MessageIDs = strings.Split(messageIDsStr.String, ",")
		}

		if participantsJSON.Valid {
			if useToList {
				c.Participants = parseAggregatedToListJSON(participantsJSON.String)
			}
			if !useToList {
				c.Participants = parseParticipantsJSON(participantsJSON.String)
			}
		}
		if composeStatusRank.Valid {
			applyComposeStatusRank(c, int(composeStatusRank.Int64))
		}

		conversations = append(conversations, c)
	}

	return conversations, nil
}

// parseParticipantsJSON parses a JSON array of {name, email} objects from
// SQLite's json_group_array into a deduplicated Address slice.
func parseParticipantsJSON(s string) []Address {
	if s == "" || s == "[]" {
		return nil
	}
	var raw []struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	participants := make([]Address, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, r := range raw {
		if seen[r.Email] {
			continue
		}
		seen[r.Email] = true
		participants = append(participants, Address{Name: r.Name, Email: r.Email})
	}
	return participants
}

// parseAggregatedToListJSON parses the nested array produced by
// `json_group_array(json(to_list))` — one outer entry per message in the
// conversation, each containing that message's parsed to_list array.
// Flattens to a deduplicated Address slice keyed by lowercased email.
//
// Used by ListConversationsByFolder when the folder type is sent or
// drafts so the row's "who" column reflects recipients rather than the
// always-self sender.
func parseAggregatedToListJSON(s string) []Address {
	if s == "" || s == "[]" {
		return nil
	}
	var nested [][]struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(s), &nested); err != nil {
		return nil
	}
	participants := make([]Address, 0)
	seen := make(map[string]bool)
	for _, inner := range nested {
		for _, r := range inner {
			key := strings.ToLower(r.Email)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			participants = append(participants, Address{Name: r.Name, Email: r.Email})
		}
	}
	return participants
}

// CountConversationsByFolder returns the count of conversations in a folder
func (s *Store) CountConversationsByFolder(folderID, filter string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT COALESCE(thread_id, id))
		FROM messages
		WHERE folder_id = ?
	` + filterWhereClause(filter, "")

	var count int
	err := s.db.QueryRow(query, folderID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count conversations: %w", err)
	}
	return count, nil
}

// GetConversation returns messages in a conversation/thread. Drafts are scoped
// strictly to the selected Drafts folder; other folders also include Sent to
// preserve the full exchanged-message context.
func (s *Store) GetConversation(threadID, folderID string) (*Conversation, error) {
	s.log.Debug().
		Str("threadID", threadID).
		Str("folderID", folderID).
		Msg("GetConversation called in store")

	// If the threadID is a UUID (not a Message-ID), resolve it to the actual
	// thread_id from the DB. This handles the case where a message's thread_id
	// was updated by thread reconciliation after initial save.
	if !strings.Contains(threadID, "@") && !strings.HasPrefix(threadID, "<") {
		var actualThreadID sql.NullString
		err := s.db.QueryRow("SELECT thread_id FROM messages WHERE id = ?", threadID).Scan(&actualThreadID)
		if err == nil && actualThreadID.Valid && actualThreadID.String != "" {
			s.log.Debug().Str("uuid", threadID).Str("resolvedThreadID", actualThreadID.String).Msg("Resolved UUID to thread_id")
			threadID = actualThreadID.String
		}
	}

	// First get the account ID and folder type
	var accountID string
	var folderType string
	err := s.db.QueryRow("SELECT account_id, folder_type FROM folders WHERE id = ?", folderID).Scan(&accountID, &folderType)
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID and folder type: %w", err)
	}

	// Normalize the thread ID for comparison
	normalizedThreadID := normalizeMessageID(threadID)
	s.log.Debug().
		Str("normalizedThreadID", normalizedThreadID).
		Str("accountID", accountID).
		Msg("GetConversation normalized")

	// Get the conversation summary from the current folder and, outside Drafts,
	// Sent for exchanged-message context.
	// Exclude messages in Trash folder unless we're viewing Trash
	// Use COALESCE to handle NULL values from aggregate functions when no rows match
	trashFilter := ""
	if folderType != "trash" {
		trashFilter = "AND f.folder_type != 'trash'"
	}

	// Drafts is an editing workspace, so its list count and detail count must both
	// describe only drafts in that folder. Other folders retain the historical
	// current-folder + Sent behavior for full exchanged-message context.
	folderFilter := "AND (m.folder_id = ? OR f.folder_type = 'sent')"
	if folderType == "drafts" {
		folderFilter = "AND m.folder_id = ?"
	}

	summaryQuery := fmt.Sprintf(`
		SELECT
			COALESCE(MIN(m.subject), '') as subject,
			COALESCE(MAX(m.snippet), '') as snippet,
			COUNT(*) as message_count,
			COALESCE(SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END), 0) as unread_count,
			COALESCE(MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END), 0) as has_attachments,
			COALESCE(MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END), 0) as is_starred,
			MAX(m.date) as latest_date
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id
		WHERE m.account_id = ? AND (
			REPLACE(REPLACE(COALESCE(m.thread_id, m.id), '<', ''), '>', '') = ?
			OR REPLACE(REPLACE(m.message_id, '<', ''), '>', '') = ?
			OR REPLACE(REPLACE(m.in_reply_to, '<', ''), '>', '') = ?
		)
		%s %s
	`, trashFilter, folderFilter)

	c := &Conversation{ThreadID: threadID}
	var latestDateStr sql.NullString

	err = s.db.QueryRow(summaryQuery, accountID, normalizedThreadID, normalizedThreadID, normalizedThreadID, folderID).Scan(
		&c.Subject,
		&c.Snippet,
		&c.MessageCount,
		&c.UnreadCount,
		&c.HasAttachments,
		&c.IsStarred,
		&latestDateStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation summary: %w", err)
	}
	if latestDateStr.Valid && latestDateStr.String != "" {
		c.LatestDate = parseTimeString(latestDateStr.String)
	}

	// Get messages using the same folder scope as the summary so the displayed
	// cards and message count cannot diverge.
	// Exclude messages in Trash folder unless we're viewing Trash
	messagesQuery := fmt.Sprintf(`
		SELECT m.id, m.account_id, m.folder_id, m.uid, m.message_id, m.in_reply_to, m.references_list, m.thread_id,
		       m.subject, m.from_name, m.from_email, m.to_list, m.cc_list, m.bcc_list, m.reply_to, m.date,
		       m.snippet, m.is_read, m.is_starred, m.is_answered, m.is_forwarded, m.is_draft, m.is_deleted,
		       m.size, m.has_attachments, m.body_text, m.body_html, m.body_fetched,
		       m.read_receipt_to, m.read_receipt_handled,
		       m.received_at
		FROM messages m
		INNER JOIN folders f ON m.folder_id = f.id
		WHERE m.account_id = ? AND (
			REPLACE(REPLACE(COALESCE(m.thread_id, m.id), '<', ''), '>', '') = ?
			OR REPLACE(REPLACE(m.message_id, '<', ''), '>', '') = ?
			OR REPLACE(REPLACE(m.in_reply_to, '<', ''), '>', '') = ?
		)
		%s %s
		ORDER BY m.date ASC
	`, trashFilter, folderFilter)

	rows, err := s.db.Query(messagesQuery, accountID, normalizedThreadID, normalizedThreadID, normalizedThreadID, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query thread messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		m := &Message{}
		var messageID, inReplyTo, references, threadIDVal, toList, ccList, bccList, replyTo, snippetVal, bodyText, bodyHTML, readReceiptTo sql.NullString
		var dateStr, receivedAtStr sql.NullString
		var uidI64 int64

		err := rows.Scan(
			&m.ID, &m.AccountID, &m.FolderID, &uidI64, &messageID, &inReplyTo, &references, &threadIDVal,
			&m.Subject, &m.FromName, &m.FromEmail, &toList, &ccList, &bccList, &replyTo, &dateStr,
			&snippetVal, &m.IsRead, &m.IsStarred, &m.IsAnswered, &m.IsForwarded, &m.IsDraft, &m.IsDeleted,
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
		if threadIDVal.Valid {
			m.ThreadID = threadIDVal.String
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
		if snippetVal.Valid {
			m.Snippet = snippetVal.String
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

		s.log.Debug().
			Str("id", m.ID).
			Str("messageID", m.MessageID).
			Str("threadID", m.ThreadID).
			Str("subject", m.Subject).
			Int("bodyTextLen", len(m.BodyText)).
			Int("bodyHTMLLen", len(m.BodyHTML)).
			Msg("GetConversation found message")

		c.Messages = append(c.Messages, m)
	}

	s.log.Debug().
		Int("messageCount", len(c.Messages)).
		Str("threadID", threadID).
		Msg("GetConversation returning")

	// Build participants from already-loaded messages (no extra query needed)
	seen := make(map[string]bool)
	for _, msg := range c.Messages {
		if seen[msg.FromEmail] {
			continue
		}
		seen[msg.FromEmail] = true
		c.Participants = append(c.Participants, Address{Name: msg.FromName, Email: msg.FromEmail})
	}

	return c, nil
}
