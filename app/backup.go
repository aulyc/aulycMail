package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/aulyc/aulycmail/internal/account"
	"github.com/aulyc/aulycmail/internal/platform"
	"github.com/aulyc/aulycmail/internal/settings"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	backupScopeAll      = "all"
	backupScopeSelected = "selected"

	backupIndexVersion = 1
)

var emailBackupMu sync.Mutex

// BackupSettings is the persisted backup preference exposed to the frontend.
type BackupSettings struct {
	Directory          string   `json:"directory"`
	Scope              string   `json:"scope"`
	SelectedAccountIDs []string `json:"selectedAccountIds"`
}

// BackupStatus summarizes what the selected directory currently contains.
type BackupStatus struct {
	Directory     string `json:"directory"`
	Mode          string `json:"mode"` // "full" or "incremental"
	HasIndex      bool   `json:"hasIndex"`
	MessageCount  int    `json:"messageCount"`
	LastRunAt     string `json:"lastRunAt,omitempty"`
	LastRunMode   string `json:"lastRunMode,omitempty"`
	LastRunResult string `json:"lastRunResult,omitempty"`
}

// BackupRunOptions controls a single backup run.
type BackupRunOptions struct {
	Directory          string   `json:"directory"`
	Scope              string   `json:"scope"`
	SelectedAccountIDs []string `json:"selectedAccountIds"`
}

// BackupRunResult is returned after a backup run finishes.
type BackupRunResult struct {
	Directory  string `json:"directory"`
	Mode       string `json:"mode"`
	Total      int    `json:"total"`
	Exported   int    `json:"exported"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
	ReportPath string `json:"reportPath,omitempty"`
}

// BackupProgress is emitted as backup:progress while RunEmailBackup executes.
type BackupProgress struct {
	Phase        string `json:"phase"`
	AccountEmail string `json:"accountEmail,omitempty"`
	FolderPath   string `json:"folderPath,omitempty"`
	Current      int    `json:"current"`
	Total        int    `json:"total"`
	Exported     int    `json:"exported"`
	Skipped      int    `json:"skipped"`
	Failed       int    `json:"failed"`
	Message      string `json:"message,omitempty"`
}

type backupIndex struct {
	Version   int                           `json:"version"`
	CreatedAt string                        `json:"createdAt"`
	UpdatedAt string                        `json:"updatedAt"`
	Messages  map[string]backupIndexMessage `json:"messages"`
	LastRun   *backupIndexRun               `json:"lastRun,omitempty"`
}

type backupIndexMessage struct {
	AccountID    string `json:"accountId"`
	AccountEmail string `json:"accountEmail"`
	FolderID     string `json:"folderId"`
	FolderPath   string `json:"folderPath"`
	UIDValidity  uint32 `json:"uidValidity"`
	UID          uint32 `json:"uid"`
	MessageID    string `json:"messageId,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Date         string `json:"date,omitempty"`
	EMLPath      string `json:"emlPath"`
	Size         int    `json:"size"`
	ExportedAt   string `json:"exportedAt"`
}

type backupIndexRun struct {
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Mode       string `json:"mode"`
	Total      int    `json:"total"`
	Exported   int    `json:"exported"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
}

type backupFailure struct {
	AccountEmail string `json:"accountEmail"`
	FolderPath   string `json:"folderPath"`
	UID          uint32 `json:"uid"`
	Subject      string `json:"subject,omitempty"`
	Error        string `json:"error"`
}

type backupReport struct {
	backupIndexRun
	Directory string          `json:"directory"`
	Failures  []backupFailure `json:"failures,omitempty"`
}

type backupMessageRow struct {
	ID           string
	AccountID    string
	AccountEmail string
	FolderID     string
	FolderPath   string
	FolderName   string
	UIDValidity  uint32
	UID          uint32
	MessageID    string
	Subject      string
	Date         time.Time
	DateRaw      string
	Size         int
}

// GetBackupSettings returns the persisted backup settings.
func (a *App) GetBackupSettings() (*BackupSettings, error) {
	directory, err := a.settingsStore.Get(settings.KeyBackupDirectory)
	if err != nil {
		return nil, err
	}
	scope, err := a.settingsStore.Get(settings.KeyBackupScope)
	if err != nil {
		return nil, err
	}
	if scope != backupScopeSelected {
		scope = backupScopeAll
	}

	rawIDs, err := a.settingsStore.Get(settings.KeyBackupSelectedAccountIDs)
	if err != nil {
		return nil, err
	}
	var selectedIDs []string
	if rawIDs != "" {
		_ = json.Unmarshal([]byte(rawIDs), &selectedIDs)
	}

	return &BackupSettings{
		Directory:          directory,
		Scope:              scope,
		SelectedAccountIDs: uniqueNonEmptyStrings(selectedIDs),
	}, nil
}

// SetBackupSettings persists backup settings without starting a backup run.
func (a *App) SetBackupSettings(cfg BackupSettings) error {
	scope := normalizeBackupScope(cfg.Scope)
	directory := strings.TrimSpace(cfg.Directory)
	if directory != "" {
		var err error
		directory, err = normalizeExistingDirectory(directory)
		if err != nil {
			return err
		}
	}

	selectedIDs := uniqueNonEmptyStrings(cfg.SelectedAccountIDs)
	selectedJSON, err := json.Marshal(selectedIDs)
	if err != nil {
		return fmt.Errorf("failed to encode selected accounts: %w", err)
	}

	if err := a.settingsStore.Set(settings.KeyBackupDirectory, directory); err != nil {
		return err
	}
	if err := a.settingsStore.Set(settings.KeyBackupScope, scope); err != nil {
		return err
	}
	return a.settingsStore.Set(settings.KeyBackupSelectedAccountIDs, string(selectedJSON))
}

// ChooseBackupDirectory opens a native directory picker for the backup target.
func (a *App) ChooseBackupDirectory() (string, error) {
	current, _ := a.settingsStore.Get(settings.KeyBackupDirectory)
	defaultDir := current
	if defaultDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultDir = filepath.Join(home, "Documents")
		}
	}

	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		DefaultDirectory: defaultDir,
		Title:            "选择备份目录",
	})
	if err != nil {
		return "", fmt.Errorf("failed to choose backup directory: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	return normalizeExistingDirectory(dir)
}

// OpenBackupDirectory opens the configured backup directory in the file manager.
func (a *App) OpenBackupDirectory(path string) error {
	saved, err := a.settingsStore.Get(settings.KeyBackupDirectory)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		path = saved
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("backup directory is not set")
	}

	cleanPath, err := normalizeExistingDirectory(path)
	if err != nil {
		return err
	}
	cleanSaved, err := normalizeExistingDirectory(saved)
	if err != nil {
		return err
	}
	if cleanPath != cleanSaved {
		return errors.New("path is not the configured backup directory")
	}

	if stdruntime.GOOS == "linux" && platform.IsFlatpak() {
		return platform.PortalOpenDirectory(cleanPath)
	}

	switch stdruntime.GOOS {
	case "linux":
		return exec.Command("xdg-open", cleanPath).Start()
	case "darwin":
		return exec.Command("open", cleanPath).Start()
	case "windows":
		return exec.Command("explorer", cleanPath).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", stdruntime.GOOS)
	}
}

// GetBackupStatus returns whether the target directory will run as full or incremental.
func (a *App) GetBackupStatus(directory string) (*BackupStatus, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	if directory == "" {
		return &BackupStatus{Mode: "full"}, nil
	}

	cleanDir, err := normalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := loadBackupIndex(cleanDir)
	if err != nil {
		return nil, err
	}
	status := &BackupStatus{
		Directory: cleanDir,
		Mode:      "full",
		HasIndex:  found,
	}
	if !found {
		return status, nil
	}

	status.Mode = "incremental"
	status.MessageCount = len(idx.Messages)
	if idx.LastRun != nil {
		status.LastRunAt = idx.LastRun.FinishedAt
		status.LastRunMode = idx.LastRun.Mode
		status.LastRunResult = fmt.Sprintf("%d exported, %d skipped, %d failed", idx.LastRun.Exported, idx.LastRun.Skipped, idx.LastRun.Failed)
	}
	return status, nil
}

// RunEmailBackup exports selected accounts to standard .eml files. Existing
// entries in .aulycmail-backup/index.json are skipped, so rerunning against the
// same directory is incremental. A different directory without an index starts
// as a full backup.
func (a *App) RunEmailBackup(options BackupRunOptions) (*BackupRunResult, error) {
	emailBackupMu.Lock()
	defer emailBackupMu.Unlock()

	startedAt := time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(options.Directory) == "" {
		options.Directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	directory, err := normalizeBackupTargetDirectory(options.Directory)
	if err != nil {
		return nil, err
	}
	options.Scope = normalizeBackupScope(options.Scope)
	if options.Scope == "" {
		options.Scope = backupScopeAll
	}

	accounts, err := a.resolveBackupAccounts(options.Scope, options.SelectedAccountIDs)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]string, 0, len(accounts))
	for _, acc := range accounts {
		accountIDs = append(accountIDs, acc.ID)
	}

	idx, found, err := loadBackupIndex(directory)
	if err != nil {
		return nil, err
	}
	mode := "full"
	if found {
		mode = "incremental"
	}
	if idx.Messages == nil {
		idx.Messages = make(map[string]backupIndexMessage)
	}
	if idx.Version == 0 {
		idx.Version = backupIndexVersion
	}
	if idx.CreatedAt == "" {
		idx.CreatedAt = startedAt
	}

	rows, err := a.listBackupMessages(accountIDs)
	if err != nil {
		return nil, err
	}

	result := &BackupRunResult{
		Directory: directory,
		Mode:      mode,
		Total:     len(rows),
	}
	failures := make([]backupFailure, 0)
	emit := func(progress BackupProgress) {
		wailsRuntime.EventsEmit(a.ctx, "backup:progress", progress)
	}
	emit(BackupProgress{Phase: "running", Total: len(rows), Message: "开始备份"})

	for i, row := range rows {
		select {
		case <-a.ctx.Done():
			return nil, a.ctx.Err()
		default:
		}

		key := backupMessageKey(row)
		existing, indexed := idx.Messages[key]
		if indexed && backupFileExists(directory, existing.EMLPath) {
			result.Skipped++
			emit(BackupProgress{
				Phase:        "running",
				AccountEmail: row.AccountEmail,
				FolderPath:   row.FolderPath,
				Current:      i + 1,
				Total:        len(rows),
				Exported:     result.Exported,
				Skipped:      result.Skipped,
				Failed:       result.Failed,
			})
			continue
		}

		raw, err := a.syncEngine.FetchRawMessage(a.ctx, row.AccountID, row.FolderID, row.UID)
		if err != nil {
			result.Failed++
			failures = append(failures, backupFailureFromRow(row, err))
			emit(BackupProgress{
				Phase:        "running",
				AccountEmail: row.AccountEmail,
				FolderPath:   row.FolderPath,
				Current:      i + 1,
				Total:        len(rows),
				Exported:     result.Exported,
				Skipped:      result.Skipped,
				Failed:       result.Failed,
				Message:      err.Error(),
			})
			continue
		}

		relPath := backupMessageRelativePath(row)
		if err := writeBackupFile(directory, relPath, raw); err != nil {
			result.Failed++
			failures = append(failures, backupFailureFromRow(row, err))
			emit(BackupProgress{
				Phase:        "running",
				AccountEmail: row.AccountEmail,
				FolderPath:   row.FolderPath,
				Current:      i + 1,
				Total:        len(rows),
				Exported:     result.Exported,
				Skipped:      result.Skipped,
				Failed:       result.Failed,
				Message:      err.Error(),
			})
			continue
		}

		idx.Messages[key] = backupIndexMessage{
			AccountID:    row.AccountID,
			AccountEmail: row.AccountEmail,
			FolderID:     row.FolderID,
			FolderPath:   row.FolderPath,
			UIDValidity:  row.UIDValidity,
			UID:          row.UID,
			MessageID:    row.MessageID,
			Subject:      row.Subject,
			Date:         row.DateRaw,
			EMLPath:      relPath,
			Size:         row.Size,
			ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		result.Exported++
		emit(BackupProgress{
			Phase:        "running",
			AccountEmail: row.AccountEmail,
			FolderPath:   row.FolderPath,
			Current:      i + 1,
			Total:        len(rows),
			Exported:     result.Exported,
			Skipped:      result.Skipped,
			Failed:       result.Failed,
		})
	}

	run := backupIndexRun{
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Mode:       mode,
		Total:      result.Total,
		Exported:   result.Exported,
		Skipped:    result.Skipped,
		Failed:     result.Failed,
	}
	idx.Version = backupIndexVersion
	idx.UpdatedAt = run.FinishedAt
	idx.LastRun = &run

	if err := saveBackupIndex(directory, idx); err != nil {
		return nil, err
	}
	reportPath, err := saveBackupReport(directory, backupReport{
		backupIndexRun: run,
		Directory:      directory,
		Failures:       failures,
	})
	if err != nil {
		return nil, err
	}
	result.ReportPath = reportPath

	emit(BackupProgress{
		Phase:    "done",
		Current:  len(rows),
		Total:    len(rows),
		Exported: result.Exported,
		Skipped:  result.Skipped,
		Failed:   result.Failed,
		Message:  "备份完成",
	})
	return result, nil
}

func (a *App) resolveBackupAccounts(scope string, selectedIDs []string) ([]*account.Account, error) {
	all, err := a.accountStore.List()
	if err != nil {
		return nil, err
	}
	selectedSet := map[string]bool{}
	for _, id := range selectedIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selectedSet[id] = true
		}
	}

	resolved := make([]*account.Account, 0, len(all))
	for _, acc := range all {
		if acc.SharedMailboxParentID != "" {
			continue
		}
		if scope == backupScopeSelected && !selectedSet[acc.ID] {
			continue
		}
		resolved = append(resolved, acc)
	}
	if len(resolved) == 0 {
		return nil, errors.New("no accounts selected for backup")
	}
	return resolved, nil
}

func (a *App) listBackupMessages(accountIDs []string) ([]backupMessageRow, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(accountIDs)), ",")
	args := make([]interface{}, 0, len(accountIDs))
	for _, id := range accountIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT
			m.id,
			m.account_id,
			a.email,
			m.folder_id,
			f.path,
			f.name,
			COALESCE(f.uid_validity, 0),
			m.uid,
			COALESCE(m.message_id, ''),
			COALESCE(m.subject, ''),
			COALESCE(m.date, ''),
			COALESCE(m.size, 0)
		FROM messages m
		INNER JOIN accounts a ON a.id = m.account_id
		INNER JOIN folders f ON f.id = m.folder_id
		WHERE m.account_id IN (%s)
		ORDER BY a.order_index ASC, f.path COLLATE NOCASE ASC, m.date ASC, m.uid ASC
	`, placeholders)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages for backup: %w", err)
	}
	defer rows.Close()

	var messages []backupMessageRow
	for rows.Next() {
		var row backupMessageRow
		var uid, uidValidity int64
		var size int64
		var dateRaw sql.NullString
		if err := rows.Scan(
			&row.ID,
			&row.AccountID,
			&row.AccountEmail,
			&row.FolderID,
			&row.FolderPath,
			&row.FolderName,
			&uidValidity,
			&uid,
			&row.MessageID,
			&row.Subject,
			&dateRaw,
			&size,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup message: %w", err)
		}
		row.UID = uint32(uid)
		row.UIDValidity = uint32(uidValidity)
		row.Size = int(size)
		if dateRaw.Valid {
			row.DateRaw = dateRaw.String
			row.Date = parseBackupTime(dateRaw.String)
		}
		if row.FolderPath == "" {
			row.FolderPath = row.FolderName
		}
		messages = append(messages, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate backup messages: %w", err)
	}
	return messages, nil
}

func normalizeBackupScope(scope string) string {
	if scope == backupScopeSelected {
		return backupScopeSelected
	}
	return backupScopeAll
}

func normalizeExistingDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid directory: %w", err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("backup directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup path is not a directory: %s", abs)
	}
	return abs, nil
}

func normalizeBackupTargetDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("backup directory is not set")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid backup directory: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("backup directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup path is not a directory: %s", abs)
	}
	return abs, nil
}

func loadBackupIndex(directory string) (*backupIndex, bool, error) {
	path := backupIndexPath(directory)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &backupIndex{Version: backupIndexVersion, Messages: map[string]backupIndexMessage{}}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to read backup index: %w", err)
	}
	var idx backupIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, false, fmt.Errorf("failed to parse backup index: %w", err)
	}
	if idx.Messages == nil {
		idx.Messages = map[string]backupIndexMessage{}
	}
	return &idx, true, nil
}

func saveBackupIndex(directory string, idx *backupIndex) error {
	path := backupIndexPath(directory)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create backup metadata directory: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode backup index: %w", err)
	}
	return writeFileAtomic(path, data, 0600)
}

func saveBackupReport(directory string, report backupReport) (string, error) {
	reportDir := filepath.Join(directory, ".aulycmail-backup", "reports")
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup report directory: %w", err)
	}
	name := time.Now().UTC().Format("20060102-150405") + ".json"
	path := filepath.Join(reportDir, name)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode backup report: %w", err)
	}
	if err := writeFileAtomic(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func backupIndexPath(directory string) string {
	return filepath.Join(directory, ".aulycmail-backup", "index.json")
}

func backupMessageKey(row backupMessageRow) string {
	return fmt.Sprintf("%s:%s:%d:%d", row.AccountID, row.FolderID, row.UIDValidity, row.UID)
}

func backupFileExists(baseDir, relPath string) bool {
	if relPath == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(baseDir, filepath.FromSlash(relPath)))
	return err == nil && !info.IsDir()
}

func backupMessageRelativePath(row backupMessageRow) string {
	datePrefix := "unknown-date"
	if !row.Date.IsZero() {
		datePrefix = row.Date.UTC().Format("20060102-150405")
	}
	subject := sanitizePathSegment(row.Subject, 80)
	if subject == "" {
		subject = "no-subject"
	}
	filename := fmt.Sprintf("%s_uv%d_uid%d_%s.eml", datePrefix, row.UIDValidity, row.UID, subject)

	parts := []string{"eml", sanitizePathSegment(row.AccountEmail, 80)}
	for _, segment := range splitFolderPath(row.FolderPath) {
		parts = append(parts, sanitizePathSegment(segment, 80))
	}
	parts = append(parts, filename)
	return filepath.ToSlash(filepath.Join(parts...))
}

func splitFolderPath(path string) []string {
	path = strings.ReplaceAll(path, "\\", "/")
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{"folder"}
	}
	return parts
}

func writeBackupFile(baseDir, relPath string, content []byte) error {
	path := filepath.Join(baseDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create backup folder: %w", err)
	}
	return writeFileAtomic(path, content, 0600)
}

func writeFileAtomic(path string, content []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace file: %w", err)
	}
	return nil
}

func sanitizePathSegment(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case unicode.IsControl(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Trim(out, ". ")
	if out == "" {
		return ""
	}
	runes := []rune(out)
	if len(runes) > maxRunes {
		out = string(runes[:maxRunes])
	}
	return out
}

func parseBackupTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		time.RFC1123Z,
		time.RFC1123,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func backupFailureFromRow(row backupMessageRow, err error) backupFailure {
	return backupFailure{
		AccountEmail: row.AccountEmail,
		FolderPath:   row.FolderPath,
		UID:          row.UID,
		Subject:      row.Subject,
		Error:        err.Error(),
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
