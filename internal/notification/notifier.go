// Package notification provides desktop notification support with click
// handling for navigating to specific content.
package notification

import "context"

// ClickHandler is called when a notification is clicked
type ClickHandler func(data NotificationData)

// SettingsHandler is called after macOS resolves the app's notification
// settings. BadgeEnabled is deliberately separate from Authorized because
// requestAuthorization may succeed while icon badges are disabled.
type SettingsHandler func(settings Settings)

// Settings contains the notification capabilities that affect app behavior.
type Settings struct {
	Authorized   bool
	BadgeEnabled bool
}

// NotificationData contains the context for a notification click.
type NotificationData struct {
	AccountID string
	FolderID  string
	ThreadID  string
}

// Notification represents a desktop notification to be shown
type Notification struct {
	Title string
	Body  string
	Icon  string
	Data  NotificationData
}

// Notifier provides notification support with click handling
type Notifier interface {
	// Start begins listening for notification events
	Start(ctx context.Context) error

	// Stop stops the notifier and cleans up resources
	Stop()

	// Show displays a notification and returns its ID
	Show(n Notification) (uint32, error)

	// SetClickHandler sets the callback for notification clicks
	SetClickHandler(handler ClickHandler)

	// SetSettingsHandler sets the callback for resolved notification settings.
	SetSettingsHandler(handler SettingsHandler)

	// RefreshSettings re-reads the current system notification settings.
	RefreshSettings()
}

// New creates a platform-specific Notifier
func New(appName string) Notifier {
	return newPlatformNotifier(appName)
}
