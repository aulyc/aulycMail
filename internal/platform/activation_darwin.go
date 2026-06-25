//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

// Implemented in activation_darwin.m
void startActivationObserver(void);
*/
import "C"

// activationHandler is invoked whenever the app is (re)activated — e.g. the
// user clicks the Dock icon or Cmd+Tabs back. Set via SetActivationHandler.
var activationHandler func()

//export goAppActivated
func goAppActivated() {
	if activationHandler == nil {
		return
	}
	// The C callback runs on the macOS main thread; the handler calls Wails
	// runtime functions that dispatch_sync to the main thread, which would
	// deadlock if invoked directly. Hand off to a goroutine (same pattern as
	// the notification click callback).
	go activationHandler()
}

// SetActivationHandler registers the function called on app (re)activation.
// Typically wired to show the window when it's hidden in background mode.
func SetActivationHandler(fn func()) {
	activationHandler = fn
}

// StartActivationObserver installs the NSApplicationDidBecomeActiveNotification
// observer. Safe to call once after the Wails app has started.
func StartActivationObserver() {
	C.startActivationObserver()
}
