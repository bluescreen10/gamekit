#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"

void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text);
NSImage *gkCocoaImageFromPixels(const unsigned char *pixels, int width, int height);

void *gkCocoaIcon(const char *symbol, const unsigned char *pixels, int width, int height, double pointSize) {
    @autoreleasepool {
        NSImage *image = nil;
        if (symbol && symbol[0]) {
            NSString *name = [NSString stringWithUTF8String:symbol];
            image = [NSImage imageWithSystemSymbolName:name accessibilityDescription:nil];
            if (image && pointSize > 0) {
                NSImageSymbolConfiguration *configuration =
                    [NSImageSymbolConfiguration configurationWithPointSize:pointSize weight:NSFontWeightRegular];
                image = [image imageWithSymbolConfiguration:configuration] ?: image;
            }
            if (!image) {
                image = [NSImage imageNamed:name];
            }
        }
        if (!image) {
            image = gkCocoaImageFromPixels(pixels, width, height);
            if (image && pointSize > 0 && width > 0 && height > 0) {
                CGFloat scale = pointSize / MAX(width, height);
                image.size = NSMakeSize(width * scale, height * scale);
            }
        }
        return [image retain];
    }
}

/* GKCocoaMenuTarget reports menu item activations to Go. */
@interface GKCocoaMenuTarget : NSObject
@property uintptr_t handle;
- (void)activate:(NSMenuItem *)item;
@end

@implementation GKCocoaMenuTarget
- (void)activate:(NSMenuItem *)item {
    NSString *identifier = item.representedObject;
    if ([identifier isKindOfClass:NSString.class]) {
        gkCocoaSend(self.handle, GK_EVENT_ACTION, 0, 0, 0, 0, identifier);
    }
}
@end

static char gkCocoaMenuTargetKey;

static GKCocoaMenuTarget *gkCocoaMenuTargetFor(NSMenu *menu) {
    for (NSMenu *current = menu; current; current = current.supermenu) {
        GKCocoaMenuTarget *target = objc_getAssociatedObject(current, &gkCocoaMenuTargetKey);
        if (target) {
            return target;
        }
    }
    return nil;
}

void *gkCocoaMenuCreate(uintptr_t handle, const char *title) {
    NSMenu *menu = [[NSMenu alloc] initWithTitle:title ? [NSString stringWithUTF8String:title] : @""];
    menu.autoenablesItems = NO;
    GKCocoaMenuTarget *target = [[GKCocoaMenuTarget new] autorelease];
    target.handle = handle;
    objc_setAssociatedObject(menu, &gkCocoaMenuTargetKey, target, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    return menu;
}

void gkCocoaMenuSetHandle(void *pointer, uintptr_t handle) {
    GKCocoaMenuTarget *target = objc_getAssociatedObject((NSMenu *)pointer, &gkCocoaMenuTargetKey);
    target.handle = handle;
}

void gkCocoaMenuClear(void *pointer) {
    [(NSMenu *)pointer removeAllItems];
}

void gkCocoaMenuAddItem(void *pointer, const char *actionID, const char *title, int enabled, int checked,
                        const char *key, int modifiers, void *icon, const char *tooltip) {
    NSMenu *menu = pointer;
    NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title ? [NSString stringWithUTF8String:title] : @""
                                                   action:@selector(activate:)
                                            keyEquivalent:key ? [NSString stringWithUTF8String:key] : @""] autorelease];
    item.target = gkCocoaMenuTargetFor(menu);
    item.representedObject = actionID ? [NSString stringWithUTF8String:actionID] : @"";
    item.enabled = enabled != 0;
    item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    item.keyEquivalentModifierMask = (NSEventModifierFlags)modifiers;
    item.image = (NSImage *)icon;
    if (tooltip && tooltip[0]) {
        item.toolTip = [NSString stringWithUTF8String:tooltip];
    }
    [menu addItem:item];
}

void gkCocoaMenuAddSeparator(void *pointer) {
    [(NSMenu *)pointer addItem:[NSMenuItem separatorItem]];
}

void *gkCocoaMenuAddSubmenu(void *pointer, const char *title) {
    NSMenu *menu = pointer;
    NSString *value = title ? [NSString stringWithUTF8String:title] : @"";
    NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:value action:nil keyEquivalent:@""] autorelease];
    NSMenu *submenu = [[[NSMenu alloc] initWithTitle:value] autorelease];
    submenu.autoenablesItems = NO;
    item.submenu = submenu;
    [menu addItem:item];
    return submenu;
}

void gkCocoaMenuSetActionState(void *pointer, const char *actionID, int enabled, int checked) {
    NSMenu *menu = pointer;
    NSString *identifier = actionID ? [NSString stringWithUTF8String:actionID] : @"";
    for (NSMenuItem *item in menu.itemArray) {
        if (item.submenu) {
            gkCocoaMenuSetActionState(item.submenu, actionID, enabled, checked);
        } else if ([item.representedObject isEqual:identifier]) {
            if (enabled >= 0) {
                item.enabled = enabled != 0;
            }
            if (checked >= 0) {
                item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
            }
        }
    }
}

void gkCocoaMenuPopup(void *pointer, void *viewPointer, double x, double y) {
    NSMenu *menu = pointer;
    NSView *view = viewPointer;
    if (!menu || !view) {
        return;
    }
    NSPoint location = NSMakePoint(x, view.isFlipped ? y : view.bounds.size.height - y);
    [menu popUpMenuPositioningItem:nil atLocation:location inView:view];
}
