#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include <stdbool.h>

// Implemented in Go (activation_darwin.go), exported via cgo.
extern void goAppActivated(void);

// --- Real-quit detection ---------------------------------------------------
//
// In background mode the window-close hook hides the window instead of
// quitting. But Dock "Quit", the menu Quit item, and Cmd+Q all want to really
// quit, yet they funnel through the same close hook and would just hide.
//
// What sets a genuine quit apart from a window close is that it goes through
// the delegate's -applicationShouldTerminate:. The window close button uses
// -windowShouldClose: instead and never hits applicationShouldTerminate:, so
// swizzling that delegate method lets us flag a real quit without affecting
// the red-X hide.
//
// We ALSO swizzle -[NSApplication terminate:] as a fallback: the menu Quit /
// Cmd+Q call it directly, whereas the Dock "Quit" sends a kAEQuitApplication
// Apple Event that reaches applicationShouldTerminate: without terminate:.
// Covering both selectors catches every quit path.

static bool gRealQuitRequested = false;
static void (*gOrigTerminate)(id, SEL, id) = NULL;
static NSApplicationTerminateReply (*gOrigShouldTerminate)(id, SEL, NSApplication *) = NULL;

bool aulycRealQuitRequested(void) { return gRealQuitRequested; }

static void aulycSwizzledTerminate(id self, SEL _cmd, id sender) {
    gRealQuitRequested = true;
    if (gOrigTerminate) {
        gOrigTerminate(self, _cmd, sender);
    }
}

static NSApplicationTerminateReply aulycSwizzledShouldTerminate(id self, SEL _cmd, NSApplication *sender) {
    gRealQuitRequested = true;
    if (gOrigShouldTerminate) {
        return gOrigShouldTerminate(self, _cmd, sender);
    }
    return NSTerminateNow;
}

void installTerminateHook(void) {
    // Fallback: -[NSApplication terminate:] (menu Quit / Cmd+Q).
    Method tm = class_getInstanceMethod([NSApplication class], @selector(terminate:));
    if (tm != NULL && gOrigTerminate == NULL) {
        gOrigTerminate = (void (*)(id, SEL, id))method_getImplementation(tm);
        method_setImplementation(tm, (IMP)aulycSwizzledTerminate);
    }

    // Primary: the delegate's -applicationShouldTerminate: (every quit path,
    // including the Dock "Quit" Apple Event). Done on the main queue so Wails'
    // delegate is guaranteed to be installed by the time we read it.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gOrigShouldTerminate != NULL) {
            return;
        }
        id delegate = [NSApp delegate];
        if (delegate == nil) {
            return;
        }
        Method sm = class_getInstanceMethod([delegate class], @selector(applicationShouldTerminate:));
        if (sm == NULL) {
            return;
        }
        gOrigShouldTerminate = (NSApplicationTerminateReply (*)(id, SEL, NSApplication *))method_getImplementation(sm);
        method_setImplementation(sm, (IMP)aulycSwizzledShouldTerminate);
    });
}

// Observer that forwards NSApplicationDidBecomeActiveNotification into Go.
// Firing on activation (Dock-icon click, Cmd+Tab back) lets the app re-show
// a window that background mode hid via orderOut — AppKit won't restore an
// ordered-out window on Dock click by itself.
@interface AulycActivationObserver : NSObject
@end

@implementation AulycActivationObserver
- (void)appActivated:(NSNotification *)note {
    (void)note;
    goAppActivated();
}
@end

static AulycActivationObserver *gAulycActivationObserver = nil;

void startActivationObserver(void) {
    if (gAulycActivationObserver != nil) {
        return;
    }
    gAulycActivationObserver = [[AulycActivationObserver alloc] init];
    [[NSNotificationCenter defaultCenter]
        addObserver:gAulycActivationObserver
           selector:@selector(appActivated:)
               name:NSApplicationDidBecomeActiveNotification
             object:nil];
}
