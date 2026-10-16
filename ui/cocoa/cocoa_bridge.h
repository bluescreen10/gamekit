#ifndef GAMEKIT_COCOA_BRIDGE_H
#define GAMEKIT_COCOA_BRIDGE_H

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
} GKCocoaEvent;

extern void gkCocoaGoEvent(uintptr_t handle, GKCocoaEvent *event);
/* gkCocoaGoQuery is like gkCocoaGoEvent but returns a decision. */
extern int gkCocoaGoQuery(uintptr_t handle, GKCocoaEvent *event);

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
void gkCocoaSetHandle(void *object, uintptr_t handle);
uintptr_t gkCocoaHandle(void *object);
void gkCocoaRelease(void *object);
void gkCocoaRetain(void *object);
void gkCocoaFree(void *pointer);

/* Widget contract. */
void gkCocoaSetAccessibility(void *view, const char *identifier, const char *name, const char *description,
                             const char *value, int required, int hidden, void *labeledBy, void **describedBy,
                             int describedByCount);
int gkCocoaFocus(void *view);
int gkCocoaIsFocused(void *view);
void gkCocoaSetTooltip(void *view, const char *text);
void gkCocoaSetCursor(void *view, int cursor);
int gkCocoaSetStyle(void *view, const char *family, double size, int weight, int italic, int foreground, int background,
                    const double *foregroundRGBA, const double *backgroundRGBA, double cornerRadius);
void gkCocoaSetMenu(void *view, void *menu);
void gkCocoaSetDragSource(void *view, int operations, const char *text, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *customTypes,
                          size_t customTypesSize, const unsigned char *customData, const size_t *customSizes,
                          int customCount);
void gkCocoaSetDropTarget(void *view, int operations, const char *types, size_t typesSize);
char *gkCocoaAccessibilityTree(void *window, size_t *size);
void *gkCocoaViewWindow(void *view);
void gkCocoaTrackFocus(void *window);
void gkCocoaForgetFocus(void *window, uintptr_t handle);
int gkCocoaHandleDragEvent(void *event);
/* gkCocoaAfterPoll runs after each event-loop pass. */
void gkCocoaAfterPoll(void);

/* Icons: an SF Symbol name or RGBA pixels. Returns a retained NSImage or NULL. */
void *gkCocoaIcon(const char *symbol, const unsigned char *pixels, int width, int height, double pointSize);

/* Menus. Items report GK_EVENT_ACTION with the action ID as text. */
void *gkCocoaMenuCreate(uintptr_t handle, const char *title);
void gkCocoaMenuClear(void *menu);
void gkCocoaMenuAddItem(void *menu, const char *actionID, const char *title, int enabled, int checked, const char *key,
                        int modifiers, void *icon, const char *tooltip);
void gkCocoaMenuAddSeparator(void *menu);
void *gkCocoaMenuAddSubmenu(void *menu, const char *title);
void gkCocoaMenuSetHandle(void *menu, uintptr_t handle);
void gkCocoaMenuPopup(void *menu, void *view, double x, double y);
void gkCocoaMenuSetActionState(void *menu, const char *actionID, int enabled, int checked);

/* Window extras. */
void gkCocoaWindowSetSizeLimits(void *window, int minWidth, int minHeight, int maxWidth, int maxHeight);
void gkCocoaWindowSetStyle(void *window, int resizable, int borderless);
void gkCocoaWindowSetAlwaysOnTop(void *window, int alwaysOnTop);
int gkCocoaWindowMinimize(void *window);
int gkCocoaWindowMaximize(void *window);
int gkCocoaWindowRestore(void *window);
int gkCocoaWindowIsMinimized(void *window);
int gkCocoaWindowIsMaximized(void *window);
int gkCocoaWindowIsFullscreen(void *window);
int gkCocoaWindowSetFullscreen(void *window, int fullscreen);
void gkCocoaWindowScreen(void *window, double *values, char **name);
void gkCocoaWindowAttach(void *window, uintptr_t handle);
void gkCocoaWindowDetach(void *window);
int gkCocoaWindowResolvedTheme(void *window);
void gkCocoaSystemPreferences(void *window, double *values, char **locale);
int gkCocoaWindowColorSpace(void *window);

/* Window events reported by the delegate and content view. */
void gkCocoaWindowNotify(void *window, int kind, int a, double x, double y, const char *text);
int gkCocoaWindowAllowClose(void *window);
void gkCocoaWindowDidResize(void *window);
void gkCocoaWindowDidChangeKey(void *window, int key);
void gkCocoaWindowAcceptFiles(void *window, int accept);
void gkCocoaWindowDidShow(void *window);
int gkCocoaWindowCanBecomeKey(void *window);
/* phase: 0 will enter, 1 did enter, 2 will exit, 3 did exit, 4 failed to enter,
   5 failed to exit. */
void gkCocoaWindowFullscreenPhase(void *window, int phase);

/* Canvas extras. */
void *gkCocoaCaptureTarget(void *event);
void gkCocoaDeliverCaptured(void *view, void *event);
void gkCocoaReleaseCaptureFor(void *view);
void gkCocoaCanvasAttach(void *view, uintptr_t handle);
int gkCocoaCanvasCapture(void *view, int capture);
int gkCocoaCanvasHasCapture(void *view);
int gkCocoaCanvasStartFrames(void *view, int start);
int gkCocoaViewColorSpace(void *view);
double gkCocoaViewScale(void *view);

/* Component extensions. */
void gkCocoaLabelConfigure(void *label, int alignment, int wrap, int truncation, int selectable, int style,
                           int maximumLines);
void gkCocoaLabelPreferredWidth(void *label, int width);
void gkCocoaButtonConfigure(void *button, void *icon, int placement, int role, const char *key, int modifiers);
void gkCocoaButtonSetBusy(void *button, int busy);
void gkCocoaButtonSetBadge(void *button, const char *text);
void gkCocoaTextFieldConfigure(void *field, int readOnly, int maximumLength, int purpose, int alignment,
                               const char *prefix, const char *suffix, int spellCheck);
void gkCocoaTextSelection(void *view, int *start, int *length);
void gkCocoaSetTextSelection(void *view, int start, int length);
void gkCocoaTextAreaConfigure(void *area, int wrap, int maximumLength, int tabMovesFocus, int spellCheck);
void gkCocoaScrollPosition(void *scroll, int *x, int *y);
void gkCocoaSetScrollPosition(void *scroll, int x, int y, int animated);
void gkCocoaCheckBoxConfigure(void *box, int allowsMixed, int state, int labelBefore, const char *text,
                              const char *description);
int gkCocoaCheckBoxState(void *box);
void gkCocoaSetChoices(void *view, const char *titles, size_t titlesSize, const int *enabled, int count);
void gkCocoaRadioGroupConfigure(void *group, int horizontal, const char *label);
void gkCocoaRadioGroupPreferredSize(void *group, int *width, int *height);
void gkCocoaSelectConfigure(void *popup, const char *placeholder, int popupWidth);
void gkCocoaSliderConfigure(void *slider, int vertical, double minimum, double maximum, double frequency, int snap);
void gkCocoaProgressConfigure(void *progress, int indeterminate, int vertical, const char *text, int state);
void gkCocoaActivityConfigure(void *activity, int size, const char *label);
void gkCocoaSeparatorConfigure(void *separator, int vertical, const char *label, int inset, int thickness);
void gkCocoaImageConfigure(void *image, int alignment, int nearest, const char *alternative, double scale);
void gkCocoaLinkConfigure(void *link, int visited, void *icon);
int gkCocoaOpenURL(const char *url, int background);
void gkCocoaTabsConfigure(void *tabs, int placement);
void gkCocoaScrollConfigure(void *scroll, int axes, int policy, int top, int right, int bottom, int left,
                            int overscroll);
void gkCocoaScrollSetZoom(void *scroll, double zoom);
double gkCocoaScrollZoom(void *scroll);
void gkCocoaScrollViewport(void *scroll, int *width, int *height);
int gkCocoaScrollToView(void *scroll, void *view, int animated);
void gkCocoaSectionConfigure(void *section, const char *subtitle, void *icon, const char *badge);
void gkCocoaSectionSetAnimated(void *section, int animated);

/* Dialogs. */
void *gkCocoaAlertCreate(int severity, const char *title, const char *message, const char *details, const char *buttons,
                         size_t buttonsSize, int cancelIndex, int destructiveMask, int prompt, int secure,
                         const char *text, const char *placeholder);
void *gkCocoaModalBegin(void *alert, void *parent);
int gkCocoaModalStep(void *modal);
void gkCocoaModalEnd(void *modal, int abort);
char *gkCocoaAlertText(void *alert);
void gkCocoaShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                      const char *credits, void *icon);

/* Services. */
long gkCocoaClipboardChangeCount(void);
void gkCocoaClipboardClear(void);
int gkCocoaClipboardWrite(const char *text, int hasText, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *types, size_t typesSize,
                          const unsigned char *data, const size_t *sizes, int count);
void gkCocoaClipboardRead(char **text, int *hasText, char **files, size_t *filesSize, unsigned char **pixels,
                          int *width, int *height, char **types, size_t *typesSize, unsigned char **data,
                          size_t **sizes, int *count);
char *gkCocoaClipboardFormats(size_t *size);
int gkCocoaRevealFile(const char *path, int select);
void gkCocoaRecentAdd(const char *path);
char *gkCocoaRecentList(size_t *size);
void gkCocoaRecentClear(void);
void gkCocoaRecentRemove(const char *path);
char *gkCocoaDefaultApplication(const char *extension);
int gkCocoaPrint(uintptr_t handle, const char *title, int showDialog, void *window);
void gkCocoaPrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w, double h);
int gkCocoaPrintText(const char *text, double x, double y, const char *family, double size, int weight, int italic);
void *gkCocoaShareBegin(const char *texts, size_t textsSize, const char *urls, size_t urlsSize, const char *files,
                        size_t filesSize, void *view);
int gkCocoaShareStep(void *picker, char **service);
void gkCocoaShareEnd(void *picker);

/* Notifications. Asynchronous calls report through gkCocoaGoNotificationResult. */
int gkCocoaNotificationsAvailable(void);
void gkCocoaNotificationsAttach(uintptr_t handle);
void gkCocoaNotificationAuthorization(uintptr_t token);
void gkCocoaNotificationRequestAuthorization(uintptr_t token);
void gkCocoaNotificationSend(uintptr_t token, const char *identifier, const char *title, const char *subtitle,
                             const char *body, const char *category, const char *thread, int sound,
                             const char *attachment, double delay, const char *actionIDs, size_t actionIDsSize,
                             const char *actionTitles, size_t actionTitlesSize, const int *actionFlags,
                             int actionCount);
void gkCocoaNotificationPending(uintptr_t token);
void gkCocoaNotificationDelivered(uintptr_t token);
void gkCocoaNotificationWithdraw(const char *ids, size_t size);
void gkCocoaSetBadge(const char *badge);
char *gkCocoaBadge(void);

/* Status items. */
void *gkCocoaTrayCreate(uintptr_t handle);
void gkCocoaTrayConfigure(void *tray, void *icon, int isTemplate, const char *title, const char *tooltip);
void gkCocoaTraySetVisible(void *tray, int visible);
void gkCocoaTraySetAttention(void *tray, int attention);
void gkCocoaTraySetMenu(void *tray, void *menu);
int gkCocoaTrayBounds(void *tray, double *x, double *y, double *width, double *height);
void gkCocoaTrayRemove(void *tray);

/* Controls. */
void gkCocoaDisconnect(void *view);
void *gkCocoaToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar);
void *gkCocoaSwitchCreate(uintptr_t window, const char *text, const char *onLabel, const char *offLabel, int checked,
                          int size);
int gkCocoaSwitchChecked(void *view);
void gkCocoaSwitchSet(void *view, const char *text, int checked);
void *gkCocoaPasswordCreate(uintptr_t window, const char *placeholder, int reveal);
char *gkCocoaPasswordText(void *view);
void gkCocoaPasswordSet(void *view, const char *text, const char *placeholder, int revealed);
void *gkCocoaSearchCreate(uintptr_t window, const char *text, const char *placeholder);
void gkCocoaSearchSetSuggestions(void *view, const char *items, size_t size);
void *gkCocoaNumberCreate(uintptr_t window, double minimum, double maximum, double value, double step, int decimals);
double gkCocoaNumberValue(void *view);
void gkCocoaNumberSetValue(void *view, double value);
void *gkCocoaComboCreate(uintptr_t window, const char *text, const char *items, size_t size);
void gkCocoaComboSetItems(void *view, const char *items, size_t size);
void *gkCocoaSegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons, const int *enabled,
                             int count, int mode);
int gkCocoaSegmentSelected(void *view, int index);
int gkCocoaSegmentClicked(void *view);
void gkCocoaSegmentSetSelected(void *view, int index, int selected);
void *gkCocoaStepperCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps);
void *gkCocoaDialCreate(uintptr_t window, double minimum, double maximum, double value, double step);
double gkCocoaControlValue(void *view);
void gkCocoaControlSetValue(void *view, double value);
void *gkCocoaRangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower, double upper,
                               double step);
void gkCocoaRangeSliderValues(void *view, double *lower, double *upper);
void gkCocoaRangeSliderSet(void *view, double lower, double upper);
void *gkCocoaDatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum, double maximum,
                              int range, double interval, int clock);
void gkCocoaDatePickerValue(void *view, double *value, double *interval);
void gkCocoaDatePickerSet(void *view, double value, double interval);
void *gkCocoaColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allowsAlpha);
void gkCocoaColorWellColor(void *view, double *rgba);
void gkCocoaColorWellSet(void *view, double r, double g, double b, double a);
void *gkCocoaFontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic);
void gkCocoaFontPickerSet(void *view, const char *family, double size, int weight, int italic);
char *gkCocoaFontPickerFont(void *view, double *size, int *weight, int *italic);
void *gkCocoaKeyRecorderCreate(uintptr_t window);
void gkCocoaKeyRecorderSet(void *view, const char *display);

/* Containers. */
void gkCocoaAddChild(void *container, void *child);
void *gkCocoaClipViewCreate(uintptr_t window);
void *gkCocoaGroupBoxCreate(uintptr_t window, const char *title, const char *subtitle);
void gkCocoaGroupBoxSet(void *box, const char *title, const char *subtitle);
void gkCocoaGroupBoxBody(void *box, int *width, int *height, int *top);
int gkCocoaGroupBoxSubtitleHeight(void *box, int width);
void gkCocoaGroupBoxChrome(void *box, int *width, int *height);
void *gkCocoaSplitViewCreate(uintptr_t window, int vertical, int panes);
void *gkCocoaSplitViewPane(void *split, int index);
void gkCocoaSplitViewSetPane(void *split, int index, double minimum, int collapsible);
void gkCocoaSplitViewPaneFrame(void *split, int index, int *x, int *y, int *width, int *height);
double gkCocoaSplitViewDivider(void *split, int index);
void gkCocoaSplitViewSetDivider(void *split, int index, double position);
void *gkCocoaPopoverCreate(uintptr_t handle, int transient);
void *gkCocoaPopoverBody(void *popover);
int gkCocoaPopoverShow(void *popover, void *anchor, int edge, int width, int height);
void gkCocoaPopoverHide(void *popover);
int gkCocoaPopoverShown(void *popover);
void gkCocoaPopoverRelease(void *popover);
int gkCocoaTitleAccessoryAdd(void *window, void *view, int alignment, int width, int height, void **controller);
void gkCocoaTitleAccessoryRemove(void *controller);
void gkCocoaViewSize(void *view, int *width, int *height);

/* Data views. */
void *gkCocoaTableCreate(uintptr_t window, int list, const char *columnIDs, size_t columnIDsSize, const char *titles,
                         size_t titlesSize, const int *widths, const int *sortable, int selection);
void gkCocoaTableSetRows(void *table, const char *ids, size_t idsSize, const char *cells, size_t cellsSize,
                         const char *details, size_t detailsSize, void **icons, int columns);
char *gkCocoaTableSelection(void *table, size_t *size);
void gkCocoaTableSelect(void *table, const char *ids, size_t size);
int gkCocoaTableScrollTo(void *table, const char *identifier);
void *gkCocoaOutlineCreate(uintptr_t window, int navigation, int multiple);
void gkCocoaOutlineSetNodes(void *outline, const char *parent, const int *depths, const int *flags, const char *ids,
                            size_t idsSize, const char *texts, size_t textsSize, const char *badges, size_t badgesSize,
                            void **icons, int count);
char *gkCocoaOutlineSelection(void *outline, size_t *size);
void gkCocoaOutlineSelect(void *outline, const char *ids, size_t size);
int gkCocoaOutlineExpanded(void *outline, const char *identifier);
int gkCocoaOutlineSetExpanded(void *outline, const char *identifier, int expanded);
void *gkCocoaCollectionCreate(uintptr_t window, int width, int height, int selection);
void gkCocoaCollectionSetItems(void *collection, const char *ids, size_t idsSize, const char *texts, size_t textsSize,
                               void **images, int count);
char *gkCocoaCollectionSelection(void *collection, size_t *size);
void gkCocoaCollectionSelect(void *collection, const char *ids, size_t size);
void *gkCocoaBreadcrumbsCreate(uintptr_t window);
void gkCocoaBreadcrumbsSetItems(void *path, const char *titles, size_t size);
void *gkCocoaPaginationCreate(uintptr_t window, int count, int selected, int compact);
void gkCocoaPaginationSet(void *pagination, int count, int selected);

/* Window chrome. */
void *gkCocoaMenuBarCreate(const char *titles, size_t titlesSize, void **menus, int count);
void gkCocoaMenuBarInstall(void *bar);
void gkCocoaMenuBarRelease(void *bar);
void *gkCocoaToolbarCreate(uintptr_t handle, void *window, const int *kinds, const char *ids, size_t idsSize,
                           const char *labels, size_t labelsSize, void **icons, const int *enabled, int count,
                           int showsLabels);
void gkCocoaToolbarSetEnabled(void *toolbar, const char *identifier, int enabled);
void gkCocoaToolbarRelease(void *toolbar);
void *gkCocoaStatusBarCreate(uintptr_t window, const char *text);
void gkCocoaStatusBarSet(void *bar, const char *text);
void *gkCocoaCommandBarCreate(uintptr_t window);
void gkCocoaCommandBarSetActions(void *bar, const char *ids, size_t idsSize, const char *titles, size_t titlesSize,
                                 void **icons, const int *enabled, int count, int compact);

/* Feedback. */
void gkCocoaAnnounce(void *element, const char *text, int priority);
void *gkCocoaInfoBarCreate(uintptr_t window, int closable, const char *ids, size_t idsSize, const char *titles,
                           size_t titlesSize, const int *enabled, int count);
void gkCocoaInfoBarSet(void *bar, int severity, const char *title, const char *detail);
void *gkCocoaTooltipCreate(void);
void gkCocoaTooltipSetText(void *tooltip, const char *text);
int gkCocoaTooltipShow(void *tooltip, void *anchor, int edge, double delay);
void gkCocoaTooltipHide(void *tooltip);
int gkCocoaTooltipShown(void *tooltip);
void *gkCocoaToastCreate(uintptr_t handle, const char *message, const char *ids, size_t idsSize, const char *titles,
                         size_t titlesSize, const int *enabled, int count, double timeout);
int gkCocoaToastShow(void *toast, void *window);
void gkCocoaToastDismiss(void *toast);
int gkCocoaToastShown(void *toast);
void gkCocoaToastRelease(void *toast);
void *gkCocoaBadgeCreate(uintptr_t window);
void gkCocoaBadgeSet(void *badge, const char *text, int severity, const char *accessible);
void *gkCocoaProgressDialogCreate(uintptr_t handle, const char *title, int cancellable);
void gkCocoaProgressDialogSet(void *dialog, const char *message, double value);
int gkCocoaProgressDialogShow(void *dialog, void *parent);
void gkCocoaProgressDialogRelease(void *dialog);
void *gkCocoaEmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t idsSize, const char *titles,
                              size_t titlesSize, const int *enabled, int count);
void gkCocoaEmptyStateSet(void *view, const char *title, const char *detail);
void *gkCocoaInlineValidationCreate(uintptr_t window);
void gkCocoaInlineValidationSet(void *view, int severity, const char *text);

/* Content. */
void *gkCocoaIconCreate(uintptr_t window);
void gkCocoaIconSet(void *icon, void *image, int width, int height, int tint, const char *name);
void *gkCocoaDocumentCreate(uintptr_t window, int editable, int markdown);
char *gkCocoaDocumentText(void *document);
void gkCocoaDocumentSetText(void *document, const char *text);
void gkCocoaDocumentSetEditable(void *document, int editable);
int gkCocoaDocumentEditable(void *document);
unsigned char *gkCocoaDocumentRTF(void *document, size_t *size);
int gkCocoaDocumentSetRTF(void *document, const unsigned char *rtf, size_t size);
int gkCocoaMarkdownSet(void *document, const char *source);
void *gkCocoaVectorCreate(uintptr_t window);
int gkCocoaVectorSet(void *view, const unsigned char *data, size_t size, const char *label, double *width,
                     double *height);
void *gkCocoaAnimationCreate(uintptr_t window, void **images, const double *durations, int count, int repeats);
void gkCocoaAnimationPlay(void *view, int play);
int gkCocoaAnimationPlaying(void *view);
void *gkCocoaVideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted);
void gkCocoaVideoPlay(void *view, int play);
int gkCocoaVideoPlaying(void *view);
double gkCocoaVideoPosition(void *view);
double gkCocoaVideoDuration(void *view);
void gkCocoaVideoSeek(void *view, double seconds);
void gkCocoaVideoRelease(void *view);
void *gkCocoaWebViewCreate(uintptr_t window, int scripts);
int gkCocoaWebViewLoad(void *view, const char *url);
void gkCocoaWebViewLoadHTML(void *view, const char *html);
char *gkCocoaWebViewURL(void *view);
void gkCocoaWebViewRelease(void *view);

/* Application events. */
void gkCocoaAppAttach(uintptr_t handle);

#endif
