#import <Cocoa/Cocoa.h>

// Set (or clear) the Dock tile badge label. Pass an empty string to clear it.
// Must touch AppKit on the main thread, so hop there if needed.
void aulycSetDockBadge(const char *label) {
    NSString *text = (label && label[0]) ? [NSString stringWithUTF8String:label] : nil;
    void (^apply)(void) = ^{
        NSDockTile *tile = [[NSApplication sharedApplication] dockTile];
        [tile setBadgeLabel:text];
        [tile display];
    };
    if ([NSThread isMainThread]) {
        apply();
    } else {
        dispatch_async(dispatch_get_main_queue(), apply);
    }
}
