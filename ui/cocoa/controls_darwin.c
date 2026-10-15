#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"
#include <string.h>

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);
NSFont *gkCocoaFont(const char *family, double size, int weight, int italic);
int gkCocoaMacKey(unsigned short code);

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

static NSControlSize gkControlSize(int size) {
    static const NSControlSize sizes[] = {NSControlSizeRegular, NSControlSizeMini, NSControlSizeSmall,
                                          NSControlSizeLarge};
    return sizes[size >= 0 && size < 4 ? size : 0];
}

/* GKCocoaTarget reports a control's action as a component event. */
@interface GKCocoaTarget : NSObject <NSTextFieldDelegate, NSComboBoxDelegate, NSSearchFieldDelegate>
@property(assign) NSView *view;
@property int kind;
- (void)fire:(id)sender;
@end

@implementation GKCocoaTarget
- (void)fire:(id)sender {
    (void)sender;
    gkCocoaSendView(self.view, self.kind, 0, 0, 0, 0, nil);
}
- (void)controlTextDidChange:(NSNotification *)notification {
    (void)notification;
    gkCocoaSendView(self.view, GK_EVENT_TEXT_CHANGE, 0, 0, 0, 0, nil);
}
- (void)comboBoxSelectionDidChange:(NSNotification *)notification {
    NSComboBox *combo = notification.object;
    NSInteger index = combo.indexOfSelectedItem;
    if (index >= 0 && index < combo.numberOfItems) {
        gkCocoaSendView(self.view, GK_EVENT_TEXT_CHANGE, 0, 0, 0, 0, [combo itemObjectValueAtIndex:index]);
    }
}
@end

static char gkCocoaTargetKey;

static GKCocoaTarget *gkCocoaInstallTarget(NSView *owner, NSControl *control, int kind) {
    GKCocoaTarget *target = [[GKCocoaTarget new] autorelease];
    target.view = owner;
    target.kind = kind;
    control.target = target;
    control.action = @selector(fire:);
    NSMutableArray *targets = objc_getAssociatedObject(owner, &gkCocoaTargetKey);
    if (!targets) {
        targets = [NSMutableArray array];
        objc_setAssociatedObject(owner, &gkCocoaTargetKey, targets, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
    [targets addObject:target];
    return target;
}

/* gkCocoaDisconnect stops a view and its controls from reporting events. */
void gkCocoaDisconnect(void *pointer) {
    NSView *view = pointer;
    NSMutableArray *targets = objc_getAssociatedObject(view, &gkCocoaTargetKey);
    for (GKCocoaTarget *target in targets) {
        target.view = nil;
    }
}

/* GKCocoaStackView lays out its subviews in a row with fixed widths. */
@interface GKCocoaRowView : GKUILayoutView
@end

@implementation GKCocoaRowView
- (void)layout {
    [super layout];
    CGFloat x = 0;
    NSArray *subviews = self.subviews;
    for (NSUInteger i = 0; i < subviews.count; i++) {
        NSView *view = subviews[i];
        NSSize size = view.fittingSize;
        BOOL last = i == subviews.count - 1;
        CGFloat width = last ? MAX(size.width, self.bounds.size.width - x) : size.width;
        if (i == 0 && subviews.count > 1 && [view isKindOfClass:NSTextField.class] &&
            ((NSTextField *)view).isEditable) {
            /* An editable field takes the space its neighbours leave. */
            CGFloat rest = 0;
            for (NSUInteger j = 1; j < subviews.count; j++) {
                rest += [subviews[j] fittingSize].width + 4;
            }
            width = MAX(40, self.bounds.size.width - rest);
        }
        view.frame = NSMakeRect(x, (self.bounds.size.height - size.height) / 2, width, size.height);
        x += width + 4;
    }
}
- (NSSize)fittingSize {
    CGFloat width = 0, height = 0;
    for (NSView *view in self.subviews) {
        NSSize size = view.fittingSize;
        width += size.width + 4;
        height = MAX(height, size.height);
    }
    return NSMakeSize(MAX(0, width - 4), height);
}
@end

/* ---- Toggle button ---- */

void *gkCocoaToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar) {
    NSButton *button = [[NSButton buttonWithTitle:gkString(text) target:nil action:nil] retain];
    [button setButtonType:NSButtonTypePushOnPushOff];
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    button.bezelStyle = toolbar ? NSBezelStyleTexturedRounded : NSBezelStyleRounded;
#pragma clang diagnostic pop
    button.image = (NSImage *)icon;
    button.imagePosition = icon ? NSImageLeading : NSNoImage;
    button.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    gkCocoaInstallTarget(button, button, GK_EVENT_VALUE);
    gkAdd(window, button);
    return button;
}

/* ---- Switch ---- */

@interface GKCocoaSwitchView : GKUILayoutView
@property(retain) NSSwitch *control;
@property(retain) NSTextField *label;
@property(copy) NSString *onLabel;
@property(copy) NSString *offLabel;
@end

@implementation GKCocoaSwitchView
- (void)dealloc {
    [_control release];
    [_label release];
    [_onLabel release];
    [_offLabel release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.control;
}
- (void)updateValue {
    NSString *description = self.control.state == NSControlStateValueOn ? self.onLabel : self.offLabel;
    self.control.accessibilityValueDescription = description.length ? description : nil;
}
- (void)layout {
    [super layout];
    NSSize toggle = self.control.fittingSize;
    self.control.frame = NSMakeRect(self.bounds.size.width - toggle.width,
                                    (self.bounds.size.height - toggle.height) / 2, toggle.width, toggle.height);
    NSSize text = self.label.fittingSize;
    self.label.frame = NSMakeRect(0, (self.bounds.size.height - text.height) / 2,
                                  MAX(0, self.bounds.size.width - toggle.width - 8), text.height);
}
- (NSSize)fittingSize {
    NSSize toggle = self.control.fittingSize;
    NSSize text = self.label.stringValue.length ? self.label.fittingSize : NSZeroSize;
    return NSMakeSize(text.width + (text.width > 0 ? 8 : 0) + toggle.width, MAX(text.height, toggle.height));
}
@end

void *gkCocoaSwitchCreate(uintptr_t window, const char *text, const char *onLabel, const char *offLabel, int checked,
                          int size) {
    GKCocoaSwitchView *view = [[GKCocoaSwitchView alloc] initWithFrame:NSZeroRect];
    view.control = [[[NSSwitch alloc] initWithFrame:NSZeroRect] autorelease];
    view.control.controlSize = gkControlSize(size);
    view.control.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    view.label = [NSTextField labelWithString:gkString(text)];
    view.control.accessibilityLabel = gkString(text);
    view.onLabel = gkString(onLabel);
    view.offLabel = gkString(offLabel);
    [view updateValue];
    [view addSubview:view.label];
    [view addSubview:view.control];
    gkCocoaInstallTarget(view, view.control, GK_EVENT_VALUE);
    gkAdd(window, view);
    return view;
}

int gkCocoaSwitchChecked(void *pointer) {
    return ((GKCocoaSwitchView *)pointer).control.state == NSControlStateValueOn;
}

void gkCocoaSwitchSet(void *pointer, const char *text, int checked) {
    GKCocoaSwitchView *view = pointer;
    view.label.stringValue = gkString(text);
    view.control.accessibilityLabel = gkString(text);
    view.control.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    [view updateValue];
    [view setNeedsLayout:YES];
}

/* ---- Password field ---- */

@interface GKCocoaPasswordView : GKUILayoutView
@property(retain) NSSecureTextField *secure;
@property(retain) NSTextField *plain;
@property(retain) NSButton *reveal;
@property(nonatomic) BOOL revealed;
@end

@implementation GKCocoaPasswordView
- (void)dealloc {
    [_secure release];
    [_plain release];
    [_reveal release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.revealed ? self.plain : self.secure;
}
- (NSTextField *)active {
    return self.revealed ? self.plain : self.secure;
}
- (void)layout {
    [super layout];
    CGFloat button = self.reveal.hidden ? 0 : 24;
    NSRect field = NSMakeRect(0, 0, MAX(0, self.bounds.size.width - button), self.bounds.size.height);
    self.secure.frame = field;
    self.plain.frame = field;
    self.reveal.frame = NSMakeRect(self.bounds.size.width - 22, (self.bounds.size.height - 20) / 2, 20, 20);
}
- (NSSize)fittingSize {
    return NSMakeSize(200, MAX(22, self.secure.fittingSize.height));
}
- (void)setRevealed:(BOOL)revealed {
    NSString *text = [[self active].stringValue copy];
    _revealed = revealed;
    self.secure.hidden = revealed;
    self.plain.hidden = !revealed;
    [self active].stringValue = text;
    [text release];
    self.reveal.image = [NSImage imageWithSystemSymbolName:revealed ? @"eye.slash" : @"eye"
                                  accessibilityDescription:revealed ? @"Hide password" : @"Show password"];
}
- (void)toggle:(id)sender {
    (void)sender;
    self.revealed = !self.revealed;
    gkCocoaSendView(self, GK_EVENT_EXPAND, self.revealed, 0, 0, 0, nil);
}
@end

void *gkCocoaPasswordCreate(uintptr_t window, const char *placeholder, int reveal) {
    GKCocoaPasswordView *view = [[GKCocoaPasswordView alloc] initWithFrame:NSZeroRect];
    view.secure = [[[NSSecureTextField alloc] initWithFrame:NSZeroRect] autorelease];
    view.plain = [[[NSTextField alloc] initWithFrame:NSZeroRect] autorelease];
    view.secure.placeholderString = gkString(placeholder);
    view.plain.placeholderString = gkString(placeholder);
    view.reveal =
        [NSButton buttonWithImage:[NSImage imageWithSystemSymbolName:@"eye" accessibilityDescription:@"Show password"]
                           target:view
                           action:@selector(toggle:)];
    view.reveal.bordered = NO;
    view.reveal.hidden = !reveal;
    [view addSubview:view.secure];
    [view addSubview:view.plain];
    [view addSubview:view.reveal];
    view.revealed = NO;
    for (NSTextField *field in @[ view.secure, view.plain ]) {
        GKCocoaTarget *target = gkCocoaInstallTarget(view, field, GK_EVENT_SUBMIT);
        field.delegate = target;
    }
    gkAdd(window, view);
    return view;
}

char *gkCocoaPasswordText(void *pointer) {
    return strdup([((GKCocoaPasswordView *)pointer) active].stringValue.UTF8String ?: "");
}

void gkCocoaPasswordSet(void *pointer, const char *text, const char *placeholder, int revealed) {
    GKCocoaPasswordView *view = pointer;
    view.revealed = revealed != 0;
    [view active].stringValue = gkString(text);
    view.secure.placeholderString = gkString(placeholder);
    view.plain.placeholderString = gkString(placeholder);
}

/* ---- Search field ---- */

@interface GKCocoaSearchTarget : GKCocoaTarget
- (void)choose:(NSMenuItem *)item;
@end

@implementation GKCocoaSearchTarget
- (void)choose:(NSMenuItem *)item {
    NSSearchField *field = (NSSearchField *)self.view;
    field.stringValue = item.title;
    gkCocoaSendView(self.view, GK_EVENT_TEXT_CHANGE, 0, 0, 0, 0, nil);
    gkCocoaSendView(self.view, GK_EVENT_SUBMIT, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaSearchCreate(uintptr_t window, const char *text, const char *placeholder) {
    NSSearchField *field = [[NSSearchField alloc] initWithFrame:NSZeroRect];
    field.stringValue = gkString(text);
    field.placeholderString = gkString(placeholder);
    field.sendsSearchStringImmediately = NO;
    field.sendsWholeSearchString = YES;
    GKCocoaSearchTarget *target = [[GKCocoaSearchTarget new] autorelease];
    target.view = field;
    target.kind = GK_EVENT_SUBMIT;
    field.target = target;
    field.action = @selector(fire:);
    field.delegate = target;
    objc_setAssociatedObject(field, &gkCocoaTargetKey, [NSMutableArray arrayWithObject:target],
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    gkAdd(window, field);
    return field;
}

void gkCocoaSearchSetSuggestions(void *pointer, const char *items, size_t size) {
    NSSearchField *field = pointer;
    GKCocoaTarget *target = [objc_getAssociatedObject(field, &gkCocoaTargetKey) firstObject];
    NSMenu *menu = [[[NSMenu alloc] initWithTitle:@"Suggestions"] autorelease];
    for (NSString *suggestion in gkSplit(items, size)) {
        NSMenuItem *item =
            [[[NSMenuItem alloc] initWithTitle:suggestion action:@selector(choose:) keyEquivalent:@""] autorelease];
        item.target = target;
        [menu addItem:item];
    }
    field.searchMenuTemplate = menu.numberOfItems ? menu : nil;
}

/* ---- Number field ---- */

@interface GKCocoaNumberView : GKCocoaRowView
@property(retain) NSTextField *field;
@property(retain) NSStepper *stepper;
@end

@implementation GKCocoaNumberView
- (void)dealloc {
    [_field release];
    [_stepper release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.field;
}
- (NSSize)fittingSize {
    return NSMakeSize(120, MAX(22, self.stepper.fittingSize.height));
}
- (void)fieldChanged:(id)sender {
    (void)sender;
    double value = self.field.doubleValue;
    value = MAX(self.stepper.minValue, MIN(self.stepper.maxValue, value));
    self.stepper.doubleValue = value;
    self.field.doubleValue = value;
    gkCocoaSendView(self, GK_EVENT_VALUE, 0, 0, value, 0, nil);
}
- (void)stepped:(id)sender {
    (void)sender;
    self.field.doubleValue = self.stepper.doubleValue;
    gkCocoaSendView(self, GK_EVENT_VALUE, 0, 0, self.stepper.doubleValue, 0, nil);
}
@end

void *gkCocoaNumberCreate(uintptr_t window, double minimum, double maximum, double value, double step, int decimals) {
    GKCocoaNumberView *view = [[GKCocoaNumberView alloc] initWithFrame:NSZeroRect];
    NSNumberFormatter *formatter = [[NSNumberFormatter new] autorelease];
    formatter.numberStyle = NSNumberFormatterDecimalStyle;
    formatter.minimum = @(minimum);
    formatter.maximum = @(maximum);
    formatter.minimumFractionDigits = decimals;
    formatter.maximumFractionDigits = decimals;
    formatter.usesGroupingSeparator = NO;
    view.field = [[[NSTextField alloc] initWithFrame:NSZeroRect] autorelease];
    view.field.formatter = formatter;
    view.field.alignment = NSTextAlignmentRight;
    view.stepper = [[[NSStepper alloc] initWithFrame:NSZeroRect] autorelease];
    view.stepper.minValue = minimum;
    view.stepper.maxValue = maximum;
    view.stepper.increment = step > 0 ? step : 1;
    view.stepper.valueWraps = NO;
    view.stepper.doubleValue = value;
    view.field.doubleValue = value;
    view.field.target = view;
    view.field.action = @selector(fieldChanged:);
    view.stepper.target = view;
    view.stepper.action = @selector(stepped:);
    [view addSubview:view.field];
    [view addSubview:view.stepper];
    gkAdd(window, view);
    return view;
}

double gkCocoaNumberValue(void *pointer) {
    return ((GKCocoaNumberView *)pointer).stepper.doubleValue;
}

void gkCocoaNumberSetValue(void *pointer, double value) {
    GKCocoaNumberView *view = pointer;
    view.stepper.doubleValue = value;
    view.field.doubleValue = value;
}

/* ---- Editable combo ---- */

void *gkCocoaComboCreate(uintptr_t window, const char *text, const char *items, size_t size) {
    NSComboBox *combo = [[NSComboBox alloc] initWithFrame:NSZeroRect];
    combo.usesDataSource = NO;
    combo.completes = YES;
    [combo addItemsWithObjectValues:gkSplit(items, size)];
    combo.stringValue = gkString(text);
    GKCocoaTarget *target = gkCocoaInstallTarget(combo, combo, GK_EVENT_SUBMIT);
    combo.delegate = target;
    gkAdd(window, combo);
    return combo;
}

void gkCocoaComboSetItems(void *pointer, const char *items, size_t size) {
    NSComboBox *combo = pointer;
    NSString *text = [[combo.stringValue copy] autorelease];
    [combo removeAllItems];
    [combo addItemsWithObjectValues:gkSplit(items, size)];
    combo.stringValue = text;
}

/* ---- Segmented control ---- */

void *gkCocoaSegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons, const int *enabled,
                             int count, int mode) {
    NSSegmentedControl *control = [[NSSegmentedControl alloc] initWithFrame:NSZeroRect];
    NSArray<NSString *> *titles = gkSplit(labels, size);
    control.segmentCount = count;
    control.trackingMode = mode == 0   ? NSSegmentSwitchTrackingMomentary
                           : mode == 2 ? NSSegmentSwitchTrackingSelectAny
                                       : NSSegmentSwitchTrackingSelectOne;
    for (int i = 0; i < count; i++) {
        if (i < (int)titles.count) {
            [control setLabel:titles[i] forSegment:i];
        }
        [control setImage:(NSImage *)icons[i] forSegment:i];
        [control setEnabled:enabled[i] != 0 forSegment:i];
    }
    gkCocoaInstallTarget(control, control, GK_EVENT_SELECTION);
    gkAdd(window, control);
    return control;
}

int gkCocoaSegmentSelected(void *pointer, int index) {
    return [(NSSegmentedControl *)pointer isSelectedForSegment:index];
}

int gkCocoaSegmentClicked(void *pointer) {
    return (int)((NSSegmentedControl *)pointer).selectedSegment;
}

void gkCocoaSegmentSetSelected(void *pointer, int index, int selected) {
    [(NSSegmentedControl *)pointer setSelected:selected != 0 forSegment:index];
}

/* ---- Stepper and dial ---- */

void *gkCocoaStepperCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps) {
    NSStepper *stepper = [[NSStepper alloc] initWithFrame:NSZeroRect];
    stepper.minValue = minimum;
    stepper.maxValue = maximum;
    stepper.increment = step > 0 ? step : 1;
    stepper.valueWraps = wraps != 0;
    stepper.doubleValue = value;
    gkCocoaInstallTarget(stepper, stepper, GK_EVENT_VALUE);
    gkAdd(window, stepper);
    return stepper;
}

void *gkCocoaDialCreate(uintptr_t window, double minimum, double maximum, double value, double step) {
    NSSlider *dial = [[NSSlider sliderWithValue:value minValue:minimum maxValue:maximum target:nil action:nil] retain];
    dial.sliderType = NSSliderTypeCircular;
    if (step > 0) {
        dial.numberOfTickMarks = (NSInteger)floor((maximum - minimum) / step) + 1;
        dial.allowsTickMarkValuesOnly = YES;
    }
    gkCocoaInstallTarget(dial, dial, GK_EVENT_VALUE);
    gkAdd(window, dial);
    return dial;
}

double gkCocoaControlValue(void *pointer) {
    return ((NSControl *)pointer).doubleValue;
}

void gkCocoaControlSetValue(void *pointer, double value) {
    ((NSControl *)pointer).doubleValue = value;
}

/* ---- Range slider ---- */

@interface GKCocoaRangeSlider : NSControl
@property double minimum;
@property double maximum;
@property double lower;
@property double upper;
@property double step;
@property NSInteger dragging;
@property NSInteger focusedKnob;
@end

@implementation GKCocoaRangeSlider
- (BOOL)isFlipped {
    return YES;
}
- (BOOL)acceptsFirstResponder {
    return YES;
}
- (NSSize)fittingSize {
    return NSMakeSize(160, 22);
}
- (CGFloat)positionFor:(double)value {
    CGFloat inset = 9;
    double span = self.maximum - self.minimum;
    double fraction = span > 0 ? (value - self.minimum) / span : 0;
    return inset + fraction * MAX(0, self.bounds.size.width - inset * 2);
}
- (double)valueAt:(CGFloat)x {
    CGFloat inset = 9;
    CGFloat width = MAX(1, self.bounds.size.width - inset * 2);
    double value = self.minimum + (MAX(0, MIN(width, x - inset)) / width) * (self.maximum - self.minimum);
    if (self.step > 0) {
        value = self.minimum + round((value - self.minimum) / self.step) * self.step;
    }
    return MAX(self.minimum, MIN(self.maximum, value));
}
- (void)drawRect:(NSRect)dirty {
    (void)dirty;
    CGFloat mid = NSMidY(self.bounds);
    NSRect track = NSMakeRect(9, mid - 2, MAX(0, self.bounds.size.width - 18), 4);
    [[NSColor.tertiaryLabelColor colorWithAlphaComponent:0.4] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:track xRadius:2 yRadius:2] fill];
    CGFloat lower = [self positionFor:self.lower];
    CGFloat upper = [self positionFor:self.upper];
    [(self.enabled ? NSColor.controlAccentColor : NSColor.disabledControlTextColor) setFill];
    [[NSBezierPath bezierPathWithRoundedRect:NSMakeRect(lower, mid - 2, upper - lower, 4) xRadius:2 yRadius:2] fill];
    for (NSInteger knob = 0; knob < 2; knob++) {
        CGFloat x = knob == 0 ? lower : upper;
        NSRect rect = NSMakeRect(x - 8, mid - 8, 16, 16);
        [NSColor.whiteColor setFill];
        NSBezierPath *path = [NSBezierPath bezierPathWithOvalInRect:rect];
        [path fill];
        [(self.window.firstResponder == self && self.focusedKnob == knob ? NSColor.keyboardFocusIndicatorColor
                                                                         : NSColor.separatorColor) setStroke];
        path.lineWidth = 1;
        [path stroke];
    }
}
- (void)report {
    [self setNeedsDisplay:YES];
    self.accessibilityValueDescription = [NSString stringWithFormat:@"%g to %g", self.lower, self.upper];
    gkCocoaSendView(self, GK_EVENT_VALUE, 0, 0, self.lower, self.upper, nil);
}
- (void)mouseDown:(NSEvent *)event {
    if (!self.enabled) {
        return;
    }
    [self.window makeFirstResponder:self];
    CGFloat x = [self convertPoint:event.locationInWindow fromView:nil].x;
    CGFloat lower = [self positionFor:self.lower];
    CGFloat upper = [self positionFor:self.upper];
    self.dragging = fabs(x - lower) <= fabs(x - upper) ? 0 : 1;
    if (lower == upper) {
        self.dragging = x < lower ? 0 : 1;
    }
    self.focusedKnob = self.dragging;
    [self mouseDragged:event];
}
- (void)mouseDragged:(NSEvent *)event {
    double value = [self valueAt:[self convertPoint:event.locationInWindow fromView:nil].x];
    if (self.dragging == 0) {
        self.lower = MIN(value, self.upper);
    } else {
        self.upper = MAX(value, self.lower);
    }
    [self report];
}
- (void)keyDown:(NSEvent *)event {
    double step = self.step > 0 ? self.step : (self.maximum - self.minimum) / 100;
    double delta = 0;
    switch (event.keyCode) {
    case 123:
    case 125:
        delta = -step;
        break;
    case 124:
    case 126:
        delta = step;
        break;
    case 48:
        self.focusedKnob = 1 - self.focusedKnob;
        [self setNeedsDisplay:YES];
        return;
    default:
        [super keyDown:event];
        return;
    }
    if (self.focusedKnob == 0) {
        self.lower = MAX(self.minimum, MIN(self.upper, self.lower + delta));
    } else {
        self.upper = MIN(self.maximum, MAX(self.lower, self.upper + delta));
    }
    [self report];
}
- (NSAccessibilityRole)accessibilityRole {
    return NSAccessibilitySliderRole;
}
- (BOOL)isAccessibilityElement {
    return YES;
}
@end

void *gkCocoaRangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower, double upper,
                               double step) {
    GKCocoaRangeSlider *slider = [[GKCocoaRangeSlider alloc] initWithFrame:NSZeroRect];
    slider.minimum = minimum;
    slider.maximum = maximum;
    slider.lower = lower;
    slider.upper = upper;
    slider.step = step;
    slider.accessibilityValueDescription = [NSString stringWithFormat:@"%g to %g", lower, upper];
    gkAdd(window, slider);
    return slider;
}

void gkCocoaRangeSliderValues(void *pointer, double *lower, double *upper) {
    GKCocoaRangeSlider *slider = pointer;
    *lower = slider.lower;
    *upper = slider.upper;
}

void gkCocoaRangeSliderSet(void *pointer, double lower, double upper) {
    GKCocoaRangeSlider *slider = pointer;
    slider.lower = lower;
    slider.upper = upper;
    slider.accessibilityValueDescription = [NSString stringWithFormat:@"%g to %g", lower, upper];
    [slider setNeedsDisplay:YES];
}

/* ---- Dates and times ----

   Dates are exchanged as seconds since 1970 in UTC. Pickers show and edit them
   in UTC so a calendar date never shifts with the local time zone. */

static NSCalendar *gkUTCCalendar(void) {
    NSCalendar *calendar = [[[NSCalendar alloc] initWithCalendarIdentifier:NSCalendarIdentifierGregorian] autorelease];
    calendar.timeZone = [NSTimeZone timeZoneForSecondsFromGMT:0];
    return calendar;
}

void *gkCocoaDatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum, double maximum,
                              int range, double interval, int clock) {
    NSDatePicker *picker = [[NSDatePicker alloc] initWithFrame:NSZeroRect];
    picker.datePickerStyle = style == 1 ? NSDatePickerStyleClockAndCalendar : NSDatePickerStyleTextFieldAndStepper;
    picker.datePickerElements = (NSDatePickerElementFlags)elements;
    picker.timeZone = [NSTimeZone timeZoneForSecondsFromGMT:0];
    picker.calendar = gkUTCCalendar();
    if (clock == 1 || clock == 2) {
        NSString *identifier = NSLocale.currentLocale.localeIdentifier;
        picker.locale = [NSLocale
            localeWithLocaleIdentifier:[identifier stringByAppendingString:clock == 2 ? @"@hours=h23" : @"@hours=h12"]];
    }
    if (minimum != 0) {
        picker.minDate = [NSDate dateWithTimeIntervalSince1970:minimum];
    }
    if (maximum != 0) {
        picker.maxDate = [NSDate dateWithTimeIntervalSince1970:maximum];
    }
    picker.datePickerMode = range ? NSDatePickerModeRange : NSDatePickerModeSingle;
    picker.dateValue = [NSDate dateWithTimeIntervalSince1970:value];
    picker.timeInterval = range ? interval : 0;
    gkCocoaInstallTarget(picker, picker, GK_EVENT_VALUE);
    gkAdd(window, picker);
    return picker;
}

void gkCocoaDatePickerValue(void *pointer, double *value, double *interval) {
    NSDatePicker *picker = pointer;
    *value = picker.dateValue.timeIntervalSince1970;
    *interval = picker.timeInterval;
}

void gkCocoaDatePickerSet(void *pointer, double value, double interval) {
    NSDatePicker *picker = pointer;
    picker.dateValue = [NSDate dateWithTimeIntervalSince1970:value];
    if (picker.datePickerMode == NSDatePickerModeRange) {
        picker.timeInterval = interval;
    }
}

/* ---- Color and font ---- */

@interface GKCocoaColorWell : NSColorWell
@property BOOL allowsAlpha;
@end

@implementation GKCocoaColorWell
- (void)activate:(BOOL)exclusive {
    NSColorPanel.sharedColorPanel.showsAlpha = self.allowsAlpha;
    [super activate:exclusive];
}
@end

void *gkCocoaColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allowsAlpha) {
    GKCocoaColorWell *well = [[GKCocoaColorWell alloc] initWithFrame:NSMakeRect(0, 0, 44, 24)];
    well.allowsAlpha = allowsAlpha != 0;
    well.color = [NSColor colorWithSRGBRed:r green:g blue:b alpha:a];
    gkCocoaInstallTarget(well, well, GK_EVENT_VALUE);
    gkAdd(window, well);
    return well;
}

void gkCocoaColorWellColor(void *pointer, double *rgba) {
    NSColor *color = [((NSColorWell *)pointer).color colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
    rgba[0] = color.redComponent;
    rgba[1] = color.greenComponent;
    rgba[2] = color.blueComponent;
    rgba[3] = color.alphaComponent;
}

void gkCocoaColorWellSet(void *pointer, double r, double g, double b, double a) {
    ((NSColorWell *)pointer).color = [NSColor colorWithSRGBRed:r green:g blue:b alpha:a];
}

@interface GKCocoaFontView : GKCocoaRowView
@property(retain) NSTextField *preview;
@property(retain) NSButton *choose;
@property(retain) NSFont *currentFont;
@end

@implementation GKCocoaFontView
- (void)dealloc {
    [_preview release];
    [_choose release];
    [_currentFont release];
    [super dealloc];
}
- (void)update {
    NSFont *font = self.currentFont;
    self.preview.stringValue =
        [NSString stringWithFormat:@"%@ %g", font.displayName ?: font.familyName, font.pointSize];
    self.preview.font = [NSFont fontWithDescriptor:font.fontDescriptor size:NSFont.systemFontSize] ?: font;
    self.accessibilityValueDescription = self.preview.stringValue;
}
- (void)open:(id)sender {
    (void)sender;
    NSFontManager *manager = NSFontManager.sharedFontManager;
    manager.target = self;
    manager.action = @selector(changeFont:);
    [manager setSelectedFont:self.currentFont isMultiple:NO];
    [manager orderFrontFontPanel:self];
}
- (void)changeFont:(id)sender {
    self.currentFont = [sender convertFont:self.currentFont];
    [self update];
    gkCocoaSendView(self, GK_EVENT_VALUE, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaFontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic) {
    GKCocoaFontView *view = [[GKCocoaFontView alloc] initWithFrame:NSZeroRect];
    view.currentFont = gkCocoaFont(family, size, weight, italic) ?: [NSFont systemFontOfSize:NSFont.systemFontSize];
    view.preview = [NSTextField labelWithString:@""];
    view.choose = [NSButton buttonWithTitle:@"Choose…" target:view action:@selector(open:)];
    [view addSubview:view.preview];
    [view addSubview:view.choose];
    [view update];
    gkAdd(window, view);
    return view;
}

void gkCocoaFontPickerSet(void *pointer, const char *family, double size, int weight, int italic) {
    GKCocoaFontView *view = pointer;
    NSFont *font = gkCocoaFont(family, size, weight, italic);
    if (font) {
        view.currentFont = font;
        [view update];
    }
}

/* gkCocoaFontPickerFont reports the chosen font's family, size, weight, and
   italic flag. */
char *gkCocoaFontPickerFont(void *pointer, double *size, int *weight, int *italic) {
    NSFont *font = ((GKCocoaFontView *)pointer).currentFont;
    *size = font.pointSize;
    NSFontTraitMask traits = [NSFontManager.sharedFontManager traitsOfFont:font];
    NSInteger appKitWeight = [NSFontManager.sharedFontManager weightOfFont:font];
    *weight = appKitWeight >= 9 ? 700 : appKitWeight >= 6 ? 500 : 400;
    *italic = (traits & NSItalicFontMask) != 0;
    NSString *family = font.familyName ?: @"";
    if ([font.fontName hasPrefix:@"."]) {
        family = @"System";
    }
    return strdup(family.UTF8String);
}

/* ---- Key recorder ---- */

@interface GKCocoaKeyRecorder : NSView
@property(copy) NSString *display;
@property BOOL recording;
@end

@implementation GKCocoaKeyRecorder
- (void)dealloc {
    [_display release];
    [super dealloc];
}
- (BOOL)isFlipped {
    return YES;
}
- (BOOL)acceptsFirstResponder {
    return YES;
}
- (NSSize)fittingSize {
    return NSMakeSize(140, 24);
}
- (void)drawRect:(NSRect)dirty {
    (void)dirty;
    NSRect bounds = NSInsetRect(self.bounds, 0.5, 0.5);
    NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:bounds xRadius:5 yRadius:5];
    [NSColor.controlBackgroundColor setFill];
    [path fill];
    [(self.recording ? NSColor.keyboardFocusIndicatorColor : NSColor.separatorColor) setStroke];
    [path stroke];
    NSString *text = self.recording ? @"Type shortcut…" : (self.display.length ? self.display : @"Record Shortcut");
    NSDictionary *attributes = @{
        NSFontAttributeName : [NSFont systemFontOfSize:NSFont.systemFontSize],
        NSForegroundColorAttributeName : self.display.length && !self.recording ? NSColor.labelColor
                                                                                : NSColor.secondaryLabelColor,
    };
    NSSize size = [text sizeWithAttributes:attributes];
    [text drawAtPoint:NSMakePoint(NSMidX(self.bounds) - size.width / 2, NSMidY(self.bounds) - size.height / 2)
        withAttributes:attributes];
}
- (void)mouseDown:(NSEvent *)event {
    (void)event;
    if ([self.window makeFirstResponder:self]) {
        self.recording = YES;
        [self setNeedsDisplay:YES];
    }
}
- (BOOL)resignFirstResponder {
    self.recording = NO;
    [self setNeedsDisplay:YES];
    return YES;
}
- (BOOL)record:(NSEvent *)event {
    if (!self.recording) {
        return NO;
    }
    unsigned short code = event.keyCode;
    NSEventModifierFlags flags = event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask;
    if (code == 53 &&
        !(flags & (NSEventModifierFlagCommand | NSEventModifierFlagControl | NSEventModifierFlagOption))) {
        self.recording = NO;
        [self setNeedsDisplay:YES];
        return YES;
    }
    if ((code == 51 || code == 117) && !flags) {
        self.recording = NO;
        gkCocoaSendView(self, GK_EVENT_VALUE, 0, 0, 0, 0, @"");
        return YES;
    }
    int key = gkCocoaMacKey(code);
    NSString *characters = event.charactersIgnoringModifiers.uppercaseString;
    int modifiers = 0;
    if (flags & NSEventModifierFlagCommand) {
        modifiers |= 1;
    }
    if (flags & NSEventModifierFlagControl) {
        modifiers |= 2;
    }
    if (flags & NSEventModifierFlagOption) {
        modifiers |= 4;
    }
    if (flags & NSEventModifierFlagShift) {
        modifiers |= 8;
    }
    self.recording = NO;
    GKCocoaEvent report = {0};
    report.kind = GK_EVENT_VALUE;
    report.a = modifiers;
    report.b = key;
    report.text = characters.UTF8String;
    uintptr_t handle = gkCocoaHandle(self);
    if (handle) {
        gkCocoaGoEvent(handle, &report);
    }
    [self setNeedsDisplay:YES];
    return YES;
}
- (void)keyDown:(NSEvent *)event {
    if (![self record:event]) {
        [super keyDown:event];
    }
}
- (BOOL)performKeyEquivalent:(NSEvent *)event {
    return self.window.firstResponder == self && [self record:event];
}
- (NSAccessibilityRole)accessibilityRole {
    return NSAccessibilityButtonRole;
}
- (BOOL)isAccessibilityElement {
    return YES;
}
- (NSString *)accessibilityLabel {
    return @"Shortcut";
}
- (NSString *)accessibilityValue {
    return self.display;
}
- (BOOL)accessibilityPerformPress {
    if ([self.window makeFirstResponder:self]) {
        self.recording = YES;
        [self setNeedsDisplay:YES];
    }
    return YES;
}
@end

void *gkCocoaKeyRecorderCreate(uintptr_t window) {
    GKCocoaKeyRecorder *recorder = [[GKCocoaKeyRecorder alloc] initWithFrame:NSZeroRect];
    gkAdd(window, recorder);
    return recorder;
}

void gkCocoaKeyRecorderSet(void *pointer, const char *display) {
    GKCocoaKeyRecorder *recorder = pointer;
    recorder.display = gkString(display);
    [recorder setNeedsDisplay:YES];
}
