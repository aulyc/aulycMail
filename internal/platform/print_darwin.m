#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// Prints HTML via an offscreen WKWebView + the native print panel. window.print()
// does nothing in WKWebView, so the JS side hands the rendered HTML to Go which
// calls printHTML here.

@interface AulycPrinter : NSObject <WKNavigationDelegate>
@property (nonatomic, retain) WKWebView* webView;
@property (nonatomic, copy) NSString* jobTitle;
@end

// Keeps printers alive across the async load→print cycle (non-ARC). Each printer
// is removed (and released) in -cleanup once printing finishes or fails.
static NSMutableArray* gPrinters = nil;

@implementation AulycPrinter

- (void)cleanup {
    if (self.webView != nil) {
        self.webView.navigationDelegate = nil;
        self.webView = nil;
    }
    [gPrinters removeObject:self];
}

- (void)doPrint {
    WKWebView* wv = self.webView;
    if (wv == nil) {
        [self cleanup];
        return;
    }

    if (@available(macOS 11.0, *)) {
        NSPrintInfo* info = [[[NSPrintInfo alloc] initWithDictionary:[[NSPrintInfo sharedPrintInfo] dictionary]] autorelease];
        [info setTopMargin:36];
        [info setBottomMargin:36];
        [info setLeftMargin:36];
        [info setRightMargin:36];

        NSPrintOperation* op = [wv printOperationWithPrintInfo:info];
        [op setShowsPrintPanel:YES];
        [op setShowsProgressPanel:YES];
        if ([self.jobTitle length] > 0) {
            [op setJobTitle:self.jobTitle];
        }

        NSWindow* win = [NSApp mainWindow];
        if (win == nil) {
            win = [[NSApp windows] firstObject];
        }
        if (win != nil) {
            [op runOperationModalForWindow:win
                                  delegate:self
                            didRunSelector:@selector(printOperationDidRun:success:contextInfo:)
                               contextInfo:NULL];
        } else {
            [op runOperation];
            [self cleanup];
        }
    } else {
        [self cleanup];
    }
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    (void)webView; (void)navigation;
    // Let layout settle before printing.
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
        [self doPrint];
    });
}

- (void)webView:(WKWebView *)webView didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    (void)webView; (void)navigation; (void)error;
    [self cleanup];
}

- (void)printOperationDidRun:(NSPrintOperation *)printOperation success:(BOOL)success contextInfo:(void *)contextInfo {
    (void)printOperation; (void)success; (void)contextInfo;
    [self cleanup];
}

@end

void printHTML(const char* html, const char* jobTitle) {
    NSString* h = html ? [NSString stringWithUTF8String:html] : @"";
    NSString* jt = jobTitle ? [NSString stringWithUTF8String:jobTitle] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gPrinters == nil) {
            gPrinters = [[NSMutableArray alloc] init];
        }
        AulycPrinter* printer = [[[AulycPrinter alloc] init] autorelease];
        printer.jobTitle = jt;

        WKWebViewConfiguration* cfg = [[[WKWebViewConfiguration alloc] init] autorelease];
        WKWebView* wv = [[[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 612, 792)
                                            configuration:cfg] autorelease];
        wv.navigationDelegate = printer;
        printer.webView = wv;
        [gPrinters addObject:printer]; // retain until the print cycle ends

        [wv loadHTMLString:h baseURL:nil];
    });
}
