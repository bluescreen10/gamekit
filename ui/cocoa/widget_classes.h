#ifndef GAMEKIT_COCOA_WIDGET_CLASSES_H
#define GAMEKIT_COCOA_WIDGET_CLASSES_H

/* AppKit view classes shared by the Objective-C files of the Cocoa backend. */

#import <Cocoa/Cocoa.h>

@protocol GKUIContainer <NSObject>
- (NSView *)gkContentView;
@end

@interface GKUILayoutView : NSView
@end

@interface GKUICanvasView : GKUILayoutView
@property uintptr_t handle;
@property(retain) NSTrackingArea *trackingArea;
@property(retain) id displayLink;
@property BOOL observing;
@end

@interface GKUITextAreaView : NSScrollView <NSTextViewDelegate>
@property uintptr_t handle;
@property NSInteger maximumLength;
@property BOOL tabMovesFocus;
@property(retain) NSTextView *editor;
@property(copy) NSString *placeholder;
@end

@interface GKUIRadioGroupView : NSView
@property uintptr_t handle;
@property BOOL horizontal;
@property(retain) NSTextField *titleLabel;
@property(retain) NSMutableArray<NSButton *> *buttons;
@property(nonatomic) NSInteger selectedIndex;
- (void)addItem:(NSString *)title;
@end

/* GKUIPushButton is the push button. It draws a spinner in place of its
   content while it is busy, keeps a badge on its top trailing corner, and
   uses a bezel that can grow when its frame is taller than one line. */
@interface GKUIPushButton : NSButton
@property(nonatomic) BOOL busy;
@property(nonatomic, copy) NSString *badge;
@property(nonatomic) BOOL stackedContent;
- (void)updateBezel;
@end

/* gkUIBadgeLabel returns a pill-shaped label that centers text vertically. */
NSTextField *gkUIBadgeLabel(NSString *text, NSColor *color);

/* GKUICenteredTextFieldCell draws its text in the vertical center of the
   cell. */
@interface GKUICenteredTextFieldCell : NSTextFieldCell
@end

@interface GKUILinkButton : NSButton
@property(copy) NSString *linkURL;
@property BOOL visited;
@end

@interface GKUIActivityIndicator : NSProgressIndicator
@property BOOL running;
@end

@interface GKUIImageView : NSView
@property(nonatomic, retain) NSImage *image;
@property NSInteger scaling;
@property NSInteger alignment;
@property NSImageInterpolation interpolation;
@property CGFloat sourceScale;
@end

@interface GKUITabsView : NSTabView <NSTabViewDelegate>
@property uintptr_t handle;
@end

@interface GKUIScrollView : NSScrollView <GKUIContainer>
@property(retain) GKUILayoutView *bodyView;
@end

@interface GKUISectionView : NSView <GKUIContainer>
@property uintptr_t handle;
@property(copy) NSString *subtitle;
@property(retain) NSTextField *badgeLabel;
@property BOOL expanded;
@property BOOL animated;
@property(copy) NSString *sectionTitle;
@property(retain) NSButton *header;
@property(retain) GKUILayoutView *bodyView;
- (instancetype)initWithTitle:(NSString *)title expanded:(BOOL)expanded;
- (void)setSectionExpanded:(BOOL)expanded;
- (void)updateHeader;
@end

@interface GKUIAction : NSObject <NSTextFieldDelegate>
@property uintptr_t handle;
@property int event;
- (void)send:(id)sender;
- (void)submit:(id)sender;
@end

/* GKUIAffixCell draws a text field's prefix and suffix outside its editable
   area. */
@interface GKUIAffixCell : NSTextFieldCell
@property(copy) NSString *prefix;
@property(copy) NSString *suffix;
@end

@interface GKUITextField : NSTextField
@property NSInteger maximumLength;
@property BOOL spellCheck;
@property NSRange pendingSelection;
@end

@interface GKUISeparatorView : NSView
@property BOOL vertical;
@property BOOL inset;
@property CGFloat thickness;
@property(copy) NSString *label;
@end

void gkUIRadioGroupRebuild(GKUIRadioGroupView *group, NSArray<NSString *> *titles, NSArray<NSNumber *> *enabled);

#endif
