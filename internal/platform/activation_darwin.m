#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include <stdbool.h>

// Implemented in Go (activation_darwin.go), exported via cgo.
extern void goAppActivated(void);
extern void goAppReopen(void);

// --- Dock-icon reopen ------------------------------------------------------
//
// didBecomeActive only fires on an inactive→active transition, so it misses
// the case where the app is ALREADY frontmost but its window is hidden
// (close-to-background can leave the app active). The canonical "Dock icon
// clicked" hook is the delegate's
// -applicationShouldHandleReopen:hasVisibleWindows:, which fires on every
// Dock click regardless of active state. We install it on Wails' delegate
// (replacing or adding) so a Dock click always brings the window back.

static BOOL (*gOrigHandleReopen)(id, SEL, NSApplication *, BOOL) = NULL;

static BOOL aulycHandleReopen(id self, SEL _cmd, NSApplication *app, BOOL hasVisibleWindows) {
    goAppReopen();
    if (gOrigHandleReopen) {
        return gOrigHandleReopen(self, _cmd, app, hasVisibleWindows);
    }
    return YES;
}

static void aulycInstallReopenHook(id delegate) {
    if (delegate == nil || gOrigHandleReopen != NULL) {
        return;
    }
    Class cls = [delegate class];
    SEL sel = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
    // Type encoding: BOOL return, self(id), _cmd(SEL), NSApplication*, BOOL.
    char types[8];
    snprintf(types, sizeof(types), "%s%s%s%s%s",
             @encode(BOOL), @encode(id), @encode(SEL), @encode(NSApplication *), @encode(BOOL));
    IMP prev = class_replaceMethod(cls, sel, (IMP)aulycHandleReopen, types);
    gOrigHandleReopen = (BOOL (*)(id, SEL, NSApplication *, BOOL))prev;
}

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
        id delegate = [NSApp delegate];
        if (delegate == nil) {
            return;
        }
        // Dock-icon reopen → always re-show the window.
        aulycInstallReopenHook(delegate);
        // Real-quit detection via applicationShouldTerminate:.
        if (gOrigShouldTerminate == NULL) {
            Method sm = class_getInstanceMethod([delegate class], @selector(applicationShouldTerminate:));
            if (sm != NULL) {
                gOrigShouldTerminate = (NSApplicationTerminateReply (*)(id, SEL, NSApplication *))method_getImplementation(sm);
                method_setImplementation(sm, (IMP)aulycSwizzledShouldTerminate);
            }
        }
    });
}

// Observer that forwards NSApplicationDidBecomeActiveNotification into Go.
// Firing on activation (Dock-icon click, Cmd+Tab back) lets the app re-show
// a window that background mode hid via orderOut — AppKit won't restore an
// ordered-out window on Dock click by itself.
static NSWindow *gBackgroundWindow = nil;

@interface AulycActivationObserver : NSObject
{
    NSWindow *pendingBackgroundHideWindow;
}
- (void)scheduleBackgroundHideForWindow:(NSWindow *)window;
- (void)cancelPendingBackgroundHide;
@end

@implementation AulycActivationObserver
- (void)appActivated:(NSNotification *)note {
    (void)note;
    goAppActivated();
}

- (void)backgroundWindowDidExitFullScreen:(NSNotification *)note {
    NSWindow *window = (NSWindow *)[note object];
    if (window == nil || window != pendingBackgroundHideWindow) {
        return;
    }

    [self cancelPendingBackgroundHide];
    [window orderOut:nil];
}

- (void)scheduleBackgroundHideForWindow:(NSWindow *)window {
    [self cancelPendingBackgroundHide];
    if (window == nil) {
        return;
    }
    // keyWindow/mainWindow may both become nil after orderOut. Keep the
    // application-owned Wails window identity so a later Dock click can show
    // the same window again.
    gBackgroundWindow = window;

    if (([window styleMask] & NSWindowStyleMaskFullScreen) == NSWindowStyleMaskFullScreen) {
        pendingBackgroundHideWindow = window;
        [[NSNotificationCenter defaultCenter]
            addObserver:self
               selector:@selector(backgroundWindowDidExitFullScreen:)
                   name:NSWindowDidExitFullScreenNotification
                 object:window];
        [window toggleFullScreen:nil];
        return;
    }

    [window orderOut:nil];
}

- (void)cancelPendingBackgroundHide {
    if (pendingBackgroundHideWindow == nil) {
        return;
    }
    [[NSNotificationCenter defaultCenter]
        removeObserver:self
                  name:NSWindowDidExitFullScreenNotification
                object:pendingBackgroundHideWindow];
    pendingBackgroundHideWindow = nil;
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

static NSWindow *aulycForegroundWindow(void) {
    NSWindow *window = [NSApp keyWindow];
    if (window == nil) {
        window = [NSApp mainWindow];
    }
    if (window == nil) {
        for (NSWindow *candidate in [NSApp orderedWindows]) {
            if ([candidate isVisible]) {
                return candidate;
            }
        }
    }
    return window;
}

void hideWindowForBackground(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gAulycActivationObserver == nil) {
            startActivationObserver();
        }
        [gAulycActivationObserver scheduleBackgroundHideForWindow:aulycForegroundWindow()];
    });
}

void showWindowFromBackground(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gAulycActivationObserver == nil) {
            startActivationObserver();
        }
        [gAulycActivationObserver cancelPendingBackgroundHide];
        NSWindow *window = gBackgroundWindow;
        if (window == nil) {
            window = aulycForegroundWindow();
        }
        if (window != nil) {
            [window makeKeyAndOrderFront:nil];
        }
        [NSApp activateIgnoringOtherApps:YES];
    });
}
