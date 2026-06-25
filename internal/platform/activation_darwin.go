//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdbool.h>

// Implemented in activation_darwin.m
void startActivationObserver(void);
void installTerminateHook(void);
bool aulycRealQuitRequested(void);
*/
import "C"

// activationHandler is invoked whenever the app is (re)activated — e.g. the
// user clicks the Dock icon or Cmd+Tabs back. Set via SetActivationHandler.
var activationHandler func()

//export goAppActivated
func goAppActivated() {
	dispatchActivation()
}

//export goAppReopen
func goAppReopen() {
	dispatchActivation()
}

// dispatchActivation invokes the activation handler off the main thread. The C
// callbacks run on the macOS main thread, and the handler calls Wails runtime
// functions that dispatch_sync to the main thread, which would deadlock if
// invoked directly. Hand off to a goroutine (same pattern as the notification
// click callback).
func dispatchActivation() {
	if activationHandler == nil {
		return
	}
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

// InstallTerminateHook swizzles -[NSApplication terminate:] so a genuine quit
// (Dock "Quit", menu Quit, Cmd+Q) can be told apart from a window close in
// background mode. Call once at startup.
func InstallTerminateHook() {
	C.installTerminateHook()
}

// RealQuitRequested reports whether terminate: has been invoked — i.e. the user
// asked to actually quit, not just close the window.
func RealQuitRequested() bool {
	return bool(C.aulycRealQuitRequested())
}
