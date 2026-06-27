//go:build !darwin

package platform

// PrintHTML is a no-op outside macOS (native WKWebView printing is macOS-only).
func PrintHTML(_ string, _ string) {}
