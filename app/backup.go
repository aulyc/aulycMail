package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/aulyc/aulycmail/internal/account"
	mailBackup "github.com/aulyc/aulycmail/internal/backup"
	"github.com/aulyc/aulycmail/internal/platform"
	"github.com/aulyc/aulycmail/internal/settings"
	mailSync "github.com/aulyc/aulycmail/internal/sync"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	backupScopeAll      = "all"
	backupScopeSelected = "selected"

	backupRawFetchBatchSize = 10
)

var (
	errEmailBackupAlreadyRunning = errors.New("email backup is already running")
	emailBackupJob               backupRunTracker
)

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
	Missing    int    `json:"missing"`
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
	Missing      int    `json:"missing"`
	Failed       int    `json:"failed"`
	Message      string `json:"message,omitempty"`
}

// BackupRunState is the process-local state for an in-flight backup run.
type BackupRunState struct {
	Running   bool            `json:"running"`
	StartedAt string          `json:"startedAt,omitempty"`
	Progress  *BackupProgress `json:"progress,omitempty"`
}

type backupRunTracker struct {
	mu        sync.Mutex
	running   bool
	startedAt string
	progress  *BackupProgress
}

func (t *backupRunTracker) start(startedAt string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return false
	}
	t.running = true
	t.startedAt = startedAt
	t.progress = &BackupProgress{
		Phase:   "running",
		Current: 0,
		Total:   0,
		Message: "开始备份",
	}
	return true
}

func (t *backupRunTracker) update(progress BackupProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	next := progress
	t.progress = &next
}

func (t *backupRunTracker) finish(progress BackupProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.running = false
	next := progress
	t.progress = &next
}

func (t *backupRunTracker) snapshot() BackupRunState {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := BackupRunState{
		Running:   t.running,
		StartedAt: t.startedAt,
	}
	if t.progress != nil {
		progress := *t.progress
		state.Progress = &progress
	}
	return state
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
		directory, err = mailBackup.NormalizeExistingDirectory(directory)
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
	return mailBackup.NormalizeExistingDirectory(dir)
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

	cleanPath, err := mailBackup.NormalizeExistingDirectory(path)
	if err != nil {
		return err
	}
	cleanSaved, err := mailBackup.NormalizeExistingDirectory(saved)
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

	cleanDir, err := mailBackup.NormalizeExistingDirectory(directory)
	if err != nil {
		return nil, err
	}
	idx, found, err := mailBackup.LoadIndex(cleanDir)
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
		status.LastRunResult = mailBackup.FormatRunResult(*idx.LastRun)
	}
	return status, nil
}

// GetBackupRunState returns the current in-process backup run state.
func (a *App) GetBackupRunState() BackupRunState {
	return emailBackupJob.snapshot()
}

// StartEmailBackup starts a background backup and returns immediately. Progress
// and completion are emitted through backup:progress and recoverable via
// GetBackupRunState.
func (a *App) StartEmailBackup(options BackupRunOptions) (BackupRunState, error) {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if !emailBackupJob.start(startedAt) {
		return emailBackupJob.snapshot(), errEmailBackupAlreadyRunning
	}
	a.emitBackupProgress(BackupProgress{Phase: "running", Current: 0, Total: 0, Message: "开始备份"})

	go func() {
		defer recoverPanic("app.backup", "email backup")
		defer func() {
			if r := recover(); r != nil {
				a.finishBackupProgress(BackupProgress{
					Phase:   "error",
					Message: "backup worker crashed",
				})
				panic(r)
			}
		}()

		result, err := a.runEmailBackup(options, startedAt)
		if err != nil {
			a.finishBackupProgress(BackupProgress{
				Phase:   "error",
				Message: err.Error(),
			})
			return
		}
		a.finishBackupProgress(backupDoneProgress(result))
	}()

	return emailBackupJob.snapshot(), nil
}

// RunEmailBackup exports selected accounts to standard .eml files. Existing
// entries in .aulycmail-backup/index.json are skipped, so rerunning against the
// same directory is incremental. A different directory without an index starts
// as a full backup.
func (a *App) RunEmailBackup(options BackupRunOptions) (result *BackupRunResult, err error) {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if !emailBackupJob.start(startedAt) {
		return nil, errEmailBackupAlreadyRunning
	}
	a.emitBackupProgress(BackupProgress{Phase: "running", Current: 0, Total: 0, Message: "开始备份"})
	defer func() {
		if err == nil {
			return
		}
		a.finishBackupProgress(BackupProgress{
			Phase:   "error",
			Message: err.Error(),
		})
	}()

	result, err = a.runEmailBackup(options, startedAt)
	if err != nil {
		return nil, err
	}
	a.finishBackupProgress(backupDoneProgress(result))
	return result, nil
}

func (a *App) runEmailBackup(options BackupRunOptions, startedAt string) (*BackupRunResult, error) {
	if strings.TrimSpace(options.Directory) == "" {
		options.Directory, _ = a.settingsStore.Get(settings.KeyBackupDirectory)
	}
	directory, err := mailBackup.NormalizeTargetDirectory(options.Directory)
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

	idx, found, err := mailBackup.LoadIndex(directory)
	if err != nil {
		return nil, err
	}
	mode := "full"
	if found {
		mode = "incremental"
	}
	if idx.Messages == nil {
		idx.Messages = make(map[string]mailBackup.IndexMessage)
	}
	if idx.Version == 0 {
		idx.Version = mailBackup.IndexVersion
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
	failures := make([]mailBackup.Failure, 0)
	missing := make([]mailBackup.Failure, 0)
	a.emitBackupProgress(BackupProgress{Phase: "running", Total: len(rows), Message: "开始备份"})

	pendingRows := make([]mailBackup.MessageRow, 0, len(rows))
	processed := 0
	for _, row := range rows {
		select {
		case <-a.ctx.Done():
			return nil, a.ctx.Err()
		default:
		}

		key := mailBackup.MessageKey(row)
		existing, indexed := idx.Messages[key]
		if indexed && mailBackup.FileExists(directory, existing.EMLPath) {
			existing.HasAttachments = mailBackup.BoolPtr(row.HasAttachments)
			idx.Messages[key] = existing
			result.Skipped++
			processed++
			a.emitBackupProgress(BackupProgress{
				Phase:        "running",
				AccountEmail: row.AccountEmail,
				FolderPath:   row.FolderPath,
				Current:      processed,
				Total:        len(rows),
				Exported:     result.Exported,
				Skipped:      result.Skipped,
				Missing:      result.Missing,
				Failed:       result.Failed,
			})
			continue
		}

		pendingRows = append(pendingRows, row)
	}

	for _, group := range mailBackup.GroupMessageRows(pendingRows) {
		for offset := 0; offset < len(group.Rows); offset += backupRawFetchBatchSize {
			select {
			case <-a.ctx.Done():
				return nil, a.ctx.Err()
			default:
			}

			end := offset + backupRawFetchBatchSize
			if end > len(group.Rows) {
				end = len(group.Rows)
			}
			chunk := group.Rows[offset:end]
			rowsByUID := mailBackup.RowsByUID(chunk)
			streamResults, streamFailures, err := a.syncEngine.StreamRawMessages(a.ctx, group.AccountID, group.FolderID, mailBackup.RowUIDs(chunk), func(uid uint32, body io.Reader) (int64, error) {
				row, ok := rowsByUID[uid]
				if !ok {
					return 0, fmt.Errorf("unexpected backup UID: %d", uid)
				}
				return mailBackup.WriteFileFromReader(directory, mailBackup.MessageRelativePathForRow(row), body)
			})
			if err != nil {
				for _, row := range chunk {
					result.Failed++
					failures = append(failures, mailBackup.FailureFromRow(row, err))
					processed++
					a.emitBackupProgress(BackupProgress{
						Phase:        "running",
						AccountEmail: row.AccountEmail,
						FolderPath:   row.FolderPath,
						Current:      processed,
						Total:        len(rows),
						Exported:     result.Exported,
						Skipped:      result.Skipped,
						Missing:      result.Missing,
						Failed:       result.Failed,
						Message:      err.Error(),
					})
				}
				continue
			}

			for _, row := range chunk {
				key := mailBackup.MessageKey(row)
				relPath := mailBackup.MessageRelativePathForRow(row)
				streamResult, ok := streamResults[row.UID]
				err := streamFailures[row.UID]
				if err == nil && !ok {
					err = mailSync.RawMessageNotFoundError{UID: row.UID}
				}
				if err != nil {
					if mailSync.IsRawMessageNotFoundError(err) {
						result.Missing++
						missing = append(missing, mailBackup.FailureFromRow(row, err))
						processed++
						a.emitBackupProgress(BackupProgress{
							Phase:        "running",
							AccountEmail: row.AccountEmail,
							FolderPath:   row.FolderPath,
							Current:      processed,
							Total:        len(rows),
							Exported:     result.Exported,
							Skipped:      result.Skipped,
							Missing:      result.Missing,
							Failed:       result.Failed,
							Message:      err.Error(),
						})
						continue
					}
					result.Failed++
					failures = append(failures, mailBackup.FailureFromRow(row, err))
					processed++
					a.emitBackupProgress(BackupProgress{
						Phase:        "running",
						AccountEmail: row.AccountEmail,
						FolderPath:   row.FolderPath,
						Current:      processed,
						Total:        len(rows),
						Exported:     result.Exported,
						Skipped:      result.Skipped,
						Missing:      result.Missing,
						Failed:       result.Failed,
						Message:      err.Error(),
					})
					continue
				}

				idx.Messages[key] = mailBackup.IndexMessage{
					AccountID:      row.AccountID,
					AccountEmail:   row.AccountEmail,
					FolderID:       row.FolderID,
					FolderPath:     row.FolderPath,
					UIDValidity:    row.UIDValidity,
					UID:            row.UID,
					MessageID:      row.MessageID,
					Subject:        row.Subject,
					Date:           row.DateRaw,
					EMLPath:        relPath,
					Size:           mailBackup.FileSizeInt(streamResult.BytesWritten),
					HasAttachments: mailBackup.BoolPtr(row.HasAttachments),
					ExportedAt:     time.Now().UTC().Format(time.RFC3339),
				}
				result.Exported++
				processed++
				a.emitBackupProgress(BackupProgress{
					Phase:        "running",
					AccountEmail: row.AccountEmail,
					FolderPath:   row.FolderPath,
					Current:      processed,
					Total:        len(rows),
					Exported:     result.Exported,
					Skipped:      result.Skipped,
					Missing:      result.Missing,
					Failed:       result.Failed,
				})
			}
		}
	}

	run := mailBackup.IndexRun{
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Mode:       mode,
		Total:      result.Total,
		Exported:   result.Exported,
		Skipped:    result.Skipped,
		Missing:    result.Missing,
		Failed:     result.Failed,
	}
	idx.Version = mailBackup.IndexVersion
	idx.UpdatedAt = run.FinishedAt
	idx.LastRun = &run

	if err := mailBackup.SaveIndex(directory, idx); err != nil {
		return nil, err
	}
	reportPath, err := mailBackup.SaveReport(directory, mailBackup.Report{
		IndexRun:        run,
		Directory:       directory,
		MissingMessages: missing,
		Failures:        failures,
	})
	if err != nil {
		return nil, err
	}
	result.ReportPath = reportPath

	return result, nil
}

func (a *App) emitBackupProgress(progress BackupProgress) {
	emailBackupJob.update(progress)
	wailsRuntime.EventsEmit(a.ctx, "backup:progress", progress)
}

func (a *App) finishBackupProgress(progress BackupProgress) {
	emailBackupJob.finish(progress)
	wailsRuntime.EventsEmit(a.ctx, "backup:progress", progress)
}

func backupDoneProgress(result *BackupRunResult) BackupProgress {
	if result == nil {
		return BackupProgress{Phase: "done", Message: "备份完成"}
	}
	return BackupProgress{
		Phase:    "done",
		Current:  result.Total,
		Total:    result.Total,
		Exported: result.Exported,
		Skipped:  result.Skipped,
		Missing:  result.Missing,
		Failed:   result.Failed,
		Message:  "备份完成",
	}
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

func (a *App) listBackupMessages(accountIDs []string) ([]mailBackup.MessageRow, error) {
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
			COALESCE(m.size, 0),
			COALESCE(m.has_attachments, 0)
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

	var messages []mailBackup.MessageRow
	for rows.Next() {
		var row mailBackup.MessageRow
		var uid, uidValidity int64
		var size int64
		var hasAttachments bool
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
			&hasAttachments,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup message: %w", err)
		}
		row.UID = uint32(uid)
		row.UIDValidity = uint32(uidValidity)
		row.Size = int(size)
		row.HasAttachments = hasAttachments
		if dateRaw.Valid {
			row.DateRaw = dateRaw.String
			row.Date = mailBackup.ParseMessageTime(dateRaw.String)
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
