package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchApplyHelperRejectsTestExecutable(t *testing.T) {
	err := launchApplyHelper("/Applications/aulycMail.app", "/Applications/.aulycMail-update-test.app")
	if err == nil || !strings.Contains(err.Error(), "signed executable") {
		t.Fatalf("launchApplyHelper() error = %v", err)
	}
}

func TestLaunchApplyHelperRequiresStarter(t *testing.T) {
	err := launchApplyHelperWithExecutable(
		"/Applications/aulycMail.app/Contents/MacOS/aulycMail",
		"/Applications/aulycMail.app",
		"/Applications/.aulycMail-update-test.app",
		42,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "launcher is unavailable") {
		t.Fatalf("launchApplyHelperWithExecutable() error = %v", err)
	}
}

func TestStartApplyHelperReportsLaunchAndCleanExitFailures(t *testing.T) {
	root := t.TempDir()
	readyPath := filepath.Join(root, applyHelperReadyFilename)
	if err := startApplyHelper(filepath.Join(root, "missing-helper"), filepath.Join(root, "plan"), readyPath); err == nil || !strings.Contains(err.Error(), "launch update helper") {
		t.Fatalf("missing helper error = %v", err)
	}

	cleanExit := filepath.Join(root, "clean-exit-helper")
	if err := os.WriteFile(cleanExit, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := startApplyHelper(cleanExit, filepath.Join(root, "plan"), readyPath); err == nil || !strings.Contains(err.Error(), "exited before signaling readiness") {
		t.Fatalf("clean helper exit error = %v", err)
	}
}

func TestApplyHelperReadinessRejectsNonRegularMarkers(t *testing.T) {
	root := t.TempDir()
	directoryMarker := filepath.Join(root, "directory-marker")
	if err := os.Mkdir(directoryMarker, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := applyHelperIsReady(directoryMarker); err == nil || !strings.Contains(err.Error(), "marker is invalid") {
		t.Fatalf("directory marker error = %v", err)
	}

	validMarker := filepath.Join(root, "valid-marker")
	if err := os.WriteFile(validMarker, applyHelperReadyContent, 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkMarker := filepath.Join(root, "symlink-marker")
	if err := os.Symlink(validMarker, symlinkMarker); err != nil {
		t.Fatal(err)
	}
	if _, err := applyHelperIsReady(symlinkMarker); err == nil || !strings.Contains(err.Error(), "marker is invalid") {
		t.Fatalf("symlink marker error = %v", err)
	}
}

func TestRunApplyPlanRejectsMalformedAndUnknownData(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "malformed JSON", data: `{`, want: "decode update plan"},
		{name: "unknown field", data: `{"unexpected":true}`, want: "decode update plan"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			helperRoot, err := os.MkdirTemp("", "aulycmail-update-helper-")
			if err != nil {
				t.Fatal(err)
			}
			planPath := filepath.Join(helperRoot, "apply-plan.json")
			if err := os.WriteFile(planPath, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			err = RunApplyPlan(planPath)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("RunApplyPlan() error = %v", err)
			}
			if _, statErr := os.Stat(helperRoot); !os.IsNotExist(statErr) {
				t.Fatalf("helper directory was not cleaned up: %v", statErr)
			}
		})
	}
}

func TestApplyPlanReportsInitialMoveFailure(t *testing.T) {
	want := errors.New("rename denied")
	err := applyPlan(
		ApplyPlan{TargetApp: "target", StagedApp: "staged", BackupApp: "backup", ParentPID: 42},
		func(int) bool { return false },
		func(string, string) error { return want },
		func(string) error { t.Fatal("launch must not run"); return nil },
	)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "move installed application aside") {
		t.Fatalf("applyPlan() error = %v", err)
	}
}

func TestPlanPathValidationRejectsUntrustedFileShapes(t *testing.T) {
	if err := validatePlanFilePath("apply-plan.json"); err == nil {
		t.Fatal("relative plan path was accepted")
	}

	helperRoot, err := os.MkdirTemp("", "aulycmail-update-helper-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(helperRoot)
	realPath := filepath.Join(helperRoot, "real-plan.json")
	if err := os.WriteFile(realPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(helperRoot, "apply-plan.json")
	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := validatePlanFilePath(symlinkPath); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink plan error = %v", err)
	}
}

func TestApplyPlanValidationRejectsUnsafeIdentityVariants(t *testing.T) {
	base := ApplyPlan{
		TargetApp: "/Applications/aulycMail.app",
		StagedApp: "/Applications/.aulycMail-update-test.app",
		BackupApp: "/Applications/.aulycMail-backup-test.app",
		ParentPID: 42,
	}
	tests := []struct {
		name   string
		mutate func(*ApplyPlan)
		want   string
	}{
		{name: "system pid", mutate: func(plan *ApplyPlan) { plan.ParentPID = 1 }, want: "target is invalid"},
		{name: "wrong target", mutate: func(plan *ApplyPlan) { plan.TargetApp = "/Applications/Other.app" }, want: "target is invalid"},
		{name: "backup directory", mutate: func(plan *ApplyPlan) { plan.BackupApp = "/tmp/.aulycMail-backup-test.app" }, want: "share the application directory"},
		{name: "backup prefix", mutate: func(plan *ApplyPlan) { plan.BackupApp = "/Applications/backup.app" }, want: "staging paths are invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := base
			test.mutate(&plan)
			if err := validateApplyPlan(plan); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateApplyPlan() error = %v", err)
			}
		})
	}
}

func TestApplyPathPrefixHelpersRejectLookalikes(t *testing.T) {
	for _, value := range []string{
		"/Applications/.aulycMail-update-.app",
		"/Applications/.aulycMail-update-test",
		"/Applications/prefix.aulycMail-update-test.app",
	} {
		if stringsHasPrefixBase(value, ".aulycMail-update-") {
			t.Fatalf("lookalike staged path %q was accepted", value)
		}
	}
	if stringsHasPrefixBaseDir("/tmp/aulycmail-update-helper-", "aulycmail-update-helper-") {
		t.Fatal("empty helper suffix was accepted")
	}
}

func TestApplyPlanRollbacksIgnoreSecondaryRenameErrors(t *testing.T) {
	plan := ApplyPlan{TargetApp: "target", StagedApp: "staged", BackupApp: "backup", ParentPID: 42}
	primary := context.Canceled
	calls := 0
	err := applyPlan(plan, func(int) bool { return false }, func(_, _ string) error {
		calls++
		if calls == 1 {
			return nil
		}
		return primary
	}, func(string) error { return nil })
	if !errors.Is(err, primary) || calls != 3 {
		t.Fatalf("staging failure = %v; rename calls = %d", err, calls)
	}
}
