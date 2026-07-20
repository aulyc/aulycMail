//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications

#include <stdlib.h>

// Implemented in dock_darwin.m
void aulycSetDockBadge(int count, const char *label);
*/
import "C"

import (
	"strconv"
	"unsafe"
)

// SetDockBadge sets the macOS Dock tile badge to the given unread count.
// A count of 0 (or less) clears the badge; counts above 99 show "99+".
func SetDockBadge(count int) {
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
	C.aulycSetDockBadge(C.int(count), c)
}
