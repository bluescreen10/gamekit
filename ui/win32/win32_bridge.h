#ifndef GAMEKIT_WIN32_BRIDGE_H
#define GAMEKIT_WIN32_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

/*
 * The native layer reports events to Go through one entry point. A handle is a
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
} GKWin32Event;

extern void gkWin32GoEvent(uintptr_t handle, GKWin32Event *event);
/* gkWin32GoQuery is like gkWin32GoEvent but returns a decision. */
extern int gkWin32GoQuery(uintptr_t handle, GKWin32Event *event);

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

/* Objects. Views and the other objects of the bridge carry a Go handle. */
void gkWin32SetHandle(void *object, uintptr_t handle);
uintptr_t gkWin32Handle(void *object);
void gkWin32Release(void *object);
void gkWin32Retain(void *object);
void gkWin32Free(void *pointer);

/* Widget contract. */
void gkWin32SetAccessibility(void *view, const char *identifier, const char *name, const char *description,
                           const char *value, int required, int hidden, int invalid, void *labeledBy,
                           void **describedBy, int describedByCount);
int gkWin32Focus(void *view);
int gkWin32IsFocused(void *view);
void gkWin32SetTooltip(void *view, const char *text);
void gkWin32SetCursor(void *view, int cursor);
int gkWin32SetStyle(void *view, const char *family, double size, int weight, int italic, int foreground, int background,
                    const double *foregroundRGBA, const double *backgroundRGBA, double cornerRadius);
void gkWin32SetMenu(void *view, void *menu);
void gkWin32SetDragSource(void *view, int operations, const char *text, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *customTypes,
                          size_t customTypesSize, const unsigned char *customData, const size_t *customSizes,
                          int customCount);
void gkWin32SetDropTarget(void *view, int operations, const char *types, size_t typesSize);
char *gkWin32AccessibilityTree(void *window, size_t *size);
void *gkWin32ViewWindow(void *view);
void gkWin32TrackFocus(void *window);
void gkWin32ForgetFocus(void *window, uintptr_t handle);
int gkWin32HandleDragEvent(void *event);
/* gkWin32AfterPoll runs after each event-loop pass. */
void gkWin32AfterPoll(void);

/* Icons: a glyph of the symbol font or RGBA pixels. Returns a retained icon or NULL. */
void *gkWin32Icon(const char *symbol, const unsigned char *pixels, int width, int height, double pointSize);

/* Menus. Items report GK_EVENT_ACTION with the action ID as text. */
void *gkWin32MenuCreate(uintptr_t handle, const char *title);
void gkWin32MenuClear(void *menu);
void gkWin32MenuAddItem(void *menu, const char *actionID, const char *title, int enabled, int checked, const char *key,
                        int modifiers, void *icon, const char *tooltip);
void gkWin32MenuAddSeparator(void *menu);
void *gkWin32MenuAddSubmenu(void *menu, const char *title);
void gkWin32MenuSetHandle(void *menu, uintptr_t handle);
void gkWin32MenuPopup(void *menu, void *view, double x, double y);
void gkWin32MenuSetActionState(void *menu, const char *actionID, int enabled, int checked);

/* Window extras. */
void gkWin32WindowSetSizeLimits(void *window, int minWidth, int minHeight, int maxWidth, int maxHeight);
void gkWin32WindowSetStyle(void *window, int resizable, int borderless);
void gkWin32WindowSetAlwaysOnTop(void *window, int alwaysOnTop);
int gkWin32WindowMinimize(void *window);
int gkWin32WindowMaximize(void *window);
int gkWin32WindowRestore(void *window);
int gkWin32WindowIsMinimized(void *window);
int gkWin32WindowIsMaximized(void *window);
int gkWin32WindowIsFullscreen(void *window);
int gkWin32WindowSetFullscreen(void *window, int fullscreen);
void gkWin32WindowScreen(void *window, double *values, char **name);
void gkWin32WindowAttach(void *window, uintptr_t handle);
void gkWin32WindowDetach(void *window);
int gkWin32WindowResolvedTheme(void *window);
void gkWin32SystemPreferences(void *window, double *values, char **locale);
int gkWin32WindowColorSpace(void *window);

/* Window events reported by the window and its content. */
void gkWin32WindowNotify(void *window, int kind, int a, double x, double y, const char *text);
int gkWin32WindowAllowClose(void *window);
void gkWin32WindowDidResize(void *window);
void gkWin32WindowDidChangeKey(void *window, int key);
void gkWin32WindowAcceptFiles(void *window, int accept);
void gkWin32WindowDidShow(void *window);
int gkWin32WindowCanBecomeKey(void *window);
/* phase: 0 will enter, 1 did enter, 2 will exit, 3 did exit, 4 failed to enter,
   5 failed to exit. */
void gkWin32WindowFullscreenPhase(void *window, int phase);

/* Canvas extras. */
void *gkWin32CaptureTarget(void *event);
void gkWin32DeliverCaptured(void *view, void *event);
void gkWin32ReleaseCaptureFor(void *view);
void gkWin32CanvasAttach(void *view, uintptr_t handle);
int gkWin32CanvasCapture(void *view, int capture);
int gkWin32CanvasHasCapture(void *view);
int gkWin32CanvasStartFrames(void *view, int start);
int gkWin32ViewColorSpace(void *view);
double gkWin32ViewScale(void *view);

/* Component extensions. */
void gkWin32LabelConfigure(void *label, int alignment, int wrap, int truncation, int selectable, int style,
                           int maximumLines);
void gkWin32LabelPreferredWidth(void *label, int width);
void gkWin32ButtonConfigure(void *button, void *icon, int placement, int role, const char *key, int modifiers);
void gkWin32ButtonSetBusy(void *button, int busy);
void gkWin32ButtonSetBadge(void *button, const char *text);
void gkWin32TextFieldConfigure(void *field, int readOnly, int maximumLength, int purpose, int alignment,
                               const char *prefix, const char *suffix, int spellCheck);
void gkWin32TextSelection(void *view, int *start, int *length);
void gkWin32SetTextSelection(void *view, int start, int length);
void gkWin32TextAreaConfigure(void *area, int wrap, int maximumLength, int tabMovesFocus, int spellCheck);
void gkWin32ScrollPosition(void *scroll, int *x, int *y);
void gkWin32SetScrollPosition(void *scroll, int x, int y, int animated);
void gkWin32CheckBoxConfigure(void *box, int allowsMixed, int state, int labelBefore, const char *text,
                              const char *description);
int gkWin32CheckBoxState(void *box);
void gkWin32SetChoices(void *view, const char *titles, size_t titlesSize, const int *enabled, int count);
void gkWin32RadioGroupConfigure(void *group, int horizontal, const char *label);
void gkWin32RadioGroupPreferredSize(void *group, int *width, int *height);
void gkWin32SelectConfigure(void *popup, const char *placeholder, int popupWidth);
void gkWin32SliderConfigure(void *slider, int vertical, double minimum, double maximum, double frequency, int snap);
void gkWin32ProgressConfigure(void *progress, int indeterminate, int vertical, const char *text, int state);
void gkWin32ActivityConfigure(void *activity, int size, const char *label);
void gkWin32SeparatorConfigure(void *separator, int vertical, const char *label, int inset, int thickness);
void gkWin32ImageConfigure(void *image, int alignment, int nearest, const char *alternative, double scale);
void gkWin32LinkConfigure(void *link, int visited, void *icon);
int gkWin32OpenURL(const char *url, int background);
void gkWin32TabsConfigure(void *tabs, int placement);
void gkWin32ScrollConfigure(void *scroll, int axes, int policy, int top, int right, int bottom, int left,
                            int overscroll);
void gkWin32ScrollSetZoom(void *scroll, double zoom);
double gkWin32ScrollZoom(void *scroll);
void gkWin32ScrollViewport(void *scroll, int *width, int *height);
int gkWin32ScrollToView(void *scroll, void *view, int animated);
void gkWin32SelectSetVisibleRows(void *popup, int rows);
void gkWin32SliderSetInverted(void *slider, int inverted);
void gkWin32TabsSetOverflow(void *tabs, int policy);
void gkWin32SectionSetAnimated(void *section, int animated);
void gkWin32SectionConfigure(void *section, const char *subtitle, void *icon, const char *badge);

/* Dialogs. */
void *gkWin32AlertCreate(int severity, const char *title, const char *message, const char *details, const char *buttons,
                         size_t buttonsSize, int cancelIndex, int destructiveMask, int prompt, int secure,
                         const char *text, const char *placeholder);
void *gkWin32ModalBegin(void *alert, void *parent);
int gkWin32ModalStep(void *modal);
void gkWin32ModalEnd(void *modal, int abort);
char *gkWin32AlertText(void *alert);
void gkWin32ShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                      const char *credits, void *icon);

/* Services. */
long gkWin32ClipboardChangeCount(void);
void gkWin32ClipboardClear(void);
int gkWin32ClipboardWrite(const char *text, int hasText, const char *files, size_t filesSize,
                          const unsigned char *pixels, int width, int height, const char *types, size_t typesSize,
                          const unsigned char *data, const size_t *sizes, int count);
void gkWin32ClipboardRead(char **text, int *hasText, char **files, size_t *filesSize, unsigned char **pixels,
                          int *width, int *height, char **types, size_t *typesSize, unsigned char **data,
                          size_t **sizes, int *count);
char *gkWin32ClipboardFormats(size_t *size);
int gkWin32RevealFile(const char *path, int select);
void gkWin32RecentAdd(const char *path);
char *gkWin32RecentList(size_t *size);
void gkWin32RecentClear(void);
void gkWin32RecentRemove(const char *path);
char *gkWin32DefaultApplication(const char *extension);
int gkWin32Print(uintptr_t handle, const char *title, int showDialog, void *window);
void gkWin32PrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w, double h);
int gkWin32PrintText(const char *text, double x, double y, const char *family, double size, int weight, int italic);
void *gkWin32ShareBegin(const char *texts, size_t textsSize, const char *urls, size_t urlsSize, const char *files,
                        size_t filesSize, void *view);
int gkWin32ShareStep(void *picker, char **service);
void gkWin32ShareEnd(void *picker);

/* Notifications. Asynchronous calls report through gkWin32GoNotificationResult. */
int gkWin32NotificationsAvailable(void);
void gkWin32NotificationsAttach(uintptr_t handle);
void gkWin32NotificationAuthorization(uintptr_t token);
void gkWin32NotificationRequestAuthorization(uintptr_t token);
void gkWin32NotificationSend(uintptr_t token, const char *identifier, const char *title, const char *subtitle,
                             const char *body, const char *category, const char *thread, int sound,
                             const char *attachment, double delay, const char *actionIDs, size_t actionIDsSize,
                             const char *actionTitles, size_t actionTitlesSize, const int *actionFlags,
                             int actionCount);
void gkWin32NotificationPending(uintptr_t token);
void gkWin32NotificationDelivered(uintptr_t token);
void gkWin32NotificationWithdraw(const char *ids, size_t size);
void gkWin32SetBadge(const char *badge);
char *gkWin32Badge(void);

/* Status items. */
void *gkWin32TrayCreate(uintptr_t handle);
void gkWin32TrayConfigure(void *tray, void *icon, int isTemplate, const char *title, const char *tooltip);
void gkWin32TraySetVisible(void *tray, int visible);
void gkWin32TraySetAttention(void *tray, int attention);
void gkWin32TraySetMenu(void *tray, void *menu);
int gkWin32TrayBounds(void *tray, double *x, double *y, double *width, double *height);
void gkWin32TrayRemove(void *tray);

/* Controls. */
void gkWin32Disconnect(void *view);
void *gkWin32ToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar);
void *gkWin32SwitchCreate(uintptr_t window, const char *text, const char *onLabel, const char *offLabel, int checked,
                          int size);
int gkWin32SwitchChecked(void *view);
void gkWin32SwitchSet(void *view, const char *text, int checked);
void *gkWin32PasswordCreate(uintptr_t window, const char *placeholder, int reveal);
char *gkWin32PasswordText(void *view);
void gkWin32PasswordSet(void *view, const char *text, const char *placeholder, int revealed);
void *gkWin32SearchCreate(uintptr_t window, const char *text, const char *placeholder);
void gkWin32SearchSetSuggestions(void *view, const char *items, size_t size);
void *gkWin32NumberCreate(uintptr_t window, double minimum, double maximum, double value, double step, int decimals);
double gkWin32NumberValue(void *view);
void gkWin32NumberSetValue(void *view, double value);
void *gkWin32ComboCreate(uintptr_t window, const char *text, const char *items, size_t size);
void gkWin32ComboSetItems(void *view, const char *items, size_t size);
void *gkWin32SegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons, const int *enabled,
                             int count, int mode);
int gkWin32SegmentSelected(void *view, int index);
int gkWin32SegmentClicked(void *view);
void gkWin32SegmentSetSelected(void *view, int index, int selected);
void *gkWin32StepperCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps);
void *gkWin32DialCreate(uintptr_t window, double minimum, double maximum, double value, double step,
                        int wraps);
double gkWin32ControlValue(void *view);
void gkWin32ControlSetValue(void *view, double value);
void *gkWin32RangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower, double upper,
                               double step);
void gkWin32RangeSliderValues(void *view, double *lower, double *upper);
void gkWin32RangeSliderSet(void *view, double lower, double upper);
void *gkWin32DatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum, double maximum,
                              int range, double interval, int clock);
void gkWin32DatePickerValue(void *view, double *value, double *interval);
void gkWin32DatePickerSet(void *view, double value, double interval);
/* Calendars. Dates are seconds since 1970 at midnight UTC. selection is 0 for
   one date, 1 for any set of dates, and 2 for a range. */
void *gkWin32CalendarCreate(uintptr_t window, int selection, const double *dates, int count, double visible);
int gkWin32CalendarDates(void *calendar, double *dates, int capacity);
void gkWin32CalendarSetDates(void *calendar, const double *dates, int count);
double gkWin32CalendarVisibleMonth(void *calendar);
void gkWin32CalendarSetVisibleMonth(void *calendar, double month);
void *gkWin32ColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allowsAlpha);
void gkWin32ColorWellColor(void *view, double *rgba);
void gkWin32ColorWellSet(void *view, double r, double g, double b, double a);
void *gkWin32FontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic);
void gkWin32FontPickerSet(void *view, const char *family, double size, int weight, int italic);
char *gkWin32FontPickerFont(void *view, double *size, int *weight, int *italic);
void *gkWin32KeyRecorderCreate(uintptr_t window);
void gkWin32KeyRecorderSet(void *view, const char *display);

/* Containers. */
void gkWin32AddChild(void *container, void *child);
void *gkWin32ClipViewCreate(uintptr_t window);
void *gkWin32GroupBoxCreate(uintptr_t window, const char *title, const char *subtitle);
void gkWin32GroupBoxSet(void *box, const char *title, const char *subtitle);
void gkWin32GroupBoxBody(void *box, int *width, int *height, int *top);
int gkWin32GroupBoxSubtitleHeight(void *box, int width);
void gkWin32GroupBoxChrome(void *box, int *width, int *height);
void *gkWin32SplitViewCreate(uintptr_t window, int vertical, int panes);
void *gkWin32SplitViewPane(void *split, int index);
void gkWin32SplitViewSetPane(void *split, int index, double minimum, int collapsible);
void gkWin32SplitViewPaneFrame(void *split, int index, int *x, int *y, int *width, int *height);
double gkWin32SplitViewDivider(void *split, int index);
void gkWin32SplitViewSetDivider(void *split, int index, double position);
void *gkWin32PopoverCreate(uintptr_t handle, int transient);
void *gkWin32PopoverBody(void *popover);
int gkWin32PopoverShow(void *popover, void *anchor, int edge, int width, int height);
void gkWin32PopoverHide(void *popover);
int gkWin32PopoverShown(void *popover);
void gkWin32PopoverRelease(void *popover);
int gkWin32TitleAccessoryAdd(void *window, void *view, int alignment, int width, int height, void **controller);
void gkWin32TitleAccessoryLayout(void *controller);
/* gkWin32BatchBegin and gkWin32BatchEnd surround a layout pass, which
   moves windows without drawing them until the outermost pass ends. */
void gkWin32BatchBegin(void);
void gkWin32BatchEnd(void);
void gkWin32TitleAccessoryRemove(void *controller);
void gkWin32ViewSize(void *view, int *width, int *height);

/* Data views. */
void *gkWin32TableCreate(uintptr_t window, int list, const char *columnIDs, size_t columnIDsSize, const char *titles,
                         size_t titlesSize, const int *widths, const int *sortable, int selection);
void gkWin32TableSetRows(void *table, const char *ids, size_t idsSize, const char *cells, size_t cellsSize,
                         const char *details, size_t detailsSize, void **icons, int columns);
char *gkWin32TableSelection(void *table, size_t *size);
void gkWin32TableSelect(void *table, const char *ids, size_t size);
int gkWin32TableScrollTo(void *table, const char *identifier);
void *gkWin32OutlineCreate(uintptr_t window, int navigation, int multiple);
void gkWin32OutlineSetNodes(void *outline, const char *parent, const int *depths, const int *flags, const char *ids,
                            size_t idsSize, const char *texts, size_t textsSize, const char *badges, size_t badgesSize,
                            void **icons, int count);
char *gkWin32OutlineSelection(void *outline, size_t *size);
void gkWin32OutlineSelect(void *outline, const char *ids, size_t size);
int gkWin32OutlineExpanded(void *outline, const char *identifier);
int gkWin32OutlineSetExpanded(void *outline, const char *identifier, int expanded);
void *gkWin32CollectionCreate(uintptr_t window, int width, int height, int selection);
void gkWin32CollectionSetItems(void *collection, const char *ids, size_t idsSize, const char *texts, size_t textsSize,
                               void **images, int count);
char *gkWin32CollectionSelection(void *collection, size_t *size);
void gkWin32CollectionSelect(void *collection, const char *ids, size_t size);
void *gkWin32BreadcrumbsCreate(uintptr_t window);
void gkWin32BreadcrumbsSetItems(void *path, const char *titles, size_t size);
void *gkWin32PaginationCreate(uintptr_t window, int count, int selected, int compact);
void gkWin32PaginationSet(void *pagination, int count, int selected);

/* Window chrome. */
/* gkWin32MenuBarCreate shows a menu bar at the top of window. */
void *gkWin32MenuBarCreate(void *window, const char *titles, size_t titlesSize, void **menus, int count);
void gkWin32MenuBarInstall(void *bar);
void gkWin32MenuBarRelease(void *bar);
void *gkWin32ToolbarCreate(uintptr_t handle, void *window, const int *kinds, const char *ids, size_t idsSize,
                           const char *labels, size_t labelsSize, void **icons, const int *enabled, int count,
                           int showsLabels);
void gkWin32ToolbarSetEnabled(void *toolbar, const char *identifier, int enabled);
void gkWin32ToolbarRelease(void *toolbar);
void *gkWin32StatusBarCreate(uintptr_t window, const char *text);
void gkWin32StatusBarSet(void *bar, const char *text);
void *gkWin32CommandBarCreate(uintptr_t window);
void gkWin32CommandBarSetActions(void *bar, const char *ids, size_t idsSize, const char *titles, size_t titlesSize,
                                 void **icons, const int *enabled, int count, int compact);

/* Feedback. */
void gkWin32Announce(void *element, const char *text, int priority);
void *gkWin32InfoBarCreate(uintptr_t window, int closable, const char *ids, size_t idsSize, const char *titles,
                           size_t titlesSize, const int *enabled, int count);
void gkWin32InfoBarSet(void *bar, int severity, const char *title, const char *detail);
void *gkWin32TooltipCreate(void);
void gkWin32TooltipSetText(void *tooltip, const char *text);
int gkWin32TooltipShow(void *tooltip, void *anchor, int edge, double delay);
void gkWin32TooltipHide(void *tooltip);
int gkWin32TooltipShown(void *tooltip);
void *gkWin32ToastCreate(uintptr_t handle, const char *message, const char *ids, size_t idsSize, const char *titles,
                         size_t titlesSize, const int *enabled, int count, double timeout);
int gkWin32ToastShow(void *toast, void *window);
void gkWin32ToastDismiss(void *toast);
int gkWin32ToastShown(void *toast);
void gkWin32ToastRelease(void *toast);
void *gkWin32BadgeCreate(uintptr_t window);
void gkWin32BadgeSet(void *badge, const char *text, int severity, const char *accessible);
void *gkWin32ProgressDialogCreate(uintptr_t handle, const char *title, int cancellable);
void gkWin32ProgressDialogSet(void *dialog, const char *message, double value);
int gkWin32ProgressDialogShow(void *dialog, void *parent);
void gkWin32ProgressDialogRelease(void *dialog);
void *gkWin32EmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t idsSize, const char *titles,
                              size_t titlesSize, const int *enabled, int count);
void gkWin32EmptyStateSet(void *view, const char *title, const char *detail);
void *gkWin32InlineValidationCreate(uintptr_t window);
void gkWin32InlineValidationSet(void *view, int severity, const char *text);

/* Content. */
void *gkWin32IconCreate(uintptr_t window);
void gkWin32IconSet(void *icon, void *image, int width, int height, int tint, const char *name);
void *gkWin32DocumentCreate(uintptr_t window, int editable, int markdown);
char *gkWin32DocumentText(void *document);
void gkWin32DocumentSetText(void *document, const char *text);
void gkWin32DocumentSetEditable(void *document, int editable);
int gkWin32DocumentEditable(void *document);
unsigned char *gkWin32DocumentRTF(void *document, size_t *size);
int gkWin32DocumentSetRTF(void *document, const unsigned char *rtf, size_t size);
int gkWin32MarkdownSet(void *document, const char *source);
void *gkWin32VectorCreate(uintptr_t window);
int gkWin32VectorSet(void *view, const unsigned char *data, size_t size, const char *label, double *width,
                     double *height);
void *gkWin32AnimationCreate(uintptr_t window, void **images, const double *durations, int count, int repeats);
void gkWin32AnimationPlay(void *view, int play);
int gkWin32AnimationPlaying(void *view);
void *gkWin32VideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted);
void gkWin32VideoPlay(void *view, int play);
int gkWin32VideoPlaying(void *view);
double gkWin32VideoPosition(void *view);
double gkWin32VideoDuration(void *view);
void gkWin32VideoSeek(void *view, double seconds);
void gkWin32VideoRelease(void *view);
void *gkWin32WebViewCreate(uintptr_t window, int scripts);
int gkWin32WebViewLoad(void *view, const char *url);
void gkWin32WebViewLoadHTML(void *view, const char *html);
char *gkWin32WebViewURL(void *view);
void gkWin32WebViewRelease(void *view);

/* Application events. */
void gkWin32AppAttach(uintptr_t handle);

#endif
