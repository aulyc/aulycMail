package app

import (
	"context"
	"fmt"

	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/updater"
)

func (a *App) initUpdater() {
	a.updateService = updater.NewService(updater.Config{
		CurrentVersion: Version,
		CurrentBuild:   updater.ParseBuildNumber(BuildNumber),
		Store:          a.settingsStore,
		Installer:      updater.NewInstaller(),
		OnChange: func(status updater.Status) {
			a.emitEvent("update:state", status)
		},
	})
}

// GetUpdateStatus returns the shared update state used by Settings, About and
// both native menus.
func (a *App) GetUpdateStatus() updater.Status {
	if a.updateService == nil {
		return updater.Status{State: updater.StateIdle, CurrentVersion: Version, CurrentBuildNumber: updater.ParseBuildNumber(BuildNumber)}
	}
	return a.updateService.Status()
}

// CheckForUpdates performs an explicit check. Manual checks bypass the
// automatic-update preference and the successful/failed throttles.
func (a *App) CheckForUpdates() (updater.Status, error) {
	if a.updateService == nil {
		return a.GetUpdateStatus(), fmt.Errorf("update service is not ready")
	}
	return a.updateService.Check(a.ctx)
}

// InstallAvailableUpdate downloads and verifies the selected formal release,
// launches the isolated replacement helper, then gracefully quits this app.
func (a *App) InstallAvailableUpdate() (updater.Status, error) {
	if a.updateService == nil {
		return a.GetUpdateStatus(), fmt.Errorf("update service is not ready")
	}
	status, err := a.updateService.Install(a.ctx)
	if err != nil {
		return status, err
	}
	a.QuitApp()
	return status, nil
}

func (a *App) checkForUpdatesFromMenu() {
	if a.updateService == nil {
		return
	}
	if _, err := a.updateService.Check(context.Background()); err != nil {
		log := logging.WithComponent("app.updater")
		log.Warn().Err(err).Msg("Manual update check failed")
	}
}
