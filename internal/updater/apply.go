package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type ApplyPlan struct {
	TargetApp string `json:"targetApp"`
	StagedApp string `json:"stagedApp"`
	BackupApp string `json:"backupApp"`
	ParentPID int    `json:"parentPID"`
}

func launchApplyHelper(targetApp, stagedApp string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate update helper: %w", err)
	}
	helperRoot, err := os.MkdirTemp("", "aulycmail-update-helper-")
	if err != nil {
		return fmt.Errorf("create update helper: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(helperRoot)
		}
	}()
	helperPath := filepath.Join(helperRoot, "aulycMail-update-helper")
	if err := copyExecutable(executable, helperPath); err != nil {
		return err
	}
	plan := ApplyPlan{
		TargetApp: targetApp,
		StagedApp: stagedApp,
		BackupApp: filepath.Join(filepath.Dir(targetApp), ".aulycMail-backup-"+uuid.NewString()+".app"),
		ParentPID: os.Getpid(),
	}
	planPath := filepath.Join(helperRoot, "apply-plan.json")
	planData, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("encode update plan: %w", err)
	}
	if err := os.WriteFile(planPath, planData, 0o600); err != nil {
		return fmt.Errorf("write update plan: %w", err)
	}
	command := exec.Command(helperPath, "--apply-update-plan", planPath)
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	if err := command.Start(); err != nil {
		return fmt.Errorf("launch update helper: %w", err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release update helper: %w", err)
	}
	cleanup = false
	return nil
}

func copyExecutable(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open update helper source: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("create update helper: %w", err)
	}
	if _, err := output.ReadFrom(input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy update helper: %w", err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return fmt.Errorf("sync update helper: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close update helper: %w", err)
	}
	return nil
}

// RunApplyPlan is the isolated helper entry point. It waits for the running app
// to exit, swaps the staged bundle into place with rollback on filesystem
// failure, and asks LaunchServices to reopen the verified bundle.
func RunApplyPlan(planPath string) error {
	if err := validatePlanFilePath(planPath); err != nil {
		return err
	}
	helperRoot := filepath.Dir(planPath)
	defer os.RemoveAll(helperRoot)
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("read update plan: %w", err)
	}
	var plan ApplyPlan
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return fmt.Errorf("decode update plan: %w", err)
	}
	if err := validateApplyPlan(plan); err != nil {
		return err
	}
	return applyPlan(plan, processAlive, os.Rename, func(target string) error {
		return exec.Command("/usr/bin/open", target).Start()
	})
}

func applyPlan(plan ApplyPlan, alive func(int) bool, rename func(string, string) error, launch func(string) error) error {
	deadline := time.Now().Add(45 * time.Second)
	for alive(plan.ParentPID) && time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
	}
	if alive(plan.ParentPID) {
		return fmt.Errorf("running application did not exit before update timeout")
	}
	if err := rename(plan.TargetApp, plan.BackupApp); err != nil {
		return fmt.Errorf("move installed application aside: %w", err)
	}
	if err := rename(plan.StagedApp, plan.TargetApp); err != nil {
		_ = rename(plan.BackupApp, plan.TargetApp)
		return fmt.Errorf("install staged application: %w", err)
	}
	if err := launch(plan.TargetApp); err != nil {
		_ = rename(plan.TargetApp, plan.StagedApp)
		_ = rename(plan.BackupApp, plan.TargetApp)
		return fmt.Errorf("reopen updated application: %w", err)
	}
	_ = os.RemoveAll(plan.BackupApp)
	return nil
}

func validatePlanFilePath(planPath string) error {
	absolute, err := filepath.Abs(planPath)
	if err != nil || absolute != planPath || filepath.Base(planPath) != "apply-plan.json" {
		return fmt.Errorf("update plan file path is invalid")
	}
	helperRoot := filepath.Dir(planPath)
	rootInfo, err := os.Lstat(helperRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || !stringsHasPrefixBaseDir(helperRoot, "aulycmail-update-helper-") {
		return fmt.Errorf("update helper directory is invalid")
	}
	relative, err := filepath.Rel(os.TempDir(), helperRoot)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return fmt.Errorf("update helper is outside the temporary directory")
	}
	planInfo, err := os.Lstat(planPath)
	if err != nil || !planInfo.Mode().IsRegular() || planInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("update plan is not a regular file")
	}
	return nil
}

func validateApplyPlan(plan ApplyPlan) error {
	if plan.ParentPID <= 1 || plan.TargetApp != "/Applications/aulycMail.app" {
		return fmt.Errorf("update plan target is invalid")
	}
	parent := filepath.Dir(plan.TargetApp)
	if filepath.Dir(plan.StagedApp) != parent || filepath.Dir(plan.BackupApp) != parent {
		return fmt.Errorf("update plan paths must share the application directory")
	}
	if !stringsHasPrefixBase(plan.StagedApp, ".aulycMail-update-") || !stringsHasPrefixBase(plan.BackupApp, ".aulycMail-backup-") {
		return fmt.Errorf("update plan staging paths are invalid")
	}
	for _, candidate := range []string{plan.TargetApp, plan.StagedApp} {
		info, err := os.Lstat(candidate)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("update plan contains an invalid application bundle")
		}
	}
	if _, err := os.Lstat(plan.BackupApp); !os.IsNotExist(err) {
		return fmt.Errorf("update backup path already exists")
	}
	return nil
}

func stringsHasPrefixBase(value, prefix string) bool {
	base := filepath.Base(value)
	return len(base) > len(prefix)+4 && base[:len(prefix)] == prefix && filepath.Ext(base) == ".app"
}

func stringsHasPrefixBaseDir(value, prefix string) bool {
	base := filepath.Base(value)
	return len(base) > len(prefix) && base[:len(prefix)] == prefix
}
