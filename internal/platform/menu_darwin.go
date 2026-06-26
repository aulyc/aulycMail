//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

typedef struct {
	const char* settings;
	const char* about;
	const char* quit;
	const char* edit;
	const char* undo;
	const char* redo;
	const char* cut;
	const char* copy;
	const char* paste;
	const char* deleteItem;
} AulycMenuLabels;

void installAppMenu(AulycMenuLabels labels);
*/
import "C"
import "unsafe"

// menuHandler is invoked when a custom App-menu item (Settings / About) is
// chosen. Set via SetMenuHandler. Receives an action key ("settings"/"about").
var menuHandler func(action string)

//export goMenuAction
func goMenuAction(action *C.char) {
	a := C.GoString(action)
	if menuHandler == nil {
		return
	}
	// The C callback runs on the macOS main thread; the handler emits Wails
	// events which dispatch_sync back to main and would deadlock if invoked
	// directly. Hand off to a goroutine (same pattern as the activation hook).
	go menuHandler(a)
}

// MenuLabels holds the localized strings for the custom application menu.
type MenuLabels struct {
	Settings, About, Quit, Edit, Undo, Redo, Cut, Copy, Paste, Delete string
}

// SetMenuHandler registers the function called when the Settings or About
// App-menu item is chosen.
func SetMenuHandler(fn func(action string)) { menuHandler = fn }

// InstallAppMenu replaces the application's main menu with a minimal one:
//
//	App menu : Settings, About, Quit
//	Edit menu: Undo, Redo, Cut, Copy, Paste, Delete (native selectors, so the
//	           webview's copy/paste/undo actually work)
//
// Call once at startup. The native side defers the work to the main queue so it
// runs after Wails has installed its own default menu, replacing it.
func InstallAppMenu(l MenuLabels) {
	cs := C.CString(l.Settings)
	ca := C.CString(l.About)
	cq := C.CString(l.Quit)
	ce := C.CString(l.Edit)
	cu := C.CString(l.Undo)
	cr := C.CString(l.Redo)
	cx := C.CString(l.Cut)
	cc := C.CString(l.Copy)
	cv := C.CString(l.Paste)
	cd := C.CString(l.Delete)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cq))
	defer C.free(unsafe.Pointer(ce))
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(cr))
	defer C.free(unsafe.Pointer(cx))
	defer C.free(unsafe.Pointer(cc))
	defer C.free(unsafe.Pointer(cv))
	defer C.free(unsafe.Pointer(cd))

	C.installAppMenu(C.AulycMenuLabels{
		settings:   cs,
		about:      ca,
		quit:       cq,
		edit:       ce,
		undo:       cu,
		redo:       cr,
		cut:        cx,
		copy:       cc,
		paste:      cv,
		deleteItem: cd,
	})
}
