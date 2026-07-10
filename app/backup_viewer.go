package app

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"sort"
	"strings"

	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/email"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/settings"
	mailSync "aulyc.local/aulycmail/internal/sync"
	gomessage "github.com/emersion/go-message"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	backupViewerDefaultLimit     = 50
	backupViewerMaxTextPartBytes = 10 * 1024 * 1024
)

// BackupViewerCatalog summarizes a backup directory for the read-only viewer.
type BackupViewerCatalog struct {
	Directory    string                       `json:"directory"`
	Accounts     []BackupViewerAccount        `json:"accounts"`
	Messages     []BackupViewerMessageSummary `json:"messages"`
	MessageCount int                          `json:"messageCount"`
	IndexReady   bool                         `json:"indexReady"`
	NeedsIndex   bool                         `json:"needsIndex"`
}

// BackupViewerAccount is an account tag in the backup viewer.
type BackupViewerAccount struct {
	AccountEmail string `json:"accountEmail"`
	MessageCount int    `json:"messageCount"`
}

// BackupViewerMessageSummary is the list/search representation of a backed-up email.
type BackupViewerMessageSummary struct {
	Key             string `json:"key"`
	AccountEmail    string `json:"accountEmail"`
	FolderPath      string `json:"folderPath"`
	Subject         string `json:"subject"`
	Date            string `json:"date"`
	Size            int    `json:"size"`
	AttachmentCount int    `json:"attachmentCount"`
	Snippet         string `json:"snippet,omitempty"`
}

// BackupViewerMessagePage is a paged list/search response for backed-up email.
type BackupViewerMessagePage struct {
	Messages []BackupViewerMessageSummary `json:"messages"`
	Total    int                          `json:"total"`
	HasMore  bool                         `json:"hasMore"`
}

// BackupViewerMessageDetail is the parsed, sanitized representation of a backed-up email.
type BackupViewerMessageDetail struct {
	Key          string                   `json:"key"`
	AccountEmail string                   `json:"accountEmail"`
	FolderPath   string                   `json:"folderPath"`
	Subject      string                   `json:"subject"`
	Date         string                   `json:"date"`
	From         []string                 `json:"from"`
	To           []string                 `json:"to"`
	Cc           []string                 `json:"cc"`
	Bcc          []string                 `json:"bcc"`
	BodyHTML     string                   `json:"bodyHTML"`
	BodyText     string                   `json:"bodyText"`
	HasHTML      bool                     `json:"hasHTML"`
	Size         int                      `json:"size"`
	Attachments  []BackupViewerAttachment `json:"attachments"`
}

// BackupViewerAttachment describes an attachment embedded in the EML backup.
type BackupViewerAttachment struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	Inline      bool   `json:"inline"`
}

type backupViewerParsedBody struct {
	html        string
	text        string
	attachments []BackupViewerAttachment
}

// GetBackupViewerCatalog loads the backup index and returns accounts + recent messages.
func (a *App) GetBackupViewerCatalog(directory string) (*BackupViewerCatalog, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return &BackupViewerCatalog{}, nil
	}

	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := mailBackup.LoadIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	catalog := &BackupViewerCatalog{Directory: cleanDir}
	if !found {
		return catalog, nil
	}

	catalog.MessageCount = len(idx.Messages)
	catalog.Accounts = backupViewerAccounts(idx)
	if viewerIndex, ok, err := mailBackup.OpenExistingViewerIndex(cleanDir); err != nil {
		return nil, err
	} else if ok {
		defer viewerIndex.Close()
		if count, err := viewerIndex.MessageCount(""); err == nil && count > 0 {
			catalog.IndexReady = true
			catalog.MessageCount = count
			if accounts, err := viewerIndex.Accounts(); err == nil {
				catalog.Accounts = backupViewerAccountsFromViewerIndex(accounts)
			}
		}
	}
	catalog.NeedsIndex = catalog.MessageCount > 0 && !catalog.IndexReady
	return catalog, nil
}

// ListBackupViewerMessages returns one page of backed-up message summaries.
func (a *App) ListBackupViewerMessages(directory, accountEmail, sortOrder string, offset, limit int) (*BackupViewerMessagePage, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return &BackupViewerMessagePage{}, nil
	}
	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := mailBackup.LoadIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	if !found {
		return &BackupViewerMessagePage{}, nil
	}
	if viewerIndex, ok, err := mailBackup.OpenExistingViewerIndex(cleanDir); err != nil {
		return nil, err
	} else if ok {
		defer viewerIndex.Close()
		if count, err := viewerIndex.MessageCount(""); err == nil && count > 0 {
			page, err := viewerIndex.ListMessages(accountEmail, sortOrder, offset, limit)
			if err != nil {
				return nil, err
			}
			return backupViewerPageFromViewerIndex(page), nil
		}
	}

	a.hydrateBackupViewerAttachmentFlags(idx)
	messages, total := backupViewerMessagesPage(idx, accountEmail, "", sortOrder, offset, limit)
	return &BackupViewerMessagePage{
		Messages: messages,
		Total:    total,
		HasMore:  offset+len(messages) < total,
	}, nil
}

// SearchBackupViewerMessages searches the whole backup viewer index.
func (a *App) SearchBackupViewerMessages(directory, accountEmail, query string, offset, limit int) (*BackupViewerMessagePage, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return &BackupViewerMessagePage{}, nil
	}
	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	if viewerIndex, ok, err := mailBackup.OpenExistingViewerIndex(cleanDir); err != nil {
		return nil, err
	} else if ok {
		defer viewerIndex.Close()
		if count, err := viewerIndex.MessageCount(""); err == nil && count > 0 {
			page, err := viewerIndex.SearchMessages(accountEmail, query, offset, limit)
			if err != nil {
				return nil, err
			}
			return backupViewerPageFromViewerIndex(page), nil
		}
	}

	idx, found, err := mailBackup.LoadIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	if !found {
		return &BackupViewerMessagePage{}, nil
	}
	a.hydrateBackupViewerAttachmentFlags(idx)
	messages, total := backupViewerMessagesPage(idx, accountEmail, query, "newest", offset, limit)
	return &BackupViewerMessagePage{
		Messages: messages,
		Total:    total,
		HasMore:  offset+len(messages) < total,
	}, nil
}

// BuildBackupViewerIndex builds the SQLite summary/FTS index for an existing backup directory.
func (a *App) BuildBackupViewerIndex(directory string) (int, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return 0, errors.New("backup directory is not set")
	}
	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return 0, err
	}
	idx, found, err := mailBackup.LoadIndex(cleanDir)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errors.New("backup index not found")
	}
	viewerIndex, err := mailBackup.OpenViewerIndex(cleanDir)
	if err != nil {
		return 0, err
	}
	defer viewerIndex.Close()
	return viewerIndex.BuildFromJSONIndex(a.ctx, cleanDir, idx)
}

// GetBackupViewerMessage opens one indexed EML file and returns a sanitized detail view.
func (a *App) GetBackupViewerMessage(directory, key string) (*BackupViewerMessageDetail, error) {
	emlPath, entry, err := a.backupViewerIndexedMessagePath(directory, key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(emlPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	detail, err := parseBackupViewerEML(key, entry, file)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// SaveBackupViewerAttachmentAs saves one attachment from an indexed backup EML
// to a user-selected path. The backup index controls which EML can be read; the
// client only selects the parsed attachment index.
func (a *App) SaveBackupViewerAttachmentAs(directory, key string, attachmentIndex int) (string, error) {
	if attachmentIndex < 0 {
		return "", errors.New("backup attachment index is invalid")
	}
	emlPath, entry, err := a.backupViewerIndexedMessagePath(directory, key)
	if err != nil {
		return "", err
	}
	file, err := os.Open(emlPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	detail, err := parseBackupViewerEML(key, entry, file)
	if err != nil {
		return "", err
	}
	if attachmentIndex >= len(detail.Attachments) {
		return "", errors.New("backup attachment not found")
	}
	attachment := detail.Attachments[attachmentIndex]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	defaultDir := filepath.Join(homeDir, "Downloads")
	defaultFilename := email.DefaultAttachmentFilename(attachment.Filename)

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultDirectory: defaultDir,
		DefaultFilename:  defaultFilename,
		Title:            "Save Attachment",
	})
	if err != nil {
		return "", fmt.Errorf("failed to show save dialog: %w", err)
	}
	if savePath == "" {
		return "", nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to rewind backup message: %w", err)
	}
	att := &message.Attachment{
		MessageID:   key,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		Size:        attachment.Size,
		IsInline:    attachment.Inline,
	}
	if _, _, err := email.NewAttachmentDownloader(a.paths.AttachmentsPath()).SaveAttachmentFromRawReader(file, att, savePath); err != nil {
		return "", fmt.Errorf("failed to save backup attachment: %w", err)
	}
	return savePath, nil
}

func (a *App) backupViewerIndexedMessagePath(directory, key string) (string, mailBackup.IndexMessage, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return "", mailBackup.IndexMessage{}, errors.New("backup directory is not set")
	}
	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return "", mailBackup.IndexMessage{}, err
	}
	idx, found, err := mailBackup.LoadIndex(cleanDir)
	if err != nil {
		return "", mailBackup.IndexMessage{}, err
	}
	if !found {
		return "", mailBackup.IndexMessage{}, errors.New("backup index not found")
	}
	entry, ok := idx.Messages[key]
	if !ok {
		return "", mailBackup.IndexMessage{}, errors.New("backup message not found")
	}
	emlPath, err := mailBackup.IndexedFilePath(cleanDir, entry.EMLPath)
	if err != nil {
		return "", mailBackup.IndexMessage{}, err
	}
	return emlPath, entry, nil
}

// OpenBackupViewerDirectory opens the viewer's selected backup directory.
func (a *App) OpenBackupViewerDirectory(directory string) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return errors.New("backup directory is not set")
	}
	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return err
	}
	if stdruntime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform: %s", stdruntime.GOOS)
	}
	return exec.Command("open", cleanDir).Start()
}

func backupViewerAccounts(idx *mailBackup.Index) []BackupViewerAccount {
	counts := make(map[string]int)
	for _, msg := range idx.Messages {
		email := strings.TrimSpace(msg.AccountEmail)
		if email == "" {
			email = "unknown"
		}
		counts[email]++
	}

	accounts := make([]BackupViewerAccount, 0, len(counts))
	for email, count := range counts {
		accounts = append(accounts, BackupViewerAccount{AccountEmail: email, MessageCount: count})
	}
	sort.Slice(accounts, func(i, j int) bool {
		return strings.ToLower(accounts[i].AccountEmail) < strings.ToLower(accounts[j].AccountEmail)
	})
	return accounts
}

func backupViewerAccountsFromViewerIndex(accounts []mailBackup.ViewerAccountRecord) []BackupViewerAccount {
	out := make([]BackupViewerAccount, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, BackupViewerAccount{
			AccountEmail: account.AccountEmail,
			MessageCount: account.MessageCount,
		})
	}
	return out
}

func backupViewerPageFromViewerIndex(page mailBackup.ViewerMessagePage) *BackupViewerMessagePage {
	messages := make([]BackupViewerMessageSummary, 0, len(page.Messages))
	for _, message := range page.Messages {
		messages = append(messages, BackupViewerMessageSummary{
			Key:             message.Key,
			AccountEmail:    message.AccountEmail,
			FolderPath:      message.FolderPath,
			Subject:         message.Subject,
			Date:            message.Date,
			Size:            message.Size,
			AttachmentCount: message.AttachmentCount,
			Snippet:         message.Snippet,
		})
	}
	return &BackupViewerMessagePage{
		Messages: messages,
		Total:    page.Total,
		HasMore:  page.HasMore,
	}
}

func (a *App) hydrateBackupViewerAttachmentFlags(idx *mailBackup.Index) {
	if a == nil || a.db == nil || idx == nil || len(idx.Messages) == 0 {
		return
	}
	hasMissing := false
	for _, msg := range idx.Messages {
		if msg.HasAttachments == nil {
			hasMissing = true
			break
		}
	}
	if !hasMissing {
		return
	}

	rows, err := a.db.Query(`
		SELECT
			m.account_id,
			m.folder_id,
			COALESCE(f.uid_validity, 0),
			m.uid,
			COALESCE(m.has_attachments, 0)
		FROM messages m
		INNER JOIN folders f ON f.id = m.folder_id
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var row mailBackup.MessageRow
		var uid, uidValidity int64
		var hasAttachments bool
		if err := rows.Scan(&row.AccountID, &row.FolderID, &uidValidity, &uid, &hasAttachments); err != nil {
			return
		}
		row.UIDValidity = uint32(uidValidity)
		row.UID = uint32(uid)
		entry, ok := idx.Messages[mailBackup.MessageKey(row)]
		if !ok || entry.HasAttachments != nil {
			continue
		}
		entry.HasAttachments = mailBackup.BoolPtr(hasAttachments)
		idx.Messages[mailBackup.MessageKey(row)] = entry
	}
}

func backupViewerMessages(idx *mailBackup.Index, accountEmail, query string, limit int) []BackupViewerMessageSummary {
	accountEmail = strings.TrimSpace(strings.ToLower(accountEmail))
	query = strings.TrimSpace(strings.ToLower(query))
	if limit <= 0 && query != "" {
		limit = backupViewerDefaultLimit
	}
	if limit > 200 {
		limit = 200
	}

	messages := make([]BackupViewerMessageSummary, 0, len(idx.Messages))
	for key, msg := range idx.Messages {
		if accountEmail != "" && strings.ToLower(msg.AccountEmail) != accountEmail {
			continue
		}
		summary := BackupViewerMessageSummary{
			Key:             key,
			AccountEmail:    msg.AccountEmail,
			FolderPath:      msg.FolderPath,
			Subject:         msg.Subject,
			Date:            msg.Date,
			Size:            msg.Size,
			AttachmentCount: backupViewerIndexedAttachmentCount(msg),
		}
		if query != "" && !backupViewerSummaryMatches(summary, query) {
			continue
		}
		messages = append(messages, summary)
	}

	sort.SliceStable(messages, func(i, j int) bool {
		left := mailBackup.ParseMessageTime(messages[i].Date)
		right := mailBackup.ParseMessageTime(messages[j].Date)
		if !left.Equal(right) {
			return right.Before(left)
		}
		return messages[i].Key > messages[j].Key
	})
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
	return messages
}

func backupViewerMessagesPage(idx *mailBackup.Index, accountEmail, query, sortOrder string, offset, limit int) ([]BackupViewerMessageSummary, int) {
	if limit <= 0 {
		limit = backupViewerDefaultLimit
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	messages := backupViewerMessages(idx, accountEmail, query, 0)
	if sortOrder == "oldest" {
		for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
			messages[left], messages[right] = messages[right], messages[left]
		}
	}
	total := len(messages)
	if offset >= total {
		return []BackupViewerMessageSummary{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return messages[offset:end], total
}

func backupViewerSummaryMatches(msg BackupViewerMessageSummary, query string) bool {
	fields := []string{msg.Subject, msg.AccountEmail, msg.FolderPath, msg.Date}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func backupViewerIndexedAttachmentCount(entry mailBackup.IndexMessage) int {
	if entry.HasAttachments != nil && *entry.HasAttachments {
		return 1
	}
	return 0
}

func backupIndexedFilePath(directory, relativePath string) (string, error) {
	return mailBackup.IndexedFilePath(directory, relativePath)
}

func parseBackupViewerEML(key string, entry mailBackup.IndexMessage, reader io.Reader) (*BackupViewerMessageDetail, error) {
	entity, err := gomessage.Read(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse backup message: %w", err)
	}

	subject := email.DecodeMIMEHeader(entity.Header.Get("Subject"))
	if strings.TrimSpace(subject) == "" {
		subject = entry.Subject
	}
	date := email.DecodeMIMEHeader(entity.Header.Get("Date"))
	if strings.TrimSpace(date) == "" {
		date = entry.Date
	}

	parsed := &backupViewerParsedBody{}
	if err := parseBackupViewerEntity(entity, parsed); err != nil {
		return nil, err
	}

	bodyHTML := ""
	if strings.TrimSpace(parsed.html) != "" {
		bodyHTML = email.NewSanitizer().Sanitize(parsed.html)
	}

	return &BackupViewerMessageDetail{
		Key:          key,
		AccountEmail: entry.AccountEmail,
		FolderPath:   entry.FolderPath,
		Subject:      subject,
		Date:         date,
		From:         email.ParseAddressHeader(entity.Header.Get("From")),
		To:           email.ParseAddressHeader(entity.Header.Get("To")),
		Cc:           email.ParseAddressHeader(entity.Header.Get("Cc")),
		Bcc:          email.ParseAddressHeader(entity.Header.Get("Bcc")),
		BodyHTML:     bodyHTML,
		BodyText:     parsed.text,
		HasHTML:      bodyHTML != "",
		Size:         entry.Size,
		Attachments:  parsed.attachments,
	}, nil
}

func parseBackupViewerEntity(entity *gomessage.Entity, parsed *backupViewerParsedBody) error {
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
			if err := parseBackupViewerEntity(part, parsed); err != nil {
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
	contentID := strings.Trim(entity.Header.Get("Content-ID"), "<>")
	filename := backupViewerPartFilename(contentParams, dispositionParams)
	isAttachment := filename != "" || disposition == "attachment" || (!strings.HasPrefix(contentType, "text/") && contentType != "")

	if isAttachment {
		if filename == "" {
			filename = backupViewerAttachmentFallbackName(contentType)
		}
		size, err := countBackupViewerAttachment(entity.Body)
		if err != nil {
			return err
		}
		index := len(parsed.attachments)
		parsed.attachments = append(parsed.attachments, BackupViewerAttachment{
			Index:       index,
			Filename:    filename,
			ContentType: contentType,
			Size:        size,
			Inline:      disposition == "inline" || contentID != "",
		})
		return nil
	}

	raw, err := readBackupViewerTextPart(entity.Body)
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

func readBackupViewerTextPart(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, backupViewerMaxTextPartBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) <= backupViewerMaxTextPartBytes {
		return raw, nil
	}
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	return raw[:backupViewerMaxTextPartBytes], nil
}

func countBackupViewerAttachment(r io.Reader) (int, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, err
	}
	maxInt := int(^uint(0) >> 1)
	if n > int64(maxInt) {
		return maxInt, nil
	}
	return int(n), nil
}

func backupViewerPartFilename(contentParams, dispositionParams map[string]string) string {
	for _, candidate := range []string{dispositionParams["filename"], contentParams["name"]} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return email.DecodeMIMEHeader(candidate)
		}
	}
	return ""
}

func backupViewerAttachmentFallbackName(contentType string) string {
	if contentType == "" {
		return "attachment.bin"
	}
	if ext, ok := strings.CutPrefix(contentType, "image/"); ok && ext != "" {
		return "attachment." + ext
	}
	return "attachment.bin"
}
