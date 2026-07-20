#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

static void aulycApplyDockBadgeLabel(NSString *text) {
    NSDockTile *tile = [[NSApplication sharedApplication] dockTile];
    [tile setBadgeLabel:text];
    [tile display];
}

// On macOS 13+, let UserNotifications own the badge so the current system
// permission is enforced. AppKit remains the compatibility path on macOS 11
// and 12, where setBadgeCount:withCompletionHandler: is unavailable.
void aulycSetDockBadge(int count, const char *label) {
    @autoreleasepool {
        NSString *text = (label && label[0]) ? [NSString stringWithUTF8String:label] : nil;
        NSInteger systemCount = count > 0 ? MIN(count, 99) : 0;

        dispatch_async(dispatch_get_main_queue(), ^{
            if (@available(macOS 13.0, *)) {
                // Clear any label left by the pre-macOS-13 path before asking
                // the notification system to apply the authoritative count.
                aulycApplyDockBadgeLabel(nil);
                [[UNUserNotificationCenter currentNotificationCenter]
                    setBadgeCount:systemCount
                    withCompletionHandler:^(NSError *error) {
                        if (error != nil && count > 0) {
                            NSLog(@"[aulycMail] Failed to set system badge count: %@", error);
                        }
                    }];
                return;
            }
            aulycApplyDockBadgeLabel(text);
        });
    }
}
