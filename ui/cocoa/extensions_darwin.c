#import <Cocoa/Cocoa.h>
#import <CoreImage/CoreImage.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_bridge.h"
#include "widget_classes.h"

NSColor *gkCocoaSemanticColor(int color);

static NSString *gkString(const char *value) {
    return value ? [NSString stringWithUTF8String:value] : @"";
}

/* ---- Classes ---- */

@implementation GKUIAffixCell
- (void)dealloc {
    [_prefix release];
    [_suffix release];
    [super dealloc];
}
- (NSDictionary *)affixAttributes {
    return @{
        NSFontAttributeName : self.font ?: [NSFont systemFontOfSize:NSFont.systemFontSize],
        NSForegroundColorAttributeName : NSColor.secondaryLabelColor,
    };
}
- (CGFloat)affixWidth:(NSString *)affix {
    return affix.length ? ceil([affix sizeWithAttributes:[self affixAttributes]].width) + 2 : 0;
}
- (NSRect)gkInset:(NSRect)rect {
    CGFloat left = [self affixWidth:self.prefix];
    CGFloat right = [self affixWidth:self.suffix];
    rect.origin.x += left;
    rect.size.width = MAX(0, rect.size.width - left - right);
    return rect;
}
- (NSRect)drawingRectForBounds:(NSRect)rect {
    return [self gkInset:[super drawingRectForBounds:rect]];
}
- (void)editWithFrame:(NSRect)rect
               inView:(NSView *)view
               editor:(NSText *)editor
             delegate:(id)delegate
                event:(NSEvent *)event {
    [super editWithFrame:[self gkInset:rect] inView:view editor:editor delegate:delegate event:event];
}
- (void)selectWithFrame:(NSRect)rect
                 inView:(NSView *)view
                 editor:(NSText *)editor
               delegate:(id)delegate
                  start:(NSInteger)start
                 length:(NSInteger)length {
    [super selectWithFrame:[self gkInset:rect] inView:view editor:editor delegate:delegate start:start length:length];
}
- (void)drawInteriorWithFrame:(NSRect)frame inView:(NSView *)view {
    NSRect text = [super drawingRectForBounds:frame];
    NSDictionary *attributes = [self affixAttributes];
    CGFloat height = [@"X" sizeWithAttributes:attributes].height;
    CGFloat y = NSMidY(text) - height / 2;
    if (self.prefix.length) {
        [self.prefix drawAtPoint:NSMakePoint(NSMinX(text), y) withAttributes:attributes];
    }
    if (self.suffix.length) {
        CGFloat width = [self affixWidth:self.suffix];
        [self.suffix drawAtPoint:NSMakePoint(NSMaxX(text) - width + 2, y) withAttributes:attributes];
    }
    [super drawInteriorWithFrame:frame inView:view];
}
@end

@implementation GKUITextField
+ (Class)cellClass {
    return GKUIAffixCell.class;
}
@end

@implementation GKUISeparatorView
- (void)dealloc {
    [_label release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (NSSize)fittingSize {
    CGFloat thickness = MAX(1, self.thickness);
    if (self.label.length) {
        NSSize text = [self.label sizeWithAttributes:[self labelAttributes]];
        return self.vertical ? NSMakeSize(text.height + 4, text.width + 24)
                             : NSMakeSize(text.width + 24, text.height + 4);
    }
    return self.vertical ? NSMakeSize(thickness, 1) : NSMakeSize(1, thickness);
}
- (NSDictionary *)labelAttributes {
    return @{
        NSFontAttributeName : [NSFont systemFontOfSize:NSFont.smallSystemFontSize],
        NSForegroundColorAttributeName : NSColor.secondaryLabelColor,
    };
}
- (NSAccessibilityRole)accessibilityRole {
    return NSAccessibilitySplitterRole;
}
- (BOOL)isAccessibilityElement {
    return self.label.length > 0;
}
- (NSString *)accessibilityLabel {
    return self.label;
}
- (void)drawRect:(NSRect)dirty {
    (void)dirty;
    CGFloat thickness = MAX(1, self.thickness);
    CGFloat inset = self.inset ? 16 : 0;
    NSRect bounds = self.bounds;
    [NSColor.separatorColor setFill];
    NSSize text = self.label.length ? [self.label sizeWithAttributes:[self labelAttributes]] : NSZeroSize;
    if (self.vertical) {
        CGFloat x = NSMidX(bounds) - thickness / 2;
        CGFloat start = NSMinY(bounds) + inset;
        CGFloat end = NSMaxY(bounds) - inset;
        if (text.width > 0) {
            CGFloat gapStart = NSMidY(bounds) - text.width / 2 - 4;
            CGFloat gapEnd = NSMidY(bounds) + text.width / 2 + 4;
            NSRectFill(NSMakeRect(x, start, thickness, MAX(0, gapStart - start)));
            NSRectFill(NSMakeRect(x, gapEnd, thickness, MAX(0, end - gapEnd)));
            NSGraphicsContext *context = NSGraphicsContext.currentContext;
            [context saveGraphicsState];
            NSAffineTransform *transform = [NSAffineTransform transform];
            [transform translateXBy:NSMidX(bounds) + text.height / 2 yBy:NSMidY(bounds) - text.width / 2];
            [transform rotateByDegrees:90];
            [transform concat];
            [self.label drawAtPoint:NSZeroPoint withAttributes:[self labelAttributes]];
            [context restoreGraphicsState];
        } else {
            NSRectFill(NSMakeRect(x, start, thickness, MAX(0, end - start)));
        }
        return;
    }
    CGFloat y = NSMidY(bounds) - thickness / 2;
    CGFloat start = NSMinX(bounds) + inset;
    CGFloat end = NSMaxX(bounds) - inset;
    if (text.width > 0) {
        CGFloat gapStart = NSMidX(bounds) - text.width / 2 - 6;
        CGFloat gapEnd = NSMidX(bounds) + text.width / 2 + 6;
        NSRectFill(NSMakeRect(start, y, MAX(0, gapStart - start), thickness));
        NSRectFill(NSMakeRect(gapEnd, y, MAX(0, end - gapEnd), thickness));
        [self.label drawAtPoint:NSMakePoint(gapStart + 6, NSMidY(bounds) - text.height / 2)
                 withAttributes:[self labelAttributes]];
    } else {
        NSRectFill(NSMakeRect(start, y, MAX(0, end - start), thickness));
    }
}
@end

/* ---- Text alignment, wrapping, and fonts ---- */

static NSTextAlignment gkAlignment(int alignment) {
    switch (alignment) {
    case 1:
        return NSTextAlignmentCenter;
    case 2:
        return NSApp.userInterfaceLayoutDirection == NSUserInterfaceLayoutDirectionRightToLeft ? NSTextAlignmentLeft
                                                                                               : NSTextAlignmentRight;
    case 3:
        return NSTextAlignmentJustified;
    default:
        return NSTextAlignmentNatural;
    }
}

static NSFont *gkTextStyleFont(int style) {
    switch (style) {
    case 1:
        return [NSFont preferredFontForTextStyle:NSFontTextStyleCaption1 options:@{}];
    case 2:
        return [NSFont preferredFontForTextStyle:NSFontTextStyleHeadline options:@{}];
    case 3:
        return [NSFont preferredFontForTextStyle:NSFontTextStyleTitle2 options:@{}];
    case 4:
        return [NSFont monospacedSystemFontOfSize:NSFont.systemFontSize weight:NSFontWeightRegular];
    default:
        return [NSFont preferredFontForTextStyle:NSFontTextStyleBody options:@{}];
    }
}

static NSLineBreakMode gkLineBreak(int wrap, int truncation) {
    switch (truncation) {
    case 1:
        return NSLineBreakByTruncatingHead;
    case 2:
        return NSLineBreakByTruncatingMiddle;
    case 3:
        return NSLineBreakByTruncatingTail;
    default:
        break;
    }
    switch (wrap) {
    case 1:
        return NSLineBreakByWordWrapping;
    case 2:
        return NSLineBreakByCharWrapping;
    default:
        return NSLineBreakByClipping;
    }
}

void gkCocoaLabelConfigure(void *pointer, int alignment, int wrap, int truncation, int selectable, int style,
                           int maximumLines) {
    NSTextField *label = pointer;
    if (![label isKindOfClass:NSTextField.class]) {
        return;
    }
    label.alignment = gkAlignment(alignment);
    label.selectable = selectable != 0;
    label.font = gkTextStyleFont(style);
    label.cell.wraps = wrap != 0;
    label.cell.scrollable = wrap == 0 && truncation == 0;
    label.lineBreakMode = gkLineBreak(wrap, truncation);
    label.cell.truncatesLastVisibleLine = truncation != 0;
    label.maximumNumberOfLines = wrap ? maximumLines : 1;
    label.usesSingleLineMode = wrap == 0;
    [label invalidateIntrinsicContentSize];
}

/* gkCocoaLabelPreferredWidth lets a wrapping label report its height for a
   width. */
void gkCocoaLabelPreferredWidth(void *pointer, int width) {
    NSTextField *label = pointer;
    if ([label isKindOfClass:NSTextField.class]) {
        label.preferredMaxLayoutWidth = width > 0 ? width : 0;
    }
}

/* ---- Button ---- */


void gkCocoaButtonConfigure(void *pointer, void *icon, int placement, int role, const char *key, int modifiers) {
    NSButton *button = pointer;
    if (![button isKindOfClass:NSButton.class]) {
        return;
    }
    button.image = (NSImage *)icon;
    switch (placement) {
    case 1:
        button.imagePosition = NSImageTrailing;
        break;
    case 2:
        button.imagePosition = NSImageAbove;
        break;
    case 3:
        button.imagePosition = NSImageBelow;
        break;
    case 4:
        button.imagePosition = NSImageOnly;
        break;
    default:
        button.imagePosition = icon ? NSImageLeading : NSNoImage;
        break;
    }
    NSString *equivalent = gkString(key);
    NSEventModifierFlags mask = (NSEventModifierFlags)modifiers;
    if (!equivalent.length) {
        mask = 0;
        if (role == 1) {
            equivalent = @"\r";
        } else if (role == 2) {
            equivalent = @"\x1b";
        }
    }
    button.keyEquivalent = equivalent;
    button.keyEquivalentModifierMask = mask;
    button.hasDestructiveAction = role == 3;
    button.contentTintColor = role == 3 ? NSColor.systemRedColor : nil;
    button.bezelColor = role == 1 ? NSColor.controlAccentColor : nil;
    if ([button isKindOfClass:GKUIPushButton.class]) {
        ((GKUIPushButton *)button).stackedContent = icon && (placement == 2 || placement == 3);
    }
}

void gkCocoaButtonSetBusy(void *pointer, int busy) {
    NSButton *button = pointer;
    if ([button isKindOfClass:GKUIPushButton.class]) {
        ((GKUIPushButton *)button).busy = busy != 0;
    }
    button.accessibilityValueDescription = busy ? @"Busy" : nil;
}

void gkCocoaButtonSetBadge(void *pointer, const char *text) {
    NSButton *button = pointer;
    NSString *value = gkString(text);
    if ([button isKindOfClass:GKUIPushButton.class]) {
        ((GKUIPushButton *)button).badge = value;
    }
    button.accessibilityValueDescription = value.length ? value : nil;
}

/* ---- Text field and text area ---- */

static NSString *gkContentType(int purpose) {
    switch (purpose) {
    case 1:
        return @"name";
    case 2:
        return @"username";
    case 3:
        return @"emailAddress";
    case 4:
        return @"URL";
    case 5:
        return @"telephoneNumber";
    default:
        return nil;
    }
}

void gkCocoaTextFieldConfigure(void *pointer, int readOnly, int maximumLength, int purpose, int alignment,
                               const char *prefix, const char *suffix, int spellCheck) {
    GKUITextField *field = pointer;
    if (![field isKindOfClass:GKUITextField.class]) {
        return;
    }
    field.editable = readOnly == 0;
    field.selectable = YES;
    field.maximumLength = maximumLength;
    field.alignment = gkAlignment(alignment);
    field.spellCheck = spellCheck != 0;
    if ([field respondsToSelector:@selector(setContentType:)]) {
        field.contentType = gkContentType(purpose);
    }
    GKUIAffixCell *cell = (GKUIAffixCell *)field.cell;
    if ([cell isKindOfClass:GKUIAffixCell.class]) {
        cell.prefix = gkString(prefix);
        cell.suffix = gkString(suffix);
    }
    NSText *editor = field.currentEditor;
    if ([editor isKindOfClass:NSTextView.class]) {
        ((NSTextView *)editor).continuousSpellCheckingEnabled = spellCheck != 0;
        ((NSTextView *)editor).editable = readOnly == 0;
    }
    [field setNeedsDisplay:YES];
}

/* Selections use UTF-16 offsets. */
void gkCocoaTextSelection(void *pointer, int *start, int *length) {
    NSView *view = pointer;
    NSRange range = NSMakeRange(0, 0);
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        range = ((GKUITextAreaView *)view).editor.selectedRange;
    } else if ([view isKindOfClass:GKUITextField.class]) {
        GKUITextField *field = (GKUITextField *)view;
        if (field.currentEditor) {
            range = field.currentEditor.selectedRange;
        } else if (field.pendingSelection.location != NSNotFound) {
            range = field.pendingSelection;
        }
    }
    *start = (int)range.location;
    *length = (int)range.length;
}

void gkCocoaSetTextSelection(void *pointer, int start, int length) {
    NSView *view = pointer;
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        NSTextView *editor = ((GKUITextAreaView *)view).editor;
        NSUInteger total = editor.string.length;
        NSUInteger location = MIN((NSUInteger)MAX(0, start), total);
        editor.selectedRange = NSMakeRange(location, MIN((NSUInteger)MAX(0, length), total - location));
    } else if ([view isKindOfClass:GKUITextField.class]) {
        GKUITextField *field = (GKUITextField *)view;
        NSUInteger total = field.stringValue.length;
        NSUInteger location = MIN((NSUInteger)MAX(0, start), total);
        NSRange range = NSMakeRange(location, MIN((NSUInteger)MAX(0, length), total - location));
        field.pendingSelection = range;
        if (field.currentEditor) {
            field.currentEditor.selectedRange = range;
        }
    }
}

void gkCocoaTextAreaConfigure(void *pointer, int wrap, int maximumLength, int tabMovesFocus, int spellCheck) {
    GKUITextAreaView *area = pointer;
    if (![area isKindOfClass:GKUITextAreaView.class]) {
        return;
    }
    NSTextView *editor = area.editor;
    area.maximumLength = maximumLength;
    area.tabMovesFocus = tabMovesFocus != 0;
    editor.continuousSpellCheckingEnabled = spellCheck != 0;
    BOOL wraps = wrap != 0;
    area.hasHorizontalScroller = !wraps;
    editor.horizontallyResizable = !wraps;
    editor.textContainer.widthTracksTextView = wraps;
    editor.textContainer.lineBreakMode = wrap == 2 ? NSLineBreakByCharWrapping : NSLineBreakByWordWrapping;
    if (wraps) {
        editor.textContainer.containerSize = NSMakeSize(area.contentSize.width, CGFLOAT_MAX);
        [editor setFrameSize:NSMakeSize(area.contentSize.width, editor.frame.size.height)];
    } else {
        editor.textContainer.containerSize = NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX);
        editor.maxSize = NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX);
    }
}

void gkCocoaScrollPosition(void *pointer, int *x, int *y) {
    NSScrollView *scroll = pointer;
    NSPoint origin = [scroll isKindOfClass:NSScrollView.class] ? scroll.documentVisibleRect.origin : NSZeroPoint;
    *x = (int)floor(origin.x);
    *y = (int)floor(origin.y);
}

/* gkCocoaScrollClipTo scrolls the clip view of scroll to origin. An animated
   scroll moves smoothly over a short time. */
static void gkCocoaScrollClipTo(NSScrollView *scroll, NSPoint origin, int animated) {
    NSClipView *clip = scroll.contentView;
    NSRect bounds = clip.bounds;
    bounds.origin = NSMakePoint(MAX(0, origin.x), MAX(0, origin.y));
    bounds = [clip constrainBoundsRect:bounds];
    if (animated) {
        [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
          context.duration = 0.2;
          [clip.animator setBoundsOrigin:bounds.origin];
        }];
    } else {
        [clip scrollToPoint:bounds.origin];
    }
    [scroll reflectScrolledClipView:clip];
}

void gkCocoaSetScrollPosition(void *pointer, int x, int y, int animated) {
    NSScrollView *scroll = pointer;
    if (![scroll isKindOfClass:NSScrollView.class]) {
        return;
    }
    gkCocoaScrollClipTo(scroll, NSMakePoint(x, y), animated);
}

/* ---- Check box ---- */

void gkCocoaCheckBoxConfigure(void *pointer, int allowsMixed, int state, int labelBefore, const char *text,
                              const char *description) {
    NSButton *box = pointer;
    if (![box isKindOfClass:NSButton.class]) {
        return;
    }
    box.allowsMixedState = allowsMixed != 0;
    box.state = state == 2 && allowsMixed ? NSControlStateValueMixed
                : state == 1              ? NSControlStateValueOn
                                          : NSControlStateValueOff;
    box.imagePosition = labelBefore ? NSImageTrailing : NSImageLeading;
    NSString *title = gkString(text);
    NSString *detail = gkString(description);
    if (detail.length) {
        NSMutableAttributedString *value = [[[NSMutableAttributedString alloc] initWithString:title] autorelease];
        [value appendAttributedString:[[[NSAttributedString alloc]
                                          initWithString:[@"\n" stringByAppendingString:detail]
                                              attributes:@{
                                                  NSForegroundColorAttributeName : NSColor.secondaryLabelColor,
                                                  NSFontAttributeName :
                                                      [NSFont systemFontOfSize:NSFont.smallSystemFontSize],
                                              }] autorelease]];
        box.attributedTitle = value;
    } else {
        box.title = title;
    }
    box.accessibilityHelp = detail.length ? detail : nil;
}

int gkCocoaCheckBoxState(void *pointer) {
    NSButton *box = pointer;
    if (box.state == NSControlStateValueMixed) {
        return 2;
    }
    return box.state == NSControlStateValueOn;
}

/* ---- Choices ---- */

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

void gkUIRadioGroupRebuild(GKUIRadioGroupView *group, NSArray<NSString *> *titles, NSArray<NSNumber *> *enabled) {
    for (NSButton *button in group.buttons) {
        [button removeFromSuperview];
    }
    [group.buttons removeAllObjects];
    for (NSUInteger i = 0; i < titles.count; i++) {
        [group addItem:titles[i]];
        group.buttons.lastObject.enabled = i < enabled.count ? enabled[i].boolValue : YES;
    }
}

void gkCocoaSetChoices(void *pointer, const char *titles, size_t titlesSize, const int *enabled, int count) {
    NSView *view = pointer;
    NSArray<NSString *> *values = gkSplit(titles, titlesSize);
    NSMutableArray<NSNumber *> *flags = [NSMutableArray array];
    for (int i = 0; i < count; i++) {
        [flags addObject:@(enabled[i] != 0)];
    }
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        GKUIRadioGroupView *group = (GKUIRadioGroupView *)view;
        NSInteger selected = group.selectedIndex;
        gkUIRadioGroupRebuild(group, values, flags);
        group.selectedIndex = selected < (NSInteger)values.count ? selected : -1;
    } else if ([view isKindOfClass:NSPopUpButton.class]) {
        NSPopUpButton *popup = (NSPopUpButton *)view;
        NSInteger selected = popup.indexOfSelectedItem;
        popup.autoenablesItems = NO;
        [popup removeAllItems];
        for (NSUInteger i = 0; i < values.count; i++) {
            [popup addItemWithTitle:@""];
            NSMenuItem *item = popup.lastItem;
            item.title = values[i];
            item.enabled = i < flags.count ? flags[i].boolValue : YES;
        }
        if (selected >= 0 && selected < (NSInteger)values.count) {
            [popup selectItemAtIndex:selected];
        } else {
            [popup selectItem:nil];
        }
    }
}

void gkCocoaRadioGroupConfigure(void *pointer, int horizontal, const char *label) {
    GKUIRadioGroupView *group = pointer;
    if (![group isKindOfClass:GKUIRadioGroupView.class]) {
        return;
    }
    group.horizontal = horizontal != 0;
    NSString *title = gkString(label);
    if (title.length && !group.titleLabel) {
        group.titleLabel = [NSTextField labelWithString:title];
        group.titleLabel.font = [NSFont boldSystemFontOfSize:NSFont.systemFontSize];
        [group addSubview:group.titleLabel];
    }
    group.titleLabel.stringValue = title;
    group.titleLabel.hidden = title.length == 0;
    group.accessibilityLabel = title.length ? title : nil;
    group.accessibilityRole = NSAccessibilityRadioGroupRole;
    [group setNeedsLayout:YES];
}

void gkCocoaRadioGroupPreferredSize(void *pointer, int *width, int *height) {
    GKUIRadioGroupView *group = pointer;
    CGFloat title = group.titleLabel && !group.titleLabel.hidden ? 22 : 0;
    if (group.horizontal) {
        CGFloat total = 0;
        for (NSButton *button in group.buttons) {
            total += button.fittingSize.width + 12;
        }
        *width = (int)ceil(MAX(120, total));
        *height = (int)ceil(24 + title);
        return;
    }
    CGFloat maxWidth = 120;
    for (NSButton *button in group.buttons) {
        maxWidth = MAX(maxWidth, button.fittingSize.width);
    }
    *width = (int)ceil(maxWidth);
    *height = (int)ceil(MAX(24, group.buttons.count * 24) + title);
}

void gkCocoaSelectConfigure(void *pointer, const char *placeholder, int popupWidth) {
    NSPopUpButton *popup = pointer;
    if (![popup isKindOfClass:NSPopUpButton.class]) {
        return;
    }
    NSString *value = gkString(placeholder);
    popup.accessibilityPlaceholderValue = value.length ? value : nil;
    popup.menu.minimumWidth = popupWidth > 0 ? popupWidth : 0;
}

/* ---- Value controls ---- */

void gkCocoaSliderConfigure(void *pointer, int vertical, double minimum, double maximum, double frequency, int snap) {
    NSSlider *slider = pointer;
    if (![slider isKindOfClass:NSSlider.class]) {
        return;
    }
    slider.vertical = vertical != 0;
    if (frequency > 0) {
        slider.numberOfTickMarks = (NSInteger)floor((maximum - minimum) / frequency) + 1;
        slider.allowsTickMarkValuesOnly = snap != 0;
    } else {
        slider.numberOfTickMarks = 0;
        slider.allowsTickMarkValuesOnly = NO;
    }
}

static char gkProgressTextKey;

void gkCocoaProgressConfigure(void *pointer, int indeterminate, int vertical, const char *text, int state) {
    NSProgressIndicator *progress = pointer;
    if (![progress isKindOfClass:NSProgressIndicator.class]) {
        return;
    }
    BOOL wasIndeterminate = progress.indeterminate;
    progress.indeterminate = indeterminate != 0;
    if (indeterminate && !wasIndeterminate) {
        [progress startAnimation:nil];
    } else if (!indeterminate && wasIndeterminate) {
        [progress stopAnimation:nil];
    }
    progress.frameCenterRotation = vertical ? 90 : 0;
    NSString *value = gkString(text);
    objc_setAssociatedObject(progress, &gkProgressTextKey, value, OBJC_ASSOCIATION_COPY_NONATOMIC);
    progress.accessibilityValueDescription = value.length ? value : nil;
    progress.toolTip = value.length ? value : nil;
    progress.wantsLayer = YES;
    progress.layerUsesCoreImageFilters = YES;
    NSColor *tint = state == 1   ? NSColor.systemGrayColor
                    : state == 2 ? NSColor.systemOrangeColor
                    : state == 3 ? NSColor.systemRedColor
                                 : nil;
    if (tint) {
        NSColor *rgb = [tint colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
        CIFilter *filter = [CIFilter filterWithName:@"CIColorMonochrome"];
        [filter setDefaults];
        [filter setValue:[CIColor colorWithRed:rgb.redComponent green:rgb.greenComponent blue:rgb.blueComponent]
                  forKey:kCIInputColorKey];
        [filter setValue:@1 forKey:kCIInputIntensityKey];
        progress.contentFilters = @[ filter ];
    } else {
        progress.contentFilters = @[];
    }
}

void gkCocoaActivityConfigure(void *pointer, int size, const char *label) {
    NSProgressIndicator *activity = pointer;
    if (![activity isKindOfClass:NSProgressIndicator.class]) {
        return;
    }
    static const NSControlSize sizes[] = {NSControlSizeRegular, NSControlSizeMini, NSControlSizeSmall,
                                          NSControlSizeLarge};
    activity.controlSize = sizes[size >= 0 && size < 4 ? size : 0];
    NSString *value = gkString(label);
    activity.accessibilityLabel = value.length ? value : nil;
    activity.toolTip = value.length ? value : nil;
    [activity sizeToFit];
}

void gkCocoaSeparatorConfigure(void *pointer, int vertical, const char *label, int inset, int thickness) {
    GKUISeparatorView *separator = pointer;
    if (![separator isKindOfClass:GKUISeparatorView.class]) {
        return;
    }
    separator.vertical = vertical != 0;
    separator.label = gkString(label);
    separator.inset = inset != 0;
    separator.thickness = MAX(1, thickness);
    [separator setNeedsDisplay:YES];
}

void gkCocoaImageConfigure(void *pointer, int alignment, int nearest, const char *alternative, double scale) {
    GKUIImageView *image = pointer;
    if (![image isKindOfClass:GKUIImageView.class]) {
        return;
    }
    image.alignment = alignment;
    image.interpolation = nearest ? NSImageInterpolationNone : NSImageInterpolationHigh;
    image.sourceScale = scale > 0 ? scale : 1;
    NSString *text = gkString(alternative);
    image.accessibilityElement = text.length > 0;
    image.accessibilityRole = NSAccessibilityImageRole;
    image.accessibilityLabel = text.length ? text : nil;
    [image setNeedsDisplay:YES];
}

void gkCocoaLinkConfigure(void *pointer, int visited, void *icon) {
    GKUILinkButton *link = pointer;
    if (![link isKindOfClass:GKUILinkButton.class]) {
        return;
    }
    link.visited = visited != 0;
    link.image = (NSImage *)icon;
    link.imagePosition = icon ? NSImageLeading : NSNoImage;
    NSString *title = link.title ?: @"";
    link.attributedTitle = [[[NSAttributedString alloc]
        initWithString:title
            attributes:@{
                NSForegroundColorAttributeName : link.visited ? NSColor.systemPurpleColor : NSColor.linkColor,
                NSUnderlineStyleAttributeName : @(NSUnderlineStyleSingle),
            }] autorelease];
    link.accessibilityRole = NSAccessibilityLinkRole;
}

int gkCocoaOpenURL(const char *url, int background) {
    NSURL *value = [NSURL URLWithString:gkString(url)];
    if (!value) {
        return 0;
    }
    if (!background) {
        return [NSWorkspace.sharedWorkspace openURL:value];
    }
    NSWorkspaceOpenConfiguration *configuration = [NSWorkspaceOpenConfiguration configuration];
    configuration.activates = NO;
    [NSWorkspace.sharedWorkspace openURL:value configuration:configuration completionHandler:nil];
    return 1;
}

void gkCocoaTabsConfigure(void *pointer, int placement) {
    NSTabView *tabs = pointer;
    if (![tabs isKindOfClass:NSTabView.class]) {
        return;
    }
    static const NSTabViewType types[] = {NSTopTabsBezelBorder, NSBottomTabsBezelBorder, NSLeftTabsBezelBorder,
                                          NSRightTabsBezelBorder};
    tabs.tabViewType = types[placement >= 0 && placement < 4 ? placement : 0];
}

/* ---- Scroll view ---- */

void gkCocoaScrollConfigure(void *pointer, int axes, int policy, int top, int right, int bottom, int left,
                            int overscroll) {
    NSScrollView *scroll = pointer;
    if (![scroll isKindOfClass:NSScrollView.class]) {
        return;
    }
    BOOL vertical = axes == 0 || axes == 2;
    BOOL horizontal = axes == 1 || axes == 2;
    scroll.hasVerticalScroller = vertical && policy != 2;
    scroll.hasHorizontalScroller = horizontal && policy != 2;
    scroll.autohidesScrollers = policy == 0;
    scroll.automaticallyAdjustsContentInsets = NO;
    scroll.contentInsets = NSEdgeInsetsMake(top, left, bottom, right);
    NSScrollElasticity elasticity = overscroll ? NSScrollElasticityNone : NSScrollElasticityAutomatic;
    scroll.verticalScrollElasticity = vertical ? elasticity : NSScrollElasticityNone;
    scroll.horizontalScrollElasticity = horizontal ? elasticity : NSScrollElasticityNone;
}

void gkCocoaScrollSetZoom(void *pointer, double zoom) {
    NSScrollView *scroll = pointer;
    scroll.allowsMagnification = YES;
    scroll.minMagnification = MIN(0.1, zoom);
    scroll.maxMagnification = MAX(16, zoom);
    scroll.magnification = zoom;
}

double gkCocoaScrollZoom(void *pointer) {
    return ((NSScrollView *)pointer).magnification;
}

void gkCocoaScrollViewport(void *pointer, int *width, int *height) {
    NSScrollView *scroll = pointer;
    NSSize size = scroll.contentView.bounds.size;
    if (size.width <= 1 || size.height <= 1) {
        size = scroll.contentSize;
    }
    *width = (int)floor(size.width);
    *height = (int)floor(size.height);
}

/* gkCocoaScrollToView scrolls the least distance that makes view visible. */
int gkCocoaScrollToView(void *scrollPointer, void *viewPointer, int animated) {
    NSScrollView *scroll = scrollPointer;
    NSView *view = viewPointer;
    if (![view isDescendantOf:scroll.documentView]) {
        return 0;
    }
    NSClipView *clip = scroll.contentView;
    NSRect target = [view convertRect:view.bounds toView:clip];
    target = NSOffsetRect(target, NSMinX(clip.bounds), NSMinY(clip.bounds));
    NSRect visible = clip.bounds;
    NSPoint origin = visible.origin;
    if (NSMaxX(target) > NSMaxX(visible)) {
        origin.x = NSMaxX(target) - visible.size.width;
    }
    if (NSMinX(target) < origin.x) {
        origin.x = NSMinX(target);
    }
    if (NSMaxY(target) > NSMaxY(visible)) {
        origin.y = NSMaxY(target) - visible.size.height;
    }
    if (NSMinY(target) < origin.y) {
        origin.y = NSMinY(target);
    }
    gkCocoaScrollClipTo(scroll, origin, animated);
    return 1;
}

/* ---- Section ---- */

void gkCocoaSectionSetAnimated(void *pointer, int animated) {
    GKUISectionView *section = pointer;
    if ([section isKindOfClass:GKUISectionView.class]) {
        section.animated = animated != 0;
    }
}

void gkCocoaSectionConfigure(void *pointer, const char *subtitle, void *icon, const char *badge) {
    GKUISectionView *section = pointer;
    if (![section isKindOfClass:GKUISectionView.class]) {
        return;
    }
    section.subtitle = gkString(subtitle);
    section.header.image = (NSImage *)icon;
    section.header.imagePosition = icon ? NSImageLeading : NSNoImage;
    NSString *badgeText = gkString(badge);
    if (badgeText.length && !section.badgeLabel) {
        section.badgeLabel = gkUIBadgeLabel(badgeText, NSColor.systemGrayColor);
        [section addSubview:section.badgeLabel];
    }
    section.badgeLabel.stringValue = badgeText;
    section.badgeLabel.hidden = badgeText.length == 0;
    [section updateHeader];
    [section setNeedsLayout:YES];
}
