//go:build !darwin

package platform

// SetActivationHandler is a no-op outside macOS (Dock re-activation handling
// is macOS-specific).
func SetActivationHandler(_ func()) {}

// StartActivationObserver is a no-op outside macOS.
func StartActivationObserver() {}
