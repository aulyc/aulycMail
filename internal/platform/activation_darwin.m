#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include <stdbool.h>

// Implemented in Go (activation_darwin.go), exported via cgo.
extern void goAppActivated(void);

// --- Real-quit detection ---------------------------------------------------
//
// In background mode the window-close hook hides the window instead of
// quitting. But Dock "Quit", the menu Quit item, and Cmd+Q all funnel through
// the same close hook, so they'd also just hide. They differ from a window
// close in one way: they call -[NSApplication terminate:]. Swizzling that lets
// us set a flag the Go close handler reads to allow a genuine quit. The window
// close button never calls terminate:, so it still hides.

static bool gRealQuitRequested = false;
static void (*gOrigTerminate)(id, SEL, id) = NULL;

bool aulycRealQuitRequested(void) { return gRealQuitRequested; }

static void aulycSwizzledTerminate(id self, SEL _cmd, id sender) {
    gRealQuitRequested = true;
    if (gOrigTerminate) {
        gOrigTerminate(self, _cmd, sender);
    }
}

void installTerminateHook(void) {
    Method m = class_getInstanceMethod([NSApplication class], @selector(terminate:));
    if (m == NULL) {
        return;
    }
    gOrigTerminate = (void (*)(id, SEL, id))method_getImplementation(m);
    method_setImplementation(m, (IMP)aulycSwizzledTerminate);
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
