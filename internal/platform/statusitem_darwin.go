//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

typedef struct {
	const char* open;
	const char* settings;
	const char* checkUpdate;
	const char* quit;
} AulycStatusItemLabels;

void aulycSetStatusItemVisible(int visible, AulycStatusItemLabels labels);
void aulycSetStatusItemUnreadLabel(const char *label);
*/
import "C"

import (
	"strconv"
	"unsafe"
)

// statusItemHandler is invoked when a custom menu bar status item action is
// chosen. Receives an action key ("show"/"settings"/"checkUpdate").
var statusItemHandler func(action string)

//export goStatusItemAction
func goStatusItemAction(action *C.char) {
	a := C.GoString(action)
	if statusItemHandler == nil {
		return
	}
	go statusItemHandler(a)
}

// StatusItemLabels holds localized strings for the macOS menu bar status item.
type StatusItemLabels struct {
	Open, Settings, CheckUpdate, Quit string
}

// SetStatusItemHandler registers the function called when a status item menu
// action is chosen.
func SetStatusItemHandler(fn func(action string)) { statusItemHandler = fn }

// SetStatusItemVisible shows or removes the macOS menu bar status item.
func SetStatusItemVisible(visible bool, l StatusItemLabels) {
	co := C.CString(l.Open)
	cs := C.CString(l.Settings)
	cu := C.CString(l.CheckUpdate)
	cq := C.CString(l.Quit)
	defer C.free(unsafe.Pointer(co))
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(cq))

	v := C.int(0)
	if visible {
		v = 1
	}
	C.aulycSetStatusItemVisible(v, C.AulycStatusItemLabels{
		open:        co,
		settings:    cs,
		checkUpdate: cu,
		quit:        cq,
	})
}

// SetStatusItemUnreadCount updates the unread count shown next to the menu bar
// status item icon. A count of 0 clears the text; counts above 99 show "99+".
func SetStatusItemUnreadCount(count int) {
	var label string
	switch {
	case count <= 0:
		label = ""
	case count > 99:
		label = "99+"
	default:
		label = strconv.Itoa(count)
	}
	c := C.CString(label)
	defer C.free(unsafe.Pointer(c))
	C.aulycSetStatusItemUnreadLabel(c)
}
