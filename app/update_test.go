package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/updater"
)

type appTestInstaller struct{ calls int }

func (i *appTestInstaller) PrepareAndLaunch(_ context.Context, _ updater.Manifest, _ *http.Client, progress func(string, int)) error {
	i.calls++
	progress(updater.StateVerifying, 90)
	return nil
}

func updateManifestFixture() updater.Manifest {
	downloads := func(file string) []updater.Download {
		return []updater.Download{
			{Source: "github", URL: "https://github.com/aulyc/aulycMail/releases/download/0.7.0/" + file},
			{Source: "gitee", URL: "https://gitee.com/aulyc/aulycMail/releases/download/0.7.0/" + file},
		}
	}
	return updater.Manifest{
		Schema: "urn:codex-engineering-standards:dual-mirror-latest:2", SchemaVersion: 2,
		Policy: "aulyc-dual-mirror-v1", ReleaseProfile: "macos-arm64-app", ReleaseChannel: "formal",
		Version: "0.7.0", BuildNumber: 40, Tag: "0.7.0",
		Commit: strings.Repeat("a", 40), BundleIdentifier: "com.aulyc.aulycmail", Architecture: "arm64",
		ReleasePageURL: "https://github.com/aulyc/aulycMail/releases/tag/0.7.0",
		Artifact:       updater.Downloadable{File: "aulycMail.dmg", SHA256: strings.Repeat("b", 64), Downloads: downloads("aulycMail.dmg")},
		Provenance: updater.Downloadable{File: "aulycMail.manifest.json", SHA256: strings.Repeat("c", 64), Downloads: []updater.Download{
			{Source: "github", URL: "https://raw.githubusercontent.com/aulyc/aulycMail/release-channel/updates/0.7.0/aulycMail.manifest.json"},
			{Source: "gitee", URL: "https://gitee.com/aulyc/aulycMail/raw/main/updates/0.7.0/aulycMail.manifest.json"},
		}},
		TeamIdentifier: "M9M7M2ARFD", MinimumSystemVersion: "11.0",
	}
}

func TestUpdateBridgeChecksSharesStatusAndRequestsQuitAfterInstall(t *testing.T) {
	manifestData, _ := json.Marshal(updateManifestFixture())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(manifestData) }))
	defer server.Close()
	installer := &appTestInstaller{}
	a := &App{ctx: context.Background()}
	a.updateService = updater.NewService(updater.Config{
		CurrentVersion: "0.6.0-beta.23", CurrentBuild: 39, HTTPClient: server.Client(), Installer: installer,
		ManifestSources: []updater.ManifestSource{{Source: "github", URL: server.URL}},
	})
	status, err := a.CheckForUpdates()
	if err != nil || status.State != updater.StateAvailable || !status.CanInstall {
		t.Fatalf("CheckForUpdates() = %#v, %v", status, err)
	}
	if got := a.GetUpdateStatus(); got.State != updater.StateAvailable {
		t.Fatalf("GetUpdateStatus() = %#v", got)
	}

	quit := make(chan struct{}, 1)
	a.quitAction = func() { quit <- struct{}{} }
	a.shuttingDown.Store(false)
	status, err = a.InstallAvailableUpdate()
	if err != nil || status.State != updater.StateInstalling || installer.calls != 1 {
		t.Fatalf("InstallAvailableUpdate() = %#v, %v; calls=%d", status, err, installer.calls)
	}
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("InstallAvailableUpdate did not request a graceful quit")
	}
}

func TestUpdateBridgeNotReadyAndInitializationFallbacks(t *testing.T) {
	a := &App{}
	if status := a.GetUpdateStatus(); status.State != updater.StateIdle {
		t.Fatalf("status = %#v", status)
	}
	if _, err := a.CheckForUpdates(); err == nil {
		t.Fatal("not-ready check succeeded")
	}
	if _, err := a.InstallAvailableUpdate(); err == nil {
		t.Fatal("not-ready install succeeded")
	}
	a.checkForUpdatesFromMenu()
	a.initUpdater()
	if a.updateService == nil {
		t.Fatal("initUpdater did not create a service")
	}
}
