#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);

static NSString *gkString(const char *value) {
    return value ? [NSString stringWithUTF8String:value] : @"";
}

static void gkAdd(uintptr_t window, NSView *view) {
    [((NSWindow *)window).contentView addSubview:view];
}

/* gkCocoaAddChild moves child into container's content area. */
void gkCocoaAddChild(void *container, void *child) {
    NSView *view = container;
    if ([view respondsToSelector:@selector(gkContentView)]) {
        view = [(id<GKUIContainer>)view gkContentView];
    }
    [(NSView *)child removeFromSuperview];
    [view addSubview:(NSView *)child];
}

/* GKCocoaClipView is a flipped container that clips its children. */
@interface GKCocoaClipView : GKUILayoutView
@end

@implementation GKCocoaClipView
- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        self.wantsLayer = YES;
        self.layer.masksToBounds = YES;
    }
    return self;
}
@end

void *gkCocoaClipViewCreate(uintptr_t window) {
    GKCocoaClipView *view = [[GKCocoaClipView alloc] initWithFrame:NSZeroRect];
    gkAdd(window, view);
    return view;
}

/* ---- Group box ---- */

@interface GKCocoaGroupBox : NSBox <GKUIContainer>
@property(retain) GKUILayoutView *body;
@property(retain) NSTextField *subtitleLabel;
@end

@implementation GKCocoaGroupBox
- (void)dealloc {
    [_body release];
    [_subtitleLabel release];
    [super dealloc];
}
- (NSView *)gkContentView {
    return self.body;
}
@end

void *gkCocoaGroupBoxCreate(uintptr_t window, const char *title, const char *subtitle) {
    GKCocoaGroupBox *box = [[GKCocoaGroupBox alloc] initWithFrame:NSZeroRect];
    box.boxType = NSBoxPrimary;
    box.titlePosition = NSAtTop;
    box.body = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    box.subtitleLabel = [NSTextField wrappingLabelWithString:@""];
    box.subtitleLabel.textColor = NSColor.secondaryLabelColor;
    box.subtitleLabel.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize];
    [box.body addSubview:box.subtitleLabel];
    box.contentView = box.body;
    box.title = gkString(title);
    box.subtitleLabel.stringValue = gkString(subtitle);
    box.subtitleLabel.hidden = box.subtitleLabel.stringValue.length == 0;
    box.accessibilityLabel = box.title;
    gkAdd(window, box);
    return box;
}

void gkCocoaGroupBoxSet(void *pointer, const char *title, const char *subtitle) {
    GKCocoaGroupBox *box = pointer;
    box.title = gkString(title);
    box.accessibilityLabel = box.title;
    box.subtitleLabel.stringValue = gkString(subtitle);
    box.subtitleLabel.hidden = box.subtitleLabel.stringValue.length == 0;
}

/* gkCocoaGroupBoxBody reports the content area size and the space the
   subtitle takes at its top. */
void gkCocoaGroupBoxBody(void *pointer, int *width, int *height, int *top) {
    GKCocoaGroupBox *box = pointer;
    [box layoutSubtreeIfNeeded];
    NSSize size = box.contentView.frame.size;
    CGFloat subtitle = 0;
    if (!box.subtitleLabel.hidden) {
        box.subtitleLabel.preferredMaxLayoutWidth = MAX(0, size.width - 8);
        NSSize text = box.subtitleLabel.fittingSize;
        box.subtitleLabel.frame = NSMakeRect(4, 2, MAX(0, size.width - 8), text.height);
        subtitle = text.height + 6;
    }
    *width = (int)floor(size.width);
    *height = (int)floor(size.height);
    *top = (int)ceil(subtitle);
}

/* gkCocoaGroupBoxChrome reports the space the frame and title take. */
void gkCocoaGroupBoxChrome(void *pointer, int *width, int *height) {
    GKCocoaGroupBox *box = pointer;
    NSRect frame = box.frame;
    NSSize content = box.contentView.frame.size;
    *width = (int)ceil(MAX(16, frame.size.width - content.width));
    *height = (int)ceil(MAX(28, frame.size.height - content.height));
}

/* ---- Split view ---- */

@interface GKCocoaSplitView : NSSplitView <NSSplitViewDelegate>
@property(retain) NSMutableArray<NSNumber *> *minimums;
@property(retain) NSMutableArray<NSNumber *> *collapsible;
@end

@implementation GKCocoaSplitView
- (void)dealloc {
    self.delegate = nil;
    [_minimums release];
    [_collapsible release];
    [super dealloc];
}
- (CGFloat)splitView:(NSSplitView *)view constrainMinCoordinate:(CGFloat)proposed ofSubviewAt:(NSInteger)index {
    CGFloat start = 0;
    NSArray *panes = view.arrangedSubviews;
    if (index < (NSInteger)panes.count) {
        NSRect frame = [panes[index] frame];
        start = view.isVertical ? NSMinX(frame) : NSMinY(frame);
    }
    CGFloat minimum = index < (NSInteger)self.minimums.count ? self.minimums[index].doubleValue : 0;
    return MAX(proposed, start + minimum);
}
- (CGFloat)splitView:(NSSplitView *)view constrainMaxCoordinate:(CGFloat)proposed ofSubviewAt:(NSInteger)index {
    NSArray *panes = view.arrangedSubviews;
    if (index + 1 >= (NSInteger)panes.count) {
        return proposed;
    }
    NSRect next = [panes[index + 1] frame];
    CGFloat end = view.isVertical ? NSMaxX(next) : NSMaxY(next);
    CGFloat minimum = index + 1 < (NSInteger)self.minimums.count ? self.minimums[index + 1].doubleValue : 0;
    return MIN(proposed, end - minimum - view.dividerThickness);
}
- (BOOL)splitView:(NSSplitView *)view canCollapseSubview:(NSView *)subview {
    NSUInteger index = [view.arrangedSubviews indexOfObject:subview];
    return index < self.collapsible.count && self.collapsible[index].boolValue;
}
- (void)splitViewDidResizeSubviews:(NSNotification *)notification {
    (void)notification;
    gkCocoaSendView(self, GK_EVENT_RESIZE, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaSplitViewCreate(uintptr_t window, int vertical, int panes) {
    GKCocoaSplitView *split = [[GKCocoaSplitView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300)];
    split.vertical = vertical != 0;
    split.dividerStyle = NSSplitViewDividerStyleThin;
    split.minimums = [NSMutableArray array];
    split.collapsible = [NSMutableArray array];
    for (int i = 0; i < panes; i++) {
        GKCocoaClipView *pane = [[[GKCocoaClipView alloc] initWithFrame:NSMakeRect(0, 0, 100, 100)] autorelease];
        [split addArrangedSubview:pane];
        [split.minimums addObject:@0];
        [split.collapsible addObject:@NO];
    }
    split.delegate = split;
    gkAdd(window, split);
    return split;
}

void *gkCocoaSplitViewPane(void *pointer, int index) {
    NSArray *panes = ((NSSplitView *)pointer).arrangedSubviews;
    return index >= 0 && index < (int)panes.count ? panes[index] : NULL;
}

void gkCocoaSplitViewSetPane(void *pointer, int index, double minimum, int collapsible) {
    GKCocoaSplitView *split = pointer;
    if (index >= 0 && index < (int)split.minimums.count) {
        split.minimums[index] = @(minimum);
        split.collapsible[index] = @(collapsible != 0);
    }
}

void gkCocoaSplitViewPaneFrame(void *pointer, int index, int *x, int *y, int *width, int *height) {
    NSView *pane = gkCocoaSplitViewPane(pointer, index);
    NSRect frame = pane ? pane.frame : NSZeroRect;
    if (pane.isHidden) {
        frame.size = NSZeroSize;
    }
    *x = (int)frame.origin.x;
    *y = (int)frame.origin.y;
    *width = (int)floor(frame.size.width);
    *height = (int)floor(frame.size.height);
}

double gkCocoaSplitViewDivider(void *pointer, int index) {
    NSSplitView *split = pointer;
    NSView *pane = gkCocoaSplitViewPane(pointer, index);
    if (!pane) {
        return 0;
    }
    return split.isVertical ? NSMaxX(pane.frame) : NSMaxY(pane.frame);
}

void gkCocoaSplitViewSetDivider(void *pointer, int index, double position) {
    NSSplitView *split = pointer;
    [split layoutSubtreeIfNeeded];
    [split setPosition:position ofDividerAtIndex:index];
    [split adjustSubviews];
}

/* ---- Popover ---- */

@interface GKCocoaPopover : NSObject <NSPopoverDelegate>
@property(retain) NSPopover *popover;
@property(retain) GKUILayoutView *body;
@property uintptr_t handle;
@end

@implementation GKCocoaPopover
- (void)dealloc {
    self.popover.delegate = nil;
    [_popover release];
    [_body release];
    [super dealloc];
}
- (void)popoverDidClose:(NSNotification *)notification {
    (void)notification;
    GKCocoaEvent event = {0};
    event.kind = GK_EVENT_CLOSE;
    if (self.handle) {
        gkCocoaGoEvent(self.handle, &event);
    }
}
@end

void *gkCocoaPopoverCreate(uintptr_t handle, int transient) {
    GKCocoaPopover *popover = [GKCocoaPopover new];
    popover.handle = handle;
    popover.body = [[[GKUILayoutView alloc] initWithFrame:NSMakeRect(0, 0, 200, 100)] autorelease];
    NSViewController *controller = [[NSViewController new] autorelease];
    controller.view = popover.body;
    popover.popover = [[NSPopover new] autorelease];
    popover.popover.contentViewController = controller;
    popover.popover.behavior = transient ? NSPopoverBehaviorTransient : NSPopoverBehaviorApplicationDefined;
    popover.popover.delegate = popover;
    return popover;
}

void *gkCocoaPopoverBody(void *pointer) {
    return ((GKCocoaPopover *)pointer).body;
}

int gkCocoaPopoverShow(void *pointer, void *anchorPointer, int edge, int width, int height) {
    GKCocoaPopover *popover = pointer;
    NSView *anchor = anchorPointer;
    if (!anchor.window || !anchor.window.isVisible) {
        return 0;
    }
    static const NSRectEdge edges[] = {NSRectEdgeMaxY, NSRectEdgeMinY, NSRectEdgeMinX, NSRectEdgeMaxX};
    NSRectEdge preferred = edges[edge >= 0 && edge < 4 ? edge : 1];
    if (anchor.isFlipped && (preferred == NSRectEdgeMaxY || preferred == NSRectEdgeMinY)) {
        preferred = preferred == NSRectEdgeMaxY ? NSRectEdgeMinY : NSRectEdgeMaxY;
    }
    popover.popover.contentSize = NSMakeSize(MAX(1, width), MAX(1, height));
    [popover.popover showRelativeToRect:anchor.bounds ofView:anchor preferredEdge:preferred];
    return 1;
}

void gkCocoaPopoverHide(void *pointer) {
    [((GKCocoaPopover *)pointer).popover close];
}

int gkCocoaPopoverShown(void *pointer) {
    return ((GKCocoaPopover *)pointer).popover.isShown;
}

void gkCocoaPopoverRelease(void *pointer) {
    GKCocoaPopover *popover = pointer;
    popover.handle = 0;
    [popover.popover close];
    [popover release];
}

/* ---- Title bar accessory ---- */

int gkCocoaTitleAccessoryAdd(void *windowPointer, void *viewPointer, int alignment, int width, int height,
                             void **controller) {
    NSWindow *window = windowPointer;
    if (!(window.styleMask & NSWindowStyleMaskTitled)) {
        return 0;
    }
    NSTitlebarAccessoryViewController *accessory = [NSTitlebarAccessoryViewController new];
    NSView *view = viewPointer;
    view.frame = NSMakeRect(0, 0, MAX(1, width), MAX(1, height));
    accessory.view = view;
    accessory.layoutAttribute = alignment == 1   ? NSLayoutAttributeLeft
                                : alignment == 3 ? NSLayoutAttributeRight
                                                 : NSLayoutAttributeBottom;
    [window addTitlebarAccessoryViewController:accessory];
    *controller = accessory;
    return 1;
}

void gkCocoaTitleAccessoryRemove(void *pointer) {
    NSTitlebarAccessoryViewController *accessory = pointer;
    [accessory removeFromParentViewController];
    [accessory release];
}

void gkCocoaViewSize(void *pointer, int *width, int *height) {
    NSView *view = pointer;
    *width = (int)floor(view.bounds.size.width);
    *height = (int)floor(view.bounds.size.height);
}
