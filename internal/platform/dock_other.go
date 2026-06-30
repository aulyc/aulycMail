//go:build !darwin

package platform

// SetDockBadge is a no-op on non-macOS platforms.
func SetDockBadge(count int) {}
