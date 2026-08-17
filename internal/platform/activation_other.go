//go:build !darwin

package platform

// SetActivationHandler is a no-op outside macOS (Dock re-activation handling
// is macOS-specific).
func SetActivationHandler(_ func()) {}

// StartActivationObserver is a no-op outside macOS.
func StartActivationObserver() {}

// InstallTerminateHook is a no-op outside macOS.
func InstallTerminateHook() {}

// RealQuitRequested always reports false outside macOS (no terminate: hook).
func RealQuitRequested() bool { return false }

// HideWindowForBackground lets non-macOS callers fall back to the Wails
// window runtime.
func HideWindowForBackground() bool { return false }

// ShowWindowFromBackground lets non-macOS callers fall back to the Wails
// window runtime.
func ShowWindowFromBackground() bool { return false }
