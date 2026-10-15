
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include "widget_bridge.h"
#include "widget_classes.h"
#include "window_bridge.h"
#include "cocoa_bridge.h"
#import <QuartzCore/QuartzCore.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

@implementation GKUILayoutView
- (BOOL)isFlipped {
    return YES;
}
@end

int gkMacMods(NSEventModifierFlags flags); /* window_darwin.c */
int gkCocoaMacKey(unsigned short code);    /* window_darwin.c */

/* A canvas reports the pointer events Cocoa delivers to it, which are exactly the
   ones that are its own: a press goes to the view under the pointer, the drags and
   the release to the view the press began in, and the wheel to the view under the
   pointer — so scrolling a scroll view beside the canvas never reaches it. */

@implementation GKUICanvasView
/* The click that activates the window is also the canvas's: without this, the first
   press into an inactive window would only bring it forward. */
- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (void)gkReportButton:(NSEvent *)event action:(int)action {
    int button = (int)[event buttonNumber];
    if (self.handle && button >= 0 && button < GK_POINTER_BUTTON_COUNT) {
        gkUIGoCanvasPointerButton(self.handle, button, action, gkMacMods([event modifierFlags]));
    }
}

- (void)mouseDown:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_PRESSED];
}
- (void)mouseUp:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_RELEASED];
}
- (void)rightMouseDown:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_PRESSED];
}
- (void)rightMouseUp:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_RELEASED];
}
- (void)otherMouseDown:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_PRESSED];
}
- (void)otherMouseUp:(NSEvent *)event {
    [self gkReportButton:event action:GK_POINTER_RELEASED];
}

- (void)dealloc {
    [self gkStopFrames];
    if (self.observing) {
        [NSNotificationCenter.defaultCenter removeObserver:self];
    }
    [_trackingArea release];
    [super dealloc];
}
- (void)gkSend:(int)kind a:(int)a b:(int)b c:(int)c x:(double)x y:(double)y z:(double)z w:(double)w {
    if (!self.handle) {
        return;
    }
    GKCocoaEvent event = {0};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.c = c;
    event.x = x;
    event.y = y;
    event.z = z;
    event.w = w;
    gkCocoaGoEvent(self.handle, &event);
}
- (BOOL)acceptsFirstResponder {
    return YES;
}
- (void)updateTrackingAreas {
    if (self.trackingArea) {
        [self removeTrackingArea:self.trackingArea];
    }
    self.trackingArea = [[[NSTrackingArea alloc] initWithRect:NSZeroRect
                                                      options:NSTrackingMouseEnteredAndExited | NSTrackingMouseMoved |
                                                              NSTrackingActiveAlways | NSTrackingInVisibleRect
                                                        owner:self
                                                     userInfo:nil] autorelease];
    [self addTrackingArea:self.trackingArea];
    [super updateTrackingAreas];
}
- (NSPoint)gkLocation:(NSEvent *)event {
    return [self convertPoint:event.locationInWindow fromView:nil];
}
- (void)mouseEntered:(NSEvent *)event {
    NSPoint point = [self gkLocation:event];
    [self gkSend:GK_EVENT_POINTER_ENTER a:0 b:0 c:0 x:point.x y:point.y z:0 w:0];
}
- (void)mouseExited:(NSEvent *)event {
    NSPoint point = [self gkLocation:event];
    [self gkSend:GK_EVENT_POINTER_LEAVE a:0 b:0 c:0 x:point.x y:point.y z:0 w:0];
}
- (void)gkReportMove:(NSEvent *)event {
    NSPoint point = [self gkLocation:event];
    [self gkSend:GK_EVENT_POINTER_MOVE
               a:gkMacMods(event.modifierFlags)
               b:0
               c:0
               x:point.x
               y:point.y
               z:event.deltaX
               w:event.deltaY];
}
- (void)mouseMoved:(NSEvent *)event {
    [self gkReportMove:event];
}
- (void)mouseDragged:(NSEvent *)event {
    [self gkReportMove:event];
}
- (void)rightMouseDragged:(NSEvent *)event {
    [self gkReportMove:event];
}
- (void)otherMouseDragged:(NSEvent *)event {
    [self gkReportMove:event];
}
- (void)gkReportKey:(NSEvent *)event action:(int)action {
    int key = gkCocoaMacKey(event.keyCode);
    if (key < 0) {
        return;
    }
    [self gkSend:GK_EVENT_KEY a:key b:event.keyCode c:action x:gkMacMods(event.modifierFlags) y:0 z:0 w:0];
}
- (void)keyDown:(NSEvent *)event {
    [self gkReportKey:event action:event.isARepeat ? GK_KEY_REPEAT : GK_KEY_PRESSED];
    [self interpretKeyEvents:@[ event ]];
}
- (void)keyUp:(NSEvent *)event {
    [self gkReportKey:event action:GK_KEY_RELEASED];
}
- (void)insertText:(id)string {
    NSString *text = [string isKindOfClass:NSAttributedString.class] ? [string string] : string;
    [text
        enumerateSubstringsInRange:NSMakeRange(0, text.length)
                           options:NSStringEnumerationByComposedCharacterSequences
                        usingBlock:^(NSString *character, NSRange range, NSRange enclosing, BOOL *stop) {
                          (void)range;
                          (void)enclosing;
                          (void)stop;
                          UTF32Char codepoint = 0;
                          if ([character getBytes:&codepoint
                                        maxLength:sizeof(codepoint)
                                       usedLength:NULL
                                         encoding:NSUTF32LittleEndianStringEncoding
                                          options:0
                                            range:NSMakeRange(0, character.length)
                                   remainingRange:NULL] &&
                              codepoint >= 0x20 && codepoint != 0x7F && !(codepoint >= 0xF700 && codepoint <= 0xF8FF)) {
                              [self gkSend:GK_EVENT_TEXT a:(int)codepoint b:0 c:0 x:0 y:0 z:0 w:0];
                          }
                        }];
}
- (void)doCommandBySelector:(SEL)selector {
    (void)selector;
}
- (void)gkFrame:(id)link {
    double interval = [link respondsToSelector:@selector(duration)] ? [link duration] : 1.0 / 60;
    double target = [link respondsToSelector:@selector(targetTimestamp)] ? [link targetTimestamp] : 0;
    double now = CACurrentMediaTime();
    [self gkSend:GK_EVENT_FRAME a:0 b:0 c:0 x:MAX(0, target - now) y:interval z:0 w:0];
}
- (BOOL)gkStartFrames {
    if (self.displayLink) {
        return YES;
    }
    if (![self respondsToSelector:@selector(displayLinkWithTarget:selector:)]) {
        return NO;
    }
    id link = [self displayLinkWithTarget:self selector:@selector(gkFrame:)];
    [link addToRunLoop:NSRunLoop.mainRunLoop forMode:NSRunLoopCommonModes];
    self.displayLink = link;
    return YES;
}
- (void)gkStopFrames {
    [self.displayLink invalidate];
    self.displayLink = nil;
}
- (void)gkReportSurface:(int)kind {
    NSSize size = [self convertSizeToBacking:self.bounds.size];
    [self gkSend:GK_EVENT_SURFACE
               a:kind
               b:gkCocoaViewColorSpace(self)
               c:0
               x:size.width
               y:size.height
               z:gkCocoaViewScale(self)
               w:0];
}
- (void)setFrameSize:(NSSize)size {
    NSSize previous = self.frame.size;
    [super setFrameSize:size];
    if (!NSEqualSizes(previous, size)) {
        [self gkReportSurface:0];
    }
}
- (void)viewDidChangeBackingProperties {
    [super viewDidChangeBackingProperties];
    [self gkReportSurface:1];
}
- (void)gkOcclusion:(NSNotification *)notification {
    NSWindow *window = notification.object;
    int occluded = (window.occlusionState & NSWindowOcclusionStateVisible) == 0;
    [self gkSend:GK_EVENT_WINDOW_OCCLUSION a:occluded b:0 c:0 x:0 y:0 z:0 w:0];
}
- (void)gkScreen:(NSNotification *)notification {
    (void)notification;
    [self gkReportSurface:2];
}
- (void)viewWillMoveToWindow:(NSWindow *)window {
    if (self.observing) {
        [NSNotificationCenter.defaultCenter removeObserver:self];
        self.observing = NO;
    }
    if (window) {
        [NSNotificationCenter.defaultCenter addObserver:self
                                               selector:@selector(gkOcclusion:)
                                                   name:NSWindowDidChangeOcclusionStateNotification
                                                 object:window];
        [NSNotificationCenter.defaultCenter addObserver:self
                                               selector:@selector(gkScreen:)
                                                   name:NSWindowDidChangeScreenNotification
                                                 object:window];
        self.observing = YES;
    }
    [super viewWillMoveToWindow:window];
}

- (void)scrollWheel:(NSEvent *)event {
    if (!self.handle) {
        return;
    }
    double dx = [event scrollingDeltaX];
    double dy = [event scrollingDeltaY];
    /* The same normalisation as the window's own scrolling (gkHandlePointerEvent):
       a wheel's lines become roughly one unit per detent, like a trackpad's pixels. */
    if (![event hasPreciseScrollingDeltas]) {
        dx *= 0.1;
        dy *= 0.1;
    }
    gkUIGoCanvasScroll(self.handle, dx, dy);
}
@end

@implementation GKUITextAreaView
- (instancetype)initWithText:(NSString *)text
                 placeholder:(NSString *)placeholder
                    readOnly:(BOOL)readOnly
                        wrap:(BOOL)wrap {
    self = [super initWithFrame:NSMakeRect(0, 0, 200, 96)];
    if (!self) {
        return nil;
    }
    self.hasVerticalScroller = YES;
    self.hasHorizontalScroller = !wrap;
    self.borderType = NSBezelBorder;
    self.editor = [[[NSTextView alloc] initWithFrame:NSZeroRect] autorelease];
    self.editor.string = text ?: @"";
    self.editor.editable = !readOnly;
    self.editor.delegate = self;
    self.editor.verticallyResizable = YES;
    self.editor.horizontallyResizable = !wrap;
    self.editor.textContainer.widthTracksTextView = wrap;
    self.documentView = self.editor;
    self.placeholder = placeholder ?: @"";
    return self;
}
- (void)dealloc {
    self.editor.delegate = nil;
    [_editor release];
    [_placeholder release];
    [super dealloc];
}
- (void)textDidChange:(NSNotification *)notification {
    (void)notification;
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_TEXT_CHANGE);
    }
}
- (BOOL)textView:(NSTextView *)textView shouldChangeTextInRange:(NSRange)range replacementString:(NSString *)text {
    if (self.maximumLength <= 0 || !text) {
        return YES;
    }
    NSUInteger length = textView.string.length - range.length + text.length;
    return length <= (NSUInteger)self.maximumLength;
}
- (BOOL)textView:(NSTextView *)textView doCommandBySelector:(SEL)selector {
    if (!self.tabMovesFocus) {
        return NO;
    }
    if (selector == @selector(insertTab:)) {
        [textView.window selectNextKeyView:nil];
        return YES;
    }
    if (selector == @selector(insertBacktab:)) {
        [textView.window selectPreviousKeyView:nil];
        return YES;
    }
    return NO;
}
@end

@implementation GKUIRadioGroupView
- (instancetype)init {
    self = [super initWithFrame:NSMakeRect(0, 0, 160, 24)];
    if (self) {
        self.buttons = [NSMutableArray array];
    }
    return self;
}
- (void)dealloc {
    [_buttons release];
    [_titleLabel release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (void)addItem:(NSString *)title {
    NSButton *button = [NSButton radioButtonWithTitle:title ?: @"" target:self action:@selector(select:)];
    button.tag = self.buttons.count;
    [self.buttons addObject:button];
    [self addSubview:button];
    [self setNeedsLayout:YES];
}
- (void)layout {
    [super layout];
    CGFloat y = 0;
    if (self.titleLabel && !self.titleLabel.hidden) {
        self.titleLabel.frame = NSMakeRect(0, 0, self.bounds.size.width, 18);
        y = 22;
    }
    CGFloat x = 0;
    for (NSButton *button in self.buttons) {
        if (self.horizontal) {
            CGFloat width = button.fittingSize.width;
            button.frame = NSMakeRect(x, y, width, 22);
            x += width + 12;
        } else {
            button.frame = NSMakeRect(0, y, self.bounds.size.width, 22);
            y += 24;
        }
    }
}
- (void)setSelectedIndex:(NSInteger)selectedIndex {
    _selectedIndex = selectedIndex;
    for (NSInteger i = 0; i < (NSInteger)self.buttons.count; i++) {
        self.buttons[i].state = i == selectedIndex ? NSControlStateValueOn : NSControlStateValueOff;
    }
}
- (void)select:(NSButton *)sender {
    self.selectedIndex = sender.tag;
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_SELECTION_CHANGE);
    }
}
@end

@implementation GKUILinkButton
- (void)dealloc {
    [_linkURL release];
    [super dealloc];
}
@end

@implementation GKUIActivityIndicator
@end

@implementation GKUIImageView
- (void)dealloc {
    [_image release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (NSSize)fittingSize {
    if (!self.image) {
        return NSZeroSize;
    }
    CGFloat scale = self.sourceScale > 0 ? self.sourceScale : 1;
    return NSMakeSize(self.image.size.width / scale, self.image.size.height / scale);
}
- (void)drawRect:(NSRect)dirtyRect {
    (void)dirtyRect;
    if (!self.image) {
        return;
    }
    NSSize source = self.image.size;
    NSRect destination = self.bounds;
    if (self.scaling != 2 && source.width > 0 && source.height > 0) {
        CGFloat sx = self.bounds.size.width / source.width;
        CGFloat sy = self.bounds.size.height / source.height;
        CGFloat scale = self.scaling == 1 ? MAX(sx, sy) : MIN(sx, sy);
        destination.size = NSMakeSize(source.width * scale, source.height * scale);
        /* Anchor order matches ui.Anchor: center, then clockwise from top-left. */
        CGFloat freeX = self.bounds.size.width - destination.size.width;
        CGFloat freeY = self.bounds.size.height - destination.size.height;
        static const CGFloat horizontal[] = {0.5, 0, 0.5, 1, 0, 1, 0, 0.5, 1};
        static const CGFloat vertical[] = {0.5, 0, 0, 0, 0.5, 0.5, 1, 1, 1};
        NSInteger anchor = self.alignment >= 0 && self.alignment < 9 ? self.alignment : 0;
        destination.origin.x = freeX * horizontal[anchor];
        destination.origin.y = freeY * vertical[anchor];
    }
    [NSGraphicsContext saveGraphicsState];
    NSGraphicsContext.currentContext.imageInterpolation =
        self.interpolation ? self.interpolation : NSImageInterpolationHigh;
    NSRectClip(self.bounds);
    [self.image drawInRect:destination
                  fromRect:NSZeroRect
                 operation:NSCompositingOperationSourceOver
                  fraction:1
            respectFlipped:YES
                     hints:nil];
    [NSGraphicsContext restoreGraphicsState];
}
@end

@implementation GKUITabsView
- (instancetype)init {
    self = [super initWithFrame:NSMakeRect(0, 0, 320, 240)];
    if (self) {
        self.delegate = self;
    }
    return self;
}
- (void)dealloc {
    self.delegate = nil;
    [super dealloc];
}
- (void)tabView:(NSTabView *)tabView didSelectTabViewItem:(NSTabViewItem *)item {
    (void)tabView;
    (void)item;
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_TAB_CHANGE);
    }
}
@end

@implementation GKUIScrollView
- (instancetype)initWithAxes:(NSInteger)axes {
    self = [super initWithFrame:NSMakeRect(0, 0, 320, 240)];
    if (!self) {
        return nil;
    }
    self.hasVerticalScroller = axes == 0 || axes == 2;
    self.hasHorizontalScroller = axes == 1 || axes == 2;
    self.autohidesScrollers = YES;
    self.borderType = NSBezelBorder;
    self.drawsBackground = YES;
    self.bodyView = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    self.documentView = self.bodyView;
    return self;
}
- (void)dealloc {
    [_bodyView release];
    [super dealloc];
}
- (NSView *)gkContentView {
    return self.bodyView;
}
@end

@implementation GKUISectionView
- (instancetype)initWithTitle:(NSString *)title expanded:(BOOL)expanded {
    self = [super initWithFrame:NSMakeRect(0, 0, 160, 28)];
    if (!self) {
        return nil;
    }
    self.sectionTitle = title ?: @"";
    self.header = [NSButton buttonWithTitle:@"" target:self action:@selector(toggle:)];
    self.header.bezelStyle = NSBezelStyleAccessoryBarAction;
    self.header.alignment = NSTextAlignmentLeft;
    self.bodyView = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    [self addSubview:self.header];
    [self addSubview:self.bodyView];
    [self setSectionExpanded:expanded];
    return self;
}
- (void)dealloc {
    [_sectionTitle release];
    [_subtitle release];
    [_badgeLabel release];
    [_header release];
    [_bodyView release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (NSView *)gkContentView {
    return self.bodyView;
}
- (void)layout {
    [super layout];
    self.header.frame = NSMakeRect(0, 0, self.bounds.size.width, 28);
    if (self.badgeLabel) {
        NSSize badge = self.badgeLabel.fittingSize;
        self.badgeLabel.frame = NSMakeRect(self.bounds.size.width - badge.width - 12, (28 - badge.height) / 2,
                                           badge.width + 4, badge.height);
    }
    self.bodyView.frame = NSMakeRect(0, 28, self.bounds.size.width, MAX(0, self.bounds.size.height - 28));
}
- (void)updateHeader {
    NSString *marker = self.expanded ? @"▾" : @"▸";
    NSString *title = [NSString stringWithFormat:@"%@  %@", marker, self.sectionTitle ?: @""];
    if (!self.subtitle.length) {
        self.header.attributedTitle = [[[NSAttributedString alloc] initWithString:title] autorelease];
        self.header.title = title;
        return;
    }
    NSMutableAttributedString *value = [[[NSMutableAttributedString alloc] initWithString:title] autorelease];
    NSDictionary *secondary = @{
        NSForegroundColorAttributeName : NSColor.secondaryLabelColor,
        NSFontAttributeName : [NSFont systemFontOfSize:NSFont.smallSystemFontSize],
    };
    [value
        appendAttributedString:[[[NSAttributedString alloc] initWithString:[@"  " stringByAppendingString:self.subtitle]
                                                                attributes:secondary] autorelease]];
    self.header.attributedTitle = value;
}
- (void)setSectionExpanded:(BOOL)expanded {
    _expanded = expanded;
    self.bodyView.hidden = !expanded;
    [self updateHeader];
}
- (void)toggle:(id)sender {
    (void)sender;
    [self setSectionExpanded:!self.expanded];
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_SECTION_CHANGE);
    }
}
@end

@implementation GKUIAction
- (void)send:(id)sender {
    (void)sender;
    if (self.handle) {
        gkUIGoEvent(self.handle, self.event);
    }
}
- (void)submit:(id)sender {
    (void)sender;
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_SUBMIT);
    }
}
- (void)controlTextDidChange:(NSNotification *)notification {
    (void)notification;
    if (self.handle) {
        gkUIGoEvent(self.handle, GK_UI_EVENT_TEXT_CHANGE);
    }
}
- (void)controlTextDidBeginEditing:(NSNotification *)notification {
    GKUITextField *field = notification.object;
    NSTextView *editor = notification.userInfo[@"NSFieldEditor"];
    if (![field isKindOfClass:GKUITextField.class] || !editor) {
        return;
    }
    editor.continuousSpellCheckingEnabled = field.spellCheck;
    if (field.pendingSelection.location != NSNotFound) {
        editor.selectedRange = field.pendingSelection;
    }
}
- (BOOL)control:(NSControl *)control textView:(NSTextView *)textView doCommandBySelector:(SEL)selector {
    (void)control;
    (void)textView;
    (void)selector;
    return NO;
}
@end

static char gkUIActionKey;

static NSWindow *gkUIWindow(uintptr_t window) {
    return (NSWindow *)window;
}

static NSView *gkUIView(void *control) {
    return (NSView *)control;
}

uintptr_t gkUIControlHandle(void *control) {
    return (uintptr_t)control;
}

static void gkUIAddToWindow(uintptr_t window, NSView *view) {
    [gkUIWindow(window).contentView addSubview:view];
}

static void gkUIInstallAction(NSControl *control, uintptr_t handle, int event) {
    GKUIAction *action = [GKUIAction new];
    action.handle = handle;
    action.event = event;
    objc_setAssociatedObject(control, &gkUIActionKey, action, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    if ([control isKindOfClass:NSTextField.class] && event == GK_UI_EVENT_TEXT_CHANGE) {
        ((NSTextField *)control).delegate = action;
    } else {
        control.target = action;
        control.action = @selector(send:);
    }
    [action release];
}

void *gkUICreateLayout(uintptr_t window) {
    GKUILayoutView *view = [GKUILayoutView new];
    gkUIAddToWindow(window, view);
    return view;
}

void *gkUICreateLabel(uintptr_t window, const char *text) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSTextField *label = [[NSTextField labelWithString:value] retain];
    gkUIAddToWindow(window, label);
    return label;
}

void *gkUICreateButton(uintptr_t window, const char *text, uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSButton *button = [[NSButton buttonWithTitle:value target:nil action:nil] retain];
    gkUIInstallAction(button, handle, GK_UI_EVENT_CLICK);
    gkUIAddToWindow(window, button);
    return button;
}

void *gkUICreateTextField(uintptr_t window, uintptr_t handle) {
    GKUITextField *field = [[GKUITextField alloc] init];
    field.pendingSelection = NSMakeRange(NSNotFound, 0);
    gkUIInstallAction(field, handle, GK_UI_EVENT_TEXT_CHANGE);
    GKUIAction *action = objc_getAssociatedObject(field, &gkUIActionKey);
    field.target = action;
    field.action = @selector(submit:);
    gkUIAddToWindow(window, field);
    return field;
}

void *gkUICreateTextArea(uintptr_t window, const char *text, const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSString *hint = placeholder ? [NSString stringWithUTF8String:placeholder] : @"";
    GKUITextAreaView *area =
        [[GKUITextAreaView alloc] initWithText:value placeholder:hint readOnly:readOnly != 0 wrap:wrap != 0];
    area.handle = handle;
    gkUIAddToWindow(window, area);
    return area;
}

void *gkUICreateCheckBox(uintptr_t window, const char *text, int checked, uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSButton *button = [[NSButton checkboxWithTitle:value target:nil action:nil] retain];
    button.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    gkUIInstallAction(button, handle, GK_UI_EVENT_TOGGLE_CHANGE);
    gkUIAddToWindow(window, button);
    return button;
}

void *gkUICreateRadioGroup(uintptr_t window, uintptr_t handle) {
    GKUIRadioGroupView *group = [GKUIRadioGroupView new];
    group.handle = handle;
    gkUIAddToWindow(window, group);
    return group;
}

void *gkUICreateSelect(uintptr_t window, uintptr_t handle) {
    NSPopUpButton *select = [[NSPopUpButton alloc] initWithFrame:NSZeroRect pullsDown:NO];
    gkUIInstallAction(select, handle, GK_UI_EVENT_SELECTION_CHANGE);
    gkUIAddToWindow(window, select);
    return select;
}

void gkUISelectionAddItem(void *control, const char *text) {
    if (!control) {
        return;
    }
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        [(GKUIRadioGroupView *)view addItem:value];
    } else if ([view isKindOfClass:NSPopUpButton.class]) {
        [(NSPopUpButton *)view addItemWithTitle:value];
    }
}

int gkUISelectionIndex(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        return (int)((GKUIRadioGroupView *)view).selectedIndex;
    }
    if ([view isKindOfClass:NSPopUpButton.class]) {
        return (int)((NSPopUpButton *)view).indexOfSelectedItem;
    }
    return -1;
}

void gkUISetSelectionIndex(void *control, int index) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        ((GKUIRadioGroupView *)view).selectedIndex = index;
    } else if ([view isKindOfClass:NSPopUpButton.class]) {
        [(NSPopUpButton *)view selectItemAtIndex:index];
    }
}

void *gkUICreateSlider(uintptr_t window, double minimum, double maximum, double value, double step, uintptr_t handle) {
    NSSlider *slider =
        [[NSSlider sliderWithValue:value minValue:minimum maxValue:maximum target:nil action:nil] retain];
    if (step > 0) {
        slider.numberOfTickMarks = (NSInteger)floor((maximum - minimum) / step) + 1;
        slider.allowsTickMarkValuesOnly = YES;
    }
    gkUIInstallAction(slider, handle, GK_UI_EVENT_VALUE_CHANGE);
    gkUIAddToWindow(window, slider);
    return slider;
}

void *gkUICreateProgressBar(uintptr_t window, double value) {
    NSProgressIndicator *progress = [[NSProgressIndicator alloc] initWithFrame:NSZeroRect];
    progress.indeterminate = NO;
    progress.minValue = 0;
    progress.maxValue = 1;
    progress.doubleValue = value;
    gkUIAddToWindow(window, progress);
    return progress;
}

void *gkUICreateActivityIndicator(uintptr_t window, int running) {
    GKUIActivityIndicator *activity = [[GKUIActivityIndicator alloc] initWithFrame:NSZeroRect];
    activity.indeterminate = YES;
    activity.style = NSProgressIndicatorStyleSpinning;
    activity.running = running != 0;
    if (running) {
        [activity startAnimation:nil];
    }
    gkUIAddToWindow(window, activity);
    return activity;
}

void *gkUICreateSeparator(uintptr_t window, int direction) {
    GKUISeparatorView *separator = [[GKUISeparatorView alloc] initWithFrame:NSZeroRect];
    separator.vertical = direction == 1;
    separator.thickness = 1;
    gkUIAddToWindow(window, separator);
    return separator;
}

void *gkUICreateImage(uintptr_t window, int scaling) {
    GKUIImageView *image = [[GKUIImageView alloc] initWithFrame:NSZeroRect];
    image.scaling = scaling;
    image.sourceScale = 1;
    gkUIAddToWindow(window, image);
    gkUIImageSetScaling(image, scaling);
    return image;
}

void gkUIImageSet(void *control, const unsigned char *pixels, int width, int height, int stride) {
    if (!control || !pixels || width <= 0 || height <= 0) {
        return;
    }
    NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL
                                                                       pixelsWide:width
                                                                       pixelsHigh:height
                                                                    bitsPerSample:8
                                                                  samplesPerPixel:4
                                                                         hasAlpha:YES
                                                                         isPlanar:NO
                                                                   colorSpaceName:NSCalibratedRGBColorSpace
                                                                     bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
                                                                      bytesPerRow:width * 4
                                                                     bitsPerPixel:32];
    if (!bitmap) {
        return;
    }
    for (int y = 0; y < height; y++) {
        memcpy(bitmap.bitmapData + y * bitmap.bytesPerRow, pixels + y * stride, (size_t)width * 4);
    }
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
    [image addRepresentation:bitmap];
    ((GKUIImageView *)gkUIView(control)).image = image;
    [image release];
    [bitmap release];
}

int gkUIImageScaling(void *control) {
    GKUIImageView *view = control ? (GKUIImageView *)gkUIView(control) : nil;
    return view ? (int)view.scaling : 0;
}

void gkUIImageSetScaling(void *control, int scaling) {
    GKUIImageView *view = control ? (GKUIImageView *)gkUIView(control) : nil;
    if (!view) {
        return;
    }
    view.scaling = scaling;
    [view setNeedsDisplay:YES];
}

static void gkUIStyleLink(GKUILinkButton *link) {
    NSString *title = link.title ?: @"";
    NSDictionary *attributes = @{
        NSForegroundColorAttributeName : link.visited ? NSColor.systemPurpleColor : NSColor.linkColor,
        NSUnderlineStyleAttributeName : @(NSUnderlineStyleSingle),
    };
    link.attributedTitle = [[[NSAttributedString alloc] initWithString:title attributes:attributes] autorelease];
    link.bordered = NO;
    link.alignment = NSTextAlignmentLeft;
}

void *gkUICreateLink(uintptr_t window, const char *text, const char *url, uintptr_t handle) {
    NSString *title = text ? [NSString stringWithUTF8String:text] : @"";
    GKUILinkButton *link = [[GKUILinkButton alloc] initWithFrame:NSZeroRect];
    link.title = title;
    link.linkURL = url ? [NSString stringWithUTF8String:url] : @"";
    gkUIStyleLink(link);
    gkUIInstallAction(link, handle, GK_UI_EVENT_CLICK);
    gkUIAddToWindow(window, link);
    return link;
}

void *gkUICreateTabs(uintptr_t window, uintptr_t handle) {
    GKUITabsView *tabs = [GKUITabsView new];
    tabs.handle = handle;
    gkUIAddToWindow(window, tabs);
    return tabs;
}

int gkUITabsAdd(void *tabsPointer, const char *title, void *content) {
    if (!tabsPointer || !content) {
        return 0;
    }
    GKUITabsView *tabs = (GKUITabsView *)gkUIView(tabsPointer);
    NSString *label = title ? [NSString stringWithUTF8String:title] : @"";
    NSTabViewItem *item = [[[NSTabViewItem alloc] initWithIdentifier:nil] autorelease];
    item.label = label;
    GKUILayoutView *page = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    item.view = page;
    [tabs addTabViewItem:item];
    [gkUIView(content) removeFromSuperview];
    [page addSubview:gkUIView(content)];
    return 1;
}

int gkUITabsSelected(void *tabsPointer) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    return tabs ? (int)[tabs indexOfTabViewItem:tabs.selectedTabViewItem] : -1;
}

void gkUITabsSetSelected(void *tabsPointer, int index) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    if (tabs && index >= 0 && index < (int)tabs.numberOfTabViewItems) {
        uintptr_t handle = tabs.handle;
        tabs.handle = 0;
        [tabs selectTabViewItemAtIndex:index];
        tabs.handle = handle;
    }
}

void gkUITabsContentSize(void *tabsPointer, int *width, int *height) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    NSSize size = tabs.selectedTabViewItem.view.bounds.size;
    if (width) {
        *width = (int)floor(size.width);
    }
    if (height) {
        *height = (int)floor(size.height);
    }
}

void *gkUICreateScrollView(uintptr_t window, int axes) {
    GKUIScrollView *scroll = [[GKUIScrollView alloc] initWithAxes:axes];
    gkUIAddToWindow(window, scroll);
    return scroll;
}

void gkUIScrollViewportSize(void *pointer, int *width, int *height) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    NSSize size = scroll ? scroll.contentSize : NSZeroSize;
    if (width) {
        *width = (int)floor(size.width);
    }
    if (height) {
        *height = (int)floor(size.height);
    }
}

void gkUIScrollSetContentSize(void *pointer, int width, int height) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    if (!scroll) {
        return;
    }
    [scroll.bodyView setFrameSize:NSMakeSize(MAX(0, width), MAX(0, height))];
    [scroll reflectScrolledClipView:scroll.contentView];
}

void gkUIScrollOffset(void *pointer, int *x, int *y) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    NSPoint offset = scroll ? scroll.documentVisibleRect.origin : NSZeroPoint;
    if (x) {
        *x = (int)floor(offset.x);
    }
    if (y) {
        *y = (int)floor(offset.y);
    }
}

void *gkUICreateCanvas(uintptr_t window, uintptr_t handle) {
    GKUICanvasView *view = [GKUICanvasView new];
    view.handle = handle;
    view.wantsLayer = YES;
    gkUIAddToWindow(window, view);
    return view;
}

void gkUICanvasPointerPosition(void *canvas, double *x, double *y) {
    NSView *view = gkUIView(canvas);
    NSPoint point = NSZeroPoint;
    if (view.window) {
        /* The view is flipped, so this is already from its top-left corner. */
        point = [view convertPoint:[view.window mouseLocationOutsideOfEventStream] fromView:nil];
    }
    if (x) {
        *x = point.x;
    }
    if (y) {
        *y = point.y;
    }
}

void *gkUICreateSection(uintptr_t window, const char *title, int expanded, uintptr_t handle) {
    NSString *value = title ? [NSString stringWithUTF8String:title] : @"";
    GKUISectionView *section = [[GKUISectionView alloc] initWithTitle:value expanded:expanded != 0];
    section.handle = handle;
    gkUIAddToWindow(window, section);
    return section;
}

void gkUIDestroy(void *control) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    [view removeFromSuperview];
    [view release];
}

void gkUIClearAction(void *control) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUICanvasView.class]) {
        GKUICanvasView *canvas = (GKUICanvasView *)view;
        canvas.handle = 0;
        [canvas gkStopFrames];
        gkCocoaReleaseCaptureFor(canvas);
    }
    if ([view isKindOfClass:GKUISectionView.class]) {
        GKUISectionView *section = (GKUISectionView *)view;
        section.handle = 0;
        section.header.target = nil;
        section.header.action = nil;
    }
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        GKUITextAreaView *area = (GKUITextAreaView *)view;
        area.handle = 0;
        area.editor.delegate = nil;
    }
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        ((GKUIRadioGroupView *)view).handle = 0;
    }
    if ([view isKindOfClass:GKUITabsView.class]) {
        ((GKUITabsView *)view).handle = 0;
    }
    GKUIAction *action = objc_getAssociatedObject(view, &gkUIActionKey);
    action.handle = 0;
    if ([view isKindOfClass:NSTextField.class]) {
        ((NSTextField *)view).delegate = nil;
    }
    if ([view isKindOfClass:NSControl.class]) {
        ((NSControl *)view).target = nil;
        ((NSControl *)view).action = nil;
    }
    objc_setAssociatedObject(view, &gkUIActionKey, nil, OBJC_ASSOCIATION_ASSIGN);
}

void gkUIAttachToWindow(uintptr_t window, void *control) {
    if (!window || !control) {
        return;
    }
    gkUIAddToWindow(window, gkUIView(control));
}

void gkUISetParent(void *control, void *parent) {
    if (!control || !parent) {
        return;
    }
    NSView *parentView = gkUIView(parent);
    NSView *contentView = parentView;
    if ([parentView respondsToSelector:@selector(gkContentView)]) {
        contentView = [(id<GKUIContainer>)parentView gkContentView];
    }
    [contentView addSubview:gkUIView(control)];
}

void gkUIBringToFront(void *control) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    if (view.superview) {
        [view.superview addSubview:view positioned:NSWindowAbove relativeTo:nil];
    }
}

void gkUISetFrame(void *control, int x, int y, int width, int height) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    CGFloat nativeY = y;
    if (view.superview && !view.superview.isFlipped) {
        nativeY = view.superview.bounds.size.height - y - MAX(0, height);
    }
    view.frame = NSMakeRect(x, nativeY, MAX(0, width), MAX(0, height));
}

void gkUISetVisible(void *control, int visible) {
    if (!control) {
        return;
    }
    gkUIView(control).hidden = !visible;
}

void gkUISetEnabled(void *control, int enabled) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:NSControl.class]) {
        ((NSControl *)view).enabled = enabled;
    }
    for (NSView *child in view.subviews) {
        gkUISetEnabled(child, enabled);
    }
}

void gkUIPreferredSize(void *control, int *width, int *height) {
    NSSize size = NSZeroSize;
    if (control) {
        NSView *view = gkUIView(control);
        size = view.fittingSize;
        if ([view isKindOfClass:GKUITextAreaView.class]) {
            size = NSMakeSize(200, 96);
        }
        if ([view isKindOfClass:GKUIRadioGroupView.class]) {
            int groupWidth = 0, groupHeight = 0;
            gkCocoaRadioGroupPreferredSize(view, &groupWidth, &groupHeight);
            size = NSMakeSize(groupWidth, groupHeight);
        }
        if ([view isKindOfClass:GKUITabsView.class] || [view isKindOfClass:GKUIScrollView.class]) {
            size = NSMakeSize(320, 240);
        }
    }
    if (width) {
        *width = (int)ceil(size.width);
    }
    if (height) {
        *height = (int)ceil(size.height);
    }
}

void gkUIFramebufferSize(void *control, int *width, int *height) {
    NSView *view = gkUIView(control);
    CGFloat scale = view.window ? view.window.backingScaleFactor : 1;
    if (width) {
        *width = (int)ceil(view.bounds.size.width * scale);
    }
    if (height) {
        *height = (int)ceil(view.bounds.size.height * scale);
    }
}

char *gkUIText(void *control) {
    if (!control) {
        return strdup("");
    }
    NSView *view = gkUIView(control);
    NSString *value = @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        value = ((GKUITextAreaView *)view).editor.string;
    } else if ([view isKindOfClass:GKUISectionView.class]) {
        value = ((GKUISectionView *)view).sectionTitle;
    } else if ([view isKindOfClass:NSButton.class]) {
        value = ((NSButton *)view).title;
    } else if ([view respondsToSelector:@selector(stringValue)]) {
        value = [(id)view stringValue];
    }
    return strdup(value.UTF8String ?: "");
}

void gkUISetText(void *control, const char *text) {
    if (!control) {
        return;
    }
    NSView *view = gkUIView(control);
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).editor.string = value;
    } else if ([view isKindOfClass:GKUISectionView.class]) {
        GKUISectionView *section = (GKUISectionView *)view;
        section.sectionTitle = value;
        [section updateHeader];
    } else if ([view isKindOfClass:GKUILinkButton.class]) {
        ((GKUILinkButton *)view).title = value;
        gkUIStyleLink((GKUILinkButton *)view);
    } else if ([view isKindOfClass:NSButton.class]) {
        ((NSButton *)view).title = value;
    } else if ([view respondsToSelector:@selector(setStringValue:)]) {
        [(id)view setStringValue:value];
    }
}

void gkUISetPlaceholder(void *control, const char *text) {
    if (!control) {
        return;
    }
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).placeholder = value;
    } else if ([view isKindOfClass:NSTextField.class]) {
        ((NSTextField *)view).placeholderString = value;
    }
}

char *gkUIPlaceholder(void *control) {
    if (!control) {
        return strdup("");
    }
    NSView *view = gkUIView(control);
    NSString *value = @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        value = ((GKUITextAreaView *)view).placeholder ?: @"";
    } else if ([view isKindOfClass:NSTextField.class]) {
        value = ((NSTextField *)view).placeholderString ?: @"";
    }
    return strdup(value.UTF8String ?: "");
}

int gkUIChecked(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:NSButton.class] && ((NSButton *)view).state == NSControlStateValueOn;
}

void gkUISetChecked(void *control, int checked) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:NSButton.class]) {
        ((NSButton *)view).state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    }
}

int gkUIReadOnly(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:GKUITextAreaView.class] && !((GKUITextAreaView *)view).editor.editable;
}

void gkUISetReadOnly(void *control, int readOnly) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).editor.editable = readOnly == 0;
    }
}

double gkUISliderValue(void *control) {
    if (!control || ![gkUIView(control) isKindOfClass:NSSlider.class]) {
        return 0;
    }
    return ((NSSlider *)gkUIView(control)).doubleValue;
}

void gkUISetSliderValue(void *control, double value) {
    if (control && [gkUIView(control) isKindOfClass:NSSlider.class]) {
        ((NSSlider *)gkUIView(control)).doubleValue = value;
    }
}

double gkUIProgressValue(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:NSProgressIndicator.class] ? ((NSProgressIndicator *)view).doubleValue : 0;
}

void gkUISetProgressValue(void *control, double value) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:NSProgressIndicator.class]) {
        ((NSProgressIndicator *)view).doubleValue = MAX(0, MIN(value, 1));
    }
}

int gkUIActivityRunning(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:GKUIActivityIndicator.class] && ((GKUIActivityIndicator *)view).running;
}

void gkUISetActivityRunning(void *control, int running) {
    NSView *view = control ? gkUIView(control) : nil;
    if (![view isKindOfClass:GKUIActivityIndicator.class]) {
        return;
    }
    GKUIActivityIndicator *activity = (GKUIActivityIndicator *)view;
    activity.running = running != 0;
    if (running) {
        [activity startAnimation:nil];
    } else {
        [activity stopAnimation:nil];
    }
}

char *gkUILinkURL(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    NSString *url = [view isKindOfClass:GKUILinkButton.class] ? ((GKUILinkButton *)view).linkURL : @"";
    return strdup(url.UTF8String ?: "");
}

void gkUISetLinkURL(void *control, const char *url) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUILinkButton.class]) {
        ((GKUILinkButton *)view).linkURL = url ? [NSString stringWithUTF8String:url] : @"";
    }
}

int gkUISectionExpanded(void *control) {
    if (!control || ![gkUIView(control) isKindOfClass:GKUISectionView.class]) {
        return 0;
    }
    return ((GKUISectionView *)gkUIView(control)).expanded;
}

void gkUISectionSetExpanded(void *control, int expanded) {
    if (control && [gkUIView(control) isKindOfClass:GKUISectionView.class]) {
        [(GKUISectionView *)gkUIView(control) setSectionExpanded:expanded != 0];
    }
}

static NSString *gkUIString(const char *value) {
    return value && value[0] ? [NSString stringWithUTF8String:value] : nil;
}

static NSArray<NSString *> *gkUIExtensions(const char *value) {
    NSString *list = gkUIString(value);
    if (!list) {
        return nil;
    }
    return [list componentsSeparatedByString:@","];
}

static void gkUIConfigurePanel(NSSavePanel *panel, const char *title, const char *directory, const char *filename,
                               const char *extensions) {
    NSString *panelTitle = gkUIString(title);
    if (panelTitle) {
        panel.title = panelTitle;
    }
    NSString *initialDirectory = gkUIString(directory);
    if (initialDirectory) {
        panel.directoryURL = [NSURL fileURLWithPath:initialDirectory isDirectory:YES];
    }
    NSString *initialFilename = gkUIString(filename);
    if (initialFilename) {
        panel.nameFieldStringValue = initialFilename;
    }
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    panel.allowedFileTypes = gkUIExtensions(extensions);
#pragma clang diagnostic pop
}

static char *gkUIPaths(NSArray<NSURL *> *urls, size_t *resultSize) {
    size_t size = 0;
    for (NSURL *url in urls) {
        size += strlen(url.path.fileSystemRepresentation) + 1;
    }
    if (resultSize) {
        *resultSize = size;
    }
    if (!size) {
        return NULL;
    }

    char *result = malloc(size);
    if (!result) {
        return NULL;
    }
    char *next = result;
    for (NSURL *url in urls) {
        const char *path = url.path.fileSystemRepresentation;
        size_t length = strlen(path) + 1;
        memcpy(next, path, length);
        next += length;
    }
    return result;
}

char *gkUIOpenPanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories, int hidden, int packages, size_t *resultSize,
                    int *cancelled) {
    (void)window;
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    gkUIConfigurePanel(panel, title, directory, filename, extensions);
    panel.showsHiddenFiles = hidden != 0;
    panel.treatsFilePackagesAsDirectories = packages == 0;
    panel.allowsMultipleSelection = multiple;
    panel.canChooseDirectories = directories;
    panel.canChooseFiles = !directories;
    if ([panel runModal] != NSModalResponseOK) {
        if (cancelled) {
            *cancelled = 1;
        }
        if (resultSize) {
            *resultSize = 0;
        }
        return NULL;
    }
    if (cancelled) {
        *cancelled = 0;
    }
    return gkUIPaths(panel.URLs, resultSize);
}

char *gkUISavePanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int hidden, const char *defaultExtension, size_t *resultSize,
                    int *cancelled) {
    (void)window;
    NSSavePanel *panel = [NSSavePanel savePanel];
    gkUIConfigurePanel(panel, title, directory, filename, extensions);
    panel.showsHiddenFiles = hidden != 0;
    if (defaultExtension && defaultExtension[0]) {
        UTType *type = [UTType typeWithFilenameExtension:[NSString stringWithUTF8String:defaultExtension]];
        if (type && !panel.allowedContentTypes.count) {
            panel.allowedContentTypes = @[ type ];
            panel.allowsOtherFileTypes = YES;
        }
    }
    if ([panel runModal] != NSModalResponseOK) {
        if (cancelled) {
            *cancelled = 1;
        }
        if (resultSize) {
            *resultSize = 0;
        }
        return NULL;
    }
    if (cancelled) {
        *cancelled = 0;
    }
    return gkUIPaths(@[ panel.URL ], resultSize);
}

int gkCocoaCanvasStartFrames(void *pointer, int start) {
    NSView *view = pointer;
    if (![view isKindOfClass:GKUICanvasView.class]) {
        return 0;
    }
    GKUICanvasView *canvas = (GKUICanvasView *)view;
    if (start) {
        return [canvas gkStartFrames];
    }
    [canvas gkStopFrames];
    return 1;
}

void gkCocoaCanvasAttach(void *pointer, uintptr_t handle) {
    NSView *view = pointer;
    if ([view isKindOfClass:GKUICanvasView.class]) {
        ((GKUICanvasView *)view).handle = handle;
    }
}

void gkCocoaDeliverCaptured(void *pointer, void *eventPointer) {
    NSView *view = pointer;
    NSEvent *event = eventPointer;
    if (![view isKindOfClass:GKUICanvasView.class]) {
        return;
    }
    GKUICanvasView *canvas = (GKUICanvasView *)view;
    switch (event.type) {
    case NSEventTypeLeftMouseDown:
    case NSEventTypeRightMouseDown:
    case NSEventTypeOtherMouseDown:
        [canvas gkReportButton:event action:GK_POINTER_PRESSED];
        break;
    case NSEventTypeLeftMouseUp:
    case NSEventTypeRightMouseUp:
    case NSEventTypeOtherMouseUp:
        [canvas gkReportButton:event action:GK_POINTER_RELEASED];
        break;
    case NSEventTypeScrollWheel:
        [canvas scrollWheel:event];
        break;
    default:
        [canvas gkReportMove:event];
        break;
    }
}
