#import <Cocoa/Cocoa.h>

// Implemented in Go (activation_darwin.go), exported via cgo.
extern void goAppActivated(void);

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
