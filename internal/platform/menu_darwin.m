#import <Cocoa/Cocoa.h>

// Implemented in Go (menu_darwin.go), exported via cgo.
extern void goMenuAction(char* action);

typedef struct {
    const char* settings;
    const char* about;
    const char* quit;
    const char* edit;
    const char* undo;
    const char* redo;
    const char* cut;
    const char* copy;
    const char* paste;
    const char* deleteItem;
} AulycMenuLabels;

// Target for the custom App-menu items (Settings / About). Routes clicks into
// Go via goMenuAction. Retained for the app's lifetime in a static.
@interface AulycMenuTarget : NSObject
@end

@implementation AulycMenuTarget
- (void)onSettings:(id)sender { (void)sender; goMenuAction("settings"); }
- (void)onAbout:(id)sender    { (void)sender; goMenuAction("about"); }
@end

static AulycMenuTarget *gMenuTarget = nil;

static NSString* aulycStr(const char* s) {
    return s ? [NSString stringWithUTF8String:s] : @"";
}

static NSMenuItem* aulycItem(NSString* title, SEL action, id target, NSString* key, NSUInteger mods) {
    NSMenuItem* it = [[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key] autorelease];
    if (target != nil) {
        it.target = target;
    }
    if (mods != 0) {
        it.keyEquivalentModifierMask = mods;
    }
    return it;
}

void installAppMenu(AulycMenuLabels labels) {
    // Copy the C strings into ObjC strings now; the C strings are freed by Go as
    // soon as this function returns, before the dispatched block runs. The block
    // copy (via dispatch_async) retains these captured NSStrings.
    NSString* settings = aulycStr(labels.settings);
    NSString* about    = aulycStr(labels.about);
    NSString* quit     = aulycStr(labels.quit);
    NSString* edit     = aulycStr(labels.edit);
    NSString* undo     = aulycStr(labels.undo);
    NSString* redo     = aulycStr(labels.redo);
    NSString* cut      = aulycStr(labels.cut);
    NSString* copy     = aulycStr(labels.copy);
    NSString* paste    = aulycStr(labels.paste);
    NSString* del      = aulycStr(labels.deleteItem);

    // Build + install on the main thread, after Wails' default menu is set, so
    // this replaces it.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gMenuTarget == nil) {
            gMenuTarget = [[AulycMenuTarget alloc] init];
        }

        NSMenu* mainMenu = [[[NSMenu alloc] init] autorelease];

        // --- App menu (macOS shows the bundle name as its bold title) ---
        NSMenuItem* appItem = [[[NSMenuItem alloc] init] autorelease];
        [mainMenu addItem:appItem];
        NSMenu* appMenu = [[[NSMenu alloc] init] autorelease];
        [appItem setSubmenu:appMenu];

        [appMenu addItem:aulycItem(settings, @selector(onSettings:), gMenuTarget, @",", NSEventModifierFlagCommand)];
        [appMenu addItem:aulycItem(about, @selector(onAbout:), gMenuTarget, @"", 0)];
        [appMenu addItem:[NSMenuItem separatorItem]];
        // Quit routes through -[NSApplication terminate:] (target nil → responder
        // chain → NSApp), which the terminate hook flags as a genuine quit.
        [appMenu addItem:aulycItem(quit, @selector(terminate:), nil, @"q", NSEventModifierFlagCommand)];

        // --- Edit menu (native selectors so copy/paste/undo work in the webview) ---
        NSMenuItem* editItem = [[[NSMenuItem alloc] init] autorelease];
        [mainMenu addItem:editItem];
        NSMenu* editMenu = [[[NSMenu alloc] initWithTitle:edit] autorelease];
        [editItem setSubmenu:editMenu];

        [editMenu addItem:aulycItem(undo, @selector(undo:), nil, @"z", NSEventModifierFlagCommand)];
        [editMenu addItem:aulycItem(redo, @selector(redo:), nil, @"z", NSEventModifierFlagShift | NSEventModifierFlagCommand)];
        [editMenu addItem:[NSMenuItem separatorItem]];
        [editMenu addItem:aulycItem(cut, @selector(cut:), nil, @"x", NSEventModifierFlagCommand)];
        [editMenu addItem:aulycItem(copy, @selector(copy:), nil, @"c", NSEventModifierFlagCommand)];
        [editMenu addItem:aulycItem(paste, @selector(paste:), nil, @"v", NSEventModifierFlagCommand)];
        // Delete has no key equivalent so it doesn't capture the plain Backspace
        // key from text fields; it acts on the selection when chosen.
        [editMenu addItem:aulycItem(del, @selector(delete:), nil, @"", 0)];

        [NSApp setMainMenu:mainMenu];
    });
}
