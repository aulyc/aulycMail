//go:build !darwin

package platform

// ClipboardFilePaths returns no paths outside macOS because Finder pasteboard
// file URLs are a macOS-only integration.
func ClipboardFilePaths() ([]string, error) { return nil, nil }
