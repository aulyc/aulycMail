//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#include <stdlib.h>

void printHTML(const char* html, const char* jobTitle);
*/
import "C"
import "unsafe"

// PrintHTML renders the given HTML document in an offscreen WKWebView and opens
// the native macOS print panel. window.print() is a no-op inside WKWebView, so
// the viewer's print button routes here instead.
func PrintHTML(html, jobTitle string) {
	ch := C.CString(html)
	cj := C.CString(jobTitle)
	defer C.free(unsafe.Pointer(ch))
	defer C.free(unsafe.Pointer(cj))
	C.printHTML(ch, cj)
}
