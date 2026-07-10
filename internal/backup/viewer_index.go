package backup

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"io"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aulyc.local/aulycmail/internal/email"
	mailSync "aulyc.local/aulycmail/internal/sync"
	gomessage "github.com/emersion/go-message"
	_ "modernc.org/sqlite"
)

const (
	ViewerIndexSchemaVersion = 1
	ViewerIndexDefaultLimit  = 200
	viewerIndexMaxTextBytes  = 10 * 1024 * 1024
)

type ViewerIndex struct {
	db   *sql.DB
	path string
}

type ViewerMessageRecord struct {
	Key             string
	AccountEmail    string
	FolderPath      string
	Subject         string
	Date            string
	Size            int
	AttachmentCount int
	Snippet         string
}

type ViewerAccountRecord struct {
	AccountEmail string
	MessageCount int
}

type ViewerMessagePage struct {
	Messages []ViewerMessageRecord
	Total    int
	HasMore  bool
}

type ViewerIndexedMessage struct {
	Sender          string
	Recipients      string
	BodyText        string
	AttachmentNames string
	AttachmentCount int
}

type viewerParsedMessage struct {
	text        string
	html        string
	attachments []string
}

func ViewerIndexPath(directory string) string {
	return filepath.Join(directory, ".aulycmail-backup", "viewer.sqlite")
}

func ViewerIndexExists(directory string) bool {
	info, err := os.Stat(ViewerIndexPath(directory))
	return err == nil && !info.IsDir()
}

func OpenViewerIndex(directory string) (*ViewerIndex, error) {
	path := ViewerIndexPath(directory)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup viewer metadata directory: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup viewer index: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping backup viewer index: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set backup viewer index permissions: %w", err)
	}
	v := &ViewerIndex{db: db, path: path}
	if err := v.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return v, nil
}

func OpenExistingViewerIndex(directory string) (*ViewerIndex, bool, error) {
	if !ViewerIndexExists(directory) {
		return nil, false, nil
	}
	idx, err := OpenViewerIndex(directory)
	if err != nil {
		return nil, true, err
	}
	return idx, true, nil
}

func (v *ViewerIndex) Close() error {
	if v == nil || v.db == nil {
		return nil
	}
	return v.db.Close()
}

func (v *ViewerIndex) initSchema() error {
	if _, err := v.db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS messages (
			key TEXT PRIMARY KEY,
			account_id TEXT NOT NULL DEFAULT '',
			account_email TEXT NOT NULL DEFAULT '',
			folder_id TEXT NOT NULL DEFAULT '',
			folder_path TEXT NOT NULL DEFAULT '',
			uid_validity INTEGER NOT NULL DEFAULT 0,
			uid INTEGER NOT NULL DEFAULT 0,
			message_id TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			sender TEXT NOT NULL DEFAULT '',
			recipients TEXT NOT NULL DEFAULT '',
			date_raw TEXT NOT NULL DEFAULT '',
			date_ts INTEGER NOT NULL DEFAULT 0,
			size INTEGER NOT NULL DEFAULT 0,
			eml_path TEXT NOT NULL DEFAULT '',
			has_attachments INTEGER NOT NULL DEFAULT 0,
			attachment_count INTEGER NOT NULL DEFAULT 0,
			attachment_names TEXT NOT NULL DEFAULT '',
			exported_at TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_backup_viewer_messages_date ON messages(date_ts DESC, key DESC);
		CREATE INDEX IF NOT EXISTS idx_backup_viewer_messages_account_date ON messages(account_email, date_ts DESC, key DESC);
	`); err != nil {
		return fmt.Errorf("failed to create backup viewer tables: %w", err)
	}

	if err := v.ensureFTSTable(); err != nil {
		return err
	}
	if _, err := v.db.Exec(`
		INSERT OR REPLACE INTO meta(key, value) VALUES('schema_version', ?);
	`, fmt.Sprintf("%d", ViewerIndexSchemaVersion)); err != nil {
		return fmt.Errorf("failed to update backup viewer schema version: %w", err)
	}
	return nil
}

func (v *ViewerIndex) ensureFTSTable() error {
	if _, err := v.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			key UNINDEXED,
			subject,
			sender,
			recipients,
			folder_path,
			body_text,
			attachment_names,
			tokenize='trigram'
		);
	`); err == nil {
		return nil
	}
	if _, err := v.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			key UNINDEXED,
			subject,
			sender,
			recipients,
			folder_path,
			body_text,
			attachment_names
		);
	`); err != nil {
		return fmt.Errorf("failed to create backup viewer FTS table: %w", err)
	}
	return nil
}

func (v *ViewerIndex) MessageCount(accountEmail string) (int, error) {
	where, args := viewerAccountWhere(accountEmail)
	var total int
	err := v.db.QueryRow("SELECT COUNT(*) FROM messages"+where, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count backup viewer messages: %w", err)
	}
	return total, nil
}

func (v *ViewerIndex) Accounts() ([]ViewerAccountRecord, error) {
	rows, err := v.db.Query(`
		SELECT account_email, COUNT(*)
		FROM messages
		GROUP BY account_email
		ORDER BY LOWER(account_email) ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backup viewer accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]ViewerAccountRecord, 0)
	for rows.Next() {
		var account ViewerAccountRecord
		if err := rows.Scan(&account.AccountEmail, &account.MessageCount); err != nil {
			return nil, fmt.Errorf("failed to scan backup viewer account: %w", err)
		}
		if strings.TrimSpace(account.AccountEmail) == "" {
			account.AccountEmail = "unknown"
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate backup viewer accounts: %w", err)
	}
	return accounts, nil
}

func (v *ViewerIndex) ListMessages(accountEmail, sortOrder string, offset, limit int) (ViewerMessagePage, error) {
	limit = normalizeViewerLimit(limit, ViewerIndexDefaultLimit)
	offset = max(offset, 0)
	total, err := v.MessageCount(accountEmail)
	if err != nil {
		return ViewerMessagePage{}, err
	}

	where, args := viewerAccountWhere(accountEmail)
	order := "date_ts DESC, key DESC"
	if sortOrder == "oldest" {
		order = "date_ts ASC, key ASC"
	}
	query := fmt.Sprintf(`
		SELECT key, account_email, folder_path, subject, date_raw, size, attachment_count, ''
		FROM messages
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, where, order)
	args = append(args, limit, offset)
	rows, err := v.db.Query(query, args...)
	if err != nil {
		return ViewerMessagePage{}, fmt.Errorf("failed to list backup viewer messages: %w", err)
	}
	defer rows.Close()

	messages, err := scanViewerMessageRows(rows)
	if err != nil {
		return ViewerMessagePage{}, err
	}
	return ViewerMessagePage{
		Messages: messages,
		Total:    total,
		HasMore:  offset+len(messages) < total,
	}, nil
}

func (v *ViewerIndex) SearchMessages(accountEmail, query string, offset, limit int) (ViewerMessagePage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ViewerMessagePage{}, nil
	}
	limit = normalizeViewerLimit(limit, 50)
	offset = max(offset, 0)

	if len([]rune(query)) < 3 {
		return v.searchMessagesLike(accountEmail, query, offset, limit)
	}
	return v.searchMessagesFTS(accountEmail, query, offset, limit)
}

func (v *ViewerIndex) searchMessagesFTS(accountEmail, query string, offset, limit int) (ViewerMessagePage, error) {
	where, args := viewerAccountWhereWithPrefix(accountEmail, "m.")
	matchQuery := quoteFTSQuery(query)
	countQuery := `
		SELECT COUNT(*)
		FROM messages m
		JOIN messages_fts ON messages_fts.key = m.key
		WHERE messages_fts MATCH ?
	` + where
	countArgs := append([]any{matchQuery}, args...)
	var total int
	if err := v.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return ViewerMessagePage{}, fmt.Errorf("failed to count backup viewer search results: %w", err)
	}

	searchQuery := `
		SELECT
			m.key,
			m.account_email,
			m.folder_path,
			m.subject,
			m.date_raw,
			m.size,
			m.attachment_count,
			snippet(messages_fts, 5, '', '', '...', 16)
		FROM messages m
		JOIN messages_fts ON messages_fts.key = m.key
		WHERE messages_fts MATCH ?
	` + where + `
		ORDER BY rank, m.date_ts DESC, m.key DESC
		LIMIT ? OFFSET ?
	`
	searchArgs := append(countArgs, limit, offset)
	rows, err := v.db.Query(searchQuery, searchArgs...)
	if err != nil {
		return ViewerMessagePage{}, fmt.Errorf("failed to search backup viewer messages: %w", err)
	}
	defer rows.Close()
	messages, err := scanViewerMessageRows(rows)
	if err != nil {
		return ViewerMessagePage{}, err
	}
	return ViewerMessagePage{Messages: messages, Total: total, HasMore: offset+len(messages) < total}, nil
}

func (v *ViewerIndex) searchMessagesLike(accountEmail, query string, offset, limit int) (ViewerMessagePage, error) {
	where, args := viewerAccountWhereWithPrefix(accountEmail, "m.")
	like := "%" + strings.ToLower(query) + "%"
	predicate := `
		(
			LOWER(m.subject) LIKE ?
			OR LOWER(m.sender) LIKE ?
			OR LOWER(m.recipients) LIKE ?
			OR LOWER(m.folder_path) LIKE ?
			OR LOWER(m.attachment_names) LIKE ?
			OR LOWER(messages_fts.body_text) LIKE ?
		)
	`
	filterArgs := []any{like, like, like, like, like, like}
	countArgs := append(filterArgs, args...)
	var total int
	if err := v.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages m
		JOIN messages_fts ON messages_fts.key = m.key
		WHERE `+predicate+where, countArgs...).Scan(&total); err != nil {
		return ViewerMessagePage{}, fmt.Errorf("failed to count backup viewer search results: %w", err)
	}

	searchArgs := append(countArgs, limit, offset)
	rows, err := v.db.Query(`
		SELECT
			m.key,
			m.account_email,
			m.folder_path,
			m.subject,
			m.date_raw,
			m.size,
			m.attachment_count,
			substr(messages_fts.body_text, 1, 180)
		FROM messages m
		JOIN messages_fts ON messages_fts.key = m.key
		WHERE `+predicate+where+`
		ORDER BY m.date_ts DESC, m.key DESC
		LIMIT ? OFFSET ?
	`, searchArgs...)
	if err != nil {
		return ViewerMessagePage{}, fmt.Errorf("failed to search backup viewer messages: %w", err)
	}
	defer rows.Close()
	messages, err := scanViewerMessageRows(rows)
	if err != nil {
		return ViewerMessagePage{}, err
	}
	return ViewerMessagePage{Messages: messages, Total: total, HasMore: offset+len(messages) < total}, nil
}

func (v *ViewerIndex) HasMessage(key string) bool {
	var exists int
	err := v.db.QueryRow("SELECT 1 FROM messages WHERE key = ? LIMIT 1", key).Scan(&exists)
	return err == nil && exists == 1
}

func (v *ViewerIndex) UpsertMessageFromFile(directory, key string, entry IndexMessage) error {
	path, err := IndexedFilePath(directory, entry.EMLPath)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	indexed, parseErr := ParseViewerIndexedMessage(entry, file)
	if parseErr != nil {
		indexed = ViewerIndexedMessage{}
	}
	return v.UpsertMessage(key, entry, indexed)
}

func (v *ViewerIndex) UpsertMessage(key string, entry IndexMessage, indexed ViewerIndexedMessage) error {
	dateTS := ParseMessageTime(entry.Date).Unix()
	if dateTS < 0 {
		dateTS = 0
	}
	hasAttachments := 0
	if entry.HasAttachments != nil && *entry.HasAttachments {
		hasAttachments = 1
	}
	if indexed.AttachmentCount > 0 {
		hasAttachments = 1
	}

	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO messages (
			key, account_id, account_email, folder_id, folder_path, uid_validity, uid,
			message_id, subject, sender, recipients, date_raw, date_ts, size, eml_path,
			has_attachments, attachment_count, attachment_names, exported_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			account_id = excluded.account_id,
			account_email = excluded.account_email,
			folder_id = excluded.folder_id,
			folder_path = excluded.folder_path,
			uid_validity = excluded.uid_validity,
			uid = excluded.uid,
			message_id = excluded.message_id,
			subject = excluded.subject,
			sender = excluded.sender,
			recipients = excluded.recipients,
			date_raw = excluded.date_raw,
			date_ts = excluded.date_ts,
			size = excluded.size,
			eml_path = excluded.eml_path,
			has_attachments = excluded.has_attachments,
			attachment_count = excluded.attachment_count,
			attachment_names = excluded.attachment_names,
			exported_at = excluded.exported_at
	`, key, entry.AccountID, entry.AccountEmail, entry.FolderID, entry.FolderPath, entry.UIDValidity, entry.UID,
		entry.MessageID, entry.Subject, indexed.Sender, indexed.Recipients, entry.Date, dateTS, entry.Size, entry.EMLPath,
		hasAttachments, indexed.AttachmentCount, indexed.AttachmentNames, entry.ExportedAt); err != nil {
		return fmt.Errorf("failed to upsert backup viewer message: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM messages_fts WHERE key = ?`, key); err != nil {
		return fmt.Errorf("failed to clear backup viewer FTS row: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO messages_fts(key, subject, sender, recipients, folder_path, body_text, attachment_names)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, key, entry.Subject, indexed.Sender, indexed.Recipients, entry.FolderPath, indexed.BodyText, indexed.AttachmentNames); err != nil {
		return fmt.Errorf("failed to upsert backup viewer FTS row: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT OR REPLACE INTO meta(key, value) VALUES('index_built_at', ?)
	`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to update backup viewer index metadata: %w", err)
	}
	return tx.Commit()
}

func (v *ViewerIndex) BuildFromJSONIndex(ctx context.Context, directory string, idx *Index) (int, error) {
	if idx == nil || len(idx.Messages) == 0 {
		return 0, nil
	}
	keys := make([]string, 0, len(idx.Messages))
	for key := range idx.Messages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	indexed := 0
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return indexed, ctx.Err()
		default:
		}
		entry := idx.Messages[key]
		if entry.EMLPath == "" || !FileExists(directory, entry.EMLPath) {
			continue
		}
		if err := v.UpsertMessageFromFile(directory, key, entry); err != nil {
			continue
		}
		indexed++
	}
	return indexed, nil
}

func ParseViewerIndexedMessage(entry IndexMessage, reader io.Reader) (ViewerIndexedMessage, error) {
	entity, err := gomessage.Read(reader)
	if err != nil {
		return ViewerIndexedMessage{}, err
	}
	parsed := &viewerParsedMessage{}
	if err := parseViewerIndexEntity(entity, parsed); err != nil {
		return ViewerIndexedMessage{}, err
	}
	bodyText := strings.TrimSpace(parsed.text)
	if bodyText == "" && parsed.html != "" {
		bodyText = stripViewerHTML(parsed.html)
	}
	return ViewerIndexedMessage{
		Sender:          strings.Join(email.ParseAddressHeader(entity.Header.Get("From")), ", "),
		Recipients:      strings.Join(viewerIndexRecipients(entity), ", "),
		BodyText:        bodyText,
		AttachmentNames: strings.Join(parsed.attachments, "\n"),
		AttachmentCount: len(parsed.attachments),
	}, nil
}

func parseViewerIndexEntity(entity *gomessage.Entity, parsed *viewerParsedMessage) error {
	if entity == nil {
		return nil
	}
	if mr := entity.MultipartReader(); mr != nil {
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			if err := parseViewerIndexEntity(part, parsed); err != nil {
				return err
			}
		}
		return nil
	}

	contentType, contentParams, _ := mime.ParseMediaType(entity.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/plain"
	}
	contentType = strings.ToLower(contentType)
	disposition, dispositionParams, _ := mime.ParseMediaType(entity.Header.Get("Content-Disposition"))
	disposition = strings.ToLower(disposition)
	filename := viewerPartFilename(contentParams, dispositionParams)
	isAttachment := filename != "" || disposition == "attachment" || (!strings.HasPrefix(contentType, "text/") && contentType != "")

	if isAttachment {
		if filename == "" {
			filename = "attachment"
		}
		parsed.attachments = append(parsed.attachments, filename)
		_, _ = io.Copy(io.Discard, entity.Body)
		return nil
	}

	raw, err := readViewerIndexTextPart(entity.Body)
	if err != nil {
		return err
	}
	decoded := mailSync.DecodeTextCharset(raw, contentParams["charset"])
	switch contentType {
	case "text/html":
		if parsed.html == "" {
			parsed.html = decoded
		}
	case "text/plain":
		if parsed.text == "" {
			parsed.text = decoded
		}
	default:
		if parsed.text == "" {
			parsed.text = decoded
		}
	}
	return nil
}

func readViewerIndexTextPart(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, viewerIndexMaxTextBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) <= viewerIndexMaxTextBytes {
		return raw, nil
	}
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	return raw[:viewerIndexMaxTextBytes], nil
}

func viewerIndexRecipients(entity *gomessage.Entity) []string {
	var out []string
	for _, header := range []string{"To", "Cc", "Bcc"} {
		out = append(out, email.ParseAddressHeader(entity.Header.Get(header))...)
	}
	return out
}

func viewerPartFilename(contentParams, dispositionParams map[string]string) string {
	for _, candidate := range []string{dispositionParams["filename"], contentParams["name"]} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return email.DecodeMIMEHeader(candidate)
		}
	}
	return ""
}

var viewerHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripViewerHTML(value string) string {
	value = viewerHTMLTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func scanViewerMessageRows(rows *sql.Rows) ([]ViewerMessageRecord, error) {
	messages := make([]ViewerMessageRecord, 0)
	for rows.Next() {
		var message ViewerMessageRecord
		var snippet sql.NullString
		if err := rows.Scan(
			&message.Key,
			&message.AccountEmail,
			&message.FolderPath,
			&message.Subject,
			&message.Date,
			&message.Size,
			&message.AttachmentCount,
			&snippet,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup viewer message: %w", err)
		}
		if snippet.Valid {
			message.Snippet = strings.TrimSpace(snippet.String)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate backup viewer messages: %w", err)
	}
	return messages, nil
}

func viewerAccountWhere(accountEmail string) (string, []any) {
	accountEmail = strings.TrimSpace(accountEmail)
	if accountEmail == "" {
		return "", nil
	}
	return " WHERE account_email = ?", []any{accountEmail}
}

func viewerAccountWhereWithPrefix(accountEmail, prefix string) (string, []any) {
	accountEmail = strings.TrimSpace(accountEmail)
	if accountEmail == "" {
		return "", nil
	}
	return " AND " + prefix + "account_email = ?", []any{accountEmail}
}

func normalizeViewerLimit(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func quoteFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	query = strings.ReplaceAll(query, `"`, `""`)
	return `"` + query + `"`
}
