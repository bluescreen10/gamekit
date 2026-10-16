#ifndef GAMEKIT_GTK_INTERNAL_H
#define GAMEKIT_GTK_INTERNAL_H

/* Types and helpers shared by the C files of the GTK backend.

   The Go layer refers to native objects with opaque pointers. A window is a
   GKWin record. Every other object is a GObject: a widget is the outermost
   GtkWidget of the component, and it carries a GKView record as object data.
   Go values are referred to with runtime/cgo handles. */

#define GDK_DISABLE_DEPRECATION_WARNINGS
#define GTK_DISABLE_DEPRECATION_WARNINGS
#include <gtk/gtk.h>
#include <gdk/x11/gdkx.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "gtk_bridge.h"
#include "widget_bridge.h"
#include "window_bridge.h"

/* ---- Layout container ----

   GKLayout places its children at the frames that the Go layout code sets,
   like an NSView with manually set subview frames. Its measured size is the
   minimum size that gkLayoutSetMinimum sets, which is zero by default. */

#define GK_TYPE_LAYOUT (gk_layout_get_type())
G_DECLARE_FINAL_TYPE(GKLayout, gk_layout, GK, LAYOUT, GtkWidget)

GtkWidget *gkLayoutNew(void);
GtkWidget *gkLayoutNewWithRole(GtkAccessibleRole role);
void gkLayoutPut(GtkWidget *layout, GtkWidget *child);
void gkLayoutMove(GtkWidget *child, int x, int y, int width, int height);
void gkLayoutSetMinimum(GtkWidget *layout, int width, int height);
/* gkLayoutSetZoom scales the children of layout and its minimum size. */
void gkLayoutSetZoom(GtkWidget *layout, double zoom);
double gkLayoutZoom(GtkWidget *layout);
/* gkLayoutOnAllocate calls callback after each allocation of layout. */
void gkLayoutOnAllocate(GtkWidget *layout, void (*callback)(GtkWidget *layout, int width, int height, gpointer data),
                        gpointer data);

/* ---- Windows ---- */

typedef struct GKWin {
    GtkWidget *window;
    GtkWidget *overlay;
    GtkWidget *box;
    GtkWidget *top;
    GtkWidget *root;
    GtkWidget *bottom;
    struct GKWin *owner;
    GtkIMContext *im;
    GtkWidget *focus;
    uintptr_t handle;
    int should_close;
    int utility;
    int borderless;
    int resizable;
    int always_on_top;
    int transparent;
    int have_position;
    int x, y;
    int width, height;
    int reported_width, reported_height;
    int min_width, min_height, max_width, max_height;
    int cursor_mode;
    double virtual_x, virtual_y;
    double last_x, last_y;
    int have_last;
    int warping;
    double scroll_x, scroll_y;
    int fullscreen;
    int fullscreen_pending;
    int fullscreen_target;
    int fullscreen_state;
    int windowed_width, windowed_height;
    int restore_width, restore_height, restore_tries;
    guint restore_timer;
    guint fullscreen_timer;
    int maximized;
    int accept_files;
    int composing;
    int occluded;
    int theme;
    int resolved_theme;
    int destroyed;
    unsigned int buttons;
    GtkWidget *menubar;
    GtkWidget *toolbar;
    GtkWidget *statusbar;
} GKWin;

GKWin *gkWinFromWidget(GtkWidget *widget);
GKWin *gkWinFromHandle(uintptr_t window);
Display *gkWinXDisplay(GKWin *win);
Window gkWinXWindow(GKWin *win);
GdkSurface *gkWinSurface(GKWin *win);
void gkWinNotify(GKWin *win, int kind, int a, double x, double y, const char *text);
/* gkWinPump runs the GTK main context until done returns TRUE or timeout
   seconds pass. */
void gkWinPump(gboolean (*done)(gpointer data), gpointer data, double timeout);
int gkResolvedTheme(void);
GdkMonitor *gkWinMonitor(GKWin *win);
void gkWinContentOrigin(GKWin *win, int *x, int *y);
/* gkWinAddChrome puts widget above (top != 0) or below the content area. */
void gkWinAddChrome(GKWin *win, GtkWidget *widget, int top);

/* ---- Views ---- */

enum {
    GK_KIND_GENERIC = 0,
    GK_KIND_LAYOUT,
    GK_KIND_LABEL,
    GK_KIND_BUTTON,
    GK_KIND_TEXT_FIELD,
    GK_KIND_TEXT_AREA,
    GK_KIND_CHECK_BOX,
    GK_KIND_RADIO_GROUP,
    GK_KIND_SELECT,
    GK_KIND_SLIDER,
    GK_KIND_PROGRESS,
    GK_KIND_ACTIVITY,
    GK_KIND_SEPARATOR,
    GK_KIND_IMAGE,
    GK_KIND_LINK,
    GK_KIND_TABS,
    GK_KIND_SCROLL,
    GK_KIND_CANVAS,
    GK_KIND_SECTION,
    GK_KIND_TOGGLE,
    GK_KIND_SWITCH,
    GK_KIND_PASSWORD,
    GK_KIND_SEARCH,
    GK_KIND_NUMBER,
    GK_KIND_COMBO,
    GK_KIND_SEGMENTED,
    GK_KIND_STEPPER,
    GK_KIND_DIAL,
    GK_KIND_RANGE,
    GK_KIND_DATE,
    GK_KIND_CALENDAR,
    GK_KIND_COLOR,
    GK_KIND_FONT,
    GK_KIND_KEY_RECORDER,
    GK_KIND_CLIP,
    GK_KIND_GROUP_BOX,
    GK_KIND_SPLIT,
    GK_KIND_TABLE,
    GK_KIND_OUTLINE,
    GK_KIND_COLLECTION,
    GK_KIND_BREADCRUMBS,
    GK_KIND_PAGINATION,
    GK_KIND_STATUS_BAR,
    GK_KIND_COMMAND_BAR,
    GK_KIND_INFO_BAR,
    GK_KIND_BADGE,
    GK_KIND_EMPTY_STATE,
    GK_KIND_VALIDATION,
    GK_KIND_ICON,
    GK_KIND_DOCUMENT,
    GK_KIND_VECTOR,
    GK_KIND_ANIMATION,
    GK_KIND_VIDEO,
    GK_KIND_WEB,
};

typedef struct GKView {
    GtkWidget *outer;
    GtkWidget *control;
    GtkWidget *body;
    GtkWidget *label;
    GKWin *window;
    int kind;
    uintptr_t handle;
    uintptr_t action;
    int suppress;
    int x, y, width, height;
    int visible;
    char *css_class;
    char *placeholder;
    char *url;
    char *accessible_id;
    char *accessible_name;
    char *accessible_value;
    GPtrArray *items;
    GPtrArray *buttons;
    int selected;
    int scaling;
    int alignment;
    int nearest;
    double source_scale;
    GdkTexture *texture;
    int maximum_length;
    int read_only;
    int running;
    double value;
    int axes;
    int canvas_buttons;
    Display *xdisplay;
    Window xwindow;
    int xmapped;
    int xwidth, xheight;
    int frames;
    guint tick;
    double last_frame;
    gpointer ext;
    GDestroyNotify ext_free;
    GtkEventController *drag_source;
    GtkEventController *drop_target;
    GtkWidget *popover_menu;
    GtkGesture *menu_gesture;
    int style_set;
    int preferred_width;
    int inverted;
    int vertical;
    int animated;
} GKView;

GKView *gkView(void *object);
/* gkViewNew attaches a new view record to outer and sinks its floating
   reference, so the Go layer owns one reference. */
GKView *gkViewNew(GtkWidget *outer, GtkWidget *control, int kind, uintptr_t window, uintptr_t action);
/* gkViewSend reports a widget-contract event through the view's handle. */
void gkViewSend(GKView *view, int kind, int a, int b, double x, double y, const char *text);
/* gkViewAction reports a GK_UI_EVENT to the view's action handle. */
void gkViewAction(GKView *view, int event);
GtkWidget *gkViewContainer(GKView *view);
/* gkViewFromDescendant returns the view of widget or its nearest ancestor. */
GKView *gkViewFromDescendant(GtkWidget *widget);
void gkRemoveFromParent(GtkWidget *widget);
void gkSend(uintptr_t handle, int kind, int a, int b, double x, double y, const char *text);

/* ---- Helpers ---- */

char *gkStrdup(const char *text);
/* gkSplit splits a NUL-separated buffer into a NULL-terminated vector. */
char **gkSplit(const char *buffer, size_t size, int *count);
/* gkJoin joins strings into a malloc'd NUL-separated buffer. */
char *gkJoin(char **values, int count, size_t *size);
GdkTexture *gkTextureFromRGBA(const unsigned char *pixels, int width, int height, int stride);
int gkModifiers(GdkModifierType state);
int gkPointerButton(guint button);
int gkKeyFromKeycode(guint keycode);
const char *gkWidgetCSSClass(GKView *view);
void gkApplyCSS(void);
void gkSetCSSRule(const char *selector, const char *rule);
/* gkSemanticColor returns the CSS color for a ui.SemanticColor, or NULL. */
const char *gkSemanticColor(int color);
void gkRunPending(void);
GtkWidget *gkIconImage(void *icon, int size);
GdkPaintable *gkIconPaintable(void *icon);
GtkWidget *gkDescendant(GtkWidget *widget, GType type);
/* gkButtonContent gives a button an icon, a label, a spinner, and a badge.
   It returns the label. */
GtkWidget *gkButtonContent(GKView *view);
void gkButtonSetText(GKView *view, const char *text);
GtkWidget *gkSectionPart(void *control, int part);
GtkWidget *gkMenuButtonFor(void *menu);
GMenuModel *gkMenuModel(void *menu);
GActionGroup *gkMenuActions(void *menu);
void gkMenuAttach(GtkWidget *widget, void *menu);
void gkGTKAnnounceText(GtkWidget *widget, const char *text, int priority);

#endif
