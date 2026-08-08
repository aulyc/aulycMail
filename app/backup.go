package app

import (
	"context"
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
	"aulyc.local/aulycmail/internal/activitylog"
	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/settings"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	backupScopeAll      = "all"
	backupScopeSelected = "selected"

	backupRawFetchBatchSize = 10
)

var errEmailBackupAlreadyRunning = errors.New("email backup is already running")

// BackupBridge is the Wails-bound backup surface embedded by App. Keeping the
// methods on a focused bridge makes the backup lifecycle independently
// testable and prevents App's direct method set from continuing to grow.
type BackupBridge struct {
	app *App
	job backupRunTracker
}

func (a *App) initBackupBridge() {
	if a.BackupBridge == nil {
		a.BackupBridge = &BackupBridge{app: a}
	}
}

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
	Directory   string `json:"directory"`
	Mode        string `json:"mode"`
	Total       int    `json:"total"`
	Exported    int    `json:"exported"`
	Skipped     int    `json:"skipped"`
	Missing     int    `json:"missing"`
	Unavailable int    `json:"unavailable"`
	Failed      int    `json:"failed"`
	ReportPath  string `json:"reportPath,omitempty"`
}

// BackupProgress is emitted as backup:progress while StartEmailBackup executes.
type BackupProgress struct {
	Phase        string `json:"phase"`
	Stage        string `json:"stage,omitempty"`
	StageCurrent int    `json:"stageCurrent"`
	StageTotal   int    `json:"stageTotal"`
	AccountEmail string `json:"accountEmail,omitempty"`
	FolderPath   string `json:"folderPath,omitempty"`
	Current      int    `json:"current"`
	Total        int    `json:"total"`
	Exported     int    `json:"exported"`
	Skipped      int    `json:"skipped"`
	Missing      int    `json:"missing"`
	Unavailable  int    `json:"unavailable"`
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
		Stage:   mailBackup.ProgressStageChecking,
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
func (b *BackupBridge) GetBackupSettings() (*BackupSettings, error) {
	directory, err := b.app.settingsStore.Get(settings.KeyBackupDirectory)
	if err != nil {
		return nil, err
	}
	scope, err := b.app.settingsStore.Get(settings.KeyBackupScope)
	if err != nil {
		return nil, err
	}
	if scope != backupScopeSelected {
		scope = backupScopeAll
	}

	rawIDs, err := b.app.settingsStore.Get(settings.KeyBackupSelectedAccountIDs)
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
func (b *BackupBridge) SetBackupSettings(cfg BackupSettings) error {
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

	if err := b.app.settingsStore.Set(settings.KeyBackupDirectory, directory); err != nil {
		return err
	}
	if err := b.app.settingsStore.Set(settings.KeyBackupScope, scope); err != nil {
		return err
	}
	return b.app.settingsStore.Set(settings.KeyBackupSelectedAccountIDs, string(selectedJSON))
}

// ChooseBackupDirectory opens a native directory picker for the backup target.
func (b *BackupBridge) ChooseBackupDirectory() (string, error) {
	current, _ := b.app.settingsStore.Get(settings.KeyBackupDirectory)
	defaultDir := current
	if defaultDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultDir = filepath.Join(home, "Documents")
		}
	}

	var dir string
	var err error
	if b.app.openDirectoryDialog != nil {
		dir, err = b.app.openDirectoryDialog(defaultDir, "选择备份目录")
	} else {
		dir, err = wailsRuntime.OpenDirectoryDialog(b.app.ctx, wailsRuntime.OpenDialogOptions{
			DefaultDirectory: defaultDir,
			Title:            "选择备份目录",
		})
	}
	if err != nil {
		return "", fmt.Errorf("failed to choose backup directory: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	return mailBackup.NormalizeExistingDirectory(dir)
}

// OpenBackupDirectory opens the configured backup directory in the file manager.
func (b *BackupBridge) OpenBackupDirectory(path string) error {
	saved, err := b.app.settingsStore.Get(settings.KeyBackupDirectory)
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

	if b.app.openSystemPath != nil {
		return b.app.openSystemPath(cleanPath, false)
	}
	if stdruntime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform: %s", stdruntime.GOOS)
	}
	return exec.Command("open", cleanPath).Start()
}

// GetBackupStatus returns whether the target directory will run as full or incremental.
func (b *BackupBridge) GetBackupStatus(directory string) (*BackupStatus, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory, _ = b.app.settingsStore.Get(settings.KeyBackupDirectory)
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
func (b *BackupBridge) GetBackupRunState() BackupRunState {
	return b.job.snapshot()
}

// StartEmailBackup starts a background backup and returns immediately. Progress
// and completion are emitted through backup:progress and recoverable via
// GetBackupRunState.
func (b *BackupBridge) StartEmailBackup(options BackupRunOptions) (BackupRunState, error) {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if !b.job.start(startedAt) {
		return b.job.snapshot(), errEmailBackupAlreadyRunning
	}
	b.emitBackupProgress(BackupProgress{Phase: "running", Stage: mailBackup.ProgressStageChecking, Current: 0, Total: 0, Message: "开始核对已有备份"})

	go func() {
		defer recoverPanic("app.backup", "email backup")
		defer func() {
			if r := recover(); r != nil {
				b.finishBackupProgress(BackupProgress{
					Phase:   "error",
					Message: "backup worker crashed",
				})
				panic(r)
			}
		}()

		result, err := b.runEmailBackup(options, startedAt)
		if err != nil {
			b.finishBackupProgress(BackupProgress{
				Phase:   "error",
				Message: err.Error(),
			})
			return
		}
		b.finishBackupProgress(backupDoneProgress(result))
	}()

	return b.job.snapshot(), nil
}

func (b *BackupBridge) runEmailBackup(options BackupRunOptions, startedAt string) (result *BackupRunResult, err error) {
	activityMode := ""
	defer func() {
		if recovered := recover(); recovered != nil {
			b.recordBackupActivity(options, nil, fmt.Errorf("backup worker panic: %v", recovered), activityMode)
			panic(recovered)
		}
		b.recordBackupActivity(options, result, err, activityMode)
	}()

	if strings.TrimSpace(options.Directory) == "" {
		options.Directory, _ = b.app.settingsStore.Get(settings.KeyBackupDirectory)
	}
	directory, err := mailBackup.NormalizeTargetDirectory(options.Directory)
	if err != nil {
		return nil, err
	}
	options.Directory = directory
	if _, found, indexErr := mailBackup.LoadIndex(directory); indexErr == nil {
		activityMode = "full"
		if found {
			activityMode = "incremental"
		}
	}
	options.Scope = normalizeBackupScope(options.Scope)
	if options.Scope == "" {
		options.Scope = backupScopeAll
	}

	accounts, err := b.resolveBackupAccounts(options.Scope, options.SelectedAccountIDs)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]string, 0, len(accounts))
	for _, acc := range accounts {
		accountIDs = append(accountIDs, acc.ID)
	}

	internalResult, err := mailBackup.Run(b.app.ctx, b.app.db, mailBackup.RunOptions{
		Directory:         directory,
		StartedAt:         startedAt,
		AccountIDs:        accountIDs,
		RawFetchBatchSize: backupRawFetchBatchSize,
		StreamRawMessages: b.app.syncEngine.StreamRawMessages,
		EmitProgress: func(progress mailBackup.Progress) {
			b.emitBackupProgress(backupProgressFromInternal(progress))
		},
	})
	if err != nil {
		return nil, err
	}
	return backupRunResultFromInternal(internalResult), nil
}

func backupActivityStatus(result *BackupRunResult, err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return activitylog.StatusCancelled
	}
	if err != nil || result == nil {
		return activitylog.StatusFailed
	}
	issues := result.Missing + result.Unavailable + result.Failed
	if issues == 0 {
		return activitylog.StatusSuccess
	}
	if result.Exported+result.Skipped > 0 {
		return activitylog.StatusPartial
	}
	return activitylog.StatusFailed
}

func backupActivitySummary(result *BackupRunResult, status string) string {
	if result == nil {
		switch status {
		case activitylog.StatusCancelled:
			return "备份已取消"
		default:
			return "备份失败"
		}
	}
	mode := "全量导出"
	if result.Mode == "incremental" {
		mode = "增量导出"
	}
	backedUp := result.Exported + result.Skipped
	notBackedUp := result.Missing + result.Unavailable + result.Failed
	return fmt.Sprintf("%s · 本次核对 %d · 已备份 %d · 未备份 %d · 本次新备份 %d · 此前已备份 %d · 服务器未返回 %d · 无可读取来源 %d · 处理失败 %d",
		mode, result.Total, backedUp, notBackedUp, result.Exported, result.Skipped, result.Missing, result.Unavailable, result.Failed)
}

func (b *BackupBridge) recordBackupActivity(options BackupRunOptions, result *BackupRunResult, runErr error, activityMode string) {
	status := backupActivityStatus(result, runErr)
	scope := normalizeBackupScope(options.Scope)
	directory := strings.TrimSpace(options.Directory)
	mode, total, completed, added, skipped, missing, unavailable, failed := activityMode, 0, 0, 0, 0, 0, 0, 0
	if result != nil {
		directory = result.Directory
		mode = result.Mode
		total = result.Total
		added = result.Exported
		skipped = result.Skipped
		missing = result.Missing
		unavailable = result.Unavailable
		failed = result.Failed
		completed = added + skipped
	} else if progress := b.job.snapshot().Progress; progress != nil {
		total = progress.Total
		added = progress.Exported
		skipped = progress.Skipped
		missing = progress.Missing
		unavailable = progress.Unavailable
		failed = progress.Failed
		completed = added + skipped
	}
	detail := ""
	if runErr != nil {
		detail = runErr.Error()
	}
	summaryResult := result
	if summaryResult == nil && (total > 0 || completed > 0 || missing > 0 || unavailable > 0 || failed > 0) {
		summaryResult = &BackupRunResult{
			Directory:   directory,
			Mode:        mode,
			Total:       total,
			Exported:    added,
			Skipped:     skipped,
			Missing:     missing,
			Unavailable: unavailable,
			Failed:      failed,
		}
	}
	entry := activitylog.Entry{
		Type:    activitylog.TypeBackup,
		Status:  status,
		Title:   "备份邮件",
		Summary: backupActivitySummary(summaryResult, status),
		Detail:  detail,
		Payload: map[string]any{
			"mode":        mode,
			"total":       total,
			"completed":   completed,
			"backedUp":    completed,
			"notBackedUp": missing + unavailable + failed,
			"added":       added,
			"skipped":     skipped,
			"missing":     missing,
			"unavailable": unavailable,
			"failed":      failed,
			"directory":   directory,
			"scope":       scope,
		},
	}
	if err := b.app.appendActivityLog(entry); err != nil {
		log := logging.WithComponent("app.backup")
		log.Warn().Err(err).Msg("Failed to persist backup activity log")
	}
}

func (b *BackupBridge) emitBackupProgress(progress BackupProgress) {
	b.job.update(progress)
	b.app.emitEvent("backup:progress", progress)
}

func (b *BackupBridge) finishBackupProgress(progress BackupProgress) {
	b.job.finish(progress)
	b.app.emitEvent("backup:progress", progress)
}

func backupDoneProgress(result *BackupRunResult) BackupProgress {
	if result == nil {
		return BackupProgress{Phase: "done", Message: "备份完成"}
	}
	return BackupProgress{
		Phase:       "done",
		Current:     result.Total,
		Total:       result.Total,
		Exported:    result.Exported,
		Skipped:     result.Skipped,
		Missing:     result.Missing,
		Unavailable: result.Unavailable,
		Failed:      result.Failed,
		Message:     "备份完成",
	}
}

func backupProgressFromInternal(progress mailBackup.Progress) BackupProgress {
	return BackupProgress{
		Phase:        progress.Phase,
		Stage:        progress.Stage,
		StageCurrent: progress.StageCurrent,
		StageTotal:   progress.StageTotal,
		AccountEmail: progress.AccountEmail,
		FolderPath:   progress.FolderPath,
		Current:      progress.Current,
		Total:        progress.Total,
		Exported:     progress.Exported,
		Skipped:      progress.Skipped,
		Missing:      progress.Missing,
		Unavailable:  progress.Unavailable,
		Failed:       progress.Failed,
		Message:      progress.Message,
	}
}

func backupRunResultFromInternal(result *mailBackup.RunResult) *BackupRunResult {
	if result == nil {
		return nil
	}
	return &BackupRunResult{
		Directory:   result.Directory,
		Mode:        result.Mode,
		Total:       result.Total,
		Exported:    result.Exported,
		Skipped:     result.Skipped,
		Missing:     result.Missing,
		Unavailable: result.Unavailable,
		Failed:      result.Failed,
		ReportPath:  result.ReportPath,
	}
}

func (b *BackupBridge) resolveBackupAccounts(scope string, selectedIDs []string) ([]*account.Account, error) {
	all, err := b.app.accountStore.List()
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
