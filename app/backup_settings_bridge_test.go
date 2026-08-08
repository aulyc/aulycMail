package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	mailBackup "aulyc.local/aulycmail/internal/backup"
	"aulyc.local/aulycmail/internal/settings"
)

func TestBackupRunStateIsIsolatedPerAppInstance(t *testing.T) {
	emit := func(context.Context, string, ...interface{}) {}
	first := &App{eventsEmit: emit}
	second := &App{eventsEmit: emit}
	first.initBackupBridge()
	second.initBackupBridge()

	const firstOnlyMessage = "first-app-only-progress"
	first.emitBackupProgress(BackupProgress{Phase: "running", Message: firstOnlyMessage})

	firstState := first.GetBackupRunState()
	if firstState.Progress == nil || firstState.Progress.Message != firstOnlyMessage {
		t.Fatalf("first app state = %#v", firstState)
	}
	secondState := second.GetBackupRunState()
	if secondState.Progress != nil && secondState.Progress.Message == firstOnlyMessage {
		t.Fatalf("second app observed first app backup state: %#v", secondState)
	}
}

func TestBackupSettingsStatusAndAccountResolution(t *testing.T) {
	a, first, _ := newContactOwnEmailsTestApp(t)
	a.settingsStore = settings.NewStore(a.db)
	a.initBackupBridge()
	directory := t.TempDir()

	initial, err := a.GetBackupSettings()
	if err != nil || initial.Directory != "" || initial.Scope != backupScopeAll || len(initial.SelectedAccountIDs) != 0 {
		t.Fatalf("initial backup settings = %#v, %v", initial, err)
	}
	if err := a.SetBackupSettings(BackupSettings{
		Directory:          "  " + directory + "  ",
		Scope:              backupScopeSelected,
		SelectedAccountIDs: []string{" " + first.ID + " ", "", first.ID},
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := a.GetBackupSettings()
	if err != nil || stored.Directory != directory || stored.Scope != backupScopeSelected || len(stored.SelectedAccountIDs) != 1 || stored.SelectedAccountIDs[0] != first.ID {
		t.Fatalf("stored backup settings = %#v, %v", stored, err)
	}
	if err := a.SetBackupSettings(BackupSettings{Directory: directory, Scope: "unknown"}); err != nil {
		t.Fatal(err)
	}
	stored, err = a.GetBackupSettings()
	if err != nil || stored.Scope != backupScopeAll {
		t.Fatalf("normalized backup scope = %#v, %v", stored, err)
	}

	status, err := a.GetBackupStatus(directory)
	if err != nil || status.Mode != "full" || status.HasIndex || status.Directory != directory {
		t.Fatalf("full backup status = %#v, %v", status, err)
	}
	idx := &mailBackup.Index{
		Version: mailBackup.IndexVersion,
		Messages: map[string]mailBackup.IndexMessage{
			"one": {AccountID: first.ID, AccountEmail: first.Email, EMLPath: "eml/one.eml"},
		},
		LastRun: &mailBackup.IndexRun{
			FinishedAt: "2026-07-30T12:00:00Z",
			Mode:       "full",
			Exported:   1,
			Skipped:    2,
		},
	}
	if err := mailBackup.SaveIndex(directory, idx); err != nil {
		t.Fatal(err)
	}
	status, err = a.GetBackupStatus("")
	if err != nil || status.Mode != "incremental" || !status.HasIndex || status.MessageCount != 1 || status.LastRunAt != idx.LastRun.FinishedAt || status.LastRunMode != "full" || status.LastRunResult == "" {
		t.Fatalf("incremental backup status = %#v, %v", status, err)
	}

	second, err := a.accountStore.Create(&account.AccountConfig{
		Name:        "Second",
		DisplayName: "Second",
		Email:       "second@example.com",
		IMAPHost:    "imap.example.com",
		SMTPHost:    "smtp.example.com",
		Username:    "second@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.accountStore.Create(&account.AccountConfig{
		Name:                  "Shared",
		DisplayName:           "Shared",
		Email:                 "shared@example.com",
		SharedMailboxParentID: first.ID,
		IMAPHost:              "imap.example.com",
		SMTPHost:              "smtp.example.com",
		Username:              "shared@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	all, err := a.resolveBackupAccounts(backupScopeAll, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("all backup accounts = %#v, %v", all, err)
	}
	selected, err := a.resolveBackupAccounts(backupScopeSelected, []string{" " + second.ID + " "})
	if err != nil || len(selected) != 1 || selected[0].ID != second.ID {
		t.Fatalf("selected backup accounts = %#v, %v", selected, err)
	}
	if _, err := a.resolveBackupAccounts(backupScopeSelected, []string{"missing"}); err == nil {
		t.Fatal("an empty selected-account resolution must fail")
	}

	otherDirectory := t.TempDir()
	if err := a.OpenBackupDirectory(otherDirectory); err == nil {
		t.Fatal("opening a directory other than the configured backup directory must fail")
	}
	_ = a.GetBackupRunState()
}

func TestBackupProgressAndResultConversions(t *testing.T) {
	progress := backupProgressFromInternal(mailBackup.Progress{
		Phase:        "running",
		Stage:        mailBackup.ProgressStageExporting,
		StageCurrent: 2,
		StageTotal:   5,
		AccountEmail: "user@example.com",
		FolderPath:   "INBOX",
		Current:      3,
		Total:        8,
		Exported:     2,
		Skipped:      1,
		Missing:      1,
		Unavailable:  2,
		Failed:       1,
		Message:      "working",
	})
	if progress.Phase != "running" || progress.Stage != mailBackup.ProgressStageExporting || progress.StageCurrent != 2 || progress.StageTotal != 5 || progress.Current != 3 || progress.Total != 8 || progress.Unavailable != 2 || progress.AccountEmail != "user@example.com" {
		t.Fatalf("converted progress = %#v", progress)
	}
	initialExportJSON, err := json.Marshal(backupProgressFromInternal(mailBackup.Progress{
		Phase:        "running",
		Stage:        mailBackup.ProgressStageExporting,
		StageCurrent: 0,
		StageTotal:   5,
		Current:      3,
		Total:        8,
	}))
	if err != nil {
		t.Fatalf("marshal initial export progress: %v", err)
	}
	if !strings.Contains(string(initialExportJSON), `"stageCurrent":0`) {
		t.Fatalf("initial export progress omitted zero stage current: %s", initialExportJSON)
	}
	if result := backupRunResultFromInternal(nil); result != nil {
		t.Fatalf("nil internal result = %#v", result)
	}
	result := backupRunResultFromInternal(&mailBackup.RunResult{
		Directory:   "/backup",
		Mode:        "incremental",
		Total:       10,
		Exported:    4,
		Skipped:     3,
		Missing:     1,
		Unavailable: 1,
		Failed:      1,
		ReportPath:  "/backup/report.json",
	})
	if result == nil || result.Mode != "incremental" || result.Total != 10 || result.ReportPath != "/backup/report.json" {
		t.Fatalf("converted result = %#v", result)
	}
	if done := backupDoneProgress(nil); done.Phase != "done" || done.Message == "" {
		t.Fatalf("nil done progress = %#v", done)
	}
}
