#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);
void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text);
NSColor *gkCocoaSemanticColor(int color);

static NSString *gkString(const char *value) {
    return value ? [NSString stringWithUTF8String:value] : @"";
}

static NSArray<NSString *> *gkSplit(const char *buffer, size_t size) {
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

static void gkAdd(uintptr_t window, NSView *view) {
    [((NSWindow *)window).contentView addSubview:view];
}

/* Severity: 0 information, 1 success, 2 warning, 3 error. */
static NSColor *gkSeverityColor(int severity) {
    switch (severity) {
    case 1:
        return NSColor.systemGreenColor;
    case 2:
        return NSColor.systemOrangeColor;
    case 3:
        return NSColor.systemRedColor;
    default:
        return NSColor.systemBlueColor;
    }
}

static NSString *gkSeveritySymbol(int severity) {
    switch (severity) {
    case 1:
        return @"checkmark.circle.fill";
    case 2:
        return @"exclamationmark.triangle.fill";
    case 3:
        return @"xmark.octagon.fill";
    default:
        return @"info.circle.fill";
    }
}

static NSImage *gkSeverityImage(int severity, CGFloat size) {
    NSImage *image = [NSImage imageWithSystemSymbolName:gkSeveritySymbol(severity) accessibilityDescription:nil];
    NSImageSymbolConfiguration *configuration =
        [NSImageSymbolConfiguration configurationWithPointSize:size weight:NSFontWeightRegular];
    return [image imageWithSymbolConfiguration:configuration] ?: image;
}

/* gkCocoaAnnounce asks assistive technologies to speak text. Priority 1 is
   polite and 2 interrupts. */
void gkCocoaAnnounce(void *pointer, const char *text, int priority) {
    NSString *message = gkString(text);
    if (!message.length || priority <= 0) {
        return;
    }
    NSAccessibilityPostNotificationWithUserInfo(
        (id)pointer ?: NSApp, NSAccessibilityAnnouncementRequestedNotification, @{
            NSAccessibilityAnnouncementKey : message,
            NSAccessibilityPriorityKey : @(priority == 2 ? NSAccessibilityPriorityHigh : NSAccessibilityPriorityMedium),
        });
}

static NSArray<NSButton *> *gkActionButtons(id target, SEL action, const char *ids, size_t idsSize, const char *titles,
                                            size_t titlesSize, const int *enabled, int count) {
    NSMutableArray *buttons = [NSMutableArray array];
    NSArray *identifiers = gkSplit(ids, idsSize);
    NSArray *names = gkSplit(titles, titlesSize);
    for (int i = 0; i < count; i++) {
        NSButton *button = [NSButton buttonWithTitle:i < (int)names.count ? names[i] : @"" target:target action:action];
        button.identifier = i < (int)identifiers.count ? identifiers[i] : @"";
        button.enabled = enabled[i] != 0;
        [buttons addObject:button];
    }
    return buttons;
}

/* ---- Info bar ---- */

@interface GKCocoaInfoBar : GKUILayoutView
@property(retain) NSImageView *icon;
@property(retain) NSTextField *message;
@property(retain) NSButton *close;
@property(retain) NSArray<NSButton *> *buttons;
@property int severity;
@end

@implementation GKCocoaInfoBar
- (void)dealloc {
    [_icon release];
    [_message release];
    [_close release];
    [_buttons release];
    [super dealloc];
}
- (void)drawRect:(NSRect)dirty {
    (void)dirty;
    NSRect bounds = NSInsetRect(self.bounds, 0.5, 0.5);
    NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:bounds xRadius:6 yRadius:6];
    [[gkSeverityColor(self.severity) colorWithAlphaComponent:0.12] setFill];
    [path fill];
    [[gkSeverityColor(self.severity) colorWithAlphaComponent:0.45] setStroke];
    [path stroke];
}
- (void)layout {
    [super layout];
    CGFloat height = self.bounds.size.height;
    self.icon.frame = NSMakeRect(10, (height - 18) / 2, 18, 18);
    CGFloat right = self.bounds.size.width - 8;
    if (!self.close.hidden) {
        self.close.frame = NSMakeRect(right - 18, (height - 18) / 2, 18, 18);
        right -= 26;
    }
    for (NSButton *button in self.buttons.reverseObjectEnumerator) {
        NSSize size = button.fittingSize;
        button.frame = NSMakeRect(right - size.width, (height - size.height) / 2, size.width, size.height);
        right -= size.width + 6;
    }
    self.message.preferredMaxLayoutWidth = MAX(0, right - 36);
    NSSize text = self.message.fittingSize;
    self.message.frame = NSMakeRect(36, (height - text.height) / 2, MAX(0, right - 36), text.height);
}
- (NSSize)fittingSize {
    CGFloat width = 36 + self.message.fittingSize.width + 8;
    for (NSButton *button in self.buttons) {
        width += button.fittingSize.width + 6;
    }
    if (!self.close.hidden) {
        width += 26;
    }
    return NSMakeSize(width, MAX(36, self.message.fittingSize.height + 16));
}
- (void)action:(NSButton *)sender {
    gkCocoaSendView(self, GK_EVENT_ACTION, 0, 0, 0, 0, sender.identifier);
}
- (void)dismiss:(id)sender {
    (void)sender;
    gkCocoaSendView(self, GK_EVENT_CLOSE, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaInfoBarCreate(uintptr_t window, int closable, const char *ids, size_t idsSize, const char *titles,
                           size_t titlesSize, const int *enabled, int count) {
    GKCocoaInfoBar *bar = [[GKCocoaInfoBar alloc] initWithFrame:NSZeroRect];
    bar.icon = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
    bar.message = [NSTextField wrappingLabelWithString:@""];
    bar.close = [NSButton buttonWithImage:[NSImage imageWithSystemSymbolName:@"xmark" accessibilityDescription:@"Close"]
                                   target:bar
                                   action:@selector(dismiss:)];
    bar.close.bordered = NO;
    bar.close.hidden = !closable;
    bar.buttons = gkActionButtons(bar, @selector(action:), ids, idsSize, titles, titlesSize, enabled, count);
    for (NSView *view in @[ bar.icon, bar.message, bar.close ]) {
        [bar addSubview:view];
    }
    for (NSButton *button in bar.buttons) {
        [bar addSubview:button];
    }
    bar.accessibilityRole = NSAccessibilityGroupRole;
    gkAdd(window, bar);
    return bar;
}

void gkCocoaInfoBarSet(void *pointer, int severity, const char *title, const char *detail) {
    GKCocoaInfoBar *bar = pointer;
    bar.severity = severity;
    bar.icon.image = gkSeverityImage(severity, 15);
    bar.icon.contentTintColor = gkSeverityColor(severity);
    NSMutableAttributedString *text = [[[NSMutableAttributedString alloc]
        initWithString:gkString(title)
            attributes:@{NSFontAttributeName : [NSFont boldSystemFontOfSize:NSFont.systemFontSize]}] autorelease];
    NSString *more = gkString(detail);
    if (more.length) {
        [text appendAttributedString:[[[NSAttributedString alloc]
                                         initWithString:[@"  " stringByAppendingString:more]
                                             attributes:@{
                                                 NSFontAttributeName : [NSFont systemFontOfSize:NSFont.systemFontSize]
                                             }] autorelease]];
    }
    bar.message.attributedStringValue = text;
    bar.accessibilityLabel = text.string;
    [bar setNeedsLayout:YES];
    [bar setNeedsDisplay:YES];
}

/* ---- Tooltip ---- */

@interface GKCocoaTooltip : NSObject
@property(retain) NSPanel *panel;
@property(retain) NSTextField *label;
@property(retain) NSTimer *timer;
@end

@implementation GKCocoaTooltip
- (void)dealloc {
    [_timer invalidate];
    [_timer release];
    [_panel close];
    [_panel release];
    [_label release];
    [super dealloc];
}
@end

void *gkCocoaTooltipCreate(void) {
    GKCocoaTooltip *tooltip = [GKCocoaTooltip new];
    tooltip.panel =
        [[[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 100, 24)
                                    styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
                                      backing:NSBackingStoreBuffered
                                        defer:YES] autorelease];
    tooltip.panel.releasedWhenClosed = NO;
    tooltip.panel.hasShadow = YES;
    tooltip.panel.opaque = NO;
    tooltip.panel.backgroundColor = NSColor.clearColor;
    tooltip.panel.level = NSStatusWindowLevel;
    tooltip.panel.ignoresMouseEvents = YES;
    NSVisualEffectView *background = [[[NSVisualEffectView alloc] initWithFrame:NSZeroRect] autorelease];
    background.material = NSVisualEffectMaterialToolTip;
    background.state = NSVisualEffectStateActive;
    background.wantsLayer = YES;
    background.layer.cornerRadius = 5;
    tooltip.panel.contentView = background;
    tooltip.label = [NSTextField labelWithString:@""];
    tooltip.label.font = [NSFont toolTipsFontOfSize:0];
    [background addSubview:tooltip.label];
    tooltip.panel.accessibilityRole = NSAccessibilityHelpTagRole;
    return tooltip;
}

void gkCocoaTooltipSetText(void *pointer, const char *text) {
    GKCocoaTooltip *tooltip = pointer;
    tooltip.label.stringValue = gkString(text);
}

int gkCocoaTooltipShow(void *pointer, void *anchorPointer, int edge, double delay) {
    GKCocoaTooltip *tooltip = pointer;
    NSView *anchor = anchorPointer;
    if (!anchor.window || !anchor.window.isVisible) {
        return 0;
    }
    [tooltip.timer invalidate];
    NSSize text = tooltip.label.fittingSize;
    NSSize size = NSMakeSize(text.width + 12, text.height + 6);
    tooltip.label.frame = NSMakeRect(6, 3, text.width, text.height);
    NSRect anchorRect = [anchor.window convertRectToScreen:[anchor convertRect:anchor.bounds toView:nil]];
    NSPoint origin;
    switch (edge) {
    case 0:
        origin = NSMakePoint(NSMidX(anchorRect) - size.width / 2, NSMaxY(anchorRect) + 4);
        break;
    case 2:
        origin = NSMakePoint(NSMinX(anchorRect) - size.width - 4, NSMidY(anchorRect) - size.height / 2);
        break;
    case 3:
        origin = NSMakePoint(NSMaxX(anchorRect) + 4, NSMidY(anchorRect) - size.height / 2);
        break;
    default:
        origin = NSMakePoint(NSMidX(anchorRect) - size.width / 2, NSMinY(anchorRect) - size.height - 4);
        break;
    }
    [tooltip.panel setFrame:NSMakeRect(origin.x, origin.y, size.width, size.height) display:NO];
    if (delay <= 0) {
        [tooltip.panel orderFront:nil];
    } else {
        NSPanel *panel = tooltip.panel;
        tooltip.timer = [NSTimer scheduledTimerWithTimeInterval:delay
                                                        repeats:NO
                                                          block:^(NSTimer *timer) {
                                                            (void)timer;
                                                            [panel orderFront:nil];
                                                          }];
    }
    return 1;
}

void gkCocoaTooltipHide(void *pointer) {
    GKCocoaTooltip *tooltip = pointer;
    [tooltip.timer invalidate];
    tooltip.timer = nil;
    [tooltip.panel orderOut:nil];
}

int gkCocoaTooltipShown(void *pointer) {
    GKCocoaTooltip *tooltip = pointer;
    return tooltip.panel.isVisible || tooltip.timer.isValid;
}

/* ---- Toast ---- */

@interface GKCocoaToast : NSVisualEffectView
@property(retain) NSTextField *message;
@property(retain) NSArray<NSButton *> *buttons;
@property(retain) NSTimer *timer;
@property uintptr_t handle;
@property NSTimeInterval timeout;
@end

@implementation GKCocoaToast
- (void)dealloc {
    [_timer invalidate];
    [_timer release];
    [_message release];
    [_buttons release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (NSSize)fittingSize {
    CGFloat width = self.message.fittingSize.width + 32;
    for (NSButton *button in self.buttons) {
        width += button.fittingSize.width + 8;
    }
    return NSMakeSize(MIN(width, 480), 40);
}
- (void)layout {
    [super layout];
    CGFloat right = self.bounds.size.width - 12;
    for (NSButton *button in self.buttons.reverseObjectEnumerator) {
        NSSize size = button.fittingSize;
        button.frame = NSMakeRect(right - size.width, (40 - size.height) / 2, size.width, size.height);
        right -= size.width + 8;
    }
    NSSize text = self.message.fittingSize;
    self.message.frame = NSMakeRect(16, (40 - text.height) / 2, MAX(0, right - 16), text.height);
}
- (void)action:(NSButton *)sender {
    gkCocoaSend(self.handle, GK_EVENT_ACTION, 0, 0, 0, 0, sender.identifier);
    [self dismiss];
}
- (void)dismiss {
    [self.timer invalidate];
    self.timer = nil;
    [self removeFromSuperview];
}
@end

void *gkCocoaToastCreate(uintptr_t handle, const char *message, const char *ids, size_t idsSize, const char *titles,
                         size_t titlesSize, const int *enabled, int count, double timeout) {
    GKCocoaToast *toast = [[GKCocoaToast alloc] initWithFrame:NSMakeRect(0, 0, 200, 40)];
    toast.handle = handle;
    toast.timeout = timeout;
    toast.material = NSVisualEffectMaterialHUDWindow;
    toast.state = NSVisualEffectStateActive;
    toast.wantsLayer = YES;
    toast.layer.cornerRadius = 10;
    toast.message = [NSTextField labelWithString:gkString(message)];
    toast.message.lineBreakMode = NSLineBreakByTruncatingTail;
    toast.buttons = gkActionButtons(toast, @selector(action:), ids, idsSize, titles, titlesSize, enabled, count);
    [toast addSubview:toast.message];
    for (NSButton *button in toast.buttons) {
        button.bezelStyle = NSBezelStyleAccessoryBarAction;
        [toast addSubview:button];
    }
    toast.accessibilityRole = NSAccessibilityGroupRole;
    toast.accessibilityLabel = toast.message.stringValue;
    return toast;
}

int gkCocoaToastShow(void *pointer, void *windowPointer) {
    GKCocoaToast *toast = pointer;
    NSWindow *window = windowPointer;
    NSView *content = window.contentView;
    if (!content) {
        return 0;
    }
    NSSize size = toast.fittingSize;
    NSRect bounds = content.bounds;
    CGFloat y = content.isFlipped ? NSMaxY(bounds) - size.height - 20 : 20;
    toast.frame = NSMakeRect(NSMidX(bounds) - size.width / 2, y, size.width, size.height);
    toast.autoresizingMask =
        NSViewMinXMargin | NSViewMaxXMargin | (content.isFlipped ? NSViewMinYMargin : NSViewMaxYMargin);
    [content addSubview:toast positioned:NSWindowAbove relativeTo:nil];
    [toast.timer invalidate];
    toast.timer = [NSTimer scheduledTimerWithTimeInterval:toast.timeout
                                                  repeats:NO
                                                    block:^(NSTimer *timer) {
                                                      (void)timer;
                                                      [toast dismiss];
                                                    }];
    NSAccessibilityPostNotificationWithUserInfo(
        NSApp, NSAccessibilityAnnouncementRequestedNotification, @{
            NSAccessibilityAnnouncementKey : toast.message.stringValue,
            NSAccessibilityPriorityKey : @(NSAccessibilityPriorityMedium),
        });
    return 1;
}

void gkCocoaToastDismiss(void *pointer) {
    [(GKCocoaToast *)pointer dismiss];
}

int gkCocoaToastShown(void *pointer) {
    return ((GKCocoaToast *)pointer).superview != nil;
}

void gkCocoaToastRelease(void *pointer) {
    GKCocoaToast *toast = pointer;
    toast.handle = 0;
    [toast dismiss];
    [toast release];
}

/* ---- Badge ---- */

@interface GKCocoaBadge : NSTextField
@end

@implementation GKCocoaBadge
+ (Class)cellClass {
    return GKUICenteredTextFieldCell.class;
}
- (NSSize)fittingSize {
    NSSize size = [super fittingSize];
    return NSMakeSize(MAX(18, size.width + 10), 18);
}
@end

void *gkCocoaBadgeCreate(uintptr_t window) {
    GKCocoaBadge *badge = [[GKCocoaBadge alloc] initWithFrame:NSZeroRect];
    badge.editable = NO;
    badge.selectable = NO;
    badge.bordered = NO;
    badge.drawsBackground = NO;
    badge.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize weight:NSFontWeightSemibold];
    badge.textColor = NSColor.whiteColor;
    badge.alignment = NSTextAlignmentCenter;
    badge.wantsLayer = YES;
    badge.layer.cornerRadius = 9;
    badge.layer.masksToBounds = YES;
    gkAdd(window, badge);
    return badge;
}

void gkCocoaBadgeSet(void *pointer, const char *text, int severity, const char *accessible) {
    GKCocoaBadge *badge = pointer;
    badge.stringValue = gkString(text);
    badge.layer.backgroundColor = gkSeverityColor(severity).CGColor;
    NSString *value = gkString(accessible);
    badge.accessibilityValueDescription = value.length ? value : badge.stringValue;
}

/* ---- Progress dialog ---- */

@interface GKCocoaProgressDialog : NSObject
@property(retain) NSPanel *panel;
@property(retain) NSTextField *message;
@property(retain) NSProgressIndicator *progress;
@property(assign) NSWindow *parent;
@property uintptr_t handle;
@property BOOL sheet;
@end

@implementation GKCocoaProgressDialog
- (void)dealloc {
    [_panel release];
    [_message release];
    [_progress release];
    [super dealloc];
}
- (void)cancel:(id)sender {
    (void)sender;
    gkCocoaSend(self.handle, GK_EVENT_CLOSE, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaProgressDialogCreate(uintptr_t handle, const char *title, int cancellable) {
    GKCocoaProgressDialog *dialog = [GKCocoaProgressDialog new];
    dialog.handle = handle;
    dialog.panel = [[[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 360, 120)
                                               styleMask:NSWindowStyleMaskTitled
                                                 backing:NSBackingStoreBuffered
                                                   defer:YES] autorelease];
    dialog.panel.releasedWhenClosed = NO;
    dialog.panel.title = gkString(title);
    NSView *content = dialog.panel.contentView;
    NSTextField *heading = [NSTextField labelWithString:gkString(title)];
    heading.font = [NSFont boldSystemFontOfSize:NSFont.systemFontSize];
    heading.frame = NSMakeRect(20, 86, 320, 18);
    dialog.message = [NSTextField labelWithString:@""];
    dialog.message.frame = NSMakeRect(20, 64, 320, 18);
    dialog.message.lineBreakMode = NSLineBreakByTruncatingTail;
    dialog.progress = [[[NSProgressIndicator alloc] initWithFrame:NSMakeRect(20, 40, 320, 16)] autorelease];
    dialog.progress.minValue = 0;
    dialog.progress.maxValue = 1;
    [content addSubview:heading];
    [content addSubview:dialog.message];
    [content addSubview:dialog.progress];
    if (cancellable) {
        NSButton *cancel = [NSButton buttonWithTitle:@"Cancel" target:dialog action:@selector(cancel:)];
        cancel.keyEquivalent = @"\x1b";
        NSSize size = cancel.fittingSize;
        cancel.frame = NSMakeRect(340 - size.width, 8, size.width, size.height);
        [content addSubview:cancel];
    }
    return dialog;
}

void gkCocoaProgressDialogSet(void *pointer, const char *message, double value) {
    GKCocoaProgressDialog *dialog = pointer;
    dialog.message.stringValue = gkString(message);
    if (value < 0) {
        if (!dialog.progress.indeterminate) {
            dialog.progress.indeterminate = YES;
            [dialog.progress startAnimation:nil];
        }
    } else {
        if (dialog.progress.indeterminate) {
            [dialog.progress stopAnimation:nil];
            dialog.progress.indeterminate = NO;
        }
        dialog.progress.doubleValue = MIN(1, value);
    }
}

int gkCocoaProgressDialogShow(void *pointer, void *parentPointer) {
    GKCocoaProgressDialog *dialog = pointer;
    NSWindow *parent = parentPointer;
    if (dialog.panel.isVisible) {
        return 1;
    }
    dialog.parent = parent;
    if (parent && parent.isVisible && !parent.attachedSheet) {
        dialog.sheet = YES;
        [parent beginSheet:dialog.panel completionHandler:nil];
    } else {
        dialog.sheet = NO;
        [dialog.panel center];
        [dialog.panel orderFront:nil];
    }
    return 1;
}

void gkCocoaProgressDialogRelease(void *pointer) {
    GKCocoaProgressDialog *dialog = pointer;
    dialog.handle = 0;
    if (dialog.sheet && dialog.parent.attachedSheet == dialog.panel) {
        [dialog.parent endSheet:dialog.panel];
    }
    [dialog.panel orderOut:nil];
    [dialog release];
}

/* ---- Empty state ---- */

@interface GKCocoaEmptyState : GKUILayoutView
@property(retain) NSImageView *icon;
@property(retain) NSTextField *title;
@property(retain) NSTextField *detail;
@property(retain) NSArray<NSButton *> *buttons;
@end

@implementation GKCocoaEmptyState
- (void)dealloc {
    [_icon release];
    [_title release];
    [_detail release];
    [_buttons release];
    [super dealloc];
}
- (CGFloat)contentHeightForWidth:(CGFloat)width {
    CGFloat height = self.icon.image ? 56 : 0;
    self.title.preferredMaxLayoutWidth = width;
    self.detail.preferredMaxLayoutWidth = width;
    height += self.title.fittingSize.height + 6;
    if (self.detail.stringValue.length) {
        height += self.detail.fittingSize.height + 12;
    }
    if (self.buttons.count) {
        height += 32;
    }
    return height;
}
- (void)layout {
    [super layout];
    CGFloat width = MIN(self.bounds.size.width - 32, 360);
    CGFloat height = [self contentHeightForWidth:width];
    CGFloat x = (self.bounds.size.width - width) / 2;
    CGFloat y = MAX(0, (self.bounds.size.height - height) / 2);
    if (self.icon.image) {
        self.icon.frame = NSMakeRect(NSMidX(self.bounds) - 24, y, 48, 48);
        y += 56;
    }
    NSSize title = self.title.fittingSize;
    self.title.frame = NSMakeRect(x, y, width, title.height);
    y += title.height + 6;
    if (self.detail.stringValue.length) {
        NSSize detail = self.detail.fittingSize;
        self.detail.frame = NSMakeRect(x, y, width, detail.height);
        y += detail.height + 12;
    }
    CGFloat total = 0;
    for (NSButton *button in self.buttons) {
        total += button.fittingSize.width + 8;
    }
    CGFloat bx = NSMidX(self.bounds) - (total - 8) / 2;
    for (NSButton *button in self.buttons) {
        NSSize size = button.fittingSize;
        button.frame = NSMakeRect(bx, y, size.width, size.height);
        bx += size.width + 8;
    }
}
- (NSSize)fittingSize {
    return NSMakeSize(280, [self contentHeightForWidth:280]);
}
- (void)action:(NSButton *)sender {
    gkCocoaSendView(self, GK_EVENT_ACTION, 0, 0, 0, 0, sender.identifier);
}
@end

void *gkCocoaEmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t idsSize, const char *titles,
                              size_t titlesSize, const int *enabled, int count) {
    GKCocoaEmptyState *view = [[GKCocoaEmptyState alloc] initWithFrame:NSZeroRect];
    view.icon = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
    NSImage *image = icon;
    NSImageSymbolConfiguration *configuration =
        [NSImageSymbolConfiguration configurationWithPointSize:40 weight:NSFontWeightLight];
    image = [image imageWithSymbolConfiguration:configuration] ?: image;
    view.icon.image = image;
    view.icon.contentTintColor = NSColor.tertiaryLabelColor;
    view.title = [NSTextField wrappingLabelWithString:@""];
    view.title.font = [NSFont preferredFontForTextStyle:NSFontTextStyleTitle3 options:@{}];
    view.title.alignment = NSTextAlignmentCenter;
    view.detail = [NSTextField wrappingLabelWithString:@""];
    view.detail.textColor = NSColor.secondaryLabelColor;
    view.detail.alignment = NSTextAlignmentCenter;
    view.buttons = gkActionButtons(view, @selector(action:), ids, idsSize, titles, titlesSize, enabled, count);
    for (NSView *subview in @[ view.icon, view.title, view.detail ]) {
        [view addSubview:subview];
    }
    for (NSButton *button in view.buttons) {
        [view addSubview:button];
    }
    view.accessibilityRole = NSAccessibilityGroupRole;
    gkAdd(window, view);
    return view;
}

void gkCocoaEmptyStateSet(void *pointer, const char *title, const char *detail) {
    GKCocoaEmptyState *view = pointer;
    view.title.stringValue = gkString(title);
    view.detail.stringValue = gkString(detail);
    view.accessibilityLabel = view.title.stringValue;
    [view setNeedsLayout:YES];
}

/* ---- Inline validation ---- */

@interface GKCocoaInlineValidation : GKUILayoutView
@property(retain) NSImageView *icon;
@property(retain) NSTextField *label;
@end

@implementation GKCocoaInlineValidation
- (void)dealloc {
    [_icon release];
    [_label release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.label;
}
- (void)layout {
    [super layout];
    self.icon.frame = NSMakeRect(0, 1, 14, 14);
    self.label.preferredMaxLayoutWidth = MAX(0, self.bounds.size.width - 18);
    NSSize size = self.label.fittingSize;
    self.label.frame = NSMakeRect(18, 0, MAX(0, self.bounds.size.width - 18), size.height);
}
- (NSSize)fittingSize {
    NSSize size = self.label.fittingSize;
    return NSMakeSize(size.width + 18, MAX(16, size.height));
}
@end

void *gkCocoaInlineValidationCreate(uintptr_t window) {
    GKCocoaInlineValidation *view = [[GKCocoaInlineValidation alloc] initWithFrame:NSZeroRect];
    view.icon = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
    view.label = [NSTextField wrappingLabelWithString:@""];
    view.label.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize];
    [view addSubview:view.icon];
    [view addSubview:view.label];
    gkAdd(window, view);
    return view;
}

void gkCocoaInlineValidationSet(void *pointer, int severity, const char *text) {
    GKCocoaInlineValidation *view = pointer;
    view.icon.image = gkSeverityImage(severity, 11);
    view.icon.contentTintColor = gkSeverityColor(severity);
    view.label.stringValue = gkString(text);
    view.label.textColor = severity >= 2 ? gkSeverityColor(severity) : NSColor.secondaryLabelColor;
    [view setNeedsLayout:YES];
}
