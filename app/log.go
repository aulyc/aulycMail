package app

import "github.com/aulyc/aulycmail/internal/logging"

// LogFrontend emits a log message from the frontend through the Go-side
// zerolog logger so frontend diagnostics appear in the same log stream as
// backend events. Messages are tagged with component=frontend for easy
// filtering. Levels: "debug", "info", "warn", "error". Unknown levels
// fall through to info so messages are never silently dropped.
func (a *App) LogFrontend(level, message string) {
	log := logging.WithComponent("frontend")
	switch level {
	case "debug":
		log.Debug().Msg(message)
	case "warn":
		log.Warn().Msg(message)
	case "error":
		log.Error().Msg(message)
	default:
		log.Info().Msg(message)
	}
}
