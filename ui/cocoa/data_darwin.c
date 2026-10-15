#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "cocoa_bridge.h"
#include "widget_classes.h"
#include <string.h>

void gkCocoaSendView(NSView *view, int kind, int a, int b, double x, double y, NSString *text);
NSTextField *gkCocoaBadgeLabel(NSString *text, NSColor *color);

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

static void gkAdd(uintptr_t window, NSView *view) {
    [((NSWindow *)window).contentView addSubview:view];
}

/* ---- Tables and lists ---- */

/* GKCocoaTableView reports Return as activation. */
@interface GKCocoaTableView : NSTableView
@end

@implementation GKCocoaTableView
- (void)keyDown:(NSEvent *)event {
    if ((event.keyCode == 36 || event.keyCode == 76) && self.selectedRow >= 0) {
        [self.target performSelector:self.doubleAction withObject:self];
        return;
    }
    [super keyDown:event];
}
@end

/* GKCocoaTable is a scroll view holding a table. Each row has an ID, one
   string per column, and an optional detail and icon used by list rows. */
@interface GKCocoaTable : NSScrollView <NSTableViewDataSource, NSTableViewDelegate>
@property(retain) GKCocoaTableView *table;
@property(retain) NSArray<NSString *> *ids;
@property(retain) NSArray<NSArray<NSString *> *> *cells;
@property(retain) NSArray<NSString *> *details;
@property(retain) NSArray *icons;
@property(retain) NSArray<NSString *> *columns;
@property BOOL list;
@property BOOL quiet;
@end

@implementation GKCocoaTable
- (void)dealloc {
    self.table.dataSource = nil;
    self.table.delegate = nil;
    [_table release];
    [_ids release];
    [_cells release];
    [_details release];
    [_icons release];
    [_columns release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.table;
}
- (NSInteger)numberOfRowsInTableView:(NSTableView *)view {
    (void)view;
    return self.ids.count;
}
- (NSView *)tableView:(NSTableView *)view viewForTableColumn:(NSTableColumn *)column row:(NSInteger)row {
    NSUInteger index = column ? [self.columns indexOfObject:column.identifier] : 0;
    NSTableCellView *cell = [view makeViewWithIdentifier:self.list ? @"list" : @"cell" owner:self];
    if (!cell) {
        cell = [[[NSTableCellView alloc] initWithFrame:NSZeroRect] autorelease];
        cell.identifier = self.list ? @"list" : @"cell";
        NSTextField *text = [NSTextField labelWithString:@""];
        text.lineBreakMode = NSLineBreakByTruncatingTail;
        text.translatesAutoresizingMaskIntoConstraints = NO;
        [cell addSubview:text];
        cell.textField = text;
        NSImageView *image = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
        image.translatesAutoresizingMaskIntoConstraints = NO;
        [cell addSubview:image];
        cell.imageView = image;
        [NSLayoutConstraint activateConstraints:@[
            [image.leadingAnchor constraintEqualToAnchor:cell.leadingAnchor constant:2],
            [image.centerYAnchor constraintEqualToAnchor:cell.centerYAnchor],
            [image.widthAnchor constraintEqualToConstant:self.list ? 16 : 0],
            [image.heightAnchor constraintEqualToConstant:16],
            [text.leadingAnchor constraintEqualToAnchor:image.trailingAnchor constant:4],
            [text.trailingAnchor constraintEqualToAnchor:cell.trailingAnchor constant:-2],
            [text.centerYAnchor constraintEqualToAnchor:cell.centerYAnchor],
        ]];
    }
    NSArray<NSString *> *values = row < (NSInteger)self.cells.count ? self.cells[row] : @[];
    NSString *value = index < values.count ? values[index] : @"";
    NSString *detail = row < (NSInteger)self.details.count ? self.details[row] : @"";
    if (self.list && detail.length) {
        NSMutableAttributedString *text = [[[NSMutableAttributedString alloc] initWithString:value] autorelease];
        [text appendAttributedString:[[[NSAttributedString alloc]
                                         initWithString:[@"  " stringByAppendingString:detail]
                                             attributes:@{NSForegroundColorAttributeName : NSColor.secondaryLabelColor}]
                                         autorelease]];
        cell.textField.attributedStringValue = text;
    } else {
        cell.textField.stringValue = value;
    }
    id icon = row < (NSInteger)self.icons.count ? self.icons[row] : nil;
    cell.imageView.image = [icon isKindOfClass:NSImage.class] ? icon : nil;
    return cell;
}
- (void)tableViewSelectionDidChange:(NSNotification *)notification {
    (void)notification;
    if (!self.quiet) {
        gkCocoaSendView(self, GK_EVENT_SELECTION, 0, 0, 0, 0, nil);
    }
}
- (void)tableView:(NSTableView *)view sortDescriptorsDidChange:(NSArray<NSSortDescriptor *> *)old {
    (void)old;
    NSSortDescriptor *sort = view.sortDescriptors.firstObject;
    if (sort) {
        gkCocoaSendView(self, GK_EVENT_SORT, sort.ascending, 0, 0, 0, sort.key);
    }
}
- (void)activate:(id)sender {
    (void)sender;
    NSInteger row = self.table.clickedRow >= 0 ? self.table.clickedRow : self.table.selectedRow;
    if (row >= 0 && row < (NSInteger)self.ids.count) {
        gkCocoaSendView(self, GK_EVENT_ACTIVATE, 0, 0, 0, 0, self.ids[row]);
    }
}
@end

void *gkCocoaTableCreate(uintptr_t window, int list, const char *columnIDs, size_t columnIDsSize, const char *titles,
                         size_t titlesSize, const int *widths, const int *sortable, int selection) {
    GKCocoaTable *scroll = [[GKCocoaTable alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)];
    scroll.list = list != 0;
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    scroll.borderType = NSBezelBorder;
    scroll.table = [[[GKCocoaTableView alloc] initWithFrame:NSZeroRect] autorelease];
    scroll.table.usesAlternatingRowBackgroundColors = !list;
    scroll.table.allowsMultipleSelection = selection == 2;
    scroll.table.allowsEmptySelection = YES;
    if (selection == 0) {
        scroll.table.allowsEmptySelection = YES;
    }
    NSArray<NSString *> *ids = gkSplit(columnIDs, columnIDsSize);
    NSArray<NSString *> *names = gkSplit(titles, titlesSize);
    scroll.columns = ids;
    for (NSUInteger i = 0; i < ids.count; i++) {
        NSTableColumn *column = [[[NSTableColumn alloc] initWithIdentifier:ids[i]] autorelease];
        column.title = i < names.count ? names[i] : @"";
        if (widths[i] > 0) {
            column.width = widths[i];
        }
        if (sortable[i]) {
            column.sortDescriptorPrototype = [NSSortDescriptor sortDescriptorWithKey:ids[i] ascending:YES];
        }
        column.resizingMask = NSTableColumnUserResizingMask | NSTableColumnAutoresizingMask;
        [scroll.table addTableColumn:column];
    }
    if (list) {
        scroll.table.headerView = nil;
        scroll.table.columnAutoresizingStyle = NSTableViewUniformColumnAutoresizingStyle;
    }
    scroll.table.dataSource = scroll;
    scroll.table.delegate = scroll;
    scroll.table.target = scroll;
    scroll.table.doubleAction = @selector(activate:);
    scroll.documentView = scroll.table;
    if (selection == 0) {
        scroll.table.selectionHighlightStyle = NSTableViewSelectionHighlightStyleNone;
    }
    gkAdd(window, scroll);
    return scroll;
}

void gkCocoaTableSetRows(void *pointer, const char *ids, size_t idsSize, const char *cells, size_t cellsSize,
                         const char *details, size_t detailsSize, void **icons, int columns) {
    GKCocoaTable *table = pointer;
    NSArray<NSString *> *rowIDs = gkSplit(ids, idsSize);
    NSArray<NSString *> *values = gkSplit(cells, cellsSize);
    NSMutableArray *rows = [NSMutableArray array];
    NSMutableArray *images = [NSMutableArray array];
    for (NSUInteger row = 0; row < rowIDs.count; row++) {
        NSMutableArray *cellsForRow = [NSMutableArray array];
        for (int column = 0; column < columns; column++) {
            NSUInteger index = row * columns + column;
            [cellsForRow addObject:index < values.count ? values[index] : @""];
        }
        [rows addObject:cellsForRow];
        id icon = icons ? (id)icons[row] : nil;
        [images addObject:icon ?: [NSNull null]];
    }
    table.quiet = YES;
    table.ids = rowIDs;
    table.cells = rows;
    table.details = gkSplit(details, detailsSize);
    table.icons = images;
    [table.table reloadData];
    table.quiet = NO;
}

char *gkCocoaTableSelection(void *pointer, size_t *size) {
    GKCocoaTable *table = pointer;
    NSMutableArray *ids = [NSMutableArray array];
    [table.table.selectedRowIndexes enumerateIndexesUsingBlock:^(NSUInteger index, BOOL *stop) {
      (void)stop;
      if (index < table.ids.count) {
          [ids addObject:table.ids[index]];
      }
    }];
    return gkJoin(ids, size);
}

void gkCocoaTableSelect(void *pointer, const char *ids, size_t size) {
    GKCocoaTable *table = pointer;
    NSMutableIndexSet *indexes = [NSMutableIndexSet indexSet];
    for (NSString *identifier in gkSplit(ids, size)) {
        NSUInteger index = [table.ids indexOfObject:identifier];
        if (index != NSNotFound) {
            [indexes addIndex:index];
        }
    }
    table.quiet = YES;
    [table.table selectRowIndexes:indexes byExtendingSelection:NO];
    table.quiet = NO;
}

int gkCocoaTableScrollTo(void *pointer, const char *identifier) {
    GKCocoaTable *table = pointer;
    NSUInteger index = [table.ids indexOfObject:[NSString stringWithUTF8String:identifier]];
    if (index == NSNotFound) {
        return 0;
    }
    [table.table scrollRowToVisible:index];
    return 1;
}

/* ---- Outlines: trees and navigation lists ---- */

@interface GKCocoaNode : NSObject
@property(copy) NSString *identifier;
@property(copy) NSString *text;
@property(copy) NSString *badge;
@property(retain) NSImage *icon;
@property BOOL hasChildren;
@property BOOL group;
@property(retain) NSMutableArray<GKCocoaNode *> *children;
@end

@implementation GKCocoaNode
- (void)dealloc {
    [_identifier release];
    [_text release];
    [_badge release];
    [_icon release];
    [_children release];
    [super dealloc];
}
@end

@interface GKCocoaOutline : NSScrollView <NSOutlineViewDataSource, NSOutlineViewDelegate>
@property(retain) NSOutlineView *outline;
@property(retain) NSMutableArray<GKCocoaNode *> *roots;
@property BOOL navigation;
@property BOOL quiet;
@end

@implementation GKCocoaOutline
- (void)dealloc {
    self.outline.dataSource = nil;
    self.outline.delegate = nil;
    [_outline release];
    [_roots release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.outline;
}
- (NSArray *)childrenOf:(GKCocoaNode *)item {
    return item ? item.children : self.roots;
}
- (NSInteger)outlineView:(NSOutlineView *)view numberOfChildrenOfItem:(id)item {
    (void)view;
    return [self childrenOf:item].count;
}
- (id)outlineView:(NSOutlineView *)view child:(NSInteger)index ofItem:(id)item {
    (void)view;
    return [self childrenOf:item][index];
}
- (BOOL)outlineView:(NSOutlineView *)view isItemExpandable:(id)item {
    (void)view;
    GKCocoaNode *node = item;
    return node.hasChildren || node.children.count > 0;
}
- (BOOL)outlineView:(NSOutlineView *)view isGroupItem:(id)item {
    (void)view;
    return ((GKCocoaNode *)item).group;
}
- (BOOL)outlineView:(NSOutlineView *)view shouldSelectItem:(id)item {
    (void)view;
    return !((GKCocoaNode *)item).group;
}
- (BOOL)outlineView:(NSOutlineView *)view shouldExpandItem:(id)item {
    (void)view;
    GKCocoaNode *node = item;
    if (node.hasChildren && node.children.count == 0) {
        gkCocoaSendView(self, GK_EVENT_EXPAND, 1, 0, 0, 0, node.identifier);
    }
    return YES;
}
- (NSView *)outlineView:(NSOutlineView *)view viewForTableColumn:(NSTableColumn *)column item:(id)item {
    (void)column;
    GKCocoaNode *node = item;
    NSString *identifier = node.group ? @"group" : @"node";
    NSTableCellView *cell = [view makeViewWithIdentifier:identifier owner:self];
    if (!cell) {
        cell = [[[NSTableCellView alloc] initWithFrame:NSZeroRect] autorelease];
        cell.identifier = identifier;
        NSImageView *image = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
        image.translatesAutoresizingMaskIntoConstraints = NO;
        NSTextField *text = [NSTextField labelWithString:@""];
        text.lineBreakMode = NSLineBreakByTruncatingTail;
        text.translatesAutoresizingMaskIntoConstraints = NO;
        NSTextField *badge = gkCocoaBadgeLabel(@"", NSColor.systemGrayColor);
        badge.translatesAutoresizingMaskIntoConstraints = NO;
        badge.tag = 7;
        [cell addSubview:image];
        [cell addSubview:text];
        [cell addSubview:badge];
        cell.imageView = image;
        cell.textField = text;
        if (node.group) {
            text.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize weight:NSFontWeightSemibold];
            text.textColor = NSColor.secondaryLabelColor;
        }
        [NSLayoutConstraint activateConstraints:@[
            [image.leadingAnchor constraintEqualToAnchor:cell.leadingAnchor constant:2],
            [image.centerYAnchor constraintEqualToAnchor:cell.centerYAnchor],
            [image.widthAnchor constraintEqualToConstant:node.group ? 0 : 16],
            [image.heightAnchor constraintEqualToConstant:16],
            [text.leadingAnchor constraintEqualToAnchor:image.trailingAnchor constant:4],
            [text.centerYAnchor constraintEqualToAnchor:cell.centerYAnchor],
            [badge.leadingAnchor constraintGreaterThanOrEqualToAnchor:text.trailingAnchor constant:4],
            [badge.trailingAnchor constraintEqualToAnchor:cell.trailingAnchor constant:-4],
            [badge.centerYAnchor constraintEqualToAnchor:cell.centerYAnchor],
        ]];
    }
    cell.textField.stringValue = node.text ?: @"";
    cell.imageView.image = node.icon;
    NSTextField *badge = [cell viewWithTag:7];
    badge.stringValue = node.badge ?: @"";
    badge.hidden = node.badge.length == 0;
    return cell;
}
- (void)outlineViewSelectionDidChange:(NSNotification *)notification {
    (void)notification;
    if (!self.quiet) {
        gkCocoaSendView(self, GK_EVENT_SELECTION, 0, 0, 0, 0, nil);
    }
}
- (GKCocoaNode *)find:(NSString *)identifier in:(NSArray<GKCocoaNode *> *)nodes {
    for (GKCocoaNode *node in nodes) {
        if ([node.identifier isEqualToString:identifier]) {
            return node;
        }
        GKCocoaNode *found = [self find:identifier in:node.children];
        if (found) {
            return found;
        }
    }
    return nil;
}
@end

void *gkCocoaOutlineCreate(uintptr_t window, int navigation, int multiple) {
    GKCocoaOutline *scroll = [[GKCocoaOutline alloc] initWithFrame:NSMakeRect(0, 0, 240, 300)];
    scroll.navigation = navigation != 0;
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    scroll.borderType = navigation ? NSNoBorder : NSBezelBorder;
    scroll.roots = [NSMutableArray array];
    NSOutlineView *outline = [[[NSOutlineView alloc] initWithFrame:NSZeroRect] autorelease];
    NSTableColumn *column = [[[NSTableColumn alloc] initWithIdentifier:@"text"] autorelease];
    [outline addTableColumn:column];
    outline.outlineTableColumn = column;
    outline.headerView = nil;
    outline.allowsMultipleSelection = multiple != 0;
    outline.columnAutoresizingStyle = NSTableViewUniformColumnAutoresizingStyle;
    if (navigation) {
        outline.style = NSTableViewStyleSourceList;
        outline.floatsGroupRows = NO;
    }
    outline.dataSource = scroll;
    outline.delegate = scroll;
    scroll.outline = outline;
    scroll.documentView = outline;
    if (navigation) {
        scroll.drawsBackground = NO;
    }
    gkAdd(window, scroll);
    return scroll;
}

/* gkCocoaOutlineNodes decodes a depth-first list of nodes: each record holds
   depth, flags (1 has children, 2 group), ID, text, and badge. */
static NSMutableArray<GKCocoaNode *> *gkCocoaOutlineNodes(const int *depths, const int *flags, NSArray *ids,
                                                          NSArray *texts, NSArray *badges, void **icons, int count) {
    NSMutableArray *roots = [NSMutableArray array];
    NSMutableArray<GKCocoaNode *> *stack = [NSMutableArray array];
    for (int i = 0; i < count; i++) {
        GKCocoaNode *node = [[GKCocoaNode new] autorelease];
        node.identifier = i < (int)ids.count ? ids[i] : @"";
        node.text = i < (int)texts.count ? texts[i] : @"";
        node.badge = i < (int)badges.count ? badges[i] : @"";
        node.icon = icons ? (NSImage *)icons[i] : nil;
        node.hasChildren = (flags[i] & 1) != 0;
        node.group = (flags[i] & 2) != 0;
        node.children = [NSMutableArray array];
        while ((int)stack.count > depths[i]) {
            [stack removeLastObject];
        }
        if (stack.count == 0) {
            [roots addObject:node];
        } else {
            [stack.lastObject.children addObject:node];
        }
        [stack addObject:node];
    }
    return roots;
}

void gkCocoaOutlineSetNodes(void *pointer, const char *parent, const int *depths, const int *flags, const char *ids,
                            size_t idsSize, const char *texts, size_t textsSize, const char *badges, size_t badgesSize,
                            void **icons, int count) {
    GKCocoaOutline *outline = pointer;
    NSMutableArray *nodes = gkCocoaOutlineNodes(depths, flags, gkSplit(ids, idsSize), gkSplit(texts, textsSize),
                                                gkSplit(badges, badgesSize), icons, count);
    outline.quiet = YES;
    if (parent) {
        GKCocoaNode *node = [outline find:[NSString stringWithUTF8String:parent] in:outline.roots];
        node.children = nodes;
        node.hasChildren = nodes.count > 0;
        [outline.outline reloadItem:node reloadChildren:YES];
    } else {
        outline.roots = nodes;
        [outline.outline reloadData];
        if (outline.navigation) {
            [outline.outline expandItem:nil expandChildren:YES];
        }
    }
    outline.quiet = NO;
}

char *gkCocoaOutlineSelection(void *pointer, size_t *size) {
    GKCocoaOutline *outline = pointer;
    NSMutableArray *ids = [NSMutableArray array];
    [outline.outline.selectedRowIndexes enumerateIndexesUsingBlock:^(NSUInteger index, BOOL *stop) {
      (void)stop;
      GKCocoaNode *node = [outline.outline itemAtRow:index];
      if (node.identifier) {
          [ids addObject:node.identifier];
      }
    }];
    return gkJoin(ids, size);
}

void gkCocoaOutlineSelect(void *pointer, const char *ids, size_t size) {
    GKCocoaOutline *outline = pointer;
    NSMutableIndexSet *indexes = [NSMutableIndexSet indexSet];
    for (NSString *identifier in gkSplit(ids, size)) {
        GKCocoaNode *node = [outline find:identifier in:outline.roots];
        NSInteger row = node ? [outline.outline rowForItem:node] : -1;
        if (row >= 0) {
            [indexes addIndex:row];
        }
    }
    outline.quiet = YES;
    [outline.outline selectRowIndexes:indexes byExtendingSelection:NO];
    outline.quiet = NO;
}

int gkCocoaOutlineExpanded(void *pointer, const char *identifier) {
    GKCocoaOutline *outline = pointer;
    GKCocoaNode *node = [outline find:[NSString stringWithUTF8String:identifier] in:outline.roots];
    return node && [outline.outline isItemExpanded:node];
}

int gkCocoaOutlineSetExpanded(void *pointer, const char *identifier, int expanded) {
    GKCocoaOutline *outline = pointer;
    GKCocoaNode *node = [outline find:[NSString stringWithUTF8String:identifier] in:outline.roots];
    if (!node) {
        return 0;
    }
    if (expanded) {
        [outline.outline expandItem:node];
    } else {
        [outline.outline collapseItem:node];
    }
    return 1;
}

/* ---- Collection view ---- */

@interface GKCocoaCollectionItem : NSCollectionViewItem
@end

@implementation GKCocoaCollectionItem
- (void)loadView {
    NSView *view = [[[NSView alloc] initWithFrame:NSMakeRect(0, 0, 96, 96)] autorelease];
    view.wantsLayer = YES;
    view.layer.cornerRadius = 6;
    NSImageView *image = [[[NSImageView alloc] initWithFrame:NSZeroRect] autorelease];
    image.imageScaling = NSImageScaleProportionallyUpOrDown;
    image.translatesAutoresizingMaskIntoConstraints = NO;
    NSTextField *text = [NSTextField labelWithString:@""];
    text.alignment = NSTextAlignmentCenter;
    text.lineBreakMode = NSLineBreakByTruncatingTail;
    text.translatesAutoresizingMaskIntoConstraints = NO;
    [view addSubview:image];
    [view addSubview:text];
    [NSLayoutConstraint activateConstraints:@[
        [image.topAnchor constraintEqualToAnchor:view.topAnchor constant:4],
        [image.leadingAnchor constraintEqualToAnchor:view.leadingAnchor constant:4],
        [image.trailingAnchor constraintEqualToAnchor:view.trailingAnchor constant:-4],
        [text.topAnchor constraintEqualToAnchor:image.bottomAnchor constant:2],
        [text.leadingAnchor constraintEqualToAnchor:view.leadingAnchor constant:2],
        [text.trailingAnchor constraintEqualToAnchor:view.trailingAnchor constant:-2],
        [text.bottomAnchor constraintEqualToAnchor:view.bottomAnchor constant:-4],
    ]];
    self.view = view;
    self.imageView = image;
    self.textField = text;
}
- (void)setSelected:(BOOL)selected {
    [super setSelected:selected];
    self.view.layer.backgroundColor =
        selected ? [NSColor.selectedContentBackgroundColor colorWithAlphaComponent:0.3].CGColor : nil;
}
@end

@interface GKCocoaCollection : NSScrollView <NSCollectionViewDataSource, NSCollectionViewDelegate>
@property(retain) NSCollectionView *collection;
@property(retain) NSArray<NSString *> *ids;
@property(retain) NSArray<NSString *> *texts;
@property(retain) NSArray *images;
@property BOOL quiet;
@end

@implementation GKCocoaCollection
- (void)dealloc {
    self.collection.dataSource = nil;
    self.collection.delegate = nil;
    [_collection release];
    [_ids release];
    [_texts release];
    [_images release];
    [super dealloc];
}
- (id)gkAccessibleElement {
    return self.collection;
}
- (NSInteger)collectionView:(NSCollectionView *)view numberOfItemsInSection:(NSInteger)section {
    (void)view;
    (void)section;
    return self.ids.count;
}
- (NSCollectionViewItem *)collectionView:(NSCollectionView *)view
     itemForRepresentedObjectAtIndexPath:(NSIndexPath *)path {
    GKCocoaCollectionItem *item = [view makeItemWithIdentifier:@"item" forIndexPath:path];
    NSInteger index = path.item;
    item.textField.stringValue = index < (NSInteger)self.texts.count ? self.texts[index] : @"";
    id image = index < (NSInteger)self.images.count ? self.images[index] : nil;
    item.imageView.image = [image isKindOfClass:NSImage.class] ? image : nil;
    return item;
}
- (void)report {
    if (!self.quiet) {
        gkCocoaSendView(self, GK_EVENT_SELECTION, 0, 0, 0, 0, nil);
    }
}
- (void)collectionView:(NSCollectionView *)view didSelectItemsAtIndexPaths:(NSSet<NSIndexPath *> *)paths {
    (void)view;
    (void)paths;
    [self report];
}
- (void)collectionView:(NSCollectionView *)view didDeselectItemsAtIndexPaths:(NSSet<NSIndexPath *> *)paths {
    (void)view;
    (void)paths;
    [self report];
}
@end

void *gkCocoaCollectionCreate(uintptr_t window, int width, int height, int selection) {
    GKCocoaCollection *scroll = [[GKCocoaCollection alloc] initWithFrame:NSMakeRect(0, 0, 320, 240)];
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    NSCollectionViewFlowLayout *layout = [[NSCollectionViewFlowLayout new] autorelease];
    layout.itemSize = NSMakeSize(width > 0 ? width : 96, height > 0 ? height : 96);
    layout.minimumInteritemSpacing = 8;
    layout.minimumLineSpacing = 8;
    layout.sectionInset = NSEdgeInsetsMake(8, 8, 8, 8);
    NSCollectionView *collection = [[[NSCollectionView alloc] initWithFrame:NSZeroRect] autorelease];
    collection.collectionViewLayout = layout;
    collection.selectable = selection != 0;
    collection.allowsMultipleSelection = selection == 2;
    [collection registerClass:GKCocoaCollectionItem.class forItemWithIdentifier:@"item"];
    collection.dataSource = scroll;
    collection.delegate = scroll;
    scroll.collection = collection;
    scroll.documentView = collection;
    gkAdd(window, scroll);
    return scroll;
}

void gkCocoaCollectionSetItems(void *pointer, const char *ids, size_t idsSize, const char *texts, size_t textsSize,
                               void **images, int count) {
    GKCocoaCollection *collection = pointer;
    NSMutableArray *pictures = [NSMutableArray array];
    for (int i = 0; i < count; i++) {
        id image = images ? (id)images[i] : nil;
        [pictures addObject:image ?: [NSNull null]];
    }
    collection.quiet = YES;
    collection.ids = gkSplit(ids, idsSize);
    collection.texts = gkSplit(texts, textsSize);
    collection.images = pictures;
    [collection.collection reloadData];
    collection.quiet = NO;
}

char *gkCocoaCollectionSelection(void *pointer, size_t *size) {
    GKCocoaCollection *collection = pointer;
    NSMutableArray *ids = [NSMutableArray array];
    for (NSIndexPath *path in collection.collection.selectionIndexPaths) {
        if (path.item < (NSInteger)collection.ids.count) {
            [ids addObject:collection.ids[path.item]];
        }
    }
    return gkJoin(ids, size);
}

void gkCocoaCollectionSelect(void *pointer, const char *ids, size_t size) {
    GKCocoaCollection *collection = pointer;
    NSMutableSet *paths = [NSMutableSet set];
    for (NSString *identifier in gkSplit(ids, size)) {
        NSUInteger index = [collection.ids indexOfObject:identifier];
        if (index != NSNotFound) {
            [paths addObject:[NSIndexPath indexPathForItem:index inSection:0]];
        }
    }
    collection.quiet = YES;
    collection.collection.selectionIndexPaths = paths;
    collection.quiet = NO;
}

/* ---- Breadcrumbs ---- */

@interface GKCocoaPathTarget : NSObject
@property(assign) NSPathControl *path;
@end

@implementation GKCocoaPathTarget
- (void)clicked:(id)sender {
    (void)sender;
    NSPathControlItem *item = self.path.clickedPathItem;
    NSUInteger index = item ? [self.path.pathItems indexOfObject:item] : NSNotFound;
    if (index != NSNotFound) {
        gkCocoaSendView(self.path, GK_EVENT_ACTIVATE, (int)index, 0, 0, 0, nil);
    }
}
@end

static char gkCocoaPathTargetKey;

void *gkCocoaBreadcrumbsCreate(uintptr_t window) {
    NSPathControl *path = [[NSPathControl alloc] initWithFrame:NSZeroRect];
    path.pathStyle = NSPathStyleStandard;
    GKCocoaPathTarget *target = [[GKCocoaPathTarget new] autorelease];
    target.path = path;
    path.target = target;
    path.action = @selector(clicked:);
    objc_setAssociatedObject(path, &gkCocoaPathTargetKey, target, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    gkAdd(window, path);
    return path;
}

void gkCocoaBreadcrumbsSetItems(void *pointer, const char *titles, size_t size) {
    NSPathControl *path = pointer;
    NSMutableArray *items = [NSMutableArray array];
    for (NSString *title in gkSplit(titles, size)) {
        NSPathControlItem *item = [[NSPathControlItem new] autorelease];
        item.title = title;
        [items addObject:item];
    }
    path.pathItems = items;
}

/* ---- Pagination ---- */

@interface GKCocoaPagination : GKUILayoutView
@property(retain) NSButton *previous;
@property(retain) NSButton *next;
@property(retain) NSTextField *label;
@property(retain) NSSegmentedControl *pages;
@property NSInteger count;
@property NSInteger selected;
@property BOOL compact;
@end

@implementation GKCocoaPagination
- (void)dealloc {
    [_previous release];
    [_next release];
    [_label release];
    [_pages release];
    [super dealloc];
}
/* window returns the zero-based first page shown by the segmented control. */
- (NSInteger)windowStart {
    NSInteger visible = MIN(self.count, 7);
    NSInteger start = self.selected - visible / 2;
    return MAX(0, MIN(start, self.count - visible));
}
- (void)update {
    self.previous.enabled = self.selected > 0;
    self.next.enabled = self.selected + 1 < self.count;
    self.label.stringValue = [NSString stringWithFormat:@"Page %ld of %ld", (long)self.selected + 1, (long)self.count];
    self.label.hidden = !self.compact;
    self.pages.hidden = self.compact;
    if (!self.compact) {
        NSInteger visible = MIN(self.count, 7);
        NSInteger start = [self windowStart];
        self.pages.segmentCount = visible;
        for (NSInteger i = 0; i < visible; i++) {
            [self.pages setLabel:[NSString stringWithFormat:@"%ld", (long)(start + i + 1)] forSegment:i];
            [self.pages setWidth:0 forSegment:i];
        }
        self.pages.selectedSegment = self.selected - start;
    }
    self.accessibilityValueDescription = self.label.stringValue;
    [self setNeedsLayout:YES];
}
- (void)layout {
    [super layout];
    NSSize button = self.previous.fittingSize;
    CGFloat height = self.bounds.size.height;
    self.previous.frame = NSMakeRect(0, (height - button.height) / 2, button.width, button.height);
    self.next.frame =
        NSMakeRect(self.bounds.size.width - button.width, (height - button.height) / 2, button.width, button.height);
    NSView *middle = self.compact ? (NSView *)self.label : self.pages;
    NSSize size = middle.fittingSize;
    middle.frame =
        NSMakeRect((self.bounds.size.width - size.width) / 2, (height - size.height) / 2, size.width, size.height);
}
- (NSSize)fittingSize {
    NSSize button = self.previous.fittingSize;
    NSView *middle = self.compact ? (NSView *)self.label : self.pages;
    NSSize size = middle.fittingSize;
    return NSMakeSize(button.width * 2 + size.width + 16, MAX(button.height, size.height));
}
- (void)choose:(NSInteger)page {
    page = MAX(0, MIN(self.count - 1, page));
    if (page == self.selected) {
        return;
    }
    self.selected = page;
    [self update];
    gkCocoaSendView(self, GK_EVENT_SELECTION, (int)page, 0, 0, 0, nil);
}
- (void)goPrevious:(id)sender {
    (void)sender;
    [self choose:self.selected - 1];
}
- (void)goNext:(id)sender {
    (void)sender;
    [self choose:self.selected + 1];
}
- (void)pick:(id)sender {
    (void)sender;
    [self choose:[self windowStart] + self.pages.selectedSegment];
}
@end

void *gkCocoaPaginationCreate(uintptr_t window, int count, int selected, int compact) {
    GKCocoaPagination *view = [[GKCocoaPagination alloc] initWithFrame:NSZeroRect];
    view.previous = [NSButton
        buttonWithImage:[NSImage imageWithSystemSymbolName:@"chevron.left" accessibilityDescription:@"Previous page"]
                 target:view
                 action:@selector(goPrevious:)];
    view.next = [NSButton
        buttonWithImage:[NSImage imageWithSystemSymbolName:@"chevron.right" accessibilityDescription:@"Next page"]
                 target:view
                 action:@selector(goNext:)];
    view.label = [NSTextField labelWithString:@""];
    view.pages = [[[NSSegmentedControl alloc] initWithFrame:NSZeroRect] autorelease];
    view.pages.trackingMode = NSSegmentSwitchTrackingSelectOne;
    view.pages.target = view;
    view.pages.action = @selector(pick:);
    [view addSubview:view.previous];
    [view addSubview:view.label];
    [view addSubview:view.pages];
    [view addSubview:view.next];
    view.count = count;
    view.selected = selected;
    view.compact = compact != 0;
    view.accessibilityRole = NSAccessibilityGroupRole;
    view.accessibilityLabel = @"Pagination";
    [view update];
    gkAdd(window, view);
    return view;
}

void gkCocoaPaginationSet(void *pointer, int count, int selected) {
    GKCocoaPagination *view = pointer;
    view.count = count;
    view.selected = selected;
    [view update];
}
