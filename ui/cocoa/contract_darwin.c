#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_bridge.h"
#include <stdlib.h>
#include <string.h>

/* ---- Handles ---- */

static char gkCocoaHandleKey;

void gkCocoaSetHandle(void *object, uintptr_t handle) {
    if (!object) {
        return;
    }
    objc_setAssociatedObject((id)object, &gkCocoaHandleKey, handle ? @(handle) : nil,
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

uintptr_t gkCocoaHandle(void *object) {
    if (!object) {
        return 0;
    }
    NSNumber *value = objc_getAssociatedObject((id)object, &gkCocoaHandleKey);
    return value ? (uintptr_t)value.unsignedLongLongValue : 0;
}

void gkCocoaRelease(void *object) {
    if (object) {
        [(id)object release];
    }
}

void gkCocoaRetain(void *object) {
    if (object) {
        [(id)object retain];
    }
}

void gkCocoaFree(void *pointer) {
    free(pointer);
}

/* gkCocoaSend reports an event to the Go value that handle names. */
void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text) {
    if (!handle) {
        return;
    }
    GKCocoaEvent event = {0};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.x = x;
    event.y = y;
    event.text = text.UTF8String;
    gkCocoaGoEvent(handle, &event);
}

/* gkCocoaHandleForView returns the handle of view or its nearest ancestor. */
static uintptr_t gkCocoaHandleForView(NSView *view) {
    for (NSView *current = view; current; current = current.superview) {
        uintptr_t handle = gkCocoaHandle(current);
        if (handle) {
            return handle;
        }
    }
    return 0;
}

/* gkCocoaSendView reports a component event for the widget that owns view. */
void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text) {
    gkCocoaSend(gkCocoaHandleForView(view), kind, a, b, x, y, text);
}

void *gkCocoaViewWindow(void *view) {
    return view ? ((NSView *)view).window : NULL;
}

/* ---- Accessibility ---- */

/* gkCocoaElement returns the object that represents view to assistive
   technologies. Composite views answer -gkAccessibleElement. */
static id gkCocoaElement(NSView *view) {
    if ([view respondsToSelector:@selector(gkAccessibleElement)]) {
        id element = [(id)view performSelector:@selector(gkAccessibleElement)];
        if (element) {
            return element;
        }
    }
    return view;
}

static NSString *gkCocoaOptionalString(const char *value) {
    return value && value[0] ? [NSString stringWithUTF8String:value] : nil;
}

void gkCocoaSetAccessibility(void *pointer, const char *identifier, const char *name, const char *description,
                             const char *value, int required, int hidden, void *labeledBy, void **describedBy,
                             int describedByCount) {
    if (!pointer) {
        return;
    }
    NSView *view = pointer;
    id element = gkCocoaElement(view);
    NSString *ident = gkCocoaOptionalString(identifier);
    [element setAccessibilityIdentifier:ident];
    if (element != view) {
        view.accessibilityIdentifier = ident;
    }
    [element setAccessibilityLabel:gkCocoaOptionalString(name)];
    [element setAccessibilityHelp:gkCocoaOptionalString(description)];
    [element setAccessibilityValueDescription:gkCocoaOptionalString(value)];
    [element setAccessibilityRequired:required != 0];
    [element setAccessibilityTitleUIElement:labeledBy ? gkCocoaElement((NSView *)labeledBy) : nil];
    NSMutableArray *linked = [NSMutableArray array];
    for (int i = 0; i < describedByCount; i++) {
        if (describedBy[i]) {
            [linked addObject:gkCocoaElement((NSView *)describedBy[i])];
        }
    }
    [element setAccessibilityLinkedUIElements:linked.count ? linked : nil];
    if (hidden) {
        [element setAccessibilityElement:NO];
    } else if ([element isKindOfClass:NSControl.class] || element != view) {
        [element setAccessibilityElement:YES];
    }
}

static NSString *gkCocoaClean(NSString *value) {
    if (!value) {
        return @"";
    }
    NSMutableString *result = [[value mutableCopy] autorelease];
    [result replaceOccurrencesOfString:@"\x1e" withString:@" " options:0 range:NSMakeRange(0, result.length)];
    [result replaceOccurrencesOfString:@"\x1f" withString:@" " options:0 range:NSMakeRange(0, result.length)];
    return result;
}

static NSString *gkCocoaValueString(id value) {
    if ([value isKindOfClass:NSString.class]) {
        return value;
    }
    if ([value isKindOfClass:NSNumber.class]) {
        return [value stringValue];
    }
    if ([value isKindOfClass:NSAttributedString.class]) {
        return [value string];
    }
    return @"";
}

static void gkCocoaAppendNode(NSMutableString *out, id element, NSWindow *window, int depth, int *count) {
    if (!element || depth > 64 || *count > 20000) {
        return;
    }
    (*count)++;
    NSString *role = [element respondsToSelector:@selector(accessibilityRole)] ? [element accessibilityRole] : nil;
    NSString *subrole =
        [element respondsToSelector:@selector(accessibilitySubrole)] ? [element accessibilitySubrole] : nil;
    NSString *identifier =
        [element respondsToSelector:@selector(accessibilityIdentifier)] ? [element accessibilityIdentifier] : nil;
    NSString *label = [element respondsToSelector:@selector(accessibilityLabel)] ? [element accessibilityLabel] : nil;
    if (!label.length && [element respondsToSelector:@selector(accessibilityTitle)]) {
        label = [element accessibilityTitle];
    }
    NSString *value = @"";
    if ([element respondsToSelector:@selector(accessibilityValue)]) {
        @try {
            value = gkCocoaValueString([element accessibilityValue]);
        } @catch(NSException * exception) {
            (void)exception;
        }
    }
    NSRect frame = NSZeroRect;
    if ([element respondsToSelector:@selector(accessibilityFrame)]) {
        frame = [element accessibilityFrame];
    }
    NSRect content = [window contentRectForFrameRect:window.frame];
    double x = NSMinX(frame) - NSMinX(content);
    double y = NSMaxY(content) - NSMaxY(frame);
    [out appendFormat:@"%d\x1f%@\x1f%@\x1f%@\x1f%@\x1f%@\x1f%.0f\x1f%.0f\x1f%.0f\x1f%.0f\x1e", depth,
                      gkCocoaClean(role), gkCocoaClean(subrole), gkCocoaClean(identifier), gkCocoaClean(label),
                      gkCocoaClean(value), x, y, frame.size.width, frame.size.height];
    NSArray *children = nil;
    if ([element respondsToSelector:@selector(accessibilityChildren)]) {
        children = [element accessibilityChildren];
    }
    for (id child in children) {
        gkCocoaAppendNode(out, child, window, depth + 1, count);
    }
}

char *gkCocoaAccessibilityTree(void *pointer, size_t *size) {
    @autoreleasepool {
        NSWindow *window = pointer;
        if (!window) {
            if (size) {
                *size = 0;
            }
            return NULL;
        }
        NSMutableString *out = [NSMutableString string];
        int count = 0;
        gkCocoaAppendNode(out, window, window, 0, &count);
        const char *utf8 = out.UTF8String;
        size_t length = strlen(utf8);
        char *result = malloc(length + 1);
        memcpy(result, utf8, length + 1);
        if (size) {
            *size = length;
        }
        return result;
    }
}

/* ---- Focus ---- */

static NSView *gkCocoaResponderView(NSResponder *responder) {
    if ([responder isKindOfClass:NSTextView.class] && ((NSTextView *)responder).isFieldEditor) {
        id delegate = ((NSTextView *)responder).delegate;
        if ([delegate isKindOfClass:NSView.class]) {
            return delegate;
        }
    }
    return [responder isKindOfClass:NSView.class] ? (NSView *)responder : nil;
}

int gkCocoaFocus(void *pointer) {
    NSView *view = pointer;
    if (!view || !view.window) {
        return 0;
    }
    id element = gkCocoaElement(view);
    NSView *target = [element isKindOfClass:NSView.class] ? element : view;
    if (!target.acceptsFirstResponder &&
        !([target isKindOfClass:NSTextField.class] &&
          (((NSTextField *)target).isEditable || ((NSTextField *)target).isSelectable))) {
        return 0;
    }
    return [view.window makeFirstResponder:target] ? 1 : 0;
}

int gkCocoaIsFocused(void *pointer) {
    NSView *view = pointer;
    if (!view || !view.window) {
        return 0;
    }
    NSView *focused = gkCocoaResponderView(view.window.firstResponder);
    return focused && (focused == view || [focused isDescendantOf:view]);
}

static char gkCocoaFocusKey;

/* gkCocoaTrackFocus reports changes of the focused widget in window. */
void gkCocoaTrackFocus(void *pointer) {
    NSWindow *window = pointer;
    if (!window) {
        return;
    }
    NSView *focused = gkCocoaResponderView(window.firstResponder);
    uintptr_t current = focused ? gkCocoaHandleForView(focused) : 0;
    /* The content view itself is the window, not a widget. */
    if (focused == window.contentView) {
        current = 0;
    }
    NSNumber *previousValue = objc_getAssociatedObject(window, &gkCocoaFocusKey);
    uintptr_t previous = previousValue ? (uintptr_t)previousValue.unsignedLongLongValue : 0;
    if (current == previous) {
        return;
    }
    objc_setAssociatedObject(window, &gkCocoaFocusKey, current ? @(current) : nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    gkCocoaSend(previous, GK_EVENT_BLUR, 0, 0, 0, 0, nil);
    gkCocoaSend(current, GK_EVENT_FOCUS, 0, 0, 0, 0, nil);
}

void gkCocoaAfterPoll(void) {
    for (NSWindow *window in NSApp.windows) {
        gkCocoaTrackFocus(window);
    }
}

/* gkCocoaForgetFocus drops a closed widget's handle from focus tracking. */
void gkCocoaForgetFocus(void *pointer, uintptr_t handle) {
    NSWindow *window = pointer;
    if (!window) {
        return;
    }
    NSNumber *previous = objc_getAssociatedObject(window, &gkCocoaFocusKey);
    if (previous && (uintptr_t)previous.unsignedLongLongValue == handle) {
        objc_setAssociatedObject(window, &gkCocoaFocusKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
}

/* ---- Presentation ---- */

void gkCocoaSetTooltip(void *pointer, const char *text) {
    NSView *view = pointer;
    if (view) {
        view.toolTip = gkCocoaOptionalString(text);
    }
}

@interface GKCocoaCursorTracker : NSObject
@property(retain) NSCursor *cursor;
@property(retain) NSTrackingArea *area;
@end

@implementation GKCocoaCursorTracker
- (void)dealloc {
    [_cursor release];
    [_area release];
    [super dealloc];
}
- (void)cursorUpdate:(NSEvent *)event {
    (void)event;
    [self.cursor set];
}
- (void)mouseEntered:(NSEvent *)event {
    (void)event;
}
- (void)mouseExited:(NSEvent *)event {
    (void)event;
}
@end

static char gkCocoaCursorKey;

/* gkCocoaWaitCursor returns an hourglass cursor. AppKit has no public wait
   cursor, because the system shows its own when an application stops
   responding. The black symbol has a white outline, so that it shows on dark
   and light content. */
static NSCursor *gkCocoaWaitCursor(void) {
    static NSCursor *cursor;
    if (cursor) {
        return cursor;
    }
    NSImageSymbolConfiguration *configuration =
        [NSImageSymbolConfiguration configurationWithPointSize:14 weight:NSFontWeightBold];
    NSImage *symbol = [[NSImage imageWithSystemSymbolName:@"hourglass" accessibilityDescription:@"Wait"]
        imageWithSymbolConfiguration:configuration];
    if (!symbol) {
        return NSCursor.arrowCursor;
    }
    NSSize size = NSMakeSize(ceil(symbol.size.width) + 4, ceil(symbol.size.height) + 4);
    NSImage *image = [NSImage imageWithSize:size
                                    flipped:NO
                             drawingHandler:^BOOL(NSRect rect) {
                               (void)rect;
                               NSImage *white = [symbol imageWithSymbolConfiguration:
                                                            [NSImageSymbolConfiguration
                                                                configurationWithPaletteColors:@[ NSColor.whiteColor ]]];
                               NSImage *black = [symbol imageWithSymbolConfiguration:
                                                            [NSImageSymbolConfiguration
                                                                configurationWithPaletteColors:@[ NSColor.blackColor ]]];
                               for (int dx = -1; dx <= 1; dx++) {
                                   for (int dy = -1; dy <= 1; dy++) {
                                       [white drawAtPoint:NSMakePoint(2 + dx * 1.5, 2 + dy * 1.5)
                                                 fromRect:NSZeroRect
                                                operation:NSCompositingOperationSourceOver
                                                 fraction:1];
                                   }
                               }
                               [black drawAtPoint:NSMakePoint(2, 2)
                                         fromRect:NSZeroRect
                                        operation:NSCompositingOperationSourceOver
                                         fraction:1];
                               return YES;
                             }];
    cursor = [[NSCursor alloc] initWithImage:image hotSpot:NSMakePoint(size.width / 2, size.height / 2)];
    return cursor;
}

static NSCursor *gkCocoaCursor(int cursor) {
    switch (cursor) {
    case 1:
        return NSCursor.pointingHandCursor;
    case 2:
        return NSCursor.IBeamCursor;
    case 3:
        return NSCursor.crosshairCursor;
    case 4:
    case 9:
        return NSCursor.openHandCursor;
    case 5:
        return NSCursor.resizeLeftRightCursor;
    case 6:
        return NSCursor.resizeUpDownCursor;
    case 7:
        return NSCursor.operationNotAllowedCursor;
    case 8:
        return gkCocoaWaitCursor();
    case 10:
        return NSCursor.closedHandCursor;
    default:
        return NSCursor.arrowCursor;
    }
}

void gkCocoaSetCursor(void *pointer, int cursor) {
    NSView *view = pointer;
    if (!view) {
        return;
    }
    GKCocoaCursorTracker *tracker = objc_getAssociatedObject(view, &gkCocoaCursorKey);
    if (tracker.area) {
        [view removeTrackingArea:tracker.area];
    }
    if (cursor == 0) {
        objc_setAssociatedObject(view, &gkCocoaCursorKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        return;
    }
    tracker = [[GKCocoaCursorTracker new] autorelease];
    tracker.cursor = gkCocoaCursor(cursor);
    tracker.area = [[[NSTrackingArea alloc]
        initWithRect:NSZeroRect
             options:NSTrackingCursorUpdate | NSTrackingActiveInKeyWindow | NSTrackingInVisibleRect
               owner:tracker
            userInfo:nil] autorelease];
    [view addTrackingArea:tracker.area];
    objc_setAssociatedObject(view, &gkCocoaCursorKey, tracker, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

NSColor *gkCocoaSemanticColor(int color) {
    switch (color) {
    case 1:
        return NSColor.labelColor;
    case 2:
        return NSColor.secondaryLabelColor;
    case 3:
        return NSColor.tertiaryLabelColor;
    case 4:
        return NSColor.windowBackgroundColor;
    case 5:
        return NSColor.controlAccentColor;
    case 6:
        return NSColor.systemGreenColor;
    case 7:
        return NSColor.systemOrangeColor;
    case 8:
        return NSColor.systemRedColor;
    case 9:
        return NSColor.linkColor;
    default:
        return nil;
    }
}

static NSFontWeight gkCocoaFontWeight(int weight) {
    if (weight <= 0) {
        return NSFontWeightRegular;
    }
    if (weight < 150) {
        return NSFontWeightUltraLight;
    }
    if (weight < 250) {
        return NSFontWeightThin;
    }
    if (weight < 350) {
        return NSFontWeightLight;
    }
    if (weight < 450) {
        return NSFontWeightRegular;
    }
    if (weight < 550) {
        return NSFontWeightMedium;
    }
    if (weight < 650) {
        return NSFontWeightSemibold;
    }
    if (weight < 750) {
        return NSFontWeightBold;
    }
    if (weight < 850) {
        return NSFontWeightHeavy;
    }
    return NSFontWeightBlack;
}

/* gkCocoaFont resolves a font request; it returns nil for an unknown family. */
NSFont *gkCocoaFont(const char *family, double size, int weight, int italic) {
    CGFloat pointSize = size > 0 ? size : NSFont.systemFontSize;
    NSString *name = gkCocoaOptionalString(family);
    NSFont *font = nil;
    if (!name || [name isEqualToString:@"System"]) {
        font = [NSFont systemFontOfSize:pointSize weight:gkCocoaFontWeight(weight)];
    } else if ([name isEqualToString:@"Monospace"]) {
        font = [NSFont monospacedSystemFontOfSize:pointSize weight:gkCocoaFontWeight(weight)];
    } else {
        font = [NSFont fontWithName:name size:pointSize];
        if (!font) {
            font = [[NSFontManager sharedFontManager] fontWithFamily:name
                                                              traits:0
                                                              weight:weight > 0 ? (weight / 100) + 1 : 5
                                                                size:pointSize];
        }
    }
    if (font && italic) {
        font = [[NSFontManager sharedFontManager] convertFont:font toHaveTrait:NSItalicFontMask];
    }
    return font;
}

/* GKCocoaFilledButton is a borderless button that a style fills with a
   background color. AppKit draws no pressed state for it, so it lightens the
   fill while the mouse is down. */
@interface GKCocoaFilledButton : NSButton
@end

@implementation GKCocoaFilledButton
- (void)mouseDown:(NSEvent *)event {
    CGColorRef fill = self.layer.backgroundColor;
    if (!fill) {
        [super mouseDown:event];
        return;
    }
    CGColorRetain(fill);
    NSColor *pressed = [[NSColor colorWithCGColor:fill] highlightWithLevel:0.3];
    self.layer.backgroundColor = pressed.CGColor;
    /* NSButton tracks the mouse until it goes up. */
    [super mouseDown:event];
    self.layer.backgroundColor = fill;
    CGColorRelease(fill);
}
@end

static NSColor *gkCocoaRGBA(const double *rgba) {
    if (!rgba) {
        return nil;
    }
    return [NSColor colorWithSRGBRed:rgba[0] green:rgba[1] blue:rgba[2] alpha:rgba[3]];
}

/* gkCocoaFillButton makes button a flat button filled with background, or
   restores the native bezel when background is nil. The title takes the
   foreground color through an attributed title. */
static void gkCocoaFillButton(NSButton *button, NSColor *background, NSColor *foreground, NSFont *font,
                              double cornerRadius) {
    if (!background) {
        button.bordered = YES;
        button.title = button.title;
        return;
    }
    if (button.class == NSButton.class) {
        /* The subclass adds no instance variables, so the change is safe. */
        object_setClass(button, GKCocoaFilledButton.class);
    }
    button.bordered = NO;
    button.wantsLayer = YES;
    /* The layer rounds its background without a mask, so a badge can extend
       past the corner. */
    button.layer.cornerRadius = cornerRadius;
    button.layer.masksToBounds = NO;
    NSMutableParagraphStyle *paragraph = [[NSMutableParagraphStyle new] autorelease];
    paragraph.alignment = NSTextAlignmentCenter;
    button.attributedTitle =
        [[[NSAttributedString alloc] initWithString:button.title
                                         attributes:@{
                                             NSForegroundColorAttributeName : foreground ?: NSColor.labelColor,
                                             NSFontAttributeName : font,
                                             NSParagraphStyleAttributeName : paragraph,
                                         }] autorelease];
}

int gkCocoaSetStyle(void *pointer, const char *family, double size, int weight, int italic, int foreground,
                    int background, const double *foregroundRGBA, const double *backgroundRGBA, double cornerRadius) {
    NSView *view = pointer;
    if (!view) {
        return 0;
    }
    NSFont *font = gkCocoaFont(family, size, weight, italic);
    if (!font) {
        return 0;
    }
    id element = gkCocoaElement(view);
    if ([element respondsToSelector:@selector(setFont:)]) {
        [element setFont:font];
    }
    NSColor *foregroundColor = gkCocoaRGBA(foregroundRGBA) ?: gkCocoaSemanticColor(foreground);
    if ([element isKindOfClass:NSTextField.class]) {
        ((NSTextField *)element).textColor = foregroundColor ?: NSColor.labelColor;
    } else if ([element isKindOfClass:NSTextView.class]) {
        ((NSTextView *)element).textColor = foregroundColor ?: NSColor.textColor;
    } else if ([element isKindOfClass:NSButton.class]) {
        ((NSButton *)element).contentTintColor = foregroundColor;
    }
    NSColor *backgroundColor = gkCocoaRGBA(backgroundRGBA) ?: gkCocoaSemanticColor(background);
    if ([element isKindOfClass:NSButton.class]) {
        gkCocoaFillButton(element, backgroundColor, foregroundColor, font, cornerRadius);
    }
    if ([element isKindOfClass:NSButton.class] && backgroundColor) {
        view.layer.backgroundColor = backgroundColor.CGColor;
    } else if ([element isKindOfClass:NSTextField.class]) {
        NSTextField *field = element;
        if (backgroundColor) {
            field.backgroundColor = backgroundColor;
            field.drawsBackground = YES;
        } else if (!field.isEditable) {
            field.drawsBackground = NO;
        }
    } else {
        view.wantsLayer = YES;
        view.layer.backgroundColor = backgroundColor.CGColor;
        view.layer.cornerRadius = cornerRadius;
        view.layer.masksToBounds = cornerRadius > 0;
    }
    [view setNeedsDisplay:YES];
    return 1;
}

void gkCocoaSetMenu(void *pointer, void *menu) {
    NSView *view = pointer;
    if (view) {
        view.menu = (NSMenu *)menu;
    }
}

/* ---- Drag and drop ---- */

@interface GKCocoaDragSource : NSObject <NSDraggingSource>
@property NSDragOperation operations;
@property(copy) NSString *text;
@property(retain) NSArray<NSURL *> *files;
@property(retain) NSImage *image;
@property(retain) NSDictionary<NSString *, NSData *> *custom;
@end

@implementation GKCocoaDragSource
- (void)dealloc {
    [_text release];
    [_files release];
    [_image release];
    [_custom release];
    [super dealloc];
}
- (NSDragOperation)draggingSession:(NSDraggingSession *)session
    sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    (void)session;
    (void)context;
    return self.operations;
}
@end

static char gkCocoaDragKey;
static char gkCocoaDropKey;

static NSDragOperation gkCocoaDragOperations(int operations) {
    NSDragOperation result = NSDragOperationNone;
    if (operations & 1) {
        result |= NSDragOperationCopy;
    }
    if (operations & 2) {
        result |= NSDragOperationMove;
    }
    if (operations & 4) {
        result |= NSDragOperationLink;
    }
    return result;
}

static int gkCocoaOperationBits(NSDragOperation operation) {
    if (operation & NSDragOperationCopy) {
        return 1;
    }
    if (operation & NSDragOperationMove) {
        return 2;
    }
    if (operation & NSDragOperationLink) {
        return 4;
    }
    return 0;
}

/* gkCocoaTypeForMIME maps a MIME type to a pasteboard type. */
NSString *gkCocoaTypeForMIME(NSString *mime) {
    if ([mime isEqualToString:@"text/plain"]) {
        return NSPasteboardTypeString;
    }
    if ([mime isEqualToString:@"text/uri-list"]) {
        return NSPasteboardTypeFileURL;
    }
    if ([mime isEqualToString:@"image/png"]) {
        return NSPasteboardTypePNG;
    }
    if ([mime isEqualToString:@"image/tiff"]) {
        return NSPasteboardTypeTIFF;
    }
    if ([mime isEqualToString:@"text/html"]) {
        return NSPasteboardTypeHTML;
    }
    if ([mime isEqualToString:@"application/rtf"] || [mime isEqualToString:@"text/rtf"]) {
        return NSPasteboardTypeRTF;
    }
    UTType *type = [UTType typeWithMIMEType:mime];
    return type ? type.identifier : mime;
}

/* gkCocoaMIMEForType maps a pasteboard type to a MIME type, or nil. */
NSString *gkCocoaMIMEForType(NSString *type) {
    if ([type isEqualToString:NSPasteboardTypeString]) {
        return @"text/plain";
    }
    if ([type isEqualToString:NSPasteboardTypeFileURL]) {
        return @"text/uri-list";
    }
    UTType *uti = [UTType typeWithIdentifier:type];
    if (uti.preferredMIMEType) {
        return uti.preferredMIMEType;
    }
    if (uti.isDynamic) {
        /* Dynamic types created from MIME types remember them. */
        NSArray *mimes = uti.tags[UTTagClassMIMEType];
        return mimes.firstObject;
    }
    return nil;
}

static NSArray<NSString *> *gkCocoaSplit(const char *buffer, size_t size) {
    NSMutableArray *values = [NSMutableArray array];
    size_t start = 0;
    for (size_t i = 0; i < size; i++) {
        if (buffer[i] == 0) {
            [values addObject:[[[NSString alloc] initWithBytes:buffer + start
                                                        length:i - start
                                                      encoding:NSUTF8StringEncoding] autorelease]
                                  ?: @""];
            start = i + 1;
        }
    }
    return values;
}

NSImage *gkCocoaImageFromPixels(const unsigned char *pixels, int width, int height) {
    if (!pixels || width <= 0 || height <= 0) {
        return nil;
    }
    NSBitmapImageRep *bitmap = [[[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL
                                                                        pixelsWide:width
                                                                        pixelsHigh:height
                                                                     bitsPerSample:8
                                                                   samplesPerPixel:4
                                                                          hasAlpha:YES
                                                                          isPlanar:NO
                                                                    colorSpaceName:NSCalibratedRGBColorSpace
                                                                      bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
                                                                       bytesPerRow:width * 4
                                                                      bitsPerPixel:32] autorelease];
    memcpy(bitmap.bitmapData, pixels, (size_t)width * height * 4);
    NSImage *image = [[[NSImage alloc] initWithSize:NSMakeSize(width, height)] autorelease];
    [image addRepresentation:bitmap];
    return image;
}

/* gkCocoaImagePixels renders image into a malloc'd RGBA buffer. */
unsigned char *gkCocoaImagePixels(NSImage *image, int *width, int *height) {
    CGImageRef cgImage = [image CGImageForProposedRect:NULL context:nil hints:nil];
    if (!cgImage) {
        return NULL;
    }
    size_t w = CGImageGetWidth(cgImage);
    size_t h = CGImageGetHeight(cgImage);
    if (!w || !h) {
        return NULL;
    }
    unsigned char *pixels = calloc(w * h, 4);
    CGColorSpaceRef space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
    CGContextRef context = CGBitmapContextCreate(pixels, w, h, 8, w * 4, space,
                                                 kCGImageAlphaPremultipliedLast | kCGBitmapByteOrderDefault);
    CGColorSpaceRelease(space);
    if (!context) {
        free(pixels);
        return NULL;
    }
    CGContextDrawImage(context, CGRectMake(0, 0, w, h), cgImage);
    CGContextRelease(context);
    /* Undo premultiplication so Go receives straight alpha. */
    for (size_t i = 0; i < w * h; i++) {
        unsigned char *p = pixels + i * 4;
        if (p[3] && p[3] != 255) {
            p[0] = (unsigned char)MIN(255, p[0] * 255 / p[3]);
            p[1] = (unsigned char)MIN(255, p[1] * 255 / p[3]);
            p[2] = (unsigned char)MIN(255, p[2] * 255 / p[3]);
        }
    }
    *width = (int)w;
    *height = (int)h;
    return pixels;
}

void gkCocoaSetDragSource(void *pointer, int operations, const char *text, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *customTypes,
                          size_t customTypesSize, const unsigned char *customData, const size_t *customSizes,
                          int customCount) {
    NSView *view = pointer;
    if (!view) {
        return;
    }
    if (!operations) {
        objc_setAssociatedObject(view, &gkCocoaDragKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        return;
    }
    GKCocoaDragSource *source = [[GKCocoaDragSource new] autorelease];
    source.operations = gkCocoaDragOperations(operations);
    source.text = gkCocoaOptionalString(text);
    NSMutableArray *urls = [NSMutableArray array];
    for (NSString *path in gkCocoaSplit(files, filesSize)) {
        [urls addObject:[NSURL fileURLWithPath:path]];
    }
    source.files = urls;
    source.image = gkCocoaImageFromPixels(pixels, width, height);
    NSMutableDictionary *custom = [NSMutableDictionary dictionary];
    NSArray *types = gkCocoaSplit(customTypes, customTypesSize);
    size_t offset = 0;
    for (int i = 0; i < customCount && i < (int)types.count; i++) {
        custom[gkCocoaTypeForMIME(types[i])] = [NSData dataWithBytes:customData + offset length:customSizes[i]];
        offset += customSizes[i];
    }
    source.custom = custom;
    objc_setAssociatedObject(view, &gkCocoaDragKey, source, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

static NSView *gkCocoaDragView;
static NSPoint gkCocoaDragStart;

static GKCocoaDragSource *gkCocoaDragSourceForView(NSView *view, NSView **owner) {
    for (NSView *current = view; current; current = current.superview) {
        GKCocoaDragSource *source = objc_getAssociatedObject(current, &gkCocoaDragKey);
        if (source) {
            *owner = current;
            return source;
        }
    }
    return nil;
}

/* gkCocoaHandleDragEvent starts a drag from a widget with a drag source. It
   returns non-zero when it consumed event. */
int gkCocoaHandleDragEvent(void *pointer) {
    NSEvent *event = pointer;
    NSWindow *window = event.window;
    if (!window) {
        return 0;
    }
    if (event.type == NSEventTypeLeftMouseDown) {
        NSView *hit =
            [window.contentView hitTest:[window.contentView convertPoint:event.locationInWindow fromView:nil]];
        NSView *owner = nil;
        gkCocoaDragView = gkCocoaDragSourceForView(hit, &owner) ? owner : nil;
        gkCocoaDragStart = event.locationInWindow;
        return 0;
    }
    if (event.type == NSEventTypeLeftMouseUp) {
        gkCocoaDragView = nil;
        return 0;
    }
    if (event.type != NSEventTypeLeftMouseDragged || !gkCocoaDragView || gkCocoaDragView.window != window) {
        return 0;
    }
    NSPoint point = event.locationInWindow;
    if (hypot(point.x - gkCocoaDragStart.x, point.y - gkCocoaDragStart.y) < 4) {
        return 1;
    }
    NSView *view = gkCocoaDragView;
    gkCocoaDragView = nil;
    GKCocoaDragSource *source = objc_getAssociatedObject(view, &gkCocoaDragKey);
    if (!source) {
        return 0;
    }
    NSMutableArray *items = [NSMutableArray array];
    NSPasteboardItem *item = [[NSPasteboardItem new] autorelease];
    BOOL hasItem = NO;
    if (source.text) {
        [item setString:source.text forType:NSPasteboardTypeString];
        hasItem = YES;
    }
    for (NSString *type in source.custom) {
        [item setData:source.custom[type] forType:type];
        hasItem = YES;
    }
    if (source.image) {
        /* Targets ask for PNG or TIFF, so the item has both. */
        NSBitmapImageRep *bitmap = [NSBitmapImageRep imageRepWithData:source.image.TIFFRepresentation];
        NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (png) {
            [item setData:png forType:NSPasteboardTypePNG];
        }
        [item setData:source.image.TIFFRepresentation forType:NSPasteboardTypeTIFF];
        hasItem = YES;
    }
    NSBitmapImageRep *snapshot = [view bitmapImageRepForCachingDisplayInRect:view.bounds];
    [view cacheDisplayInRect:view.bounds toBitmapImageRep:snapshot];
    NSImage *dragImage = [[[NSImage alloc] initWithSize:view.bounds.size] autorelease];
    [dragImage addRepresentation:snapshot];
    if (hasItem) {
        NSDraggingItem *dragging = [[[NSDraggingItem alloc] initWithPasteboardWriter:item] autorelease];
        [dragging setDraggingFrame:view.bounds contents:dragImage];
        [items addObject:dragging];
    }
    for (NSURL *url in source.files) {
        NSDraggingItem *dragging = [[[NSDraggingItem alloc] initWithPasteboardWriter:url] autorelease];
        [dragging setDraggingFrame:view.bounds contents:dragImage];
        [items addObject:dragging];
    }
    if (!items.count) {
        return 0;
    }
    [view beginDraggingSessionWithItems:items event:event source:source];
    return 1;
}

/* Drop targets add the NSDraggingDestination methods to a per-class subclass,
   the same technique Key-Value Observing uses, because AppKit asks the view
   itself and most targets are standard controls. */

@interface GKCocoaDropTarget : NSObject
@property NSDragOperation operations;
@property(retain) NSArray<NSString *> *types;
@end

@implementation GKCocoaDropTarget
- (void)dealloc {
    [_types release];
    [super dealloc];
}
@end

extern void gkCocoaGoDrop(uintptr_t handle, int operation, double x, double y, const char *text, const char *files,
                          size_t filesSize, unsigned char *pixels, int width, int height, const char *types,
                          size_t typesSize, const unsigned char *data, const size_t *sizes, int count);

static NSDragOperation gkCocoaDropOperation(id self, id<NSDraggingInfo> info) {
    GKCocoaDropTarget *target = objc_getAssociatedObject(self, &gkCocoaDropKey);
    if (!target) {
        return NSDragOperationNone;
    }
    NSDragOperation allowed = info.draggingSourceOperationMask & target.operations;
    if (allowed & NSDragOperationCopy) {
        return NSDragOperationCopy;
    }
    if (allowed & NSDragOperationMove) {
        return NSDragOperationMove;
    }
    if (allowed & NSDragOperationLink) {
        return NSDragOperationLink;
    }
    return NSDragOperationNone;
}

static NSDragOperation gkCocoaDropEntered(id self, SEL command, id<NSDraggingInfo> info) {
    (void)command;
    return gkCocoaDropOperation(self, info);
}

static BOOL gkCocoaDropPrepare(id self, SEL command, id<NSDraggingInfo> info) {
    (void)command;
    return gkCocoaDropOperation(self, info) != NSDragOperationNone;
}

static BOOL gkCocoaDropPerform(id self, SEL command, id<NSDraggingInfo> info) {
    (void)command;
    NSView *view = self;
    NSDragOperation operation = gkCocoaDropOperation(self, info);
    uintptr_t handle = gkCocoaHandleForView(view);
    if (operation == NSDragOperationNone || !handle) {
        return NO;
    }
    GKCocoaDropTarget *target = objc_getAssociatedObject(self, &gkCocoaDropKey);
    NSPasteboard *pasteboard = info.draggingPasteboard;
    NSPoint point = [view convertPoint:info.draggingLocation fromView:nil];
    if (!view.isFlipped) {
        point.y = view.bounds.size.height - point.y;
    }
    NSString *text = [pasteboard stringForType:NSPasteboardTypeString];
    NSArray<NSURL *> *urls =
        [pasteboard readObjectsForClasses:@[ NSURL.class ] options:@{
            NSPasteboardURLReadingFileURLsOnlyKey : @YES
        }];
    NSMutableData *files = [NSMutableData data];
    for (NSURL *url in urls) {
        const char *path = url.path.fileSystemRepresentation;
        [files appendBytes:path length:strlen(path) + 1];
    }
    int width = 0, height = 0;
    unsigned char *pixels = NULL;
    NSArray *images = [pasteboard readObjectsForClasses:@[ NSImage.class ] options:nil];
    if (images.count && !urls.count) {
        pixels = gkCocoaImagePixels(images.firstObject, &width, &height);
    }
    NSMutableData *types = [NSMutableData data];
    NSMutableData *data = [NSMutableData data];
    NSMutableArray<NSNumber *> *sizes = [NSMutableArray array];
    for (NSString *type in target.types) {
        if ([type isEqualToString:NSPasteboardTypeString] || [type isEqualToString:NSPasteboardTypeFileURL]) {
            continue;
        }
        NSData *value = [pasteboard dataForType:type];
        NSString *mime = gkCocoaMIMEForType(type) ?: type;
        if (!value) {
            continue;
        }
        [types appendBytes:mime.UTF8String length:strlen(mime.UTF8String) + 1];
        [data appendData:value];
        [sizes addObject:@(value.length)];
    }
    size_t *sizeValues = calloc(sizes.count + 1, sizeof(size_t));
    for (NSUInteger i = 0; i < sizes.count; i++) {
        sizeValues[i] = sizes[i].unsignedLongValue;
    }
    gkCocoaGoDrop(handle, gkCocoaOperationBits(operation), point.x, point.y, text.UTF8String, files.bytes, files.length,
                  pixels, width, height, types.bytes, types.length, data.bytes, sizeValues, (int)sizes.count);
    free(sizeValues);
    free(pixels);
    return YES;
}

static Class gkCocoaDropClass(Class base) {
    NSString *name = [@"GKCocoaDrop_" stringByAppendingString:NSStringFromClass(base)];
    Class subclass = NSClassFromString(name);
    if (subclass) {
        return subclass;
    }
    subclass = objc_allocateClassPair(base, name.UTF8String, 0);
    if (!subclass) {
        return nil;
    }
    NSString *operationTypes = [NSString stringWithFormat:@"%s@:@", @encode(NSDragOperation)];
    NSString *boolTypes = [NSString stringWithFormat:@"%s@:@", @encode(BOOL)];
    class_addMethod(subclass, @selector(draggingEntered:), (IMP)gkCocoaDropEntered, operationTypes.UTF8String);
    class_addMethod(subclass, @selector(draggingUpdated:), (IMP)gkCocoaDropEntered, operationTypes.UTF8String);
    class_addMethod(subclass, @selector(prepareForDragOperation:), (IMP)gkCocoaDropPrepare, boolTypes.UTF8String);
    class_addMethod(subclass, @selector(performDragOperation:), (IMP)gkCocoaDropPerform, boolTypes.UTF8String);
    objc_registerClassPair(subclass);
    return subclass;
}

void gkCocoaSetDropTarget(void *pointer, int operations, const char *types, size_t typesSize) {
    NSView *view = pointer;
    if (!view) {
        return;
    }
    if (!operations) {
        [view unregisterDraggedTypes];
        objc_setAssociatedObject(view, &gkCocoaDropKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        return;
    }
    if (![NSStringFromClass(object_getClass(view)) hasPrefix:@"GKCocoaDrop_"]) {
        Class subclass = gkCocoaDropClass(object_getClass(view));
        if (subclass) {
            object_setClass(view, subclass);
        }
    }
    NSMutableArray *pasteboardTypes = [NSMutableArray array];
    for (NSString *mime in gkCocoaSplit(types, typesSize)) {
        [pasteboardTypes addObject:gkCocoaTypeForMIME(mime)];
        /* Many applications put images on the pasteboard only as TIFF. The
           drop reads any image type and converts it. */
        if ([mime hasPrefix:@"image/"] && ![pasteboardTypes containsObject:NSPasteboardTypeTIFF]) {
            [pasteboardTypes addObject:NSPasteboardTypeTIFF];
        }
    }
    if (!pasteboardTypes.count) {
        [pasteboardTypes addObjectsFromArray:@[ NSPasteboardTypeString, NSPasteboardTypeFileURL ]];
    }
    GKCocoaDropTarget *target = [[GKCocoaDropTarget new] autorelease];
    target.operations = gkCocoaDragOperations(operations);
    target.types = pasteboardTypes;
    objc_setAssociatedObject(view, &gkCocoaDropKey, target, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    [view registerForDraggedTypes:pasteboardTypes];
}
