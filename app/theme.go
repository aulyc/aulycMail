package app

import (
	"context"

	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/platform"
)

// initThemeMonitor initializes the system theme monitor for portal-based theme detection.
// On Linux, this uses the XDG Settings Portal. On other platforms, it's a no-op
// and the frontend falls back to matchMedia.
func (a *App) initThemeMonitor(ctx context.Context) {
	log := logging.WithComponent("app.theme")

	var monitor platform.ThemeMonitor
	if a.newThemeMonitor != nil {
		monitor = a.newThemeMonitor()
	} else {
		monitor = platform.NewThemeMonitor()
	}

	if err := monitor.Start(ctx); err != nil {
		log.Debug().Err(err).Msg("System theme monitor not available, frontend will use matchMedia fallback")
		a.setThemeMonitor(nil)
		return
	}
	a.setThemeMonitor(monitor)

	// Emit initial theme value so the frontend can use it immediately
	initialTheme := monitor.GetTheme()
	if initialTheme != platform.SystemThemeNoPreference {
		a.emitEvent("theme:system-preference", string(initialTheme))
	}

	go a.processThemeEvents(ctx, monitor)

	log.Info().Msg("System theme monitor initialized")
}

// processThemeEvents listens for system theme changes and emits events to the frontend
func (a *App) processThemeEvents(ctx context.Context, monitor platform.ThemeMonitor) {
	defer recoverPanic("app.theme", "process theme events")
	if monitor == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case theme, ok := <-monitor.Events():
			if !ok {
				return
			}
			a.emitEvent("theme:system-preference", string(theme))
		}
	}
}

// GetSystemTheme returns the current system theme preference detected via
// the XDG Settings Portal on Linux. Returns "light", "dark", or "" if not available.
func (a *App) GetSystemTheme() string {
	monitor := a.currentThemeMonitor()
	if monitor == nil {
		return ""
	}
	return string(monitor.GetTheme())
}

func (a *App) setThemeMonitor(monitor platform.ThemeMonitor) {
	a.themeMonitorMu.Lock()
	a.themeMonitor = monitor
	a.themeMonitorMu.Unlock()
}

func (a *App) currentThemeMonitor() platform.ThemeMonitor {
	a.themeMonitorMu.RLock()
	defer a.themeMonitorMu.RUnlock()
	return a.themeMonitor
}
