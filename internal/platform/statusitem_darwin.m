#import <Cocoa/Cocoa.h>

// Implemented in Go (statusitem_darwin.go), exported via cgo.
extern void goStatusItemAction(char* action);

typedef struct {
    const char* open;
    const char* settings;
    const char* quit;
} AulycStatusItemLabels;

@interface AulycStatusItemTarget : NSObject
@end

@implementation AulycStatusItemTarget
- (void)onOpen:(id)sender     { (void)sender; goStatusItemAction("show"); }
- (void)onSettings:(id)sender { (void)sender; goStatusItemAction("settings"); }
- (void)onQuit:(id)sender     { (void)sender; [NSApp terminate:nil]; }
@end

static NSStatusItem *gStatusItem = nil;
static AulycStatusItemTarget *gStatusItemTarget = nil;
static NSString *gStatusUnreadText = nil;

static NSString* aulycStatusStr(const char* s) {
    return s ? [NSString stringWithUTF8String:s] : @"";
}

static NSMenuItem* aulycStatusItem(NSString* title, SEL action, id target) {
    NSMenuItem* item = [[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:@""] autorelease];
    item.target = target;
    [item setImage:nil];
    [item setOnStateImage:nil];
    [item setOffStateImage:nil];
    [item setMixedStateImage:nil];
    return item;
}

static NSImage* aulycStatusMailImage(void) {
    NSImage *image = [[[NSImage alloc] initWithSize:NSMakeSize(18, 18)] autorelease];
    [image lockFocus];

    [[NSColor blackColor] setStroke];
    NSBezierPath *outline = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(3.0, 5.0, 12.0, 8.5) xRadius:1.5 yRadius:1.5];
    [outline setLineWidth:1.6];
    [outline stroke];

    NSBezierPath *flap = [NSBezierPath bezierPath];
    [flap moveToPoint:NSMakePoint(3.5, 13.0)];
    [flap lineToPoint:NSMakePoint(9.0, 8.5)];
    [flap lineToPoint:NSMakePoint(14.5, 13.0)];
    [flap setLineWidth:1.6];
    [flap stroke];

    [image unlockFocus];
    [image setTemplate:YES];
    return image;
}

static void aulycApplyStatusItemAppearance(void) {
    if (gStatusItem == nil) {
        return;
    }
    NSStatusBarButton *button = gStatusItem.button;
    if (button == nil) {
        return;
    }
    button.image = aulycStatusMailImage();
    button.imagePosition = NSImageLeft;
    button.title = gStatusUnreadText ?: @"";
    button.font = [NSFont menuBarFontOfSize:0];
    button.toolTip = @"aulycmail";
}

void aulycSetStatusItemVisible(int visible, AulycStatusItemLabels labels) {
    NSString* open = aulycStatusStr(labels.open);
    NSString* settings = aulycStatusStr(labels.settings);
    NSString* quit = aulycStatusStr(labels.quit);

    dispatch_async(dispatch_get_main_queue(), ^{
        if (!visible) {
            if (gStatusItem != nil) {
                [[NSStatusBar systemStatusBar] removeStatusItem:gStatusItem];
                gStatusItem = nil;
            }
            return;
        }

        if (gStatusItemTarget == nil) {
            gStatusItemTarget = [[AulycStatusItemTarget alloc] init];
        }
        if (gStatusItem == nil) {
            gStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
        }

        NSMenu *menu = [[[NSMenu alloc] init] autorelease];
        [menu addItem:aulycStatusItem(open, @selector(onOpen:), gStatusItemTarget)];
        [menu addItem:aulycStatusItem(settings, @selector(onSettings:), gStatusItemTarget)];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItem:aulycStatusItem(quit, @selector(onQuit:), gStatusItemTarget)];
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
