#import <UserNotifications/UserNotifications.h>

// Forward declaration of the Go callback (defined via //export in notifier_darwin.go)
extern void goNotificationCallback(char *accountId, char *folderId, char *threadId);
extern void goNotificationSettingsCallback(int authorized, int badgeEnabled);

// Delegate that handles notification interactions and foreground presentation
@interface AulycMailNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation AulycMailNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    NSDictionary *userInfo = response.notification.request.content.userInfo;
    NSString *accountId = userInfo[@"accountId"] ?: @"";
    NSString *folderId  = userInfo[@"folderId"]  ?: @"";
    NSString *threadId  = userInfo[@"threadId"]  ?: @"";

    goNotificationCallback(
        (char *)[accountId UTF8String],
        (char *)[folderId  UTF8String],
        (char *)[threadId  UTF8String]
    );

    completionHandler();
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    // Show notification and play sound even when app is in foreground.
    // UNNotificationPresentationOptionBanner requires macOS 11.0+.
    UNNotificationPresentationOptions opts = UNNotificationPresentationOptionSound;
    if (@available(macOS 11.0, *)) {
        opts |= UNNotificationPresentationOptionBanner;
    }
    completionHandler(opts);
}

@end

static AulycMailNotificationDelegate *notifDelegate = nil;

static void publishNotificationSettings(UNUserNotificationCenter *center) {
    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        BOOL authorized = settings.authorizationStatus == UNAuthorizationStatusAuthorized
            || settings.authorizationStatus == UNAuthorizationStatusProvisional;
        BOOL badgeEnabled = settings.badgeSetting == UNNotificationSettingEnabled;
        NSLog(@"[aulycMail] Notification settings: authorized=%d badgeEnabled=%d",
              authorized, badgeEnabled);
        goNotificationSettingsCallback(authorized ? 1 : 0, badgeEnabled ? 1 : 0);
    }];
}

// setupNotifications initializes UNUserNotificationCenter and requests authorization.
// Dispatches to the main queue since UNUserNotificationCenter delegate must be
// configured on the main thread for reliable callback delivery.
void setupNotifications(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        notifDelegate = [[AulycMailNotificationDelegate alloc] init];
        center.delegate = notifDelegate;

        // Badge is required for the Dock unread-count badge to render — without
        // it macOS hides the "Badge app icon" setting and suppresses
        // [[NSApp dockTile] setBadgeLabel:].
        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge)
                              completionHandler:^(BOOL granted, NSError *error) {
            if (error != nil) {
                NSLog(@"[aulycMail] Notification authorization error: %@", error);
            } else {
                NSLog(@"[aulycMail] Notification authorization granted: %d", granted);
            }
            // The granted flag only means at least one requested capability was
            // accepted. Read badgeSetting explicitly, then notify Go so the
            // current unread count is replayed after authorization settles.
            publishNotificationSettings(center);
        }];
    });
}

// refreshNotificationSettings re-reads permissions after the app is activated,
// covering changes made in System Settings while the app remains running.
void refreshNotificationSettings(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        publishNotificationSettings([UNUserNotificationCenter currentNotificationCenter]);
    });
}

// showNotification creates and submits a notification asynchronously — never blocks.
void showNotification(const char *title, const char *body,
                      const char *accountId, const char *folderId, const char *threadId) {
    // Autorelease pool for the Go goroutine thread (which has no pool of its own).
    // The NSStrings are autoreleased by stringWithUTF8String:, and dispatch_async
    // retains them when copying the block to the heap, so they survive past the pool drain.
    @autoreleasepool {
    NSString *nsTitle     = [NSString stringWithUTF8String:title];
    NSString *nsBody      = [NSString stringWithUTF8String:body];
    NSString *nsAccountId = [NSString stringWithUTF8String:accountId];
    NSString *nsFolderId  = [NSString stringWithUTF8String:folderId];
    NSString *nsThreadId  = [NSString stringWithUTF8String:threadId];

    dispatch_async(dispatch_get_main_queue(), ^{
        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = nsTitle;
        content.body  = nsBody;
        content.sound = [UNNotificationSound defaultSound];
        content.userInfo = @{
            @"accountId": nsAccountId,
            @"folderId":  nsFolderId,
            @"threadId":  nsThreadId,
        };

        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                              content:content
                                                                              trigger:nil];

        [[UNUserNotificationCenter currentNotificationCenter]
            addNotificationRequest:request
             withCompletionHandler:^(NSError *error) {
                if (error != nil) {
                    NSLog(@"[aulycMail] Failed to deliver notification: %@", error);
                }
            }];
    });
    } // @autoreleasepool
}

// cancelNotifications removes all delivered notifications.
void cancelNotifications(void) {
    [[UNUserNotificationCenter currentNotificationCenter] removeAllDeliveredNotifications];
}
