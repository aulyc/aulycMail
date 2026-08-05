//go:build !darwin

package platform

// StatusItemLabels holds localized strings for the macOS menu bar status item.
type StatusItemLabels struct {
	Open, Settings, CheckUpdate, Quit string
}

// SetStatusItemHandler is a no-op outside macOS.
func SetStatusItemHandler(_ func(action string)) {}

// SetStatusItemVisible is a no-op outside macOS.
func SetStatusItemVisible(_ bool, _ StatusItemLabels) {}

// SetStatusItemUnreadCount is a no-op outside macOS.
func SetStatusItemUnreadCount(_ int) {}
