#ifndef GAMEKIT_GTK_BRIDGE_H
#define GAMEKIT_GTK_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

/*
 * The AppKit layer reports events to Go through one entry point. A handle is a
 * runtime/cgo Handle naming the Go receiver; kind selects the event and the
 * remaining fields carry its payload. Strings are UTF-8 and valid only for the
 * duration of the call.
 */
typedef struct {
    int kind;
    int a, b, c;
    double x, y, z, w;
    const char *text;
    const char *text2;
    void *object;
} GKGTKEvent;

extern void gkGTKGoEvent(uintptr_t handle, GKGTKEvent *event);
/* gkGTKGoQuery is like gkGTKGoEvent but returns a decision. */
extern int gkGTKGoQuery(uintptr_t handle, GKGTKEvent *event);

enum {
    GK_EVENT_FOCUS = 1,
    GK_EVENT_BLUR,
    GK_EVENT_DROP,
    GK_EVENT_WINDOW_RESIZE,
    GK_EVENT_WINDOW_CLOSE_REQUEST,
    GK_EVENT_WINDOW_FOCUS,
    GK_EVENT_WINDOW_BLUR,
    GK_EVENT_WINDOW_THEME,
    GK_EVENT_WINDOW_FILE_DROP,
    GK_EVENT_WINDOW_OCCLUSION,
    GK_EVENT_WINDOW_SCREEN,
    GK_EVENT_POINTER_MOVE,
    GK_EVENT_POINTER_ENTER,
    GK_EVENT_POINTER_LEAVE,
    GK_EVENT_IME,
    GK_EVENT_TOUCH,
    GK_EVENT_PEN,
    GK_EVENT_GESTURE,
    GK_EVENT_DEVICE,
    GK_EVENT_KEY,
    GK_EVENT_TEXT,
    GK_EVENT_FRAME,
    GK_EVENT_SURFACE,
    GK_EVENT_ACTION,
    GK_EVENT_VALUE,
    GK_EVENT_SELECTION,
    GK_EVENT_TEXT_CHANGE,
    GK_EVENT_SUBMIT,
    GK_EVENT_EXPAND,
    GK_EVENT_CLOSE,
    GK_EVENT_ACTIVATE,
    GK_EVENT_SORT,
    GK_EVENT_LINK,
    GK_EVENT_NAVIGATE,
    GK_EVENT_RESIZE,
    GK_EVENT_PREFERENCES,
    GK_EVENT_APP_ACTIVE,
    GK_EVENT_APP_INACTIVE,
    GK_EVENT_APP_REOPEN,
    GK_EVENT_APP_QUIT_REQUEST,
    GK_EVENT_NOTIFICATION,
    GK_EVENT_CLIPBOARD,
    GK_EVENT_TIMER,
    GK_EVENT_WINDOW_FULLSCREEN,
};

/* Objects. Views and other Objective-C objects carry a Go handle. */
void gkGTKSetHandle(void *object, uintptr_t handle);
uintptr_t gkGTKHandle(void *object);
void gkGTKRelease(void *object);
void gkGTKRetain(void *object);
void gkGTKFree(void *pointer);

/* Widget contract. */
void gkGTKSetAccessibility(void *view, const char *identifier, const char *name, const char *description,
                           const char *value, int required, int hidden, int invalid, void *labeledBy,
                           void **describedBy, int describedByCount);
int gkGTKFocus(void *view);
int gkGTKIsFocused(void *view);
void gkGTKSetTooltip(void *view, const char *text);
void gkGTKSetCursor(void *view, int cursor);
int gkGTKSetStyle(void *view, const char *family, double size, int weight, int italic, int foreground, int background,
                    const double *foregroundRGBA, const double *backgroundRGBA, double cornerRadius);
void gkGTKSetMenu(void *view, void *menu);
void gkGTKSetDragSource(void *view, int operations, const char *text, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *customTypes,
                          size_t customTypesSize, const unsigned char *customData, const size_t *customSizes,
                          int customCount);
void gkGTKSetDropTarget(void *view, int operations, const char *types, size_t typesSize);
char *gkGTKAccessibilityTree(void *window, size_t *size);
void *gkGTKViewWindow(void *view);
void gkGTKTrackFocus(void *window);
void gkGTKForgetFocus(void *window, uintptr_t handle);
int gkGTKHandleDragEvent(void *event);
/* gkGTKAfterPoll runs after each event-loop pass. */
void gkGTKAfterPoll(void);

/* Icons: an SF Symbol name or RGBA pixels. Returns a retained NSImage or NULL. */
void *gkGTKIcon(const char *symbol, const unsigned char *pixels, int width, int height, double pointSize);

/* Menus. Items report GK_EVENT_ACTION with the action ID as text. */
void *gkGTKMenuCreate(uintptr_t handle, const char *title);
void gkGTKMenuClear(void *menu);
void gkGTKMenuAddItem(void *menu, const char *actionID, const char *title, int enabled, int checked, const char *key,
                        int modifiers, void *icon, const char *tooltip);
void gkGTKMenuAddSeparator(void *menu);
void *gkGTKMenuAddSubmenu(void *menu, const char *title);
void gkGTKMenuSetHandle(void *menu, uintptr_t handle);
void gkGTKMenuPopup(void *menu, void *view, double x, double y);
void gkGTKMenuSetActionState(void *menu, const char *actionID, int enabled, int checked);

/* Window extras. */
void gkGTKWindowSetSizeLimits(void *window, int minWidth, int minHeight, int maxWidth, int maxHeight);
void gkGTKWindowSetStyle(void *window, int resizable, int borderless);
void gkGTKWindowSetAlwaysOnTop(void *window, int alwaysOnTop);
int gkGTKWindowMinimize(void *window);
int gkGTKWindowMaximize(void *window);
int gkGTKWindowRestore(void *window);
int gkGTKWindowIsMinimized(void *window);
int gkGTKWindowIsMaximized(void *window);
int gkGTKWindowIsFullscreen(void *window);
int gkGTKWindowSetFullscreen(void *window, int fullscreen);
void gkGTKWindowScreen(void *window, double *values, char **name);
void gkGTKWindowAttach(void *window, uintptr_t handle);
void gkGTKWindowDetach(void *window);
int gkGTKWindowResolvedTheme(void *window);
void gkGTKSystemPreferences(void *window, double *values, char **locale);
int gkGTKWindowColorSpace(void *window);

/* Window events reported by the delegate and content view. */
void gkGTKWindowNotify(void *window, int kind, int a, double x, double y, const char *text);
int gkGTKWindowAllowClose(void *window);
void gkGTKWindowDidResize(void *window);
void gkGTKWindowDidChangeKey(void *window, int key);
void gkGTKWindowAcceptFiles(void *window, int accept);
void gkGTKWindowDidShow(void *window);
int gkGTKWindowCanBecomeKey(void *window);
/* phase: 0 will enter, 1 did enter, 2 will exit, 3 did exit, 4 failed to enter,
   5 failed to exit. */
void gkGTKWindowFullscreenPhase(void *window, int phase);

/* Canvas extras. */
void *gkGTKCaptureTarget(void *event);
void gkGTKDeliverCaptured(void *view, void *event);
void gkGTKReleaseCaptureFor(void *view);
void gkGTKCanvasAttach(void *view, uintptr_t handle);
int gkGTKCanvasCapture(void *view, int capture);
int gkGTKCanvasHasCapture(void *view);
int gkGTKCanvasStartFrames(void *view, int start);
int gkGTKViewColorSpace(void *view);
double gkGTKViewScale(void *view);

/* Component extensions. */
void gkGTKLabelConfigure(void *label, int alignment, int wrap, int truncation, int selectable, int style,
                           int maximumLines);
void gkGTKLabelPreferredWidth(void *label, int width);
void gkGTKButtonConfigure(void *button, void *icon, int placement, int role, const char *key, int modifiers);
void gkGTKButtonSetBusy(void *button, int busy);
void gkGTKButtonSetBadge(void *button, const char *text);
void gkGTKTextFieldConfigure(void *field, int readOnly, int maximumLength, int purpose, int alignment,
                               const char *prefix, const char *suffix, int spellCheck);
void gkGTKTextSelection(void *view, int *start, int *length);
void gkGTKSetTextSelection(void *view, int start, int length);
void gkGTKTextAreaConfigure(void *area, int wrap, int maximumLength, int tabMovesFocus, int spellCheck);
void gkGTKScrollPosition(void *scroll, int *x, int *y);
void gkGTKSetScrollPosition(void *scroll, int x, int y, int animated);
void gkGTKCheckBoxConfigure(void *box, int allowsMixed, int state, int labelBefore, const char *text,
                              const char *description);
int gkGTKCheckBoxState(void *box);
void gkGTKSetChoices(void *view, const char *titles, size_t titlesSize, const int *enabled, int count);
void gkGTKRadioGroupConfigure(void *group, int horizontal, const char *label);
void gkGTKRadioGroupPreferredSize(void *group, int *width, int *height);
void gkGTKSelectConfigure(void *popup, const char *placeholder, int popupWidth);
void gkGTKSliderConfigure(void *slider, int vertical, double minimum, double maximum, double frequency, int snap);
void gkGTKProgressConfigure(void *progress, int indeterminate, int vertical, const char *text, int state);
void gkGTKActivityConfigure(void *activity, int size, const char *label);
void gkGTKSeparatorConfigure(void *separator, int vertical, const char *label, int inset, int thickness);
void gkGTKImageConfigure(void *image, int alignment, int nearest, const char *alternative, double scale);
void gkGTKLinkConfigure(void *link, int visited, void *icon);
int gkGTKOpenURL(const char *url, int background);
void gkGTKTabsConfigure(void *tabs, int placement);
void gkGTKScrollConfigure(void *scroll, int axes, int policy, int top, int right, int bottom, int left,
                            int overscroll);
void gkGTKScrollSetZoom(void *scroll, double zoom);
double gkGTKScrollZoom(void *scroll);
void gkGTKScrollViewport(void *scroll, int *width, int *height);
int gkGTKScrollToView(void *scroll, void *view, int animated);
void gkGTKSelectSetVisibleRows(void *popup, int rows);
void gkGTKSliderSetInverted(void *slider, int inverted);
void gkGTKTabsSetOverflow(void *tabs, int policy);
void gkGTKSectionSetAnimated(void *section, int animated);
void gkGTKSectionConfigure(void *section, const char *subtitle, void *icon, const char *badge);

/* Dialogs. */
void *gkGTKAlertCreate(int severity, const char *title, const char *message, const char *details, const char *buttons,
                         size_t buttonsSize, int cancelIndex, int destructiveMask, int prompt, int secure,
                         const char *text, const char *placeholder);
void *gkGTKModalBegin(void *alert, void *parent);
int gkGTKModalStep(void *modal);
void gkGTKModalEnd(void *modal, int abort);
char *gkGTKAlertText(void *alert);
void gkGTKShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                      const char *credits, void *icon);

/* Services. */
long gkGTKClipboardChangeCount(void);
void gkGTKClipboardClear(void);
int gkGTKClipboardWrite(const char *text, int hasText, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *types, size_t typesSize,
                          const unsigned char *data, const size_t *sizes, int count);
void gkGTKClipboardRead(char **text, int *hasText, char **files, size_t *filesSize, unsigned char **pixels,
                          int *width, int *height, char **types, size_t *typesSize, unsigned char **data,
                          size_t **sizes, int *count);
char *gkGTKClipboardFormats(size_t *size);
int gkGTKRevealFile(const char *path, int select);
void gkGTKRecentAdd(const char *path);
char *gkGTKRecentList(size_t *size);
void gkGTKRecentClear(void);
void gkGTKRecentRemove(const char *path);
char *gkGTKDefaultApplication(const char *extension);
int gkGTKPrint(uintptr_t handle, const char *title, int showDialog, void *window);
void gkGTKPrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w, double h);
int gkGTKPrintText(const char *text, double x, double y, const char *family, double size, int weight, int italic);
void *gkGTKShareBegin(const char *texts, size_t textsSize, const char *urls, size_t urlsSize, const char *files,
                        size_t filesSize, void *view);
int gkGTKShareStep(void *picker, char **service);
void gkGTKShareEnd(void *picker);

/* Notifications. Asynchronous calls report through gkGTKGoNotificationResult. */
int gkGTKNotificationsAvailable(void);
void gkGTKNotificationsAttach(uintptr_t handle);
void gkGTKNotificationAuthorization(uintptr_t token);
void gkGTKNotificationRequestAuthorization(uintptr_t token);
void gkGTKNotificationSend(uintptr_t token, const char *identifier, const char *title, const char *subtitle,
                             const char *body, const char *category, const char *thread, int sound,
                             const char *attachment, double delay, const char *actionIDs, size_t actionIDsSize,
                             const char *actionTitles, size_t actionTitlesSize, const int *actionFlags,
                             int actionCount);
void gkGTKNotificationPending(uintptr_t token);
void gkGTKNotificationDelivered(uintptr_t token);
void gkGTKNotificationWithdraw(const char *ids, size_t size);
void gkGTKSetBadge(const char *badge);
char *gkGTKBadge(void);

/* Status items. */
void *gkGTKTrayCreate(uintptr_t handle);
void gkGTKTrayConfigure(void *tray, void *icon, int isTemplate, const char *title, const char *tooltip);
void gkGTKTraySetVisible(void *tray, int visible);
void gkGTKTraySetAttention(void *tray, int attention);
void gkGTKTraySetMenu(void *tray, void *menu);
int gkGTKTrayBounds(void *tray, double *x, double *y, double *width, double *height);
void gkGTKTrayRemove(void *tray);

/* Controls. */
void gkGTKDisconnect(void *view);
void *gkGTKToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar);
void *gkGTKSwitchCreate(uintptr_t window, const char *text, const char *onLabel, const char *offLabel, int checked,
                          int size);
int gkGTKSwitchChecked(void *view);
void gkGTKSwitchSet(void *view, const char *text, int checked);
void *gkGTKPasswordCreate(uintptr_t window, const char *placeholder, int reveal);
char *gkGTKPasswordText(void *view);
void gkGTKPasswordSet(void *view, const char *text, const char *placeholder, int revealed);
void *gkGTKSearchCreate(uintptr_t window, const char *text, const char *placeholder);
void gkGTKSearchSetSuggestions(void *view, const char *items, size_t size);
void *gkGTKNumberCreate(uintptr_t window, double minimum, double maximum, double value, double step, int decimals);
double gkGTKNumberValue(void *view);
void gkGTKNumberSetValue(void *view, double value);
void *gkGTKComboCreate(uintptr_t window, const char *text, const char *items, size_t size);
void gkGTKComboSetItems(void *view, const char *items, size_t size);
void *gkGTKSegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons, const int *enabled,
                             int count, int mode);
int gkGTKSegmentSelected(void *view, int index);
int gkGTKSegmentClicked(void *view);
void gkGTKSegmentSetSelected(void *view, int index, int selected);
void *gkGTKStepperCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps);
void *gkGTKDialCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps);
double gkGTKControlValue(void *view);
void gkGTKControlSetValue(void *view, double value);
void *gkGTKRangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower, double upper,
                               double step);
void gkGTKRangeSliderValues(void *view, double *lower, double *upper);
void gkGTKRangeSliderSet(void *view, double lower, double upper);
void *gkGTKDatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum, double maximum,
                              int range, double interval, int clock);
void gkGTKDatePickerValue(void *view, double *value, double *interval);
void gkGTKDatePickerSet(void *view, double value, double interval);
/* Calendars. Dates are seconds since 1970 at midnight UTC. selection is 0 for
   one date, 1 for any set of dates, and 2 for a range. */
void *gkGTKCalendarCreate(uintptr_t window, int selection, const double *dates, int count, double visible);
int gkGTKCalendarDates(void *calendar, double *dates, int capacity);
void gkGTKCalendarSetDates(void *calendar, const double *dates, int count);
double gkGTKCalendarVisibleMonth(void *calendar);
void gkGTKCalendarSetVisibleMonth(void *calendar, double month);
void *gkGTKColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allowsAlpha);
void gkGTKColorWellColor(void *view, double *rgba);
void gkGTKColorWellSet(void *view, double r, double g, double b, double a);
void *gkGTKFontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic);
void gkGTKFontPickerSet(void *view, const char *family, double size, int weight, int italic);
char *gkGTKFontPickerFont(void *view, double *size, int *weight, int *italic);
void *gkGTKKeyRecorderCreate(uintptr_t window);
void gkGTKKeyRecorderSet(void *view, const char *display);

/* Containers. */
void gkGTKAddChild(void *container, void *child);
void *gkGTKClipViewCreate(uintptr_t window);
void *gkGTKGroupBoxCreate(uintptr_t window, const char *title, const char *subtitle);
void gkGTKGroupBoxSet(void *box, const char *title, const char *subtitle);
void gkGTKGroupBoxBody(void *box, int *width, int *height, int *top);
int gkGTKGroupBoxSubtitleHeight(void *box, int width);
void gkGTKGroupBoxChrome(void *box, int *width, int *height);
void *gkGTKSplitViewCreate(uintptr_t window, int vertical, int panes);
void *gkGTKSplitViewPane(void *split, int index);
void gkGTKSplitViewSetPane(void *split, int index, double minimum, int collapsible);
void gkGTKSplitViewPaneFrame(void *split, int index, int *x, int *y, int *width, int *height);
double gkGTKSplitViewDivider(void *split, int index);
void gkGTKSplitViewSetDivider(void *split, int index, double position);
void *gkGTKPopoverCreate(uintptr_t handle, int transient);
void *gkGTKPopoverBody(void *popover);
int gkGTKPopoverShow(void *popover, void *anchor, int edge, int width, int height);
void gkGTKPopoverHide(void *popover);
int gkGTKPopoverShown(void *popover);
void gkGTKPopoverRelease(void *popover);
int gkGTKTitleAccessoryAdd(void *window, void *view, int alignment, int width, int height, void **controller);
void gkGTKTitleAccessoryRemove(void *controller);
void gkGTKViewSize(void *view, int *width, int *height);

/* Data views. */
void *gkGTKTableCreate(uintptr_t window, int list, const char *columnIDs, size_t columnIDsSize, const char *titles,
                         size_t titlesSize, const int *widths, const int *sortable, int selection);
void gkGTKTableSetRows(void *table, const char *ids, size_t idsSize, const char *cells, size_t cellsSize,
                         const char *details, size_t detailsSize, void **icons, int columns);
char *gkGTKTableSelection(void *table, size_t *size);
void gkGTKTableSelect(void *table, const char *ids, size_t size);
int gkGTKTableScrollTo(void *table, const char *identifier);
void *gkGTKOutlineCreate(uintptr_t window, int navigation, int multiple);
void gkGTKOutlineSetNodes(void *outline, const char *parent, const int *depths, const int *flags, const char *ids,
                            size_t idsSize, const char *texts, size_t textsSize, const char *badges, size_t badgesSize,
                            void **icons, int count);
char *gkGTKOutlineSelection(void *outline, size_t *size);
void gkGTKOutlineSelect(void *outline, const char *ids, size_t size);
int gkGTKOutlineExpanded(void *outline, const char *identifier);
int gkGTKOutlineSetExpanded(void *outline, const char *identifier, int expanded);
void *gkGTKCollectionCreate(uintptr_t window, int width, int height, int selection);
void gkGTKCollectionSetItems(void *collection, const char *ids, size_t idsSize, const char *texts, size_t textsSize,
                               void **images, int count);
char *gkGTKCollectionSelection(void *collection, size_t *size);
void gkGTKCollectionSelect(void *collection, const char *ids, size_t size);
void *gkGTKBreadcrumbsCreate(uintptr_t window);
void gkGTKBreadcrumbsSetItems(void *path, const char *titles, size_t size);
void *gkGTKPaginationCreate(uintptr_t window, int count, int selected, int compact);
void gkGTKPaginationSet(void *pagination, int count, int selected);

/* Window chrome. */
/* gkGTKMenuBarCreate shows a menu bar at the top of window. */
void *gkGTKMenuBarCreate(void *window, const char *titles, size_t titlesSize, void **menus, int count);
void gkGTKMenuBarInstall(void *bar);
void gkGTKMenuBarRelease(void *bar);
void *gkGTKToolbarCreate(uintptr_t handle, void *window, const int *kinds, const char *ids, size_t idsSize,
                           const char *labels, size_t labelsSize, void **icons, const int *enabled, int count,
                           int showsLabels);
void gkGTKToolbarSetEnabled(void *toolbar, const char *identifier, int enabled);
void gkGTKToolbarRelease(void *toolbar);
void *gkGTKStatusBarCreate(uintptr_t window, const char *text);
void gkGTKStatusBarSet(void *bar, const char *text);
void *gkGTKCommandBarCreate(uintptr_t window);
void gkGTKCommandBarSetActions(void *bar, const char *ids, size_t idsSize, const char *titles, size_t titlesSize,
                                 void **icons, const int *enabled, int count, int compact);

/* Feedback. */
void gkGTKAnnounce(void *element, const char *text, int priority);
void *gkGTKInfoBarCreate(uintptr_t window, int closable, const char *ids, size_t idsSize, const char *titles,
                           size_t titlesSize, const int *enabled, int count);
void gkGTKInfoBarSet(void *bar, int severity, const char *title, const char *detail);
void *gkGTKTooltipCreate(void);
void gkGTKTooltipSetText(void *tooltip, const char *text);
int gkGTKTooltipShow(void *tooltip, void *anchor, int edge, double delay);
void gkGTKTooltipHide(void *tooltip);
int gkGTKTooltipShown(void *tooltip);
void *gkGTKToastCreate(uintptr_t handle, const char *message, const char *ids, size_t idsSize, const char *titles,
                         size_t titlesSize, const int *enabled, int count, double timeout);
int gkGTKToastShow(void *toast, void *window);
void gkGTKToastDismiss(void *toast);
int gkGTKToastShown(void *toast);
void gkGTKToastRelease(void *toast);
void *gkGTKBadgeCreate(uintptr_t window);
void gkGTKBadgeSet(void *badge, const char *text, int severity, const char *accessible);
void *gkGTKProgressDialogCreate(uintptr_t handle, const char *title, int cancellable);
void gkGTKProgressDialogSet(void *dialog, const char *message, double value);
int gkGTKProgressDialogShow(void *dialog, void *parent);
void gkGTKProgressDialogRelease(void *dialog);
void *gkGTKEmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t idsSize, const char *titles,
                              size_t titlesSize, const int *enabled, int count);
void gkGTKEmptyStateSet(void *view, const char *title, const char *detail);
void *gkGTKInlineValidationCreate(uintptr_t window);
void gkGTKInlineValidationSet(void *view, int severity, const char *text);

/* Content. */
void *gkGTKIconCreate(uintptr_t window);
void gkGTKIconSet(void *icon, void *image, int width, int height, int tint, const char *name);
void *gkGTKDocumentCreate(uintptr_t window, int editable, int markdown);
char *gkGTKDocumentText(void *document);
void gkGTKDocumentSetText(void *document, const char *text);
void gkGTKDocumentSetEditable(void *document, int editable);
int gkGTKDocumentEditable(void *document);
unsigned char *gkGTKDocumentRTF(void *document, size_t *size);
int gkGTKDocumentSetRTF(void *document, const unsigned char *rtf, size_t size);
int gkGTKMarkdownSet(void *document, const char *source);
void *gkGTKVectorCreate(uintptr_t window);
int gkGTKVectorSet(void *view, const unsigned char *data, size_t size, const char *label, double *width,
                     double *height);
void *gkGTKAnimationCreate(uintptr_t window, void **images, const double *durations, int count, int repeats);
void gkGTKAnimationPlay(void *view, int play);
int gkGTKAnimationPlaying(void *view);
void *gkGTKVideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted);
void gkGTKVideoPlay(void *view, int play);
int gkGTKVideoPlaying(void *view);
double gkGTKVideoPosition(void *view);
double gkGTKVideoDuration(void *view);
void gkGTKVideoSeek(void *view, double seconds);
void gkGTKVideoRelease(void *view);
void *gkGTKWebViewCreate(uintptr_t window, int scripts);
int gkGTKWebViewLoad(void *view, const char *url);
void gkGTKWebViewLoadHTML(void *view, const char *html);
char *gkGTKWebViewURL(void *view);
void gkGTKWebViewRelease(void *view);

/* Application events. */
void gkGTKAppAttach(uintptr_t handle);

#endif
