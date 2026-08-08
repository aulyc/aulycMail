//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

char *aulycClipboardFilePathsJSON(void) {
	@autoreleasepool {
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		NSDictionary *options = @{ NSPasteboardURLReadingFileURLsOnlyKey: @YES };
		NSArray<NSURL *> *urls = [pasteboard
			readObjectsForClasses:@[[NSURL class]]
			options:options];
		NSMutableArray<NSString *> *paths = [NSMutableArray array];
		for (NSURL *url in urls) {
			if (![url isFileURL] || url.path.length == 0) {
				continue;
			}
			[paths addObject:url.path];
		}
		NSData *data = [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil];
		if (data == nil) {
			return strdup("[]");
		}
		NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		if (json == nil) {
			return strdup("[]");
		}
		char *result = strdup(json.UTF8String);
		[json release];
		return result;
	}
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// ClipboardFilePaths returns file URLs currently copied from Finder as local
// paths. It reads file-URL pasteboard objects only; clipboard text and file
// contents are not exposed.
func ClipboardFilePaths() ([]string, error) {
	raw := C.aulycClipboardFilePathsJSON()
	if raw == nil {
		return nil, fmt.Errorf("failed to read clipboard file URLs")
	}
	defer C.free(unsafe.Pointer(raw))

	var paths []string
	if err := json.Unmarshal([]byte(C.GoString(raw)), &paths); err != nil {
		return nil, fmt.Errorf("failed to decode clipboard file URLs: %w", err)
	}
	return paths, nil
}
