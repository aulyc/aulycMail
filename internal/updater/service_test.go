package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type memorySettings struct {
	mu        sync.Mutex
	values    map[string]string
	automatic bool
}

type fakeInstaller struct {
	err   error
	calls int
}

func (f *fakeInstaller) PrepareAndLaunch(_ context.Context, _ Manifest, _ *http.Client, progress func(string, int)) error {
	f.calls++
	progress(StateDownloading, 50)
	progress(StateVerifying, 90)
	return f.err
}

func (m *memorySettings) Get(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[key], nil
}
func (m *memorySettings) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}
func (m *memorySettings) GetAutomaticUpdateChecks() (bool, error) { return m.automatic, nil }

func TestCheckFallsBackToGiteeWithoutWeakeningValidation(t *testing.T) {
	manifestData, _ := json.Marshal(validManifest())
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"invalid":true}`)) }))
	defer github.Close()
	gitee := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(manifestData) }))
	defer gitee.Close()
	store := &memorySettings{values: map[string]string{}, automatic: true}
	var observed []string
	service := NewService(Config{
		CurrentVersion: "0.6.0-beta.23", CurrentBuild: 39, Store: store,
		HTTPClient: gitee.Client(), ManifestSources: []ManifestSource{{Source: "github", URL: github.URL}, {Source: "gitee", URL: gitee.URL}},
		OnChange: func(status Status) { observed = append(observed, status.State) },
	})
	status, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.State != StateAvailable || status.Source != "gitee" || !status.CanInstall {
		t.Fatalf("unexpected status: %#v", status)
	}
	if len(observed) < 2 || observed[0] != StateChecking || observed[len(observed)-1] != StateAvailable {
		t.Fatalf("unexpected state sequence: %v", observed)
	}
}

func TestCheckIfDueHonorsSuccessfulThrottleAndPreference(t *testing.T) {
	manifestData, _ := json.Marshal(validManifest())
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests++; _, _ = w.Write(manifestData) }))
	defer server.Close()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	store := &memorySettings{values: map[string]string{}, automatic: true}
	service := NewService(Config{
		CurrentVersion: "0.7.0", CurrentBuild: 40, Store: store, HTTPClient: server.Client(), Now: func() time.Time { return now },
		ManifestSources: []ManifestSource{{Source: "github", URL: server.URL}},
	})
	service.CheckIfDue(context.Background())
	service.CheckIfDue(context.Background())
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	store.automatic = false
	now = now.Add(25 * time.Hour)
	service.CheckIfDue(context.Background())
	if requests != 1 {
		t.Fatalf("disabled automatic checks made %d requests", requests)
	}
}

func TestValidateApplyPlanRejectsTargetsOutsideApplications(t *testing.T) {
	err := validateApplyPlan(ApplyPlan{TargetApp: "/tmp/aulycMail.app", StagedApp: "/tmp/.aulycMail-update-test.app", BackupApp: "/tmp/.aulycMail-backup-test.app", ParentPID: 42})
	if err == nil {
		t.Fatal("unsafe update target was accepted")
	}
}

func TestPlanPathValidationUsesPrivateTemporaryDirectory(t *testing.T) {
	helperRoot, err := os.MkdirTemp("", "aulycmail-update-helper-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(helperRoot)
	planPath := filepath.Join(helperRoot, "apply-plan.json")
	if err := os.WriteFile(planPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validatePlanFilePath(planPath); err != nil {
		t.Fatalf("valid plan path rejected: %v", err)
	}
	if err := validatePlanFilePath(filepath.Join(t.TempDir(), "apply-plan.json")); err == nil {
		t.Fatal("untrusted plan path accepted")
	}
	if !stringsHasPrefixBase("/Applications/.aulycMail-update-test.app", ".aulycMail-update-") {
		t.Fatal("valid staged name rejected")
	}
	if err := RunApplyPlan(filepath.Join(t.TempDir(), "wrong-name.json")); err == nil {
		t.Fatal("invalid helper plan path was accepted")
	}
	if err := RunApplyPlan(planPath); err == nil {
		t.Fatal("empty helper plan was accepted")
	}
	if !processAlive(os.Getpid()) {
		t.Fatal("current test process was reported dead")
	}
}

func TestApplyPlanValidationRejectsEachUnsafePathClass(t *testing.T) {
	base := ApplyPlan{TargetApp: "/Applications/aulycMail.app", StagedApp: "/tmp/.aulycMail-update-test.app", BackupApp: "/Applications/.aulycMail-backup-test.app", ParentPID: 42}
	if err := validateApplyPlan(base); err == nil {
		t.Fatal("cross-directory staged app was accepted")
	}
	base.StagedApp = "/Applications/staged.app"
	if err := validateApplyPlan(base); err == nil {
		t.Fatal("untrusted staged name was accepted")
	}
	base.StagedApp = "/Applications/.aulycMail-update-test.app"
	if err := validateApplyPlan(base); err == nil {
		t.Fatal("missing application bundles were accepted")
	}
}

func TestApplyPlanSwapsBundlesAndRollsBackFailures(t *testing.T) {
	makePlan := func() ApplyPlan {
		root := t.TempDir()
		plan := ApplyPlan{TargetApp: filepath.Join(root, "aulycMail.app"), StagedApp: filepath.Join(root, "staged.app"), BackupApp: filepath.Join(root, "backup.app"), ParentPID: 42}
		if err := os.Mkdir(plan.TargetApp, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(plan.StagedApp, 0o700); err != nil {
			t.Fatal(err)
		}
		return plan
	}
	plan := makePlan()
	launched := false
	if err := applyPlan(plan, func(int) bool { return false }, os.Rename, func(target string) error { launched = target == plan.TargetApp; return nil }); err != nil {
		t.Fatal(err)
	}
	if !launched {
		t.Fatal("updated application was not launched")
	}
	if _, err := os.Stat(plan.TargetApp); err != nil {
		t.Fatalf("new target missing: %v", err)
	}
	if _, err := os.Stat(plan.BackupApp); !os.IsNotExist(err) {
		t.Fatal("backup was not removed after success")
	}

	plan = makePlan()
	renameCalls := 0
	err := applyPlan(plan, func(int) bool { return false }, func(from, to string) error {
		renameCalls++
		if renameCalls == 2 {
			return context.DeadlineExceeded
		}
		return os.Rename(from, to)
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("staging rename failure was ignored")
	}
	if _, statErr := os.Stat(plan.TargetApp); statErr != nil {
		t.Fatal("old application was not rolled back")
	}

	plan = makePlan()
	err = applyPlan(plan, func(int) bool { return false }, os.Rename, func(string) error { return context.Canceled })
	if err == nil {
		t.Fatal("launch failure was ignored")
	}
	if _, statErr := os.Stat(plan.TargetApp); statErr != nil {
		t.Fatal("old application was not restored after launch failure")
	}
}

func TestInstallPublishesProgressAndHandlesFailure(t *testing.T) {
	installer := &fakeInstaller{}
	service := NewService(Config{CurrentVersion: "0.6.0", CurrentBuild: 1, Installer: installer})
	manifest := validManifest()
	service.manifest = &manifest
	service.status = Status{State: StateAvailable, CurrentVersion: "0.6.0", CurrentBuildNumber: 1, LatestVersion: manifest.Version, LatestBuildNumber: manifest.BuildNumber, CanInstall: true}
	status, err := service.Install(context.Background())
	if err != nil || status.State != StateInstalling || installer.calls != 1 {
		t.Fatalf("Install() = %#v, %v; calls=%d", status, err, installer.calls)
	}

	failing := &fakeInstaller{err: context.DeadlineExceeded}
	service.installer = failing
	service.status.State, service.status.CanInstall = StateAvailable, true
	status, err = service.Install(context.Background())
	if err == nil || status.State != StateFailed || status.FailureOperation != FailureOperationInstall {
		t.Fatalf("failing Install() = %#v, %v", status, err)
	}

	service.status.State, service.status.CanInstall = StateIdle, false
	if _, err := service.Install(context.Background()); err == nil {
		t.Fatal("install without an available update succeeded")
	}
}

func TestSchedulerStartsStopsAndChecks(t *testing.T) {
	manifestData, _ := json.Marshal(validManifest())
	checked := make(chan struct{}, 1)
	completed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(manifestData)
		select {
		case checked <- struct{}{}:
		default:
		}
	}))
	defer server.Close()
	store := &memorySettings{values: map[string]string{}, automatic: true}
	service := NewService(Config{
		CurrentVersion: "0.7.0", CurrentBuild: 40, Store: store, HTTPClient: server.Client(),
		ManifestSources: []ManifestSource{{Source: "github", URL: server.URL}}, StartupDelay: time.Millisecond, TickInterval: time.Hour,
		OnChange: func(status Status) {
			if status.State == StateUpToDate {
				select {
				case completed <- struct{}{}:
				default:
				}
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)
	service.Start(ctx)
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("scheduled update check did not run")
	}
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("scheduled update check did not complete")
	}
	if service.Status().State != StateUpToDate {
		t.Fatalf("status = %#v", service.Status())
	}
	service.Stop()
	service.Stop()
}

func TestRedirectBuildAndFailureHelpers(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/file", nil)
	if err := safeRedirect(request, nil); err != nil {
		t.Fatal(err)
	}
	unsafe, _ := http.NewRequest(http.MethodGet, "https://example.test/file", nil)
	if err := safeRedirect(unsafe, nil); err == nil {
		t.Fatal("unsafe redirect was accepted")
	}
	if ParseBuildNumber("42") != 42 || ParseBuildNumber("bad") != 0 || ParseBuildNumber("-1") != 0 {
		t.Fatal("unexpected build parsing")
	}

	store := &memorySettings{values: map[string]string{}, automatic: true}
	service := NewService(Config{CurrentVersion: "invalid", CurrentBuild: 1, Store: store, ManifestSources: []ManifestSource{{Source: "github", URL: "://bad"}}})
	status, err := service.Check(context.Background())
	if err == nil || status.State != StateFailed || status.FailureOperation != FailureOperationCheck {
		t.Fatalf("invalid source check = %#v, %v", status, err)
	}
}

func TestApplyHelperUsesTheSignedExecutableInsideTheInstalledBundle(t *testing.T) {
	target := "/Applications/aulycMail.app"
	executable := filepath.Join(target, "Contents", "MacOS", "aulycMail")
	if err := validateUpdateHelperExecutable(executable, target); err != nil {
		t.Fatalf("signed in-bundle executable rejected: %v", err)
	}
	if err := validateUpdateHelperExecutable(filepath.Join(t.TempDir(), "aulycMail-update-helper"), target); err == nil {
		t.Fatal("standalone copied executable was accepted as an update helper")
	}

	staged := "/Applications/.aulycMail-update-test.app"
	var helperRoot string
	err := launchApplyHelperWithExecutable(executable, target, staged, 42, func(actualExecutable, planPath, readyPath string) error {
		if actualExecutable != executable {
			t.Fatalf("helper executable = %q, want %q", actualExecutable, executable)
		}
		helperRoot = filepath.Dir(planPath)
		if readyPath != filepath.Join(helperRoot, applyHelperReadyFilename) {
			t.Fatalf("ready path = %q", readyPath)
		}
		data, readErr := os.ReadFile(planPath)
		if readErr != nil {
			return readErr
		}
		var plan ApplyPlan
		if decodeErr := json.Unmarshal(data, &plan); decodeErr != nil {
			return decodeErr
		}
		if plan.TargetApp != target || plan.StagedApp != staged || plan.ParentPID != 42 {
			t.Fatalf("unexpected apply plan: %#v", plan)
		}
		return nil
	})
	if helperRoot != "" {
		defer os.RemoveAll(helperRoot)
	}
	if err != nil {
		t.Fatalf("launchApplyHelperWithExecutable() error = %v", err)
	}
}

func TestApplyHelperLaunchFailureCleansItsPrivatePlan(t *testing.T) {
	var helperRoot string
	err := launchApplyHelperWithExecutable(
		"/Applications/aulycMail.app/Contents/MacOS/aulycMail",
		"/Applications/aulycMail.app",
		"/Applications/.aulycMail-update-test.app",
		42,
		func(_ string, planPath, _ string) error {
			helperRoot = filepath.Dir(planPath)
			return context.Canceled
		},
	)
	if err == nil {
		t.Fatal("helper launch failure was ignored")
	}
	if _, statErr := os.Stat(helperRoot); !os.IsNotExist(statErr) {
		t.Fatalf("failed helper plan was not removed: %v", statErr)
	}
}

func TestApplyHelperReadinessHandshakeAcceptsOnlyItsOwnedMarker(t *testing.T) {
	root := t.TempDir()
	readyPath := filepath.Join(root, applyHelperReadyFilename)
	ready, err := applyHelperIsReady(readyPath)
	if err != nil || ready {
		t.Fatalf("missing readiness marker = %v, %v", ready, err)
	}
	if err := signalApplyHelperReady(readyPath); err != nil {
		t.Fatalf("signalApplyHelperReady() error = %v", err)
	}
	ready, err = applyHelperIsReady(readyPath)
	if err != nil || !ready {
		t.Fatalf("valid readiness marker = %v, %v", ready, err)
	}
	if err := signalApplyHelperReady(readyPath); err == nil {
		t.Fatal("existing readiness marker was overwritten")
	}
	if err := os.WriteFile(readyPath, []byte("unexpected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := applyHelperIsReady(readyPath); err == nil {
		t.Fatal("malformed readiness marker was accepted")
	}
}

func TestStartApplyHelperWaitsForReadinessAndReportsEarlyExit(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "apply-plan.json")
	readyPath := filepath.Join(root, applyHelperReadyFilename)
	helperPath := filepath.Join(root, "ready-helper")
	helper := "#!/bin/sh\nprintf 'ready\\n' > \"$(dirname \"$2\")/" + applyHelperReadyFilename + "\"\nsleep 0.1\n"
	if err := os.WriteFile(helperPath, []byte(helper), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := startApplyHelper(helperPath, planPath, readyPath); err != nil {
		t.Fatalf("ready helper failed: %v", err)
	}

	earlyExitPath := filepath.Join(root, "early-exit-helper")
	if err := os.WriteFile(earlyExitPath, []byte("#!/bin/sh\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(readyPath); err != nil {
		t.Fatal(err)
	}
	if err := startApplyHelper(earlyExitPath, planPath, readyPath); err == nil {
		t.Fatal("helper exit before readiness was accepted")
	}
}
