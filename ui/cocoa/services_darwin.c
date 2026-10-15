#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <UserNotifications/UserNotifications.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include <stdlib.h>
#include <string.h>

void gkCocoaSend(uintptr_t handle, int kind, int a, int b, double x, double y, NSString *text);
NSImage *gkCocoaImageFromPixels(const unsigned char *pixels, int width, int height);
unsigned char *gkCocoaImagePixels(NSImage *image, int *width, int *height);
NSString *gkCocoaTypeForMIME(NSString *mime);
NSString *gkCocoaMIMEForType(NSString *type);
NSFont *gkCocoaFont(const char *family, double size, int weight, int italic);

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

static char *gkJoin(NSArray<NSString *> *values, size_t *size) {
    NSMutableData *data = [NSMutableData data];
    for (NSString *value in values) {
        const char *utf8 = value.UTF8String ?: "";
        [data appendBytes:utf8 length:strlen(utf8) + 1];
    }
    *size = data.length;
    if (!data.length) {
        return NULL;
    }
    char *result = malloc(data.length);
    memcpy(result, data.bytes, data.length);
    return result;
}

static char *gkCopy(NSString *value) {
    return strdup(value.UTF8String ?: "");
}

/* ---- Modal dialogs ---- */

/* GKCocoaModal runs an alert as a sheet on a visible parent window, or as an
   application-modal session otherwise. Go steps it so a context can cancel a
   dialog that is already visible. */
@interface GKCocoaModal : NSObject
@property(retain) NSAlert *alert;
@property(assign) NSWindow *parent;
@property NSModalSession session;
@property NSModalResponse response;
@property BOOL sheet;
@property BOOL done;
@end

@implementation GKCocoaModal
- (void)dealloc {
    [_alert release];
    [super dealloc];
}
@end

static NSAlertStyle gkAlertStyle(int severity) {
    switch (severity) {
    case 2:
        return NSAlertStyleWarning;
    case 3:
        return NSAlertStyleCritical;
    default:
        return NSAlertStyleInformational;
    }
}

void *gkCocoaAlertCreate(int severity, const char *title, const char *message, const char *details, const char *buttons,
                         size_t buttonsSize, int cancelIndex, int destructiveMask, int prompt, int secure,
                         const char *text, const char *placeholder) {
    @autoreleasepool {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.alertStyle = gkAlertStyle(severity);
        NSString *heading = gkString(title);
        NSString *body = gkString(message);
        NSString *detail = gkString(details);
        alert.messageText = heading.length ? heading : body;
        NSMutableArray *informative = [NSMutableArray array];
        if (heading.length && body.length) {
            [informative addObject:body];
        }
        if (detail.length) {
            [informative addObject:detail];
        }
        alert.informativeText = [informative componentsJoinedByString:@"\n\n"];
        NSArray<NSString *> *titles = gkSplit(buttons, buttonsSize);
        for (NSUInteger i = 0; i < titles.count; i++) {
            NSButton *button = [alert addButtonWithTitle:titles[i]];
            if ((int)i == cancelIndex) {
                button.keyEquivalent = @"\x1b";
            }
            if (destructiveMask & (1 << i)) {
                button.hasDestructiveAction = YES;
            }
        }
        if (prompt) {
            NSTextField *field = secure
                                     ? [[[NSSecureTextField alloc] initWithFrame:NSMakeRect(0, 0, 260, 24)] autorelease]
                                     : [[[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 260, 24)] autorelease];
            field.stringValue = gkString(text);
            field.placeholderString = gkString(placeholder);
            alert.accessoryView = field;
            [alert layout];
            alert.window.initialFirstResponder = field;
        }
        return alert;
    }
}

void *gkCocoaModalBegin(void *alertPointer, void *parentPointer) {
    NSAlert *alert = alertPointer;
    NSWindow *parent = parentPointer;
    GKCocoaModal *modal = [GKCocoaModal new];
    modal.alert = alert;
    modal.parent = parent;
    if (parent && parent.isVisible && !parent.attachedSheet) {
        modal.sheet = YES;
        [alert beginSheetModalForWindow:parent
                      completionHandler:^(NSModalResponse response) {
                        modal.response = response;
                        modal.done = YES;
                      }];
    } else {
        [alert layout];
        [alert.window center];
        modal.session = [NSApp beginModalSessionForWindow:alert.window];
    }
    return modal;
}

/* gkCocoaModalStep processes pending events and returns the zero-based button
   index, or -1 while the dialog is open. */
int gkCocoaModalStep(void *pointer) {
    @autoreleasepool {
        GKCocoaModal *modal = pointer;
        if (modal.sheet) {
            NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                                untilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]
                                                   inMode:NSDefaultRunLoopMode
                                                  dequeue:YES];
            if (event) {
                [NSApp sendEvent:event];
            }
        } else {
            NSModalResponse response = [NSApp runModalSession:modal.session];
            if (response != NSModalResponseContinue) {
                modal.response = response;
                modal.done = YES;
            }
        }
        if (!modal.done) {
            return -1;
        }
        return (int)(modal.response - NSAlertFirstButtonReturn);
    }
}

void gkCocoaModalEnd(void *pointer, int abort) {
    GKCocoaModal *modal = pointer;
    if (modal.sheet) {
        if (!modal.done) {
            [modal.parent endSheet:modal.alert.window returnCode:NSModalResponseAbort];
        }
    } else if (modal.session) {
        [NSApp endModalSession:modal.session];
        [modal.alert.window orderOut:nil];
    }
    (void)abort;
    [modal release];
}

char *gkCocoaAlertText(void *pointer) {
    NSAlert *alert = pointer;
    NSTextField *field = (NSTextField *)alert.accessoryView;
    return gkCopy([field isKindOfClass:NSTextField.class] ? field.stringValue : @"");
}

void gkCocoaShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                      const char *credits, void *icon) {
    @autoreleasepool {
        NSMutableDictionary *options = [NSMutableDictionary dictionary];
        NSString *applicationName = gkString(name);
        if (applicationName.length) {
            options[NSAboutPanelOptionApplicationName] = applicationName;
        }
        NSString *applicationVersion = gkString(version);
        if (applicationVersion.length) {
            options[NSAboutPanelOptionApplicationVersion] = applicationVersion;
            options[NSAboutPanelOptionVersion] = @"";
        }
        if (icon) {
            options[NSAboutPanelOptionApplicationIcon] = (NSImage *)icon;
        }
        NSMutableAttributedString *text = [[[NSMutableAttributedString alloc] init] autorelease];
        NSDictionary *attributes = @{
            NSFontAttributeName : [NSFont systemFontOfSize:NSFont.smallSystemFontSize],
            NSForegroundColorAttributeName : NSColor.labelColor,
        };
        NSString *creditText = gkString(credits);
        if (creditText.length) {
            [text appendAttributedString:[[[NSAttributedString alloc] initWithString:creditText attributes:attributes]
                                             autorelease]];
        }
        NSString *site = gkString(website);
        if (site.length) {
            if (text.length) {
                [text appendAttributedString:[[[NSAttributedString alloc] initWithString:@"\n"] autorelease]];
            }
            NSMutableDictionary *linkAttributes = [[attributes mutableCopy] autorelease];
            linkAttributes[NSLinkAttributeName] = [NSURL URLWithString:site] ?: site;
            [text appendAttributedString:[[[NSAttributedString alloc] initWithString:site attributes:linkAttributes]
                                             autorelease]];
        }
        if (text.length) {
            options[NSAboutPanelOptionCredits] = text;
        }
        NSString *copyrightText = gkString(copyright);
        if (copyrightText.length) {
            options[@"Copyright"] = copyrightText;
        }
        [NSApp orderFrontStandardAboutPanelWithOptions:options];
    }
}

/* ---- Clipboard ---- */

long gkCocoaClipboardChangeCount(void) {
    return NSPasteboard.generalPasteboard.changeCount;
}

void gkCocoaClipboardClear(void) {
    [NSPasteboard.generalPasteboard clearContents];
}

int gkCocoaClipboardWrite(const char *text, int hasText, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *types, size_t typesSize,
                          const unsigned char *data, const size_t *sizes, int count) {
    @autoreleasepool {
        NSPasteboard *pasteboard = NSPasteboard.generalPasteboard;
        [pasteboard clearContents];
        NSMutableArray *objects = [NSMutableArray array];
        NSPasteboardItem *item = [[NSPasteboardItem new] autorelease];
        BOOL hasItem = NO;
        if (hasText) {
            [item setString:gkString(text) forType:NSPasteboardTypeString];
            hasItem = YES;
        }
        NSImage *image = gkCocoaImageFromPixels(pixels, width, height);
        if (image) {
            NSBitmapImageRep *bitmap = (NSBitmapImageRep *)image.representations.firstObject;
            NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
            if (png) {
                [item setData:png forType:NSPasteboardTypePNG];
            }
            [item setData:image.TIFFRepresentation forType:NSPasteboardTypeTIFF];
            hasItem = YES;
        }
        NSArray<NSString *> *mimes = gkSplit(types, typesSize);
        size_t offset = 0;
        for (int i = 0; i < count && i < (int)mimes.count; i++) {
            [item setData:[NSData dataWithBytes:data + offset length:sizes[i]] forType:gkCocoaTypeForMIME(mimes[i])];
            offset += sizes[i];
            hasItem = YES;
        }
        NSArray<NSString *> *paths = gkSplit(files, filesSize);
        if (paths.count) {
            /* The first file shares the item with the other representations. */
            [item setString:[NSURL fileURLWithPath:paths[0]].absoluteString forType:NSPasteboardTypeFileURL];
            hasItem = YES;
            for (NSUInteger i = 1; i < paths.count; i++) {
                NSPasteboardItem *fileItem = [[NSPasteboardItem new] autorelease];
                [fileItem setString:[NSURL fileURLWithPath:paths[i]].absoluteString forType:NSPasteboardTypeFileURL];
                [objects addObject:fileItem];
            }
        }
        if (hasItem) {
            [objects insertObject:item atIndex:0];
        }
        return objects.count == 0 || [pasteboard writeObjects:objects];
    }
}

void gkCocoaClipboardRead(char **text, int *hasText, char **files, size_t *filesSize, unsigned char **pixels,
                          int *width, int *height, char **types, size_t *typesSize, unsigned char **data,
                          size_t **sizes, int *count) {
    @autoreleasepool {
        NSPasteboard *pasteboard = NSPasteboard.generalPasteboard;
        NSString *string = [pasteboard stringForType:NSPasteboardTypeString];
        *hasText = string != nil;
        *text = gkCopy(string);
        NSArray<NSURL *> *urls =
            [pasteboard readObjectsForClasses:@[ NSURL.class ] options:@{
                NSPasteboardURLReadingFileURLsOnlyKey : @YES
            }];
        NSMutableArray *paths = [NSMutableArray array];
        for (NSURL *url in urls) {
            [paths addObject:url.path];
        }
        *files = gkJoin(paths, filesSize);
        *pixels = NULL;
        *width = 0;
        *height = 0;
        if ([pasteboard canReadObjectForClasses:@[ NSImage.class ] options:nil] &&
            ([pasteboard.types containsObject:NSPasteboardTypePNG] ||
             [pasteboard.types containsObject:NSPasteboardTypeTIFF])) {
            NSImage *image = [[[NSImage alloc] initWithPasteboard:pasteboard] autorelease];
            if (image) {
                *pixels = gkCocoaImagePixels(image, width, height);
            }
        }
        NSMutableArray *mimes = [NSMutableArray array];
        NSMutableData *buffer = [NSMutableData data];
        NSMutableArray<NSNumber *> *lengths = [NSMutableArray array];
        NSSet *standard = [NSSet setWithArray:@[
            NSPasteboardTypeString, NSPasteboardTypeFileURL, NSPasteboardTypePNG, NSPasteboardTypeTIFF
        ]];
        for (NSString *type in pasteboard.pasteboardItems.firstObject.types) {
            if ([standard containsObject:type] || [type hasPrefix:@"dyn."] == NO && [type hasPrefix:@"com.apple."]) {
                continue;
            }
            NSString *mime = gkCocoaMIMEForType(type);
            NSData *value = [pasteboard.pasteboardItems.firstObject dataForType:type];
            if (!mime || !value) {
                continue;
            }
            [mimes addObject:mime];
            [buffer appendData:value];
            [lengths addObject:@(value.length)];
        }
        *types = gkJoin(mimes, typesSize);
        *count = (int)lengths.count;
        *data = NULL;
        *sizes = NULL;
        if (lengths.count) {
            *data = malloc(buffer.length + 1);
            memcpy(*data, buffer.bytes, buffer.length);
            *sizes = calloc(lengths.count, sizeof(size_t));
            for (NSUInteger i = 0; i < lengths.count; i++) {
                (*sizes)[i] = lengths[i].unsignedLongValue;
            }
        }
    }
}

char *gkCocoaClipboardFormats(size_t *size) {
    @autoreleasepool {
        NSMutableOrderedSet *mimes = [NSMutableOrderedSet orderedSet];
        for (NSString *type in NSPasteboard.generalPasteboard.types) {
            NSString *mime = gkCocoaMIMEForType(type);
            if (mime) {
                [mimes addObject:mime];
            }
        }
        return gkJoin(mimes.array, size);
    }
}

/* ---- Files and URLs ---- */

int gkCocoaRevealFile(const char *path, int select) {
    @autoreleasepool {
        NSString *value = gkString(path);
        NSURL *url = [NSURL fileURLWithPath:value];
        if (![NSFileManager.defaultManager fileExistsAtPath:url.path]) {
            return 0;
        }
        if (select) {
            [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ url ]];
            return 1;
        }
        BOOL directory = NO;
        [NSFileManager.defaultManager fileExistsAtPath:url.path isDirectory:&directory];
        return [NSWorkspace.sharedWorkspace openURL:directory ? url : url.URLByDeletingLastPathComponent];
    }
}

void gkCocoaRecentAdd(const char *path) {
    @autoreleasepool {
        [NSDocumentController.sharedDocumentController noteNewRecentDocumentURL:[NSURL fileURLWithPath:gkString(path)]];
    }
}

char *gkCocoaRecentList(size_t *size) {
    @autoreleasepool {
        NSMutableArray *paths = [NSMutableArray array];
        for (NSURL *url in NSDocumentController.sharedDocumentController.recentDocumentURLs) {
            [paths addObject:url.path];
        }
        return gkJoin(paths, size);
    }
}

void gkCocoaRecentClear(void) {
    [NSDocumentController.sharedDocumentController clearRecentDocuments:nil];
}

/* gkCocoaRecentRemove rebuilds the list without path, because AppKit has no
   API to remove one entry. */
void gkCocoaRecentRemove(const char *path) {
    @autoreleasepool {
        NSDocumentController *controller = NSDocumentController.sharedDocumentController;
        NSString *target = [NSURL fileURLWithPath:gkString(path)].URLByStandardizingPath.path;
        NSArray<NSURL *> *urls = [[controller.recentDocumentURLs copy] autorelease];
        [controller clearRecentDocuments:nil];
        for (NSURL *url in urls.reverseObjectEnumerator) {
            if (![url.URLByStandardizingPath.path isEqualToString:target]) {
                [controller noteNewRecentDocumentURL:url];
            }
        }
    }
}

char *gkCocoaDefaultApplication(const char *extension) {
    @autoreleasepool {
        NSString *value = gkString(extension);
        if ([value hasPrefix:@"."]) {
            value = [value substringFromIndex:1];
        }
        UTType *type = [UTType typeWithFilenameExtension:value];
        NSURL *application = type ? [NSWorkspace.sharedWorkspace URLForApplicationToOpenContentType:type] : nil;
        return gkCopy(application.path ?: @"");
    }
}

/* ---- Printing ---- */

extern int gkCocoaGoPrintPageCount(uintptr_t handle);
extern int gkCocoaGoPrintPage(uintptr_t handle, int page, double width, double height);

@interface GKCocoaPrintView : NSView
@property uintptr_t handle;
@property NSInteger pages;
@end

@implementation GKCocoaPrintView
- (BOOL)isFlipped {
    return YES;
}
- (BOOL)knowsPageRange:(NSRangePointer)range {
    self.pages = MAX(0, gkCocoaGoPrintPageCount(self.handle));
    *range = NSMakeRange(1, self.pages);
    return YES;
}
- (NSRect)rectForPage:(NSInteger)page {
    NSSize size = NSPrintOperation.currentOperation.printInfo.imageablePageBounds.size;
    return NSMakeRect(0, (page - 1) * size.height, size.width, size.height);
}
- (void)drawRect:(NSRect)dirty {
    NSSize size = NSPrintOperation.currentOperation.printInfo.imageablePageBounds.size;
    NSInteger page = size.height > 0 ? (NSInteger)floor(NSMinY(dirty) / size.height) : 0;
    [NSGraphicsContext saveGraphicsState];
    NSAffineTransform *transform = [NSAffineTransform transform];
    [transform translateXBy:0 yBy:page * size.height];
    [transform concat];
    gkCocoaGoPrintPage(self.handle, (int)page, size.width, size.height);
    [NSGraphicsContext restoreGraphicsState];
}
@end

int gkCocoaPrint(uintptr_t handle, const char *title, int showDialog, void *window) {
    @autoreleasepool {
        NSPrintInfo *info = [[NSPrintInfo.sharedPrintInfo copy] autorelease];
        NSSize paper = info.imageablePageBounds.size;
        GKCocoaPrintView *view =
            [[[GKCocoaPrintView alloc] initWithFrame:NSMakeRect(0, 0, paper.width, paper.height)] autorelease];
        view.handle = handle;
        NSRange range;
        [view knowsPageRange:&range];
        [view setFrameSize:NSMakeSize(paper.width, paper.height * MAX(1, view.pages))];
        NSPrintOperation *operation = [NSPrintOperation printOperationWithView:view printInfo:info];
        operation.jobTitle = gkString(title);
        operation.showsPrintPanel = showDialog != 0;
        operation.showsProgressPanel = showDialog != 0;
        (void)window;
        return [operation runOperation] ? (int)view.pages : -1;
    }
}

void gkCocoaPrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w, double h) {
    NSImage *image = gkCocoaImageFromPixels(pixels, width, height);
    [image drawInRect:NSMakeRect(x, y, w, h)
              fromRect:NSZeroRect
             operation:NSCompositingOperationSourceOver
              fraction:1
        respectFlipped:YES
                 hints:nil];
}

int gkCocoaPrintText(const char *text, double x, double y, const char *family, double size, int weight, int italic) {
    NSFont *font = gkCocoaFont(family, size, weight, italic);
    if (!font) {
        return 0;
    }
    [gkString(text) drawAtPoint:NSMakePoint(x, y)
                 withAttributes:@{NSFontAttributeName : font, NSForegroundColorAttributeName : NSColor.blackColor}];
    return 1;
}

/* ---- Sharing ---- */

@interface GKCocoaSharePicker : NSObject <NSSharingServicePickerDelegate>
@property(copy) NSString *service;
@property BOOL done;
@property(retain) NSSharingServicePicker *picker;
@end

@implementation GKCocoaSharePicker
- (void)dealloc {
    [_service release];
    [_picker release];
    [super dealloc];
}
- (void)sharingServicePicker:(NSSharingServicePicker *)picker didChooseSharingService:(NSSharingService *)service {
    (void)picker;
    self.service = service.title ?: @"";
    self.done = YES;
}
@end

void *gkCocoaShareBegin(const char *texts, size_t textsSize, const char *urls, size_t urlsSize, const char *files,
                        size_t filesSize, void *viewPointer) {
    @autoreleasepool {
        NSMutableArray *items = [NSMutableArray array];
        for (NSString *text in gkSplit(texts, textsSize)) {
            [items addObject:text];
        }
        for (NSString *url in gkSplit(urls, urlsSize)) {
            NSURL *value = [NSURL URLWithString:url];
            if (value) {
                [items addObject:value];
            }
        }
        for (NSString *path in gkSplit(files, filesSize)) {
            [items addObject:[NSURL fileURLWithPath:path]];
        }
        NSView *view = viewPointer;
        if (!view || !view.window) {
            return NULL;
        }
        GKCocoaSharePicker *picker = [GKCocoaSharePicker new];
        picker.picker = [[[NSSharingServicePicker alloc] initWithItems:items] autorelease];
        picker.picker.delegate = picker;
        [picker.picker showRelativeToRect:view.bounds ofView:view preferredEdge:NSRectEdgeMinY];
        return picker;
    }
}

int gkCocoaShareStep(void *pointer, char **service) {
    @autoreleasepool {
        GKCocoaSharePicker *picker = pointer;
        NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                            untilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]
                                               inMode:NSDefaultRunLoopMode
                                              dequeue:YES];
        if (event) {
            [NSApp sendEvent:event];
        }
        if (!picker.done) {
            return 0;
        }
        *service = gkCopy(picker.service);
        return 1;
    }
}

void gkCocoaShareEnd(void *pointer) {
    GKCocoaSharePicker *picker = pointer;
    picker.picker.delegate = nil;
    [picker release];
}

/* ---- Notifications ---- */

int gkCocoaNotificationsAvailable(void) {
    NSBundle *bundle = NSBundle.mainBundle;
    return bundle.bundleIdentifier.length > 0 && [bundle.bundlePath hasSuffix:@".app"];
}

extern void gkCocoaGoNotificationResult(uintptr_t token, int status, char *ids, size_t idsSize, char *error);

static uintptr_t gkCocoaNotificationHandle;
static NSMutableDictionary<NSString *, UNNotificationCategory *> *gkCocoaCategories;

/* GKCocoaNotificationDelegate becomes the notification center delegate so
   that it can report notification responses. An application that embeds
   GameKit can have its own delegate: this delegate keeps it as previous,
   lets it decide how to present notifications, gives it every response, and
   forwards every other message to it. */
@interface GKCocoaNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@property(retain) id<UNUserNotificationCenterDelegate> previous;
@end

@implementation GKCocoaNotificationDelegate
- (BOOL)respondsToSelector:(SEL)selector {
    return [super respondsToSelector:selector] || [self.previous respondsToSelector:selector];
}
- (id)forwardingTargetForSelector:(SEL)selector {
    if ([self.previous respondsToSelector:selector]) {
        return self.previous;
    }
    return [super forwardingTargetForSelector:selector];
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completion {
    if ([self.previous respondsToSelector:_cmd]) {
        [self.previous userNotificationCenter:center
                      willPresentNotification:notification
                        withCompletionHandler:completion];
        return;
    }
    completion(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
               UNNotificationPresentationOptionSound);
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
    didReceiveNotificationResponse:(UNNotificationResponse *)response
             withCompletionHandler:(void (^)(void))completion {
    [self reportResponse:response];
    if ([self.previous respondsToSelector:_cmd]) {
        [self.previous userNotificationCenter:center
               didReceiveNotificationResponse:response
                        withCompletionHandler:completion];
        return;
    }
    completion();
}
/* reportResponse sends a response to the GameKit subscribers. */
- (void)reportResponse:(UNNotificationResponse *)response {
    NSString *identifier = [response.notification.request.identifier copy];
    NSString *action = [response.actionIdentifier copy];
    NSString *reply = [response isKindOfClass:UNTextInputNotificationResponse.class]
                          ? [((UNTextInputNotificationResponse *)response).userText copy]
                          : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
      NSString *actionID = [action isEqualToString:UNNotificationDefaultActionIdentifier] ||
                                   [action isEqualToString:UNNotificationDismissActionIdentifier]
                               ? @""
                               : action;
      GKCocoaEvent event = {0};
      event.kind = GK_EVENT_NOTIFICATION;
      event.text = identifier.UTF8String;
      event.text2 = actionID.UTF8String;
      event.object = (void *)reply.UTF8String;
      if (gkCocoaNotificationHandle && ![action isEqualToString:UNNotificationDismissActionIdentifier]) {
          gkCocoaGoEvent(gkCocoaNotificationHandle, &event);
      }
      [identifier release];
      [action release];
      [reply release];
    });
}
@end

static GKCocoaNotificationDelegate *gkCocoaNotificationDelegate;

static UNUserNotificationCenter *gkCocoaCenter(void) {
    UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
    if (!gkCocoaNotificationDelegate) {
        gkCocoaNotificationDelegate = [GKCocoaNotificationDelegate new];
        gkCocoaCategories = [NSMutableDictionary new];
        gkCocoaNotificationDelegate.previous = center.delegate;
        center.delegate = gkCocoaNotificationDelegate;
    }
    return center;
}

void gkCocoaNotificationsAttach(uintptr_t handle) {
    gkCocoaNotificationHandle = handle;
    if (gkCocoaNotificationsAvailable()) {
        gkCocoaCenter();
    }
}

static int gkAuthorization(UNAuthorizationStatus status) {
    switch (status) {
    case UNAuthorizationStatusDenied:
        return 1;
    case UNAuthorizationStatusAuthorized:
        return 2;
    case UNAuthorizationStatusProvisional:
        return 3;
    default:
        return 0;
    }
}

void gkCocoaNotificationAuthorization(uintptr_t token) {
    [gkCocoaCenter() getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
      gkCocoaGoNotificationResult(token, gkAuthorization(settings.authorizationStatus), NULL, 0, NULL);
    }];
}

void gkCocoaNotificationRequestAuthorization(uintptr_t token) {
    UNUserNotificationCenter *center = gkCocoaCenter();
    [center requestAuthorizationWithOptions:UNAuthorizationOptionAlert | UNAuthorizationOptionSound |
                                            UNAuthorizationOptionBadge
                          completionHandler:^(BOOL granted, NSError *error) {
                            (void)granted;
                            if (error) {
                                gkCocoaGoNotificationResult(token, -1, NULL, 0, gkCopy(error.localizedDescription));
                                return;
                            }
                            [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *s) {
                              gkCocoaGoNotificationResult(token, gkAuthorization(s.authorizationStatus), NULL, 0, NULL);
                            }];
                          }];
}

void gkCocoaNotificationSend(uintptr_t token, const char *identifier, const char *title, const char *subtitle,
                             const char *body, const char *category, const char *thread, int sound,
                             const char *attachment, double delay, const char *actionIDs, size_t actionIDsSize,
                             const char *actionTitles, size_t actionTitlesSize, const int *actionFlags,
                             int actionCount) {
    @autoreleasepool {
        UNUserNotificationCenter *center = gkCocoaCenter();
        UNMutableNotificationContent *content = [[UNMutableNotificationContent new] autorelease];
        content.title = gkString(title);
        content.subtitle = gkString(subtitle);
        content.body = gkString(body);
        content.threadIdentifier = gkString(thread);
        if (sound) {
            content.sound = UNNotificationSound.defaultSound;
        }
        NSString *path = gkString(attachment);
        if (path.length) {
            NSError *error = nil;
            UNNotificationAttachment *file =
                [UNNotificationAttachment attachmentWithIdentifier:@"icon"
                                                               URL:[NSURL fileURLWithPath:path]
                                                           options:nil
                                                             error:&error];
            if (file) {
                content.attachments = @[ file ];
            }
        }
        NSString *categoryID = gkString(category);
        NSArray<NSString *> *ids = gkSplit(actionIDs, actionIDsSize);
        NSArray<NSString *> *titles = gkSplit(actionTitles, actionTitlesSize);
        if (actionCount > 0) {
            NSMutableArray *actions = [NSMutableArray array];
            for (int i = 0; i < actionCount && i < (int)ids.count && i < (int)titles.count; i++) {
                UNNotificationActionOptions options = UNNotificationActionOptionForeground;
                if (actionFlags[i] & 1) {
                    options |= UNNotificationActionOptionDestructive;
                }
                if (actionFlags[i] & 2) {
                    [actions addObject:[UNTextInputNotificationAction actionWithIdentifier:ids[i]
                                                                                     title:titles[i]
                                                                                   options:options
                                                                      textInputButtonTitle:titles[i]
                                                                      textInputPlaceholder:@""]];
                } else {
                    [actions
                        addObject:[UNNotificationAction actionWithIdentifier:ids[i] title:titles[i] options:options]];
                }
            }
            gkCocoaCategories[categoryID] = [UNNotificationCategory categoryWithIdentifier:categoryID
                                                                                   actions:actions
                                                                         intentIdentifiers:@[]
                                                                                   options:0];
            [center setNotificationCategories:[NSSet setWithArray:gkCocoaCategories.allValues]];
        }
        if (categoryID.length) {
            content.categoryIdentifier = categoryID;
        }
        UNNotificationTrigger *trigger = nil;
        if (delay > 0) {
            trigger = [UNTimeIntervalNotificationTrigger triggerWithTimeInterval:MAX(1, delay) repeats:NO];
        }
        UNNotificationRequest *request =
            [UNNotificationRequest requestWithIdentifier:gkString(identifier) content:content trigger:trigger];
        [center addNotificationRequest:request
                 withCompletionHandler:^(NSError *error) {
                   gkCocoaGoNotificationResult(token, error ? -1 : 0, NULL, 0,
                                               error ? gkCopy(error.localizedDescription) : NULL);
                 }];
    }
}

void gkCocoaNotificationPending(uintptr_t token) {
    [gkCocoaCenter() getPendingNotificationRequestsWithCompletionHandler:^(NSArray<UNNotificationRequest *> *requests) {
      NSMutableArray *ids = [NSMutableArray array];
      for (UNNotificationRequest *request in requests) {
          [ids addObject:request.identifier];
      }
      size_t size = 0;
      char *buffer = gkJoin(ids, &size);
      gkCocoaGoNotificationResult(token, 0, buffer, size, NULL);
    }];
}

void gkCocoaNotificationDelivered(uintptr_t token) {
    [gkCocoaCenter() getDeliveredNotificationsWithCompletionHandler:^(NSArray<UNNotification *> *notifications) {
      NSMutableArray *ids = [NSMutableArray array];
      for (UNNotification *notification in notifications) {
          [ids addObject:notification.request.identifier];
      }
      size_t size = 0;
      char *buffer = gkJoin(ids, &size);
      gkCocoaGoNotificationResult(token, 0, buffer, size, NULL);
    }];
}

void gkCocoaNotificationWithdraw(const char *ids, size_t size) {
    @autoreleasepool {
        NSArray *identifiers = gkSplit(ids, size);
        UNUserNotificationCenter *center = gkCocoaCenter();
        [center removePendingNotificationRequestsWithIdentifiers:identifiers];
        [center removeDeliveredNotificationsWithIdentifiers:identifiers];
    }
}

void gkCocoaSetBadge(const char *badge) {
    NSString *value = gkString(badge);
    [NSApplication sharedApplication].dockTile.badgeLabel = value.length ? value : nil;
}

char *gkCocoaBadge(void) {
    return gkCopy([NSApplication sharedApplication].dockTile.badgeLabel ?: @"");
}

/* ---- Status items ---- */

@interface GKCocoaTray : NSObject
@property(retain) NSStatusItem *item;
@property uintptr_t handle;
- (void)activate:(id)sender;
@end

@implementation GKCocoaTray
- (void)dealloc {
    [_item release];
    [super dealloc];
}
- (void)activate:(id)sender {
    (void)sender;
    gkCocoaSend(self.handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, nil);
}
@end

void *gkCocoaTrayCreate(uintptr_t handle) {
    GKCocoaTray *tray = [GKCocoaTray new];
    tray.handle = handle;
    tray.item = [NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength];
    tray.item.button.target = tray;
    tray.item.button.action = @selector(activate:);
    return tray;
}

void gkCocoaTrayConfigure(void *pointer, void *icon, int isTemplate, const char *title, const char *tooltip) {
    GKCocoaTray *tray = pointer;
    NSImage *image = icon;
    if (image) {
        image = [[image copy] autorelease];
        image.template = isTemplate != 0;
        if (image.size.height > 18) {
            CGFloat scale = 18 / image.size.height;
            image.size = NSMakeSize(image.size.width * scale, 18);
        }
    }
    tray.item.button.image = image;
    tray.item.button.title = gkString(title);
    tray.item.button.imagePosition = image && tray.item.button.title.length ? NSImageLeading
                                     : image                                ? NSImageOnly
                                                                            : NSNoImage;
    NSString *tip = gkString(tooltip);
    tray.item.button.toolTip = tip.length ? tip : nil;
}

void gkCocoaTraySetVisible(void *pointer, int visible) {
    ((GKCocoaTray *)pointer).item.visible = visible != 0;
}

void gkCocoaTraySetAttention(void *pointer, int attention) {
    GKCocoaTray *tray = pointer;
    [tray.item.button highlight:attention != 0];
    tray.item.button.contentTintColor = attention ? NSColor.controlAccentColor : nil;
}

void gkCocoaTraySetMenu(void *pointer, void *menu) {
    ((GKCocoaTray *)pointer).item.menu = (NSMenu *)menu;
}

int gkCocoaTrayBounds(void *pointer, double *x, double *y, double *width, double *height) {
    GKCocoaTray *tray = pointer;
    NSWindow *window = tray.item.button.window;
    if (!window) {
        return 0;
    }
    NSRect frame = window.frame;
    NSScreen *primary = NSScreen.screens.firstObject;
    *x = NSMinX(frame);
    *y = (primary ? NSMaxY(primary.frame) : NSMaxY(frame)) - NSMaxY(frame);
    *width = frame.size.width;
    *height = frame.size.height;
    return frame.size.width > 0 && frame.size.height > 0;
}

void gkCocoaTrayRemove(void *pointer) {
    GKCocoaTray *tray = pointer;
    tray.handle = 0;
    tray.item.button.target = nil;
    [NSStatusBar.systemStatusBar removeStatusItem:tray.item];
    [tray release];
}
