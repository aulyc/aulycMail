package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/aulyc/aulycmail/internal/email"
	"github.com/aulyc/aulycmail/internal/platform"
	"github.com/aulyc/aulycmail/internal/settings"
	gomessage "github.com/emersion/go-message"
	msgcharset "github.com/emersion/go-message/charset"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding/htmlindex"
)

const backupViewerDefaultLimit = 50

// BackupViewerCatalog summarizes a backup directory for the read-only viewer.
type BackupViewerCatalog struct {
	Directory    string                       `json:"directory"`
	Accounts     []BackupViewerAccount        `json:"accounts"`
	Messages     []BackupViewerMessageSummary `json:"messages"`
	MessageCount int                          `json:"messageCount"`
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

	cleanDir, err := normalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := loadBackupIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	catalog := &BackupViewerCatalog{Directory: cleanDir}
	if !found {
		return catalog, nil
	}

	catalog.MessageCount = len(idx.Messages)
	catalog.Accounts = backupViewerAccounts(idx)
	catalog.Messages = backupViewerMessages(cleanDir, idx, "", "", 0)
	return catalog, nil
}

// SearchBackupViewerMessages searches the backup index without opening every EML file.
func (a *App) SearchBackupViewerMessages(directory, accountEmail, query string, limit int) ([]BackupViewerMessageSummary, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return []BackupViewerMessageSummary{}, nil
	}
	cleanDir, err := normalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := loadBackupIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	if !found {
		return []BackupViewerMessageSummary{}, nil
	}
	return backupViewerMessages(cleanDir, idx, accountEmail, query, limit), nil
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
	raw, err := os.ReadFile(emlPath)
	if err != nil {
		return "", err
	}

	detail, err := parseBackupViewerEML(key, entry, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	if attachmentIndex >= len(detail.Attachments) {
		return "", errors.New("backup attachment not found")
	}
	attachment := detail.Attachments[attachmentIndex]

	content, err := email.NewAttachmentDownloader(a.paths.AttachmentsPath()).ExtractAttachmentContent(raw, attachment.Filename)
	if err != nil {
		return "", fmt.Errorf("failed to extract backup attachment: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	defaultDir := filepath.Join(homeDir, "Downloads")
	defaultFilename := email.DefaultAttachmentFilename(attachment.Filename)

	var savePath string
	if platform.IsFlatpak() {
		savePath, err = platform.PortalSaveFile("Save Attachment", defaultFilename, defaultDir)
		if err != nil {
			return "", fmt.Errorf("failed to show save dialog: %w", err)
		}
	} else {
		savePath, err = wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
			DefaultDirectory: defaultDir,
			DefaultFilename:  defaultFilename,
			Title:            "Save Attachment",
		})
		if err != nil {
			return "", fmt.Errorf("failed to show save dialog: %w", err)
		}
	}
	if savePath == "" {
		return "", nil
	}
	if err := os.WriteFile(savePath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to save backup attachment: %w", err)
	}
	return savePath, nil
}

func (a *App) backupViewerIndexedMessagePath(directory, key string) (string, backupIndexMessage, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return "", backupIndexMessage{}, errors.New("backup directory is not set")
	}
	cleanDir, err := normalizeExistingDirectory(directory)
	if err != nil {
		return "", backupIndexMessage{}, err
	}
	idx, found, err := loadBackupIndex(cleanDir)
	if err != nil {
		return "", backupIndexMessage{}, err
	}
	if !found {
		return "", backupIndexMessage{}, errors.New("backup index not found")
	}
	entry, ok := idx.Messages[key]
	if !ok {
		return "", backupIndexMessage{}, errors.New("backup message not found")
	}
	emlPath, err := backupIndexedFilePath(cleanDir, entry.EMLPath)
	if err != nil {
		return "", backupIndexMessage{}, err
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
	cleanDir, err := normalizeExistingDirectory(directory)
	if err != nil {
		return err
	}
	if stdruntime.GOOS == "linux" && platform.IsFlatpak() {
		return platform.PortalOpenDirectory(cleanDir)
	}
	switch stdruntime.GOOS {
	case "linux":
		return exec.Command("xdg-open", cleanDir).Start()
	case "darwin":
		return exec.Command("open", cleanDir).Start()
	case "windows":
		return exec.Command("explorer", cleanDir).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", stdruntime.GOOS)
	}
}

func backupViewerAccounts(idx *backupIndex) []BackupViewerAccount {
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

func backupViewerMessages(directory string, idx *backupIndex, accountEmail, query string, limit int) []BackupViewerMessageSummary {
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
			Key:          key,
			AccountEmail: msg.AccountEmail,
			FolderPath:   msg.FolderPath,
			Subject:      msg.Subject,
			Date:         msg.Date,
			Size:         msg.Size,
		}
		if query != "" && !backupViewerSummaryMatches(summary, query) {
			continue
		}
		messages = append(messages, summary)
	}

	sort.SliceStable(messages, func(i, j int) bool {
		left := parseBackupTime(messages[i].Date)
		right := parseBackupTime(messages[j].Date)
		if !left.Equal(right) {
			return right.Before(left)
		}
		return messages[i].Key > messages[j].Key
	})
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
	for i := range messages {
		if entry, ok := idx.Messages[messages[i].Key]; ok {
			messages[i].AttachmentCount = backupViewerAttachmentCount(directory, entry)
		}
	}
	return messages
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

func backupViewerAttachmentCount(directory string, entry backupIndexMessage) int {
	emlPath, err := backupIndexedFilePath(directory, entry.EMLPath)
	if err != nil {
		return 0
	}
	file, err := os.Open(emlPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	entity, err := gomessage.Read(file)
	if err != nil {
		return 0
	}
	return countBackupViewerEntityAttachments(entity)
}

func countBackupViewerEntityAttachments(entity *gomessage.Entity) int {
	if entity == nil {
		return 0
	}
	if mr := entity.MultipartReader(); mr != nil {
		count := 0
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			count += countBackupViewerEntityAttachments(part)
		}
		return count
	}

	contentType, contentParams, _ := mime.ParseMediaType(entity.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/plain"
	}
	contentType = strings.ToLower(contentType)
	disposition, dispositionParams, _ := mime.ParseMediaType(entity.Header.Get("Content-Disposition"))
	disposition = strings.ToLower(disposition)
	filename := backupViewerPartFilename(contentParams, dispositionParams)
	if filename != "" || disposition == "attachment" || (!strings.HasPrefix(contentType, "text/") && contentType != "") {
		return 1
	}
	return 0
}

func backupIndexedFilePath(directory, relativePath string) (string, error) {
	relativePath = strings.TrimSpace(filepath.FromSlash(relativePath))
	if relativePath == "" {
		return "", errors.New("backup message path is empty")
	}
	if filepath.IsAbs(relativePath) {
		return "", errors.New("backup message path must be relative")
	}

	cleanRel := filepath.Clean(relativePath)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", errors.New("backup message path escapes backup directory")
	}

	baseAbs, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(baseAbs, cleanRel)
	rel, err := filepath.Rel(baseAbs, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("backup message path escapes backup directory")
	}
	return fullPath, nil
}

func parseBackupViewerEML(key string, entry backupIndexMessage, reader io.Reader) (*BackupViewerMessageDetail, error) {
	entity, err := gomessage.Read(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse backup message: %w", err)
	}

	subject := decodeBackupHeader(entity.Header.Get("Subject"))
	if strings.TrimSpace(subject) == "" {
		subject = entry.Subject
	}
	date := decodeBackupHeader(entity.Header.Get("Date"))
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
		From:         parseBackupAddressHeader(entity.Header.Get("From")),
		To:           parseBackupAddressHeader(entity.Header.Get("To")),
		Cc:           parseBackupAddressHeader(entity.Header.Get("Cc")),
		Bcc:          parseBackupAddressHeader(entity.Header.Get("Bcc")),
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

	raw, err := io.ReadAll(entity.Body)
	if err != nil {
		return err
	}

	if isAttachment {
		if filename == "" {
			filename = backupViewerAttachmentFallbackName(contentType)
		}
		index := len(parsed.attachments)
		parsed.attachments = append(parsed.attachments, BackupViewerAttachment{
			Index:       index,
			Filename:    filename,
			ContentType: contentType,
			Size:        len(raw),
			Inline:      disposition == "inline" || contentID != "",
		})
		return nil
	}

	decoded := decodeBackupBody(raw, contentParams["charset"])
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

func backupViewerPartFilename(contentParams, dispositionParams map[string]string) string {
	for _, candidate := range []string{dispositionParams["filename"], contentParams["name"]} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return decodeBackupHeader(candidate)
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

func decodeBackupHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoder := &mime.WordDecoder{CharsetReader: backupCharsetReader}
	decoded, err := decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func parseBackupAddressHeader(value string) []string {
	value = decodeBackupHeader(value)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return []string{value}
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		name := strings.TrimSpace(address.Name)
		if name == "" {
			out = append(out, address.Address)
			continue
		}
		out = append(out, fmt.Sprintf("%s <%s>", name, address.Address))
	}
	return out
}

func decodeBackupBody(raw []byte, charsetName string) string {
	charsetName = strings.Trim(strings.TrimSpace(charsetName), `"`)
	if charsetName == "" || strings.EqualFold(charsetName, "utf-8") || strings.EqualFold(charsetName, "us-ascii") {
		if utf8.Valid(raw) {
			return string(raw)
		}
	}
	reader, err := backupCharsetReader(charsetName, bytes.NewReader(raw))
	if err == nil {
		if decoded, readErr := io.ReadAll(reader); readErr == nil {
			return string(decoded)
		}
	}
	return string(raw)
}

func backupCharsetReader(charsetName string, reader io.Reader) (io.Reader, error) {
	if charsetName == "" {
		return reader, nil
	}
	if decoded, err := msgcharset.Reader(charsetName, reader); err == nil {
		return decoded, nil
	}
	encoding, err := htmlindex.Get(charsetName)
	if err != nil {
		return nil, fmt.Errorf("unknown charset: %s", charsetName)
	}
	return encoding.NewDecoder().Reader(reader), nil
}
