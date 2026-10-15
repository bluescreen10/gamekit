#import <Cocoa/Cocoa.h>
#import <GameController/GameController.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"

void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text);

/* ---- Window state ---- */

/* GKCocoaWindowState holds what a window restores when it leaves the
   maximized state, and the fullscreen request it has yet to apply. */
@interface GKCocoaWindowState : NSObject
@property BOOL maximized;
@property NSRect normalFrame;
@property BOOL acceptsFiles;
@property BOOL resizable;
@property BOOL borderless;
/* pending is set when a fullscreen request waits for the window to be shown
   or for a transition to finish; target is the requested state. */
@property BOOL pending;
@property BOOL target;
@property BOOL transitioning;
/* fullscreen is set when a transition into fullscreen completes, so it
   reports the finished state rather than one in progress. */
@property BOOL fullscreen;
/* windowedSaved is set while savedMaxSize and restoreBorderless hold the
   windowed state that fullscreen replaced. */
@property BOOL windowedSaved;
@property NSSize savedMaxSize;
@property BOOL restoreBorderless;
@end

@implementation GKCocoaWindowState
@end

static char gkCocoaWindowStateKey;

static GKCocoaWindowState *gkCocoaState(NSWindow *window) {
    GKCocoaWindowState *state = objc_getAssociatedObject(window, &gkCocoaWindowStateKey);
    if (!state) {
        state = [[GKCocoaWindowState new] autorelease];
        state.resizable = (window.styleMask & NSWindowStyleMaskResizable) != 0;
        state.borderless = (window.styleMask & NSWindowStyleMaskTitled) == 0;
        objc_setAssociatedObject(window, &gkCocoaWindowStateKey, state, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
    return state;
}

void gkCocoaWindowAttach(void *pointer, uintptr_t handle) {
    NSWindow *window = pointer;
    if (!window) {
        return;
    }
    gkCocoaSetHandle(window, handle);
    gkCocoaState(window);
}

void gkCocoaWindowDetach(void *pointer) {
    NSWindow *window = pointer;
    if (window) {
        gkCocoaSetHandle(window, 0);
    }
}

/* gkCocoaWindowNotify reports a window event to the window's Go value. */
void gkCocoaWindowNotify(void *pointer, int kind, int a, double x, double y, const char *text) {
    NSWindow *window = pointer;
    uintptr_t handle = gkCocoaHandle(window);
    if (!handle) {
        return;
    }
    GKCocoaEvent event = {0};
    event.kind = kind;
    event.a = a;
    event.x = x;
    event.y = y;
    event.text = text;
    gkCocoaGoEvent(handle, &event);
}

int gkCocoaWindowAllowClose(void *pointer) {
    NSWindow *window = pointer;
    uintptr_t handle = gkCocoaHandle(window);
    if (!handle) {
        return 1;
    }
    GKCocoaEvent event = {0};
    event.kind = GK_EVENT_WINDOW_CLOSE_REQUEST;
    return gkCocoaGoQuery(handle, &event);
}

void gkCocoaWindowSetSizeLimits(void *pointer, int minWidth, int minHeight, int maxWidth, int maxHeight) {
    NSWindow *window = pointer;
    NSSize maximum = NSMakeSize(maxWidth > 0 ? maxWidth : CGFLOAT_MAX, maxHeight > 0 ? maxHeight : CGFLOAT_MAX);
    window.contentMinSize = NSMakeSize(MAX(1, minWidth), MAX(1, minHeight));
    if (gkCocoaState(window).windowedSaved) {
        /* Applied when the window leaves fullscreen. */
        gkCocoaState(window).savedMaxSize = maximum;
        return;
    }
    window.contentMaxSize = maximum;
    NSRect content = [window contentRectForFrameRect:window.frame];
    NSSize size = content.size;
    size.width = MAX(size.width, window.contentMinSize.width);
    size.height = MAX(size.height, window.contentMinSize.height);
    if (maxWidth > 0) {
        size.width = MIN(size.width, maxWidth);
    }
    if (maxHeight > 0) {
        size.height = MIN(size.height, maxHeight);
    }
    if (!NSEqualSizes(size, content.size)) {
        [window setContentSize:size];
    }
}

static NSWindowStyleMask gkCocoaStyleMask(NSWindow *window, BOOL resizable, BOOL borderless) {
    NSWindowStyleMask style = window.styleMask & NSWindowStyleMaskUtilityWindow;
    BOOL utility = [window isKindOfClass:NSPanel.class];
    if (borderless) {
        style = NSWindowStyleMaskBorderless;
    } else {
        style |= NSWindowStyleMaskTitled | NSWindowStyleMaskClosable;
        if (!utility) {
            style |= NSWindowStyleMaskMiniaturizable;
        }
    }
    if (resizable) {
        style |= NSWindowStyleMaskResizable;
    }
    return style;
}

/* gkCocoaInFullscreenSpace reports whether AppKit has the window in, or
   moving into, its fullscreen Space. */
static BOOL gkCocoaInFullscreenSpace(NSWindow *window) {
    return (window.styleMask & NSWindowStyleMaskFullScreen) != 0;
}

/* gkCocoaIsFullscreen reports a completed fullscreen transition. */
static BOOL gkCocoaIsFullscreen(NSWindow *window) {
    return gkCocoaState(window).fullscreen;
}

void gkCocoaWindowSetStyle(void *pointer, int resizable, int borderless) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    state.resizable = resizable != 0;
    state.borderless = borderless != 0;
    if (gkCocoaIsFullscreen(window) || state.transitioning) {
        /* Applied when the window leaves fullscreen. */
        return;
    }
    NSRect content = [window contentRectForFrameRect:window.frame];
    window.styleMask = gkCocoaStyleMask(window, resizable, borderless);
    [window setFrame:[window frameRectForContentRect:content] display:YES];
}

void gkCocoaWindowSetAlwaysOnTop(void *pointer, int alwaysOnTop) {
    NSWindow *window = pointer;
    window.level = alwaysOnTop ? NSFloatingWindowLevel : NSNormalWindowLevel;
}

/* gkCocoaWait runs the event loop until done returns YES or a second passes.
   AppKit completes minimizing and restoring asynchronously. */
static void gkCocoaWait(BOOL (^done)(void)) {
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:1.0];
    while (!done() && [deadline timeIntervalSinceNow] > 0) {
        @autoreleasepool {
            NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                                untilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]
                                                   inMode:NSDefaultRunLoopMode
                                                  dequeue:YES];
            if (event) {
                [NSApp sendEvent:event];
            }
        }
    }
}

int gkCocoaWindowIsMinimized(void *pointer) {
    return ((NSWindow *)pointer).isMiniaturized;
}

int gkCocoaWindowMinimize(void *pointer) {
    NSWindow *window = pointer;
    if (window.isMiniaturized) {
        return 1;
    }
    if (gkCocoaIsFullscreen(window)) {
        /* macOS does not minimize fullscreen windows. */
        return 0;
    }
    if (!(window.styleMask & NSWindowStyleMaskMiniaturizable)) {
        /* Borderless and utility windows have no minimize control, but
           -miniaturize: still works on them once the flag is present. */
        window.styleMask |= NSWindowStyleMaskMiniaturizable;
    }
    [window miniaturize:nil];
    gkCocoaWait(^BOOL {
      return window.isMiniaturized;
    });
    return window.isMiniaturized;
}

static NSScreen *gkCocoaScreen(NSWindow *window) {
    return window.screen ?: NSScreen.mainScreen ?: NSScreen.screens.firstObject;
}

static void gkCocoaDeminiaturize(NSWindow *window) {
    if (!window.isMiniaturized) {
        return;
    }
    [window deminiaturize:nil];
    gkCocoaWait(^BOOL {
      return !window.isMiniaturized;
    });
}

int gkCocoaWindowIsMaximized(void *pointer) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    if (gkCocoaIsFullscreen(window) || window.isMiniaturized) {
        return 0;
    }
    if (state.maximized) {
        return 1;
    }
    return window.isVisible && (window.styleMask & NSWindowStyleMaskTitled) && window.isZoomed;
}

int gkCocoaWindowMaximize(void *pointer) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    if (gkCocoaIsFullscreen(window) || state.transitioning) {
        return 0;
    }
    gkCocoaDeminiaturize(window);
    if (state.maximized) {
        return 1;
    }
    state.normalFrame = window.frame;
    state.maximized = YES;
    [window setFrame:gkCocoaScreen(window).visibleFrame display:YES animate:NO];
    return 1;
}

/* ---- Fullscreen ----

   Fullscreen is macOS's native fullscreen: the window moves to its own Space,
   which enables Game Mode, keeps content clear of the camera housing, and
   hides the menu bar until the pointer reaches the top of the screen. AppKit
   animates the transition, so it completes asynchronously and reports through
   the window delegate. A window must be visible to enter fullscreen; a request
   for a hidden window is applied when the window is shown. */

int gkCocoaWindowIsFullscreen(void *pointer) {
    return gkCocoaIsFullscreen((NSWindow *)pointer);
}

static void gkCocoaToggleFullscreen(NSWindow *window) {
    GKCocoaWindowState *state = gkCocoaState(window);
    BOOL entering = !gkCocoaInFullscreenSpace(window);
    window.collectionBehavior = (window.collectionBehavior & ~(NSWindowCollectionBehaviorFullScreenAuxiliary |
                                                               NSWindowCollectionBehaviorFullScreenNone)) |
                                NSWindowCollectionBehaviorFullScreenPrimary;
    if (entering) {
        if (state.maximized) {
            [window setFrame:state.normalFrame display:NO];
            state.maximized = NO;
        }
        /* A maximum size smaller than the screen would stop the window from
           filling it, and macOS only takes titled windows into fullscreen.
           A failed transition can leave the windowed state saved; keep it. */
        if (!state.windowedSaved) {
            state.windowedSaved = YES;
            state.savedMaxSize = window.contentMaxSize;
            state.restoreBorderless = (window.styleMask & NSWindowStyleMaskTitled) == 0;
        }
        window.contentMaxSize = NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX);
        if ((window.styleMask & NSWindowStyleMaskTitled) == 0) {
            window.styleMask = gkCocoaStyleMask(window, YES, NO);
        }
        /* macOS refuses fullscreen to a window of an inactive application,
           and a fullscreen game needs keyboard focus. */
        [NSApp activateIgnoringOtherApps:YES];
        [window makeKeyAndOrderFront:nil];
    }
    state.transitioning = YES;
    [window toggleFullScreen:nil];
}

/* gkCocoaApplyFullscreen starts a pending request when the window can take it. */
static void gkCocoaApplyFullscreen(NSWindow *window) {
    GKCocoaWindowState *state = gkCocoaState(window);
    if (!state.pending || state.transitioning || !window.isVisible) {
        return;
    }
    state.pending = NO;
    if (state.target != gkCocoaInFullscreenSpace(window)) {
        gkCocoaToggleFullscreen(window);
    }
}

/* gkCocoaDisplayAvailable reports whether macOS can show a fullscreen Space:
   not while the session is locked or the main display is asleep. */
static BOOL gkCocoaDisplayAvailable(void) {
    BOOL available = !CGDisplayIsAsleep(CGMainDisplayID());
    CFDictionaryRef session = CGSessionCopyCurrentDictionary();
    if (session) {
        CFBooleanRef locked = CFDictionaryGetValue(session, CFSTR("CGSSessionScreenIsLocked"));
        if (locked && CFGetTypeID(locked) == CFBooleanGetTypeID() && CFBooleanGetValue(locked)) {
            available = NO;
        }
        CFRelease(session);
    }
    return available;
}

/* gkCocoaWindowSetFullscreen returns 0 when a visible window cannot change
   fullscreen state now; a hidden window keeps the request until shown. */
int gkCocoaWindowSetFullscreen(void *pointer, int fullscreen) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    if ((window.isVisible || window.isMiniaturized) && (fullscreen != 0) != gkCocoaInFullscreenSpace(window) &&
        !gkCocoaDisplayAvailable()) {
        return 0;
    }
    gkCocoaDeminiaturize(window);
    state.target = fullscreen != 0;
    state.pending = YES;
    gkCocoaApplyFullscreen(window);
    return 1;
}

/* gkCocoaWindowDidShow applies a fullscreen request made while hidden. */
void gkCocoaWindowDidShow(void *pointer) {
    NSWindow *window = pointer;
    if (gkCocoaState(window).pending) {
        /* Let the window finish ordering in before the transition starts. */
        dispatch_async(dispatch_get_main_queue(), ^{
          gkCocoaApplyFullscreen(window);
        });
    }
}

/* Fullscreen delegate phases. */
enum {
    GK_FULLSCREEN_WILL_ENTER,
    GK_FULLSCREEN_DID_ENTER,
    GK_FULLSCREEN_WILL_EXIT,
    GK_FULLSCREEN_DID_EXIT,
    GK_FULLSCREEN_FAILED_ENTER,
    GK_FULLSCREEN_FAILED_EXIT,
};

/* gkCocoaRestoreWindowed restores the state that fullscreen replaced. */
static void gkCocoaRestoreWindowed(NSWindow *window) {
    GKCocoaWindowState *state = gkCocoaState(window);
    if (!state.windowedSaved || state.transitioning || state.fullscreen) {
        return;
    }
    state.windowedSaved = NO;
    window.contentMaxSize = state.savedMaxSize.width > 0 ? state.savedMaxSize : NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX);
    NSRect content = [window contentRectForFrameRect:window.frame];
    window.styleMask = gkCocoaStyleMask(window, state.resizable, state.borderless);
    if (state.restoreBorderless || state.borderless) {
        [window setFrame:[window frameRectForContentRect:content] display:YES];
    }
    state.restoreBorderless = NO;
}

/* gkCocoaFinishWindowedRetries limits the wait for AppKit to clear
   NSWindowStyleMaskFullScreen, in steps of 50 ms. */
enum { gkCocoaFinishWindowedRetries = 100 };

static void gkCocoaFinishWindowed(NSWindow *window, int retries);

/* gkCocoaScheduleFinishWindowed runs gkCocoaFinishWindowed on a later
   main-loop iteration. The block retains the window. */
static void gkCocoaScheduleFinishWindowed(NSWindow *window, int64_t delay, int retries) {
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, delay), dispatch_get_main_queue(), ^{
      gkCocoaFinishWindowed(window, retries);
    });
}

/* gkCocoaFinishWindowed completes a fullscreen exit or a failed enter: it
   restores the windowed state and starts a request that came during the
   transition. It must not run inside windowDidExitFullScreen. AppKit
   dismantles its transition controller after that method returns, and a
   styleMask change before then raises NSInternalInconsistencyException:
   "Must have content controller". The window stays transitioning until this
   function runs, so no new transition or style change starts before it. */
static void gkCocoaFinishWindowed(NSWindow *window, int retries) {
    GKCocoaWindowState *state = gkCocoaState(window);
    if (gkCocoaInFullscreenSpace(window) && retries > 0) {
        /* After a failed transition, AppKit clears NSWindowStyleMaskFullScreen
           later. A styleMask change before that raises NSGenericException:
           "NSWindowStyleMaskFullScreen cleared on a window outside of a full
           screen transition". */
        gkCocoaScheduleFinishWindowed(window, 50 * NSEC_PER_MSEC, retries - 1);
        return;
    }
    state.transitioning = NO;
    if (!gkCocoaInFullscreenSpace(window)) {
        gkCocoaRestoreWindowed(window);
    }
    gkCocoaApplyFullscreen(window);
}

void gkCocoaWindowFullscreenPhase(void *pointer, int phase) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    switch (phase) {
    case GK_FULLSCREEN_WILL_ENTER:
    case GK_FULLSCREEN_WILL_EXIT:
        state.transitioning = YES;
        return;
    case GK_FULLSCREEN_DID_ENTER:
        state.transitioning = NO;
        state.fullscreen = YES;
        gkCocoaWindowNotify(window, GK_EVENT_WINDOW_FULLSCREEN, 1, 0, 0, NULL);
        break;
    case GK_FULLSCREEN_DID_EXIT:
        /* transitioning stays set until gkCocoaFinishWindowed runs. */
        state.fullscreen = NO;
        gkCocoaWindowNotify(window, GK_EVENT_WINDOW_FULLSCREEN, 0, 0, 0, NULL);
        gkCocoaScheduleFinishWindowed(window, 0, gkCocoaFinishWindowedRetries);
        return;
    case GK_FULLSCREEN_FAILED_ENTER:
        gkCocoaScheduleFinishWindowed(window, 0, gkCocoaFinishWindowedRetries);
        return;
    default:
        state.transitioning = NO;
        break;
    }
    gkCocoaApplyFullscreen(window);
}

int gkCocoaWindowRestore(void *pointer) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    if (gkCocoaIsFullscreen(window) || state.transitioning || state.pending) {
        /* Leaving fullscreen completes asynchronously. */
        state.target = NO;
        state.pending = YES;
        gkCocoaApplyFullscreen(window);
    }
    gkCocoaDeminiaturize(window);
    if (state.maximized) {
        state.maximized = NO;
        [window setFrame:state.normalFrame display:YES animate:NO];
    } else if (!gkCocoaIsFullscreen(window) && !state.transitioning && window.isVisible &&
               (window.styleMask & NSWindowStyleMaskTitled) && window.isZoomed) {
        /* A zoom during a fullscreen transition interrupts it. */
        [window zoom:nil];
    }
    return !window.isMiniaturized;
}

/* gkCocoaWindowDidResize clears the maximized state when the user resizes. */
void gkCocoaWindowDidResize(void *pointer) {
    NSWindow *window = pointer;
    GKCocoaWindowState *state = gkCocoaState(window);
    if (state.maximized && !NSEqualRects(window.frame, gkCocoaScreen(window).visibleFrame)) {
        state.maximized = NO;
    }
    NSRect content = [window contentRectForFrameRect:window.frame];
    gkCocoaWindowNotify(window, GK_EVENT_WINDOW_RESIZE, 0, content.size.width, content.size.height, NULL);
}

void gkCocoaWindowDidChangeKey(void *pointer, int key) {
    gkCocoaWindowNotify(pointer, key ? GK_EVENT_WINDOW_FOCUS : GK_EVENT_WINDOW_BLUR, 0, 0, 0, NULL);
}

/* ---- Screen and preferences ---- */

static NSRect gkCocoaTopLeft(NSRect rect) {
    NSScreen *primary = NSScreen.screens.firstObject;
    CGFloat top = primary ? NSMaxY(primary.frame) : NSMaxY(rect);
    return NSMakeRect(NSMinX(rect), top - NSMaxY(rect), rect.size.width, rect.size.height);
}

void gkCocoaWindowScreen(void *pointer, double *values, char **name) {
    @autoreleasepool {
        NSScreen *screen = gkCocoaScreen((NSWindow *)pointer);
        NSRect frame = gkCocoaTopLeft(screen.frame);
        NSRect visible = gkCocoaTopLeft(screen.visibleFrame);
        values[0] = frame.origin.x;
        values[1] = frame.origin.y;
        values[2] = frame.size.width;
        values[3] = frame.size.height;
        values[4] = visible.origin.x;
        values[5] = visible.origin.y;
        values[6] = visible.size.width;
        values[7] = visible.size.height;
        values[8] = screen.backingScaleFactor;
        NSNumber *displayNumber = screen.deviceDescription[@"NSScreenNumber"];
        CGDirectDisplayID display = displayNumber ? displayNumber.unsignedIntValue : CGMainDisplayID();
        CGSize millimetres = CGDisplayScreenSize(display);
        double dpi = 72.0 * screen.backingScaleFactor;
        if (millimetres.width > 0) {
            dpi = (double)CGDisplayPixelsWide(display) * screen.backingScaleFactor / (millimetres.width / 25.4);
        }
        values[9] = dpi;
        double refresh = (double)screen.maximumFramesPerSecond;
        if (refresh <= 0) {
            CGDisplayModeRef mode = CGDisplayCopyDisplayMode(display);
            refresh = mode ? CGDisplayModeGetRefreshRate(mode) : 0;
            CGDisplayModeRelease(mode);
        }
        values[10] = refresh > 0 ? refresh : 60;
        *name = strdup(screen.localizedName.UTF8String ?: "");
    }
}

static int gkCocoaThemeFor(NSAppearance *appearance) {
    NSAppearanceName match =
        [appearance bestMatchFromAppearancesWithNames:@[ NSAppearanceNameAqua, NSAppearanceNameDarkAqua ]];
    return [match isEqualToString:NSAppearanceNameDarkAqua] ? 2 : 1;
}

int gkCocoaWindowResolvedTheme(void *pointer) {
    NSWindow *window = pointer;
    return gkCocoaThemeFor(window ? window.effectiveAppearance : NSApp.effectiveAppearance);
}

void gkCocoaSystemPreferences(void *pointer, double *values, char **locale) {
    @autoreleasepool {
        NSWorkspace *workspace = NSWorkspace.sharedWorkspace;
        values[0] = gkCocoaWindowResolvedTheme(pointer);
        values[1] = 1;
        values[2] = workspace.accessibilityDisplayShouldIncreaseContrast;
        values[3] = workspace.accessibilityDisplayShouldReduceMotion;
        values[4] = workspace.accessibilityDisplayShouldReduceTransparency;
        NSColor *accent = [NSColor.controlAccentColor colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
        values[5] = accent.redComponent;
        values[6] = accent.greenComponent;
        values[7] = accent.blueComponent;
        values[8] = accent.alphaComponent;
        values[9] = NSApp.userInterfaceLayoutDirection == NSUserInterfaceLayoutDirectionRightToLeft;
        NSString *language = NSLocale.preferredLanguages.firstObject;
        if (!language) {
            language =
                [NSLocale.currentLocale.localeIdentifier stringByReplacingOccurrencesOfString:@"_" withString:@"-"];
        }
        *locale = strdup(language.UTF8String ?: "en");
    }
}

int gkCocoaColorSpaceKind(NSColorSpace *space) {
    if (!space) {
        return 1;
    }
    if ([space isEqual:NSColorSpace.displayP3ColorSpace]) {
        return 2;
    }
    if ([space isEqual:NSColorSpace.extendedSRGBColorSpace]) {
        return 3;
    }
    CGColorSpaceRef cg = space.CGColorSpace;
    CFStringRef name = cg ? CGColorSpaceCopyName(cg) : NULL;
    int kind = 1;
    if (name) {
        if (CFEqual(name, kCGColorSpaceDisplayP3)) {
            kind = 2;
        } else if (CFEqual(name, kCGColorSpaceExtendedSRGB)) {
            kind = 3;
        }
        CFRelease(name);
    }
    return kind;
}

int gkCocoaWindowColorSpace(void *pointer) {
    NSWindow *window = pointer;
    return gkCocoaColorSpaceKind(window.colorSpace ?: gkCocoaScreen(window).colorSpace);
}

/* ---- Application ---- */

static uintptr_t gkCocoaAppHandle;

/* GKCocoaAppObserver becomes the application delegate so that it can report
   reopen and quit requests. An application that embeds GameKit can have its
   own delegate: the observer keeps it as previous and forwards every delegate
   message to it, such as application:openURLs:. */
@interface GKCocoaAppObserver : NSObject <NSApplicationDelegate>
@property(retain) id<NSApplicationDelegate> previous;
@end

@implementation GKCocoaAppObserver
- (BOOL)respondsToSelector:(SEL)selector {
    return [super respondsToSelector:selector] || [self.previous respondsToSelector:selector];
}
- (id)forwardingTargetForSelector:(SEL)selector {
    if ([self.previous respondsToSelector:selector]) {
        return self.previous;
    }
    return [super forwardingTargetForSelector:selector];
}
- (void)active:(NSNotification *)notification {
    (void)notification;
    gkCocoaSend(gkCocoaAppHandle, GK_EVENT_APP_ACTIVE, 0, 0, 0, 0, nil);
}
- (void)inactive:(NSNotification *)notification {
    (void)notification;
    gkCocoaSend(gkCocoaAppHandle, GK_EVENT_APP_INACTIVE, 0, 0, 0, 0, nil);
}
- (void)preferences:(NSNotification *)notification {
    (void)notification;
    gkCocoaSend(gkCocoaAppHandle, GK_EVENT_PREFERENCES, 0, 0, 0, 0, nil);
}
- (void)device:(NSNotification *)notification {
    int kind = 0;
    int connected = 0;
    NSString *name = @"";
    NSString *type = notification.name;
    if ([type isEqualToString:GCKeyboardDidConnectNotification] ||
        [type isEqualToString:GCKeyboardDidDisconnectNotification]) {
        kind = 0;
        connected = [type isEqualToString:GCKeyboardDidConnectNotification];
        name = [notification.object vendorName] ?: @"Keyboard";
    } else if ([type isEqualToString:GCMouseDidConnectNotification] ||
               [type isEqualToString:GCMouseDidDisconnectNotification]) {
        kind = 1;
        connected = [type isEqualToString:GCMouseDidConnectNotification];
        name = [notification.object vendorName] ?: @"Mouse";
    } else {
        kind = 4;
        connected = [type isEqualToString:GCControllerDidConnectNotification];
        name = [notification.object vendorName] ?: @"Controller";
    }
    int identifier = (int)([notification.object hash] & 0x7fffffff);
    GKCocoaEvent event = {0};
    event.kind = GK_EVENT_DEVICE;
    event.a = kind;
    event.b = identifier;
    event.c = connected;
    event.text = name.UTF8String;
    gkCocoaGoEvent(gkCocoaAppHandle, &event);
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)flag {
    gkCocoaSend(gkCocoaAppHandle, GK_EVENT_APP_REOPEN, flag, 0, 0, 0, nil);
    if ([self.previous respondsToSelector:_cmd]) {
        return [self.previous applicationShouldHandleReopen:sender hasVisibleWindows:flag];
    }
    return YES;
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    GKCocoaEvent event = {0};
    event.kind = GK_EVENT_APP_QUIT_REQUEST;
    if (gkCocoaAppHandle && !gkCocoaGoQuery(gkCocoaAppHandle, &event)) {
        return NSTerminateCancel;
    }
    if ([self.previous respondsToSelector:_cmd]) {
        return [self.previous applicationShouldTerminate:sender];
    }
    return NSTerminateNow;
}
@end

static GKCocoaAppObserver *gkCocoaObserver;

void gkCocoaAppAttach(uintptr_t handle) {
    gkCocoaAppHandle = handle;
    if (gkCocoaObserver) {
        return;
    }
    gkCocoaObserver = [GKCocoaAppObserver new];
    NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
    [center addObserver:gkCocoaObserver
               selector:@selector(active:)
                   name:NSApplicationDidBecomeActiveNotification
                 object:nil];
    [center addObserver:gkCocoaObserver
               selector:@selector(inactive:)
                   name:NSApplicationDidResignActiveNotification
                 object:nil];
    for (NSNotificationName name in @[
             NSSystemColorsDidChangeNotification, NSCurrentLocaleDidChangeNotification,
             NSApplicationDidChangeScreenParametersNotification
         ]) {
        [center addObserver:gkCocoaObserver selector:@selector(preferences:) name:name object:nil];
    }
    [NSWorkspace.sharedWorkspace.notificationCenter
        addObserver:gkCocoaObserver
           selector:@selector(preferences:)
               name:NSWorkspaceAccessibilityDisplayOptionsDidChangeNotification
             object:nil];
    [NSDistributedNotificationCenter.defaultCenter addObserver:gkCocoaObserver
                                                      selector:@selector(preferences:)
                                                          name:@"AppleInterfaceThemeChangedNotification"
                                                        object:nil];
    for (NSNotificationName name in @[
             GCKeyboardDidConnectNotification, GCKeyboardDidDisconnectNotification, GCMouseDidConnectNotification,
             GCMouseDidDisconnectNotification, GCControllerDidConnectNotification, GCControllerDidDisconnectNotification
         ]) {
        [center addObserver:gkCocoaObserver selector:@selector(device:) name:name object:nil];
    }
    NSApplication *app = [NSApplication sharedApplication];
    if (app.delegate != gkCocoaObserver) {
        gkCocoaObserver.previous = app.delegate;
        /* NSApplication asks the delegate which messages it handles when the
           delegate is set, so set it after previous. */
        app.delegate = gkCocoaObserver;
    }
}

/* ---- Canvas ---- */

static NSView *gkCocoaCaptureView;

int gkCocoaCanvasCapture(void *pointer, int capture) {
    NSView *view = pointer;
    if (capture) {
        if (!view.window) {
            return 0;
        }
        gkCocoaCaptureView = view;
    } else if (gkCocoaCaptureView == view) {
        gkCocoaCaptureView = nil;
    }
    return 1;
}

int gkCocoaCanvasHasCapture(void *pointer) {
    return pointer && gkCocoaCaptureView == pointer;
}

/* gkCocoaCaptureTarget returns the capturing view for a pointer event outside
   it, so the event loop can route the event there. */
void *gkCocoaCaptureTarget(void *pointer) {
    NSEvent *event = pointer;
    if (!gkCocoaCaptureView || !gkCocoaCaptureView.window || event.window != gkCocoaCaptureView.window) {
        return NULL;
    }
    switch (event.type) {
    case NSEventTypeLeftMouseDown:
    case NSEventTypeLeftMouseUp:
    case NSEventTypeRightMouseDown:
    case NSEventTypeRightMouseUp:
    case NSEventTypeOtherMouseDown:
    case NSEventTypeOtherMouseUp:
    case NSEventTypeMouseMoved:
    case NSEventTypeLeftMouseDragged:
    case NSEventTypeRightMouseDragged:
    case NSEventTypeOtherMouseDragged:
    case NSEventTypeScrollWheel:
        return gkCocoaCaptureView;
    default:
        return NULL;
    }
}

void gkCocoaReleaseCaptureFor(void *pointer) {
    if (gkCocoaCaptureView == pointer) {
        gkCocoaCaptureView = nil;
    }
}

double gkCocoaViewScale(void *pointer) {
    NSView *view = pointer;
    return view.window ? view.window.backingScaleFactor : NSScreen.mainScreen.backingScaleFactor;
}

int gkCocoaViewColorSpace(void *pointer) {
    NSView *view = pointer;
    return view.window ? gkCocoaWindowColorSpace(view.window) : gkCocoaColorSpaceKind(NSScreen.mainScreen.colorSpace);
}

int gkCocoaWindowCanBecomeKey(void *pointer) {
    return ((NSWindow *)pointer).canBecomeKeyWindow;
}
