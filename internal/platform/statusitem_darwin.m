#import <Cocoa/Cocoa.h>

// Implemented in Go (statusitem_darwin.go), exported via cgo.
extern void goStatusItemAction(char* action);

typedef struct {
    const char* open;
    const char* settings;
    const char* checkUpdate;
    const char* quit;
} AulycStatusItemLabels;

@interface AulycStatusItemTarget : NSObject
@end

@implementation AulycStatusItemTarget
- (void)onOpen:(id)sender     { (void)sender; goStatusItemAction("show"); }
- (void)onSettings:(id)sender { (void)sender; goStatusItemAction("settings"); }
- (void)onCheckUpdate:(id)sender { (void)sender; goStatusItemAction("checkUpdate"); }
- (void)onQuit:(id)sender     { (void)sender; [NSApp terminate:nil]; }
@end

static NSStatusItem *gStatusItem = nil;
static AulycStatusItemTarget *gStatusItemTarget = nil;
static NSString *gStatusUnreadText = nil;

static NSString* aulycStatusStr(const char* s) {
    return s ? [NSString stringWithUTF8String:s] : @"";
}

static NSMenuItem* aulycStatusItem(NSString* title, SEL action, id target, NSString* keyEquivalent) {
    NSMenuItem* item = [[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:keyEquivalent] autorelease];
    item.target = target;
    [item setImage:nil];
    [item setOnStateImage:nil];
    [item setOffStateImage:nil];
    [item setMixedStateImage:nil];
    return item;
}

static NSImage* aulycStatusMenuIcon(NSString* systemName) {
    if (@available(macOS 11.0, *)) {
        NSImageSymbolConfiguration *configuration = [NSImageSymbolConfiguration configurationWithPointSize:14 weight:NSFontWeightRegular];
        NSImage *image = [[NSImage imageWithSystemSymbolName:systemName accessibilityDescription:nil] imageWithSymbolConfiguration:configuration];
        [image setTemplate:YES];
        return image;
    }
    return nil;
}

static NSMenuItem* aulycStatusItemWithIcon(NSString* title, SEL action, id target, NSString* keyEquivalent, NSString* systemName) {
    NSMenuItem *item = aulycStatusItem(title, action, target, keyEquivalent);
    item.image = aulycStatusMenuIcon(systemName);
    return item;
}

static NSImage* aulycFallbackStatusMailImage(void) {
    NSImage *image = [[[NSImage alloc] initWithSize:NSMakeSize(20, 20)] autorelease];
    [image lockFocus];

    [[NSColor blackColor] setStroke];
    NSBezierPath *outline = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(2.5, 5.0, 15.0, 10.0) xRadius:2.0 yRadius:2.0];
    [outline setLineWidth:2.0];
    [outline stroke];

    NSBezierPath *flap = [NSBezierPath bezierPath];
    [flap moveToPoint:NSMakePoint(3.2, 14.2)];
    [flap lineToPoint:NSMakePoint(10.0, 9.0)];
    [flap lineToPoint:NSMakePoint(16.8, 14.2)];
    [flap setLineWidth:2.0];
    [flap stroke];

    [image unlockFocus];
    [image setTemplate:YES];
    return image;
}

static NSImage* aulycStatusAppIconImage(void) {
    static NSImage *cachedImage = nil;
    if (cachedImage != nil) {
        return cachedImage;
    }

    NSString *path = [[NSBundle mainBundle] pathForResource:@"MenuBarIcon" ofType:@"png"];
    if (path != nil) {
        NSImage *image = [[[NSImage alloc] initWithContentsOfFile:path] autorelease];
        if (image != nil) {
            [image setSize:NSMakeSize(21, 21)];
            [image setTemplate:YES];
            cachedImage = [image retain];
            return cachedImage;
        }
    }

    return aulycFallbackStatusMailImage();
}

static void aulycApplyStatusItemAppearance(void) {
    if (gStatusItem == nil) {
        return;
    }
    NSStatusBarButton *button = gStatusItem.button;
    if (button == nil) {
        return;
    }
    button.image = aulycStatusAppIconImage();
    button.imageScaling = NSImageScaleProportionallyUpOrDown;
    button.imagePosition = NSImageLeft;
    button.title = gStatusUnreadText ?: @"";
    button.font = [NSFont menuBarFontOfSize:0];
    button.toolTip = @"aulycMail";
}

void aulycSetStatusItemVisible(int visible, AulycStatusItemLabels labels) {
    NSString* open = aulycStatusStr(labels.open);
    NSString* settings = aulycStatusStr(labels.settings);
    NSString* checkUpdate = aulycStatusStr(labels.checkUpdate);
    NSString* quit = aulycStatusStr(labels.quit);

    dispatch_async(dispatch_get_main_queue(), ^{
        if (!visible) {
            if (gStatusItem != nil) {
                [[NSStatusBar systemStatusBar] removeStatusItem:gStatusItem];
                [gStatusItem release];
                gStatusItem = nil;
            }
            return;
        }

        if (gStatusItemTarget == nil) {
            gStatusItemTarget = [[AulycStatusItemTarget alloc] init];
        }
        if (gStatusItem == nil) {
            gStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
        }

        NSMenu *menu = [[[NSMenu alloc] init] autorelease];
        [menu addItem:aulycStatusItemWithIcon(open, @selector(onOpen:), gStatusItemTarget, @"", @"envelope.open")];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItem:aulycStatusItemWithIcon(settings, @selector(onSettings:), gStatusItemTarget, @"", @"gearshape")];
        [menu addItem:aulycStatusItemWithIcon(checkUpdate, @selector(onCheckUpdate:), gStatusItemTarget, @"", @"arrow.triangle.2.circlepath")];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItem:aulycStatusItemWithIcon(quit, @selector(onQuit:), gStatusItemTarget, @"q", @"power")];
        gStatusItem.menu = menu;

        aulycApplyStatusItemAppearance();
    });
}

void aulycSetStatusItemUnreadLabel(const char *label) {
    NSString *text = (label && label[0]) ? [NSString stringWithUTF8String:label] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        [gStatusUnreadText release];
        gStatusUnreadText = [text copy];
        aulycApplyStatusItemAppearance();
    });
}
