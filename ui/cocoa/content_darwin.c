#import <AVFoundation/AVFoundation.h>
#import <AVKit/AVKit.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);
NSColor *gkCocoaSemanticColor(int color);

static NSString *gkString(const char *value) {
    return value ? [NSString stringWithUTF8String:value] : @"";
}

static void gkAdd(uintptr_t window, NSView *view) {
    [((NSWindow *)window).contentView addSubview:view];
}

/* ---- Icon ---- */

@interface GKCocoaIconView : NSImageView
@property NSSize preferred;
@end

@implementation GKCocoaIconView
- (NSSize)fittingSize {
    return self.preferred.width > 0 ? self.preferred : (self.image ? self.image.size : NSMakeSize(16, 16));
}
@end

void *gkCocoaIconCreate(uintptr_t window) {
    GKCocoaIconView *icon = [[GKCocoaIconView alloc] initWithFrame:NSZeroRect];
    icon.imageScaling = NSImageScaleProportionallyUpOrDown;
    gkAdd(window, icon);
    return icon;
}

void gkCocoaIconSet(void *pointer, void *image, int width, int height, int tint, const char *name) {
    GKCocoaIconView *icon = pointer;
    icon.image = (NSImage *)image;
    icon.preferred = NSMakeSize(width, height);
    icon.contentTintColor = gkCocoaSemanticColor(tint);
    NSString *label = gkString(name);
    icon.accessibilityElement = label.length > 0;
    icon.accessibilityLabel = label.length ? label : nil;
    icon.accessibilityRole = NSAccessibilityImageRole;
}

/* ---- Text documents ---- */

@interface GKCocoaDocumentView : NSScrollView <NSTextViewDelegate>
@property(retain) NSTextView *text;
@property BOOL markdown;
@end

@implementation GKCocoaDocumentView
- (void)dealloc {
    self.text.delegate = nil;
    [_text release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.text;
}
- (void)textDidChange:(NSNotification *)notification {
    (void)notification;
    gkCocoaSendView(self, GK_EVENT_TEXT_CHANGE, 0, 0, 0, 0, nil);
}
- (BOOL)textView:(NSTextView *)view clickedOnLink:(id)link atIndex:(NSUInteger)index {
    (void)view;
    (void)index;
    NSString *url = [link isKindOfClass:NSURL.class] ? [link absoluteString] : [link description];
    GKCocoaEvent event = {0};
    event.kind = GK_EVENT_LINK;
    event.text = url.UTF8String;
    uintptr_t handle = gkCocoaHandle(self);
    return handle && gkCocoaGoQuery(handle, &event);
}
@end

void *gkCocoaDocumentCreate(uintptr_t window, int editable, int markdown) {
    GKCocoaDocumentView *scroll = [[GKCocoaDocumentView alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)];
    scroll.markdown = markdown != 0;
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    scroll.borderType = markdown ? NSNoBorder : NSBezelBorder;
    scroll.drawsBackground = !markdown;
    NSTextView *text = [[[NSTextView alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)] autorelease];
    text.richText = YES;
    text.editable = editable != 0;
    text.selectable = YES;
    text.verticallyResizable = YES;
    text.horizontallyResizable = NO;
    text.autoresizingMask = NSViewWidthSizable;
    text.textContainer.widthTracksTextView = YES;
    text.textContainerInset = NSMakeSize(4, 6);
    text.drawsBackground = !markdown;
    text.delegate = scroll;
    text.allowsUndo = YES;
    scroll.text = text;
    scroll.documentView = text;
    gkAdd(window, scroll);
    return scroll;
}

char *gkCocoaDocumentText(void *pointer) {
    return strdup(((GKCocoaDocumentView *)pointer).text.string.UTF8String ?: "");
}

void gkCocoaDocumentSetText(void *pointer, const char *text) {
    GKCocoaDocumentView *view = pointer;
    view.text.string = gkString(text);
}

void gkCocoaDocumentSetEditable(void *pointer, int editable) {
    ((GKCocoaDocumentView *)pointer).text.editable = editable != 0;
}

int gkCocoaDocumentEditable(void *pointer) {
    return ((GKCocoaDocumentView *)pointer).text.editable;
}

unsigned char *gkCocoaDocumentRTF(void *pointer, size_t *size) {
    NSTextView *text = ((GKCocoaDocumentView *)pointer).text;
    NSData *data = [text RTFFromRange:NSMakeRange(0, text.string.length)];
    *size = data.length;
    if (!data.length) {
        return NULL;
    }
    unsigned char *result = malloc(data.length);
    memcpy(result, data.bytes, data.length);
    return result;
}

int gkCocoaDocumentSetRTF(void *pointer, const unsigned char *rtf, size_t size) {
    NSTextView *text = ((GKCocoaDocumentView *)pointer).text;
    NSData *data = [NSData dataWithBytes:rtf length:size];
    NSAttributedString *value = [[[NSAttributedString alloc] initWithRTF:data documentAttributes:nil] autorelease];
    if (!value) {
        return 0;
    }
    [text.textStorage setAttributedString:value];
    return 1;
}

/* gkCocoaMarkdownStyle applies fonts for headings, code, and quotes, which
   Foundation records as presentation intents but does not style. */
static NSAttributedString *gkCocoaStyleMarkdown(NSAttributedString *source) {
    NSMutableAttributedString *result = [[source mutableCopy] autorelease];
    NSFont *body = [NSFont preferredFontForTextStyle:NSFontTextStyleBody options:@{}];
    [result addAttribute:NSFontAttributeName value:body range:NSMakeRange(0, result.length)];
    [result addAttribute:NSForegroundColorAttributeName value:NSColor.labelColor range:NSMakeRange(0, result.length)];
    __block NSInteger previousIdentity = -1;
    [source enumerateAttribute:NSPresentationIntentAttributeName
                       inRange:NSMakeRange(0, source.length)
                       options:0
                    usingBlock:^(NSPresentationIntent *intent, NSRange range, BOOL *stop) {
                      (void)stop;
                      if (!intent) {
                          return;
                      }
                      NSFont *font = nil;
                      BOOL quote = NO;
                      for (NSPresentationIntent *current = intent; current; current = current.parentIntent) {
                          switch (current.intentKind) {
                          case NSPresentationIntentKindHeader: {
                              NSInteger level = current.headerLevel;
                              NSFontTextStyle style = level <= 1   ? NSFontTextStyleLargeTitle
                                                      : level == 2 ? NSFontTextStyleTitle1
                                                      : level == 3 ? NSFontTextStyleTitle2
                                                                   : NSFontTextStyleHeadline;
                              font = font ?: [NSFont preferredFontForTextStyle:style options:@{}];
                              break;
                          }
                          case NSPresentationIntentKindCodeBlock:
                              font = font
                                         ?: [NSFont monospacedSystemFontOfSize:NSFont.systemFontSize
                                                                        weight:NSFontWeightRegular];
                              break;
                          case NSPresentationIntentKindBlockQuote:
                              quote = YES;
                              break;
                          default:
                              break;
                          }
                      }
                      if (font) {
                          [result addAttribute:NSFontAttributeName value:font range:range];
                      }
                      if (quote) {
                          [result addAttribute:NSForegroundColorAttributeName
                                         value:NSColor.secondaryLabelColor
                                         range:range];
                      }
                      (void)previousIdentity;
                    }];
    /* Inline code. */
    [source enumerateAttribute:NSInlinePresentationIntentAttributeName
                       inRange:NSMakeRange(0, source.length)
                       options:0
                    usingBlock:^(NSNumber *value, NSRange range, BOOL *stop) {
                      (void)stop;
                      NSInlinePresentationIntent intent = value.unsignedIntegerValue;
                      NSFontManager *manager = NSFontManager.sharedFontManager;
                      NSFont *font = [result attribute:NSFontAttributeName atIndex:range.location effectiveRange:NULL];
                      if (intent & NSInlinePresentationIntentCode) {
                          font = [NSFont monospacedSystemFontOfSize:font.pointSize weight:NSFontWeightRegular];
                      }
                      if (intent & NSInlinePresentationIntentStronglyEmphasized) {
                          font = [manager convertFont:font toHaveTrait:NSBoldFontMask];
                      }
                      if (intent & NSInlinePresentationIntentEmphasized) {
                          font = [manager convertFont:font toHaveTrait:NSItalicFontMask];
                      }
                      if (font) {
                          [result addAttribute:NSFontAttributeName value:font range:range];
                      }
                      if (intent & NSInlinePresentationIntentStrikethrough) {
                          [result addAttribute:NSStrikethroughStyleAttributeName
                                         value:@(NSUnderlineStyleSingle)
                                         range:range];
                      }
                    }];
    /* Separate blocks, which the parser runs together, with line breaks. */
    __block NSInteger lastIdentity = -1;
    NSMutableArray<NSNumber *> *breaks = [NSMutableArray array];
    [source enumerateAttribute:NSPresentationIntentAttributeName
                       inRange:NSMakeRange(0, source.length)
                       options:0
                    usingBlock:^(NSPresentationIntent *intent, NSRange range, BOOL *stop) {
                      (void)stop;
                      NSInteger identity = intent ? intent.identity : -1;
                      if (lastIdentity != -1 && identity != lastIdentity && range.location > 0) {
                          [breaks addObject:@(range.location)];
                      }
                      lastIdentity = identity;
                    }];
    for (NSNumber *location in breaks.reverseObjectEnumerator) {
        [result insertAttributedString:[[[NSAttributedString alloc] initWithString:@"\n"] autorelease]
                               atIndex:location.unsignedIntegerValue];
    }
    return result;
}

int gkCocoaMarkdownSet(void *pointer, const char *source) {
    GKCocoaDocumentView *view = pointer;
    NSAttributedStringMarkdownParsingOptions *options = [[NSAttributedStringMarkdownParsingOptions new] autorelease];
    options.interpretedSyntax = NSAttributedStringMarkdownInterpretedSyntaxFull;
    options.failurePolicy = NSAttributedStringMarkdownParsingFailureReturnPartiallyParsedIfPossible;
    NSError *error = nil;
    NSAttributedString *parsed =
        [[[NSAttributedString alloc] initWithMarkdownString:gkString(source) options:options baseURL:nil error:&error]
            autorelease];
    if (!parsed) {
        return 0;
    }
    [view.text.textStorage setAttributedString:gkCocoaStyleMarkdown(parsed)];
    return 1;
}

/* ---- SVG ---- */

@interface GKCocoaVectorView : NSImageView
@end

@implementation GKCocoaVectorView
- (NSSize)fittingSize {
    return self.image ? self.image.size : NSZeroSize;
}
@end

void *gkCocoaVectorCreate(uintptr_t window) {
    GKCocoaVectorView *view = [[GKCocoaVectorView alloc] initWithFrame:NSZeroRect];
    view.imageScaling = NSImageScaleProportionallyUpOrDown;
    gkAdd(window, view);
    return view;
}

/* gkCocoaVectorSet loads SVG data; it fails when AppKit cannot parse it. */
int gkCocoaVectorSet(void *pointer, const unsigned char *data, size_t size, const char *label, double *width,
                     double *height) {
    GKCocoaVectorView *view = pointer;
    NSImage *image = [[[NSImage alloc] initWithData:[NSData dataWithBytes:data length:size]] autorelease];
    if (!image) {
        return 0;
    }
    view.image = image;
    *width = image.size.width;
    *height = image.size.height;
    NSString *text = gkString(label);
    view.accessibilityElement = text.length > 0;
    view.accessibilityLabel = text.length ? text : nil;
    view.accessibilityRole = NSAccessibilityImageRole;
    return 1;
}

/* ---- Animated image ---- */

@interface GKCocoaAnimationView : NSImageView
@property(retain) NSArray<NSImage *> *frames;
@property(retain) NSArray<NSNumber *> *durations;
@property(retain) NSTimer *timer;
@property NSUInteger index;
@property BOOL repeats;
@end

@implementation GKCocoaAnimationView
- (void)dealloc {
    [_timer invalidate];
    [_timer release];
    [_frames release];
    [_durations release];
    [super dealloc];
}
- (NSSize)fittingSize {
    return self.frames.firstObject ? self.frames.firstObject.size : NSZeroSize;
}
- (void)viewWillMoveToWindow:(NSWindow *)window {
    if (!window) {
        [self pause];
    }
    [super viewWillMoveToWindow:window];
}
- (void)schedule {
    [self.timer invalidate];
    if (!self.frames.count) {
        return;
    }
    double duration = self.durations[self.index].doubleValue;
    GKCocoaAnimationView *view = self;
    self.timer = [NSTimer scheduledTimerWithTimeInterval:duration
                                                 repeats:NO
                                                   block:^(NSTimer *timer) {
                                                     (void)timer;
                                                     [view advance];
                                                   }];
}
- (void)advance {
    NSUInteger next = self.index + 1;
    if (next >= self.frames.count) {
        if (!self.repeats) {
            [self pause];
            return;
        }
        next = 0;
    }
    self.index = next;
    self.image = self.frames[next];
    [self schedule];
}
- (void)play {
    if (!self.timer.isValid) {
        [self schedule];
    }
}
- (void)pause {
    [self.timer invalidate];
    self.timer = nil;
}
@end

void *gkCocoaAnimationCreate(uintptr_t window, void **images, const double *durations, int count, int repeats) {
    GKCocoaAnimationView *view = [[GKCocoaAnimationView alloc] initWithFrame:NSZeroRect];
    view.imageScaling = NSImageScaleProportionallyUpOrDown;
    NSMutableArray *frames = [NSMutableArray array];
    NSMutableArray *times = [NSMutableArray array];
    for (int i = 0; i < count; i++) {
        [frames addObject:(NSImage *)images[i]];
        [times addObject:@(durations[i])];
    }
    view.frames = frames;
    view.durations = times;
    view.repeats = repeats != 0;
    view.image = frames.firstObject;
    view.accessibilityRole = NSAccessibilityImageRole;
    gkAdd(window, view);
    return view;
}

void gkCocoaAnimationPlay(void *pointer, int play) {
    GKCocoaAnimationView *view = pointer;
    if (play) {
        [view play];
    } else {
        [view pause];
    }
}

int gkCocoaAnimationPlaying(void *pointer) {
    return ((GKCocoaAnimationView *)pointer).timer.isValid;
}

/* ---- Video ---- */

void *gkCocoaVideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted) {
    NSString *value = gkString(source);
    NSURL *url = nil;
    if ([value rangeOfString:@"://"].location != NSNotFound) {
        url = [NSURL URLWithString:value];
    } else {
        url = [NSURL fileURLWithPath:value.stringByStandardizingPath];
    }
    if (!url) {
        return NULL;
    }
    AVPlayerView *view = [[AVPlayerView alloc] initWithFrame:NSMakeRect(0, 0, 320, 180)];
    view.player = [AVPlayer playerWithURL:url];
    view.player.muted = muted != 0;
    view.controlsStyle = controls ? AVPlayerViewControlsStyleInline : AVPlayerViewControlsStyleNone;
    if (autoplay) {
        [view.player play];
    }
    gkAdd(window, view);
    return view;
}

void gkCocoaVideoPlay(void *pointer, int play) {
    AVPlayer *player = ((AVPlayerView *)pointer).player;
    if (play) {
        [player play];
    } else {
        [player pause];
    }
}

int gkCocoaVideoPlaying(void *pointer) {
    return ((AVPlayerView *)pointer).player.rate != 0;
}

double gkCocoaVideoPosition(void *pointer) {
    double seconds = CMTimeGetSeconds(((AVPlayerView *)pointer).player.currentTime);
    return isfinite(seconds) ? seconds : 0;
}

double gkCocoaVideoDuration(void *pointer) {
    AVPlayerItem *item = ((AVPlayerView *)pointer).player.currentItem;
    double seconds = item ? CMTimeGetSeconds(item.duration) : 0;
    return isfinite(seconds) ? seconds : 0;
}

void gkCocoaVideoSeek(void *pointer, double seconds) {
    [((AVPlayerView *)pointer).player seekToTime:CMTimeMakeWithSeconds(seconds, 600)
                                 toleranceBefore:kCMTimeZero
                                  toleranceAfter:kCMTimeZero];
}

void gkCocoaVideoRelease(void *pointer) {
    AVPlayerView *view = pointer;
    [view.player pause];
    view.player = nil;
}

/* ---- Web view ---- */

@interface GKCocoaWebView : WKWebView <WKNavigationDelegate>
@end

@implementation GKCocoaWebView
- (void)webView:(WKWebView *)view didFinishNavigation:(WKNavigation *)navigation {
    (void)navigation;
    gkCocoaSendView(self, GK_EVENT_NAVIGATE, 0, 0, 0, 0, view.URL.absoluteString ?: @"");
}
@end

void *gkCocoaWebViewCreate(uintptr_t window, int scripts) {
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration new] autorelease];
    configuration.defaultWebpagePreferences.allowsContentJavaScript = scripts != 0;
    GKCocoaWebView *view =
        [[GKCocoaWebView alloc] initWithFrame:NSMakeRect(0, 0, 320, 240) configuration:configuration];
    view.navigationDelegate = view;
    gkAdd(window, view);
    return view;
}

int gkCocoaWebViewLoad(void *pointer, const char *url) {
    NSURL *value = [NSURL URLWithString:gkString(url)];
    if (!value || !value.scheme) {
        return 0;
    }
    [(WKWebView *)pointer loadRequest:[NSURLRequest requestWithURL:value]];
    return 1;
}

void gkCocoaWebViewLoadHTML(void *pointer, const char *html) {
    [(WKWebView *)pointer loadHTMLString:gkString(html) baseURL:nil];
}

char *gkCocoaWebViewURL(void *pointer) {
    return strdup(((WKWebView *)pointer).URL.absoluteString.UTF8String ?: "");
}

void gkCocoaWebViewRelease(void *pointer) {
    GKCocoaWebView *view = pointer;
    view.navigationDelegate = nil;
    [view stopLoading];
}
