package app

import (
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

	"aulyc.local/aulycmail/internal/account"
	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/settings"
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

	if stdruntime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform: %s", stdruntime.GOOS)
	}
	return exec.Command("open", cleanPath).Start()
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

	result, err := mailBackup.Run(a.ctx, a.db, mailBackup.RunOptions{
		Directory:         directory,
		StartedAt:         startedAt,
		AccountIDs:        accountIDs,
		RawFetchBatchSize: backupRawFetchBatchSize,
		StreamRawMessages: a.syncEngine.StreamRawMessages,
		EmitProgress: func(progress mailBackup.Progress) {
			a.emitBackupProgress(backupProgressFromInternal(progress))
		},
	})
	if err != nil {
		return nil, err
	}
	return backupRunResultFromInternal(result), nil
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

func backupProgressFromInternal(progress mailBackup.Progress) BackupProgress {
	return BackupProgress{
		Phase:        progress.Phase,
		AccountEmail: progress.AccountEmail,
		FolderPath:   progress.FolderPath,
		Current:      progress.Current,
		Total:        progress.Total,
		Exported:     progress.Exported,
		Skipped:      progress.Skipped,
		Missing:      progress.Missing,
		Failed:       progress.Failed,
		Message:      progress.Message,
	}
}

func backupRunResultFromInternal(result *mailBackup.RunResult) *BackupRunResult {
	if result == nil {
		return nil
	}
	return &BackupRunResult{
		Directory:  result.Directory,
		Mode:       result.Mode,
		Total:      result.Total,
		Exported:   result.Exported,
		Skipped:    result.Skipped,
		Missing:    result.Missing,
		Failed:     result.Failed,
		ReportPath: result.ReportPath,
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
