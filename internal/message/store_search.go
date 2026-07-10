package message

import (
	"database/sql"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// SearchConversations searches for conversations in a folder using FTS5
// Returns conversations with highlighted text and the total count
func (s *Store) SearchConversations(folderID, query string, offset, limit int, filter string) ([]*ConversationSearchResult, int, error) {
	if query == "" {
		return nil, 0, nil
	}

	// Prepare the FTS query - escape special characters and add prefix matching
	ftsQuery := prepareFTSQuery(query)

	// First, get the total count
	countQuery := `
		SELECT COUNT(DISTINCT COALESCE(m.thread_id, m.id))
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		WHERE m.folder_id = ? AND messages_fts MATCH ?
	` + filterWhereClause(filter, "m.")
	var totalCount int
	err := s.db.QueryRow(countQuery, folderID, ftsQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	if totalCount == 0 {
		return nil, 0, nil
	}

	// Get folder info for displaying in results
	var folderName, folderType string
	err = s.db.QueryRow("SELECT name, folder_type FROM folders WHERE id = ?", folderID).Scan(&folderName, &folderType)
	if err != nil {
		folderName = "Unknown"
		folderType = "folder"
	}

	// Get matching conversations with relevance ranking
	searchQuery := `
		SELECT 
			COALESCE(m.thread_id, m.id) as conv_thread_id,
			MIN(m.subject) as subject,
			MAX(m.snippet) as snippet,
			MIN(m.from_name) as from_name,
			COUNT(*) as message_count,
			SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) as unread_count,
			MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END) as has_attachments,
			MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END) as is_starred,
			MAX(m.date) as latest_date,
			GROUP_CONCAT(m.id) as message_ids,
			MAX(CASE WHEN m.smime_encrypted = 1 OR m.pgp_encrypted = 1 THEN 1 ELSE 0 END) as is_encrypted,
			json_group_array(DISTINCT json_object('name', m.from_name, 'email', m.from_email)) as participants_json
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		WHERE m.folder_id = ? AND messages_fts MATCH ?
		GROUP BY COALESCE(m.thread_id, m.id)` +
		filterHavingClause(filter, "m.") + `
		ORDER BY latest_date DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(searchQuery, folderID, ftsQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search conversations: %w", err)
	}
	defer rows.Close()

	var results []*ConversationSearchResult
	for rows.Next() {
		c := &ConversationSearchResult{}
		var latestDateStr sql.NullString
		var snippet sql.NullString
		var fromName sql.NullString
		var messageIDsStr sql.NullString
		var participantsJSON sql.NullString

		err := rows.Scan(
			&c.ThreadID,
			&c.Subject,
			&snippet,
			&fromName,
			&c.MessageCount,
			&c.UnreadCount,
			&c.HasAttachments,
			&c.IsStarred,
			&latestDateStr,
			&messageIDsStr,
			&c.IsEncrypted,
			&participantsJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan search result: %w", err)
		}

		if snippet.Valid {
			c.Snippet = snippet.String
		}
		if latestDateStr.Valid && latestDateStr.String != "" {
			c.LatestDate = parseTimeString(latestDateStr.String)
		}
		if messageIDsStr.Valid && messageIDsStr.String != "" {
			c.MessageIDs = strings.Split(messageIDsStr.String, ",")
		}

		// Set folder info
		c.FolderName = folderName
		c.FolderType = folderType

		// Apply highlighting to displayable fields
		c.HighlightedSubject = highlightMatches(c.Subject, query)
		c.HighlightedSnippet = highlightMatches(c.Snippet, query)
		if fromName.Valid {
			c.HighlightedFromName = highlightMatches(fromName.String, query)
		}

		if participantsJSON.Valid {
			c.Participants = parseParticipantsJSON(participantsJSON.String)
		}

		results = append(results, c)
	}

	return results, totalCount, nil
}

// SearchConversationsUnifiedInbox searches across all inbox folders for all accounts
func (s *Store) SearchConversationsUnifiedInbox(query string, offset, limit int, filter string) ([]*ConversationSearchResult, int, error) {
	if query == "" {
		return nil, 0, nil
	}

	ftsQuery := prepareFTSQuery(query)

	// Count total results across all inbox folders
	countQuery := `
		SELECT COUNT(DISTINCT COALESCE(m.thread_id, m.id) || '-' || a.id)
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'inbox'
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		WHERE messages_fts MATCH ?
	` + filterWhereClause(filter, "m.")
	var totalCount int
	err := s.db.QueryRow(countQuery, ftsQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count unified search results: %w", err)
	}

	if totalCount == 0 {
		return nil, 0, nil
	}

	// Search across all inbox folders with account info
	searchQuery := `
		SELECT 
			COALESCE(m.thread_id, m.id) as conv_thread_id,
			MIN(m.subject) as subject,
			MAX(m.snippet) as snippet,
			MIN(m.from_name) as from_name,
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
			f.name as folder_name,
			f.folder_type as folder_type,
			json_group_array(DISTINCT json_object('name', m.from_name, 'email', m.from_email)) as participants_json
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		INNER JOIN folders f ON m.folder_id = f.id AND f.folder_type = 'inbox'
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		WHERE messages_fts MATCH ?
		GROUP BY COALESCE(m.thread_id, m.id), a.id` +
		filterHavingClause(filter, "m.") + `
		ORDER BY latest_date DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(searchQuery, ftsQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search unified inbox: %w", err)
	}
	defer rows.Close()

	var results []*ConversationSearchResult
	for rows.Next() {
		c := &ConversationSearchResult{}
		var latestDateStr sql.NullString
		var snippet sql.NullString
		var fromName sql.NullString
		var messageIDsStr sql.NullString
		var participantsJSON sql.NullString

		err := rows.Scan(
			&c.ThreadID,
			&c.Subject,
			&snippet,
			&fromName,
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
			&c.FolderName,
			&c.FolderType,
			&participantsJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan unified search result: %w", err)
		}

		if snippet.Valid {
			c.Snippet = snippet.String
		}
		if latestDateStr.Valid && latestDateStr.String != "" {
			c.LatestDate = parseTimeString(latestDateStr.String)
		}
		if messageIDsStr.Valid && messageIDsStr.String != "" {
			c.MessageIDs = strings.Split(messageIDsStr.String, ",")
		}

		// Apply highlighting
		c.HighlightedSubject = highlightMatches(c.Subject, query)
		c.HighlightedSnippet = highlightMatches(c.Snippet, query)
		if fromName.Valid {
			c.HighlightedFromName = highlightMatches(fromName.String, query)
		}

		if participantsJSON.Valid {
			c.Participants = parseParticipantsJSON(participantsJSON.String)
		}

		results = append(results, c)
	}

	return results, totalCount, nil
}

// prepareFTSQuery prepares a user query for FTS5
// Handles special characters and adds prefix matching for better UX
func prepareFTSQuery(query string) string {
	// Trim whitespace
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into words
	words := strings.Fields(query)
	var processedWords []string

	for _, word := range words {
		// Escape special FTS5 characters
		// FTS5 special chars: " ' ( ) * : ^
		escaped := word
		escaped = strings.ReplaceAll(escaped, "\"", "\"\"")

		// Add prefix matching (word*) for partial matches
		// But only if the word doesn't already end with *
		if !strings.HasSuffix(escaped, "*") && len(escaped) > 0 {
			escaped = "\"" + escaped + "\"*"
		}

		processedWords = append(processedWords, escaped)
	}

	// Join with spaces - FTS5 will AND them together by default
	return strings.Join(processedWords, " ")
}

// highlightMatches wraps matching terms in <mark> tags for highlighting
// The text is HTML-escaped to prevent XSS
func highlightMatches(text, query string) string {
	if text == "" || query == "" {
		return html.EscapeString(text)
	}

	// First, escape the text to prevent XSS
	escapedText := html.EscapeString(text)

	// Get individual search terms
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return escapedText
	}

	// Build a regex pattern that matches any of the search terms
	// Use word boundaries for better matching
	var patterns []string
	for _, term := range terms {
		// Escape regex special characters in the term
		escaped := regexp.QuoteMeta(term)
		patterns = append(patterns, escaped)
	}

	// Create a case-insensitive pattern
	pattern := "(?i)(" + strings.Join(patterns, "|") + ")"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return escapedText
	}

	// Replace matches with highlighted version
	highlighted := re.ReplaceAllStringFunc(escapedText, func(match string) string {
		return "<mark>" + match + "</mark>"
	})

	return highlighted
}
