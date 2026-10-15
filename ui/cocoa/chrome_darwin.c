#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);
void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text);

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

/* ---- Menu bar ---- */

/* gkCocoaMenuBarCreate builds a main menu: the application menu, then the
   given menus. It returns NULL when a menu already belongs to another menu. */
void *gkCocoaMenuBarCreate(const char *titles, size_t titlesSize, void **menus, int count) {
    @autoreleasepool {
        NSString *name = NSRunningApplication.currentApplication.localizedName ?: NSProcessInfo.processInfo.processName;
        NSMenu *bar = [[NSMenu alloc] initWithTitle:@"Main Menu"];
        NSMenuItem *appItem = [[[NSMenuItem alloc] initWithTitle:name action:nil keyEquivalent:@""] autorelease];
        NSMenu *appMenu = [[[NSMenu alloc] initWithTitle:name] autorelease];
        [appMenu addItemWithTitle:[@"About " stringByAppendingString:name]
                           action:@selector(orderFrontStandardAboutPanel:)
                    keyEquivalent:@""];
        [appMenu addItem:[NSMenuItem separatorItem]];
        [appMenu addItemWithTitle:[@"Hide " stringByAppendingString:name] action:@selector(hide:) keyEquivalent:@"h"];
        NSMenuItem *others =
            [appMenu addItemWithTitle:@"Hide Others" action:@selector(hideOtherApplications:) keyEquivalent:@"h"];
        others.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagOption;
        [appMenu addItemWithTitle:@"Show All" action:@selector(unhideAllApplications:) keyEquivalent:@""];
        [appMenu addItem:[NSMenuItem separatorItem]];
        [appMenu addItemWithTitle:[@"Quit " stringByAppendingString:name]
                           action:@selector(terminate:)
                    keyEquivalent:@"q"];
        appItem.submenu = appMenu;
        [bar addItem:appItem];
        NSArray<NSString *> *names = gkSplit(titles, titlesSize);
        for (int i = 0; i < count; i++) {
            NSMenu *menu = menus[i];
            if (menu.supermenu) {
                for (NSMenuItem *item in bar.itemArray) {
                    if (item != appItem) {
                        item.submenu = nil;
                    }
                }
                [bar release];
                return NULL;
            }
            NSString *title = i < (int)names.count ? names[i] : @"";
            menu.title = title;
            NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""] autorelease];
            item.submenu = menu;
            [bar addItem:item];
        }
        return bar;
    }
}

static NSMenu *gkCocoaPreviousMenu;

void gkCocoaMenuBarInstall(void *pointer) {
    NSMenu *bar = pointer;
    if (NSApp.mainMenu == bar) {
        return;
    }
    if (!gkCocoaPreviousMenu) {
        gkCocoaPreviousMenu = [NSApp.mainMenu retain];
    }
    NSApp.mainMenu = bar;
}

void gkCocoaMenuBarRelease(void *pointer) {
    NSMenu *bar = pointer;
    if (NSApp.mainMenu == bar) {
        NSApp.mainMenu = gkCocoaPreviousMenu;
        [gkCocoaPreviousMenu release];
        gkCocoaPreviousMenu = nil;
    }
    /* The user's menus outlive the bar, so detach them. */
    for (NSMenuItem *item in bar.itemArray) {
        if (item != bar.itemArray.firstObject) {
            item.submenu = nil;
        }
    }
    [bar release];
}

/* ---- Toolbar ---- */

@interface GKCocoaToolbar : NSObject <NSToolbarDelegate>
@property(retain) NSToolbar *toolbar;
@property(assign) NSWindow *window;
@property(retain) NSArray<NSString *> *identifiers;
@property(retain) NSMutableDictionary<NSString *, NSToolbarItem *> *items;
@property uintptr_t handle;
@end

@implementation GKCocoaToolbar
- (void)dealloc {
    self.toolbar.delegate = nil;
    [_toolbar release];
    [_identifiers release];
    [_items release];
    [super dealloc];
}
- (NSToolbarItem *)toolbar:(NSToolbar *)toolbar
        itemForItemIdentifier:(NSToolbarItemIdentifier)identifier
    willBeInsertedIntoToolbar:(BOOL)flag {
    (void)toolbar;
    (void)flag;
    return self.items[identifier];
}
- (NSArray<NSToolbarItemIdentifier> *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar {
    (void)toolbar;
    return self.identifiers;
}
- (NSArray<NSToolbarItemIdentifier> *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar {
    (void)toolbar;
    return self.identifiers;
}
- (void)activate:(NSToolbarItem *)item {
    gkCocoaSend(self.handle, GK_EVENT_ACTION, 0, 0, 0, 0, item.itemIdentifier);
}
@end

static NSUInteger gkCocoaToolbarCount;

/* Item kinds: 0 action, 1 separator, 2 space, 3 flexible space. */
void *gkCocoaToolbarCreate(uintptr_t handle, void *windowPointer, const int *kinds, const char *ids, size_t idsSize,
                           const char *labels, size_t labelsSize, void **icons, const int *enabled, int count,
                           int showsLabels) {
    @autoreleasepool {
        NSWindow *window = windowPointer;
        GKCocoaToolbar *controller = [GKCocoaToolbar new];
        controller.handle = handle;
        controller.window = window;
        controller.items = [NSMutableDictionary dictionary];
        NSArray<NSString *> *actionIDs = gkSplit(ids, idsSize);
        NSArray<NSString *> *names = gkSplit(labels, labelsSize);
        NSMutableArray *identifiers = [NSMutableArray array];
        for (int i = 0; i < count; i++) {
            switch (kinds[i]) {
            case 1:
            case 2:
                [identifiers addObject:NSToolbarSpaceItemIdentifier];
                break;
            case 3:
                [identifiers addObject:NSToolbarFlexibleSpaceItemIdentifier];
                break;
            default: {
                NSString *identifier = i < (int)actionIDs.count ? actionIDs[i] : @"";
                NSToolbarItem *item = [[[NSToolbarItem alloc] initWithItemIdentifier:identifier] autorelease];
                item.label = i < (int)names.count ? names[i] : identifier;
                item.paletteLabel = item.label;
                item.toolTip = item.label;
                item.image = (NSImage *)icons[i];
                item.target = controller;
                item.action = @selector(activate:);
                item.autovalidates = NO;
                item.enabled = enabled[i] != 0;
                controller.items[identifier] = item;
                [identifiers addObject:identifier];
                break;
            }
            }
        }
        controller.identifiers = identifiers;
        NSString *name = [NSString stringWithFormat:@"dev.gamekit.toolbar.%lu", (unsigned long)++gkCocoaToolbarCount];
        controller.toolbar = [[[NSToolbar alloc] initWithIdentifier:name] autorelease];
        controller.toolbar.delegate = controller;
        controller.toolbar.displayMode = showsLabels ? NSToolbarDisplayModeIconAndLabel : NSToolbarDisplayModeIconOnly;
        controller.toolbar.allowsUserCustomization = NO;
        window.toolbar = controller.toolbar;
        return controller;
    }
}

void gkCocoaToolbarSetEnabled(void *pointer, const char *identifier, int enabled) {
    GKCocoaToolbar *controller = pointer;
    controller.items[gkString(identifier)].enabled = enabled != 0;
}

void gkCocoaToolbarRelease(void *pointer) {
    GKCocoaToolbar *controller = pointer;
    if (controller.window.toolbar == controller.toolbar) {
        controller.window.toolbar = nil;
    }
    controller.handle = 0;
    [controller release];
}

/* ---- Status bar ---- */

@interface GKCocoaStatusBar : GKUILayoutView
@property(retain) NSTextField *label;
@end

@implementation GKCocoaStatusBar
- (void)dealloc {
    [_label release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.label;
}
- (void)drawRect:(NSRect)dirty {
    (void)dirty;
    [NSColor.windowBackgroundColor setFill];
    NSRectFill(self.bounds);
    [NSColor.separatorColor setFill];
    NSRectFill(NSMakeRect(0, 0, self.bounds.size.width, 1));
}
- (void)layout {
    [super layout];
    NSSize size = self.label.fittingSize;
    self.label.frame = NSMakeRect(8, (self.bounds.size.height - size.height) / 2 + 0.5,
                                  MAX(0, self.bounds.size.width - 16), size.height);
}
- (NSSize)fittingSize {
    return NSMakeSize(self.label.fittingSize.width + 16, 22);
}
@end

void *gkCocoaStatusBarCreate(uintptr_t window, const char *text) {
    GKCocoaStatusBar *bar = [[GKCocoaStatusBar alloc] initWithFrame:NSZeroRect];
    bar.label = [NSTextField labelWithString:gkString(text)];
    bar.label.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize];
    bar.label.textColor = NSColor.secondaryLabelColor;
    bar.label.lineBreakMode = NSLineBreakByTruncatingTail;
    bar.label.accessibilityRole = NSAccessibilityStaticTextRole;
    bar.accessibilityRole = NSAccessibilityGroupRole;
    [bar addSubview:bar.label];
    gkAdd(window, bar);
    return bar;
}

void gkCocoaStatusBarSet(void *pointer, const char *text) {
    GKCocoaStatusBar *bar = pointer;
    bar.label.stringValue = gkString(text);
    NSAccessibilityPostNotification(bar.label, NSAccessibilityValueChangedNotification);
}

/* ---- Command bar ---- */

/* GKCocoaCommandBar shows one button per action and moves the actions that do
   not fit into an overflow menu. */
@interface GKCocoaCommandBar : GKUILayoutView
@property(retain) NSMutableArray<NSButton *> *buttons;
@property(retain) NSPopUpButton *overflow;
@end

@implementation GKCocoaCommandBar
- (void)dealloc {
    [_buttons release];
    [_overflow release];
    [super dealloc];
}
- (void)activate:(NSButton *)sender {
    gkCocoaSendView(self, GK_EVENT_ACTION, 0, 0, 0, 0, sender.identifier);
}
- (void)chooseOverflow:(NSPopUpButton *)sender {
    NSString *identifier = sender.selectedItem.representedObject;
    if (identifier) {
        gkCocoaSendView(self, GK_EVENT_ACTION, 0, 0, 0, 0, identifier);
    }
}
- (void)layout {
    [super layout];
    CGFloat available = self.bounds.size.width;
    CGFloat overflowWidth = self.overflow.fittingSize.width;
    CGFloat x = 0;
    [self.overflow.menu removeAllItems];
    [self.overflow.menu addItemWithTitle:@"" action:nil keyEquivalent:@""];
    self.overflow.menu.itemArray.firstObject.image =
        [NSImage imageWithSystemSymbolName:@"ellipsis" accessibilityDescription:@"More"];
    BOOL overflowing = NO;
    for (NSUInteger i = 0; i < self.buttons.count; i++) {
        NSButton *button = self.buttons[i];
        NSSize size = button.fittingSize;
        BOOL last = i == self.buttons.count - 1;
        CGFloat reserve = last ? 0 : overflowWidth + 4;
        if (overflowing || x + size.width + reserve > available) {
            overflowing = YES;
            button.hidden = YES;
            NSMenuItem *item = [self.overflow.menu addItemWithTitle:button.title.length ? button.title
                                                                                     : button.toolTip ?: @""
                                                             action:nil
                                                      keyEquivalent:@""];
            item.representedObject = button.identifier;
            item.enabled = button.enabled;
            continue;
        }
        button.hidden = NO;
        button.frame = NSMakeRect(x, (self.bounds.size.height - size.height) / 2, size.width, size.height);
        x += size.width + 4;
    }
    self.overflow.hidden = !overflowing;
    NSSize size = self.overflow.fittingSize;
    self.overflow.frame = NSMakeRect(MIN(x, available - size.width), (self.bounds.size.height - size.height) / 2,
                                     size.width, size.height);
}
- (NSSize)fittingSize {
    CGFloat width = 0, height = 0;
    for (NSButton *button in self.buttons) {
        NSSize size = button.fittingSize;
        width += size.width + 4;
        height = MAX(height, size.height);
    }
    return NSMakeSize(MAX(0, width - 4), height);
}
@end

void *gkCocoaCommandBarCreate(uintptr_t window) {
    GKCocoaCommandBar *bar = [[GKCocoaCommandBar alloc] initWithFrame:NSZeroRect];
    bar.buttons = [NSMutableArray array];
    bar.overflow = [[[NSPopUpButton alloc] initWithFrame:NSZeroRect pullsDown:YES] autorelease];
    bar.overflow.bordered = NO;
    bar.overflow.target = bar;
    bar.overflow.action = @selector(chooseOverflow:);
    bar.overflow.hidden = YES;
    bar.accessibilityRole = NSAccessibilityToolbarRole;
    [bar addSubview:bar.overflow];
    gkAdd(window, bar);
    return bar;
}

void gkCocoaCommandBarSetActions(void *pointer, const char *ids, size_t idsSize, const char *titles, size_t titlesSize,
                                 void **icons, const int *enabled, int count, int compact) {
    GKCocoaCommandBar *bar = pointer;
    for (NSButton *button in bar.buttons) {
        [button removeFromSuperview];
    }
    [bar.buttons removeAllObjects];
    NSArray<NSString *> *identifiers = gkSplit(ids, idsSize);
    NSArray<NSString *> *names = gkSplit(titles, titlesSize);
    for (int i = 0; i < count; i++) {
        NSString *title = i < (int)names.count ? names[i] : @"";
        NSImage *icon = icons[i];
        NSButton *button =
            [NSButton buttonWithTitle:compact && icon ? @"" : title target:bar action:@selector(activate:)];
        button.identifier = i < (int)identifiers.count ? identifiers[i] : @"";
        button.image = icon;
        button.imagePosition = !icon ? NSNoImage : compact ? NSImageOnly : NSImageLeading;
        button.bezelStyle = NSBezelStyleAccessoryBarAction;
        button.toolTip = title;
        button.accessibilityLabel = title;
        button.enabled = enabled[i] != 0;
        [bar.buttons addObject:button];
        [bar addSubview:button positioned:NSWindowBelow relativeTo:bar.overflow];
    }
    [bar setNeedsLayout:YES];
}
