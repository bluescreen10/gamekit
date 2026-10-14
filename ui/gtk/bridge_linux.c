//go:build linux && cgo

#include "bridge.h"

/* GameKit supports GTK 4.0. New GTK versions deprecate the X11 and file chooser
   compatibility APIs that the backend must use. */
#define GDK_DISABLE_DEPRECATION_WARNINGS
#define GTK_DISABLE_DEPRECATION_WARNINGS
#include <gtk/gtk.h>
#include <gdk/x11/gdkx.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct GKGTKWindow {
    GtkWidget *window;
    GtkWidget *overlay;
    GtkWidget *content;
    struct GKGTKWindow *owner;
    gboolean should_close;
    gboolean utility;
    gboolean have_position;
    int x;
    int y;
    int width;
    int height;
    unsigned int buttons;
    uintptr_t handle;
} GKGTKWindow;

typedef struct GKGTKWidget {
    GtkWidget *widget;
    GtkWidget *content;
    GtkWidget *control;
    GKGTKWindow *window;
    GPtrArray *items;
    GdkPixbuf *pixbuf;
    int kind;
    int scaling;
    int selected;
    gboolean running;
    unsigned int buttons;
    uintptr_t handle;
    gboolean suppress;
    Display *xdisplay;
    Window xwindow;
    gboolean xwindow_mapped;
} GKGTKWidget;

static gboolean gkGTKInitialized;
static gboolean gkGTKSystemDark;

static int gkGTKModifiers(GdkModifierType state) {
    int modifiers = 0;
    if (state & GDK_SHIFT_MASK) {
        modifiers |= GK_GTK_MOD_SHIFT;
    }
    if (state & GDK_CONTROL_MASK) {
        modifiers |= GK_GTK_MOD_CONTROL;
    }
    if (state & GDK_ALT_MASK) {
        modifiers |= GK_GTK_MOD_ALT;
    }
    if (state & GDK_SUPER_MASK) {
        modifiers |= GK_GTK_MOD_SUPER;
    }
    if (state & GDK_LOCK_MASK) {
        modifiers |= GK_GTK_MOD_CAPS_LOCK;
    }
    return modifiers;
}

static int gkGTKPointerButton(guint button) {
    switch (button) {
    case 1:
        return GK_GTK_POINTER_BUTTON_LEFT;
    case 3:
        return GK_GTK_POINTER_BUTTON_RIGHT;
    case 2:
        return GK_GTK_POINTER_BUTTON_MIDDLE;
    default:
        if (button >= 8 && button < 8 + GK_GTK_POINTER_BUTTON_COUNT - 3) {
            return 3 + (int)button - 8;
        }
        return -1;
    }
}

static void gkGTKReleaseWindowButtons(GKGTKWindow *native, int modifiers) {
    if (!native) {
        return;
    }
    for (int button = 0; button < GK_GTK_POINTER_BUTTON_COUNT; button++) {
        if ((native->buttons & (1u << button)) && native->handle) {
            gkGTKGoWindowPointerButton(native->handle, button, GK_GTK_POINTER_RELEASED, modifiers);
        }
    }
    native->buttons = 0;
}

static void gkGTKReleaseCanvasButtons(GKGTKWidget *native, int modifiers) {
    if (!native) {
        return;
    }
    for (int button = 0; button < GK_GTK_POINTER_BUTTON_COUNT; button++) {
        if ((native->buttons & (1u << button)) && native->handle) {
            gkGTKGoCanvasPointerButton(native->handle, button, GK_GTK_POINTER_RELEASED, modifiers);
        }
    }
    native->buttons = 0;
}

static void gkGTKWindowButton(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKGTKWindow *native = data;
    int button = gkGTKPointerButton(gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture)));
    if (!native || button < 0 || button >= GK_GTK_POINTER_BUTTON_COUNT) {
        return;
    }
    native->buttons |= 1u << button;
    if (native->handle) {
        GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
        gkGTKGoWindowPointerButton(native->handle, button, GK_GTK_POINTER_PRESSED, gkGTKModifiers(state));
    }
}

static void gkGTKWindowButtonReleased(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKGTKWindow *native = data;
    int button = gkGTKPointerButton(gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture)));
    if (!native || button < 0 || button >= GK_GTK_POINTER_BUTTON_COUNT) {
        return;
    }
    unsigned int mask = 1u << button;
    if (!(native->buttons & mask)) {
        return;
    }
    native->buttons &= ~mask;
    if (native->handle) {
        GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
        gkGTKGoWindowPointerButton(native->handle, button, GK_GTK_POINTER_RELEASED, gkGTKModifiers(state));
    }
}

static gboolean gkGTKWindowScroll(GtkEventControllerScroll *controller, double dx, double dy, gpointer data) {
    (void)controller;
    GKGTKWindow *native = data;
    if (native && native->handle) {
        gkGTKGoWindowScroll(native->handle, dx, -dy);
    }
    return FALSE;
}

static void gkGTKWindowActive(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    if (!gtk_window_is_active(GTK_WINDOW(object))) {
        gkGTKReleaseWindowButtons(data, 0);
    }
}

static gboolean gkGTKCloseRequest(GtkWindow *window, gpointer data) {
    GKGTKWindow *native = data;
    native->should_close = TRUE;
    if (native->utility) {
        gtk_widget_set_visible(GTK_WIDGET(window), FALSE);
    }
    return TRUE;
}

static GdkSurface *gkGTKWindowSurface(GKGTKWindow *native) {
    if (!native || !native->window) {
        return NULL;
    }
    gtk_widget_realize(native->window);
    GtkNative *gtk_native = gtk_widget_get_native(native->window);
    return gtk_native ? gtk_native_get_surface(gtk_native) : NULL;
}

static Window gkGTKWindowXWindow(GKGTKWindow *native) {
    GdkSurface *surface = gkGTKWindowSurface(native);
    return surface && GDK_IS_X11_SURFACE(surface) ? gdk_x11_surface_get_xid(surface) : 0;
}

static Display *gkGTKWindowXDisplayValue(GKGTKWindow *native) {
    GdkSurface *surface = gkGTKWindowSurface(native);
    if (!surface || !GDK_IS_X11_SURFACE(surface)) {
        return NULL;
    }
    return gdk_x11_display_get_xdisplay(gdk_surface_get_display(surface));
}

static void gkGTKWindowOrigin(GKGTKWindow *native, int *x, int *y) {
    int origin_x = native ? native->x : 0;
    int origin_y = native ? native->y : 0;
    Display *display = gkGTKWindowXDisplayValue(native);
    Window window = gkGTKWindowXWindow(native);
    if (display && window) {
        Window child = 0;
        XTranslateCoordinates(display, window, DefaultRootWindow(display), 0, 0, &origin_x, &origin_y, &child);
    }
    if (x) {
        *x = origin_x;
    }
    if (y) {
        *y = origin_y;
    }
}

static void gkGTKWindowPositionValues(GKGTKWindow *native, int *x, int *y) {
    int window_x = 0;
    int window_y = 0;
    int owner_x = 0;
    int owner_y = 0;
    if (native) {
        gkGTKWindowOrigin(native, &window_x, &window_y);
        if (native->owner) {
            gkGTKWindowOrigin(native->owner, &owner_x, &owner_y);
        }
    }
    if (x) {
        *x = window_x - owner_x;
    }
    if (y) {
        *y = window_y - owner_y;
    }
}

static void gkGTKPollWindowPosition(GKGTKWindow *native) {
    if (!native) {
        return;
    }
    int x = 0;
    int y = 0;
    gkGTKWindowPositionValues(native, &x, &y);
    if (native->have_position && native->x == x && native->y == y) {
        return;
    }
    native->x = x;
    native->y = y;
    native->have_position = TRUE;
    if (native->handle) {
        gkGTKGoWindowMove(native->handle, x, y);
    }
}

static void gkGTKSetX11WindowProperties(GKGTKWindow *native, uint32_t flags) {
    Display *display = gkGTKWindowXDisplayValue(native);
    Window window = gkGTKWindowXWindow(native);
    if (!display || !window) {
        return;
    }
    if ((flags & (1u << 4)) || native->utility) {
        Atom state = XInternAtom(display, "_NET_WM_STATE", False);
        Atom values[2];
        int count = 0;
        if (flags & (1u << 4)) {
            values[count++] = XInternAtom(display, "_NET_WM_STATE_ABOVE", False);
        }
        if (native->utility) {
            values[count++] = XInternAtom(display, "_NET_WM_STATE_SKIP_TASKBAR", False);
        }
        XChangeProperty(display, window, state, XA_ATOM, 32, PropModeReplace, (unsigned char *)values, count);
    }
    if (native->utility) {
        Atom property = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
        Atom utility = XInternAtom(display, "_NET_WM_WINDOW_TYPE_UTILITY", False);
        XChangeProperty(display, window, property, XA_ATOM, 32, PropModeReplace, (unsigned char *)&utility, 1);
    }
    XFlush(display);
}

static void gkGTKApplyTheme(int theme) {
    GtkSettings *settings = gtk_settings_get_default();
    if (!settings) {
        return;
    }
    gboolean dark = theme == 2 || (theme == 0 && gkGTKSystemDark);
    g_object_set(settings, "gtk-application-prefer-dark-theme", dark, NULL);
}

void *gkGTKWindowCreate(const char *title, int width, int height, uint32_t flags, int theme, void *owner_pointer, int x,
                        int y, char *error, size_t error_size) {
    if (!gkGTKInitialized) {
        gdk_set_allowed_backends("x11");
        if (!gtk_init_check()) {
            if (error && error_size) {
                snprintf(error, error_size, "GTK 4 could not initialize an X11 display");
            }
            return NULL;
        }
        GtkSettings *settings = gtk_settings_get_default();
        if (settings) {
            g_object_get(settings, "gtk-application-prefer-dark-theme", &gkGTKSystemDark, NULL);
        }
        gkGTKInitialized = TRUE;
    }

    GKGTKWindow *native = calloc(1, sizeof(*native));
    if (!native) {
        return NULL;
    }
    native->window = gtk_window_new();
    native->overlay = gtk_overlay_new();
    native->owner = owner_pointer;
    native->utility = (flags & (1u << 6)) != 0;
    native->width = MAX(1, width);
    native->height = MAX(1, height);
    native->x = x;
    native->y = y;
    native->have_position = native->owner != NULL;
    g_object_set_data(G_OBJECT(native->window), "GameKitGTKWindow", native);
    gtk_window_set_child(GTK_WINDOW(native->window), native->overlay);
    gtk_window_set_title(GTK_WINDOW(native->window), title ? title : "");
    gtk_window_set_default_size(GTK_WINDOW(native->window), native->width, native->height);
    gtk_window_set_resizable(GTK_WINDOW(native->window), (flags & 1u) != 0);
    gtk_window_set_decorated(GTK_WINDOW(native->window), (flags & (1u << 2)) == 0);
    if (native->owner) {
        gtk_window_set_transient_for(GTK_WINDOW(native->window), GTK_WINDOW(native->owner->window));
        gtk_window_set_destroy_with_parent(GTK_WINDOW(native->window), TRUE);
    }
    g_signal_connect(native->window, "close-request", G_CALLBACK(gkGTKCloseRequest), native);
    g_signal_connect(native->window, "notify::is-active", G_CALLBACK(gkGTKWindowActive), native);

    GtkGesture *click = gtk_gesture_click_new();
    gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(click), 0);
    gtk_event_controller_set_propagation_phase(GTK_EVENT_CONTROLLER(click), GTK_PHASE_CAPTURE);
    g_signal_connect(click, "pressed", G_CALLBACK(gkGTKWindowButton), native);
    g_signal_connect(click, "released", G_CALLBACK(gkGTKWindowButtonReleased), native);
    gtk_widget_add_controller(native->window, GTK_EVENT_CONTROLLER(click));

    GtkEventController *scroll = gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_BOTH_AXES);
    gtk_event_controller_set_propagation_phase(scroll, GTK_PHASE_CAPTURE);
    g_signal_connect(scroll, "scroll", G_CALLBACK(gkGTKWindowScroll), native);
    gtk_widget_add_controller(native->window, scroll);

    gkGTKApplyTheme(theme);
    gtk_widget_realize(native->window);
    gkGTKSetX11WindowProperties(native, flags);
    if (native->owner) {
        int owner_x = 0;
        int owner_y = 0;
        gkGTKWindowOrigin(native->owner, &owner_x, &owner_y);
        Display *display = gkGTKWindowXDisplayValue(native);
        Window window = gkGTKWindowXWindow(native);
        if (display && window) {
            XMoveWindow(display, window, owner_x + x, owner_y + y);
            XFlush(display);
        }
    }
    if (flags & (1u << 3)) {
        gtk_window_maximize(GTK_WINDOW(native->window));
    }
    if (!(flags & (1u << 1))) {
        gtk_window_present(GTK_WINDOW(native->window));
    }
    return native;
}

void gkGTKWindowDestroy(void *pointer) {
    GKGTKWindow *native = pointer;
    if (!native) {
        return;
    }
    if (native->window) {
        g_object_set_data(G_OBJECT(native->window), "GameKitGTKWindow", NULL);
        gkGTKReleaseWindowButtons(native, 0);
        gtk_window_destroy(GTK_WINDOW(native->window));
    }
    free(native);
}

void gkGTKWindowPoll(void *pointer) {
    while (g_main_context_iteration(NULL, FALSE)) {
    }
    gkGTKPollWindowPosition(pointer);
}

int gkGTKWindowShouldClose(void *pointer) {
    GKGTKWindow *native = pointer;
    return !native || native->should_close;
}

void gkGTKWindowSetShouldClose(void *pointer, int close) {
    if (pointer) {
        ((GKGTKWindow *)pointer)->should_close = close != 0;
    }
}

void gkGTKWindowSize(void *pointer, int *width, int *height) {
    GKGTKWindow *native = pointer;
    int value_width = native ? native->width : 0;
    int value_height = native ? native->height : 0;
    if (native) {
        GtkWidget *content = native->content ? native->content : native->overlay;
        int allocated_width = gtk_widget_get_width(content);
        int allocated_height = gtk_widget_get_height(content);
        if (allocated_width > 0) {
            value_width = allocated_width;
            native->width = allocated_width;
        }
        if (allocated_height > 0) {
            value_height = allocated_height;
            native->height = allocated_height;
        }
    }
    if (width) {
        *width = value_width;
    }
    if (height) {
        *height = value_height;
    }
}

void gkGTKWindowFramebufferSize(void *pointer, int *width, int *height) {
    GKGTKWindow *native = pointer;
    int logical_width = 0;
    int logical_height = 0;
    gkGTKWindowSize(pointer, &logical_width, &logical_height);
    int scale = native ? gtk_widget_get_scale_factor(native->window) : 1;
    if (width) {
        *width = logical_width * scale;
    }
    if (height) {
        *height = logical_height * scale;
    }
}

void gkGTKWindowSetSize(void *pointer, int width, int height) {
    GKGTKWindow *native = pointer;
    if (native) {
        native->width = MAX(1, width);
        native->height = MAX(1, height);
        gtk_window_set_default_size(GTK_WINDOW(native->window), native->width, native->height);
    }
}

void gkGTKWindowPosition(void *pointer, int *x, int *y) {
    gkGTKWindowPositionValues(pointer, x, y);
}

void gkGTKWindowSetPosition(void *pointer, int x, int y) {
    GKGTKWindow *native = pointer;
    if (!native) {
        return;
    }
    int owner_x = 0;
    int owner_y = 0;
    if (native->owner) {
        gkGTKWindowOrigin(native->owner, &owner_x, &owner_y);
    }
    native->x = x;
    native->y = y;
    native->have_position = TRUE;
    Display *display = gkGTKWindowXDisplayValue(native);
    Window window = gkGTKWindowXWindow(native);
    if (display && window) {
        XMoveWindow(display, window, owner_x + x, owner_y + y);
        XFlush(display);
    }
}

int gkGTKWindowVisible(void *pointer) {
    GKGTKWindow *native = pointer;
    return native && gtk_widget_get_visible(native->window);
}

void gkGTKWindowSetVisible(void *pointer, int visible) {
    GKGTKWindow *native = pointer;
    if (!native) {
        return;
    }
    if (visible) {
        gtk_window_present(GTK_WINDOW(native->window));
    } else {
        gtk_widget_set_visible(native->window, FALSE);
    }
}

void gkGTKWindowBringToFront(void *pointer) {
    GKGTKWindow *native = pointer;
    if (native && gtk_widget_get_visible(native->window)) {
        gtk_window_present(GTK_WINDOW(native->window));
    }
}

void gkGTKWindowSetHandle(void *pointer, uintptr_t handle) {
    if (pointer) {
        ((GKGTKWindow *)pointer)->handle = handle;
    }
}

void gkGTKWindowSetTitle(void *pointer, const char *title) {
    GKGTKWindow *native = pointer;
    if (native) {
        gtk_window_set_title(GTK_WINDOW(native->window), title ? title : "");
    }
}

void gkGTKWindowSetTheme(void *pointer, int theme) {
    (void)pointer;
    gkGTKApplyTheme(theme);
}

static void gkGTKPointerPosition(GtkWidget *widget, double *x, double *y) {
    double pointer_x = 0;
    double pointer_y = 0;
    GtkNative *native = widget ? gtk_widget_get_native(widget) : NULL;
    GdkSurface *surface = native ? gtk_native_get_surface(native) : NULL;
    if (surface) {
        GdkDisplay *display = gdk_surface_get_display(surface);
        GdkSeat *seat = gdk_display_get_default_seat(display);
        GdkDevice *device = seat ? gdk_seat_get_pointer(seat) : NULL;
        double surface_x = 0;
        double surface_y = 0;
        if (device && gdk_surface_get_device_position(surface, device, &surface_x, &surface_y, NULL)) {
            GtkWidget *root = GTK_WIDGET(native);
            graphene_point_t source = GRAPHENE_POINT_INIT((float)surface_x, (float)surface_y);
            graphene_point_t destination = GRAPHENE_POINT_INIT(0, 0);
            if (root == widget || gtk_widget_compute_point(root, widget, &source, &destination)) {
                pointer_x = root == widget ? surface_x : destination.x;
                pointer_y = root == widget ? surface_y : destination.y;
            }
        }
    }
    if (x) {
        *x = pointer_x;
    }
    if (y) {
        *y = pointer_y;
    }
}

void gkGTKWindowPointerPosition(void *pointer, double *x, double *y) {
    GKGTKWindow *native = pointer;
    gkGTKPointerPosition(native ? native->window : NULL, x, y);
}

uintptr_t gkGTKWindowXID(void *pointer) {
    return (uintptr_t)gkGTKWindowXWindow(pointer);
}

uintptr_t gkGTKWindowXDisplay(void *pointer) {
    return (uintptr_t)gkGTKWindowXDisplayValue(pointer);
}

static void gkGTKRemoveFromParent(GtkWidget *widget) {
    GtkWidget *parent = widget ? gtk_widget_get_parent(widget) : NULL;
    if (!parent) {
        return;
    }
    GtkWidget *notebook = gtk_widget_get_ancestor(widget, GTK_TYPE_NOTEBOOK);
    if (notebook) {
        int page = gtk_notebook_page_num(GTK_NOTEBOOK(notebook), widget);
        if (page >= 0) {
            gtk_notebook_remove_page(GTK_NOTEBOOK(notebook), page);
            return;
        }
    }
    if (GTK_IS_FIXED(parent)) {
        gtk_fixed_remove(GTK_FIXED(parent), widget);
    } else if (GTK_IS_BOX(parent)) {
        gtk_box_remove(GTK_BOX(parent), widget);
    } else if (GTK_IS_OVERLAY(parent)) {
        if (gtk_overlay_get_child(GTK_OVERLAY(parent)) == widget) {
            gtk_overlay_set_child(GTK_OVERLAY(parent), NULL);
        } else {
            gtk_overlay_remove_overlay(GTK_OVERLAY(parent), widget);
        }
    } else if (GTK_IS_EXPANDER(parent)) {
        gtk_expander_set_child(GTK_EXPANDER(parent), NULL);
    } else if (GTK_IS_SCROLLED_WINDOW(parent)) {
        gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(parent), NULL);
    } else if (GTK_IS_VIEWPORT(parent)) {
        gtk_viewport_set_child(GTK_VIEWPORT(parent), NULL);
    } else {
        gtk_widget_unparent(widget);
    }
}

void gkGTKWindowSetContent(void *window_pointer, void *widget_pointer) {
    GKGTKWindow *window = window_pointer;
    GKGTKWidget *widget = widget_pointer;
    if (!window || !widget) {
        return;
    }
    if (window->content && window->content != widget->widget) {
        gkGTKRemoveFromParent(window->content);
    }
    gkGTKRemoveFromParent(widget->widget);
    gtk_overlay_set_child(GTK_OVERLAY(window->overlay), widget->widget);
    window->content = widget->widget;
    widget->window = window;
    gtk_widget_set_visible(widget->widget, TRUE);
}

static GKGTKWidget *gkGTKWrap(GtkWidget *widget, int kind, uintptr_t handle) {
    if (!widget) {
        return NULL;
    }
    GKGTKWidget *native = calloc(1, sizeof(*native));
    if (!native) {
        g_object_ref_sink(widget);
        g_object_unref(widget);
        return NULL;
    }
    native->widget = g_object_ref_sink(widget);
    native->kind = kind;
    native->handle = handle;
    gtk_widget_set_visible(widget, TRUE);
    return native;
}

static int gkGTKEventForKind(int kind) {
    if (kind == GK_GTK_WIDGET_BUTTON || kind == GK_GTK_WIDGET_LINK) {
        return GK_GTK_EVENT_CLICK;
    }
    if (kind == GK_GTK_WIDGET_TEXT_FIELD || kind == GK_GTK_WIDGET_TEXT_AREA) {
        return GK_GTK_EVENT_TEXT_CHANGE;
    }
    if (kind == GK_GTK_WIDGET_SLIDER) {
        return GK_GTK_EVENT_VALUE_CHANGE;
    }
    if (kind == GK_GTK_WIDGET_CHECK_BOX) {
        return GK_GTK_EVENT_TOGGLE_CHANGE;
    }
    if (kind == GK_GTK_WIDGET_RADIO_GROUP || kind == GK_GTK_WIDGET_SELECT) {
        return GK_GTK_EVENT_SELECTION_CHANGE;
    }
    if (kind == GK_GTK_WIDGET_TABS) {
        return GK_GTK_EVENT_TAB_CHANGE;
    }
    return GK_GTK_EVENT_SECTION_CHANGE;
}

static void gkGTKSignal(gpointer object, gpointer data) {
    (void)object;
    GKGTKWidget *native = data;
    if (native && !native->suppress && native->handle) {
        gkGTKGoWidgetEvent(native->handle, gkGTKEventForKind(native->kind));
    }
}

static void gkGTKNotifySignal(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    gkGTKSignal(object, data);
}

static void gkGTKRadioSignal(GtkCheckButton *button, gpointer data) {
    GKGTKWidget *native = data;
    if (!gtk_check_button_get_active(button)) {
        return;
    }
    for (guint i = 0; native->items && i < native->items->len; i++) {
        if (g_ptr_array_index(native->items, i) == GTK_WIDGET(button)) {
            native->selected = (int)i;
            break;
        }
    }
    gkGTKSignal(button, data);
}

static void gkGTKTabSignal(GtkNotebook *notebook, GtkWidget *page, guint page_number, gpointer data) {
    (void)notebook;
    (void)page;
    (void)page_number;
    gkGTKSignal(NULL, data);
}

static gboolean gkGTKLinkSignal(GtkWidget *widget, gpointer data) {
    gkGTKSignal(widget, data);
    return TRUE;
}

static void gkGTKImageDraw(GtkDrawingArea *area, cairo_t *context, int width, int height, gpointer data) {
    (void)area;
    GKGTKWidget *native = data;
    if (!native->pixbuf) {
        return;
    }
    int image_width = gdk_pixbuf_get_width(native->pixbuf);
    int image_height = gdk_pixbuf_get_height(native->pixbuf);
    double scale_x = image_width ? (double)width / image_width : 1;
    double scale_y = image_height ? (double)height / image_height : 1;
    if (native->scaling != 2) {
        double scale = native->scaling == 1 ? MAX(scale_x, scale_y) : MIN(scale_x, scale_y);
        scale_x = scale;
        scale_y = scale;
    }
    double draw_width = image_width * scale_x;
    double draw_height = image_height * scale_y;
    cairo_save(context);
    cairo_translate(context, (width - draw_width) / 2, (height - draw_height) / 2);
    cairo_scale(context, scale_x, scale_y);
    gdk_cairo_set_source_pixbuf(context, native->pixbuf, 0, 0);
    cairo_paint(context);
    cairo_restore(context);
}

static void gkGTKFreePixels(guchar *pixels, gpointer data) {
    (void)data;
    g_free(pixels);
}

void *gkGTKCreateLayout(void) {
    return gkGTKWrap(gtk_fixed_new(), GK_GTK_WIDGET_LAYOUT, 0);
}

void *gkGTKCreateLabel(const char *text) {
    GtkWidget *label = gtk_label_new(text ? text : "");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    return gkGTKWrap(label, GK_GTK_WIDGET_LABEL, 0);
}

void *gkGTKCreateButton(const char *text, uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_button_new_with_label(text ? text : ""), GK_GTK_WIDGET_BUTTON, handle);
    if (native) {
        g_signal_connect(native->widget, "clicked", G_CALLBACK(gkGTKSignal), native);
    }
    return native;
}

void *gkGTKCreateTextField(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_entry_new(), GK_GTK_WIDGET_TEXT_FIELD, handle);
    if (native) {
        g_signal_connect(native->widget, "changed", G_CALLBACK(gkGTKSignal), native);
    }
    return native;
}

void *gkGTKCreateTextArea(const char *text, const char *placeholder, int read_only, int wrap, uintptr_t handle) {
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), wrap ? GTK_POLICY_NEVER : GTK_POLICY_AUTOMATIC,
                                   GTK_POLICY_AUTOMATIC);
    GtkWidget *editor = gtk_text_view_new();
    gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(editor), wrap ? GTK_WRAP_WORD_CHAR : GTK_WRAP_NONE);
    gtk_text_view_set_editable(GTK_TEXT_VIEW(editor), read_only == 0);
    GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(editor));
    gtk_text_buffer_set_text(buffer, text ? text : "", -1);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), editor);
    GKGTKWidget *native = gkGTKWrap(scroll, GK_GTK_WIDGET_TEXT_AREA, handle);
    if (!native) {
        return NULL;
    }
    native->control = editor;
    g_object_set_data_full(G_OBJECT(editor), "gamekit-placeholder", g_strdup(placeholder ? placeholder : ""), g_free);
    g_signal_connect(buffer, "changed", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateCheckBox(const char *text, int checked, uintptr_t handle) {
    GtkWidget *button = gtk_check_button_new_with_label(text ? text : "");
    gtk_check_button_set_active(GTK_CHECK_BUTTON(button), checked != 0);
    GKGTKWidget *native = gkGTKWrap(button, GK_GTK_WIDGET_CHECK_BOX, handle);
    if (native) {
        g_signal_connect(button, "toggled", G_CALLBACK(gkGTKSignal), native);
    }
    return native;
}

void *gkGTKCreateRadioGroup(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_box_new(GTK_ORIENTATION_VERTICAL, 2), GK_GTK_WIDGET_RADIO_GROUP, handle);
    if (native) {
        native->items = g_ptr_array_new();
        native->selected = -1;
    }
    return native;
}

void *gkGTKCreateSelect(uintptr_t handle) {
    GtkStringList *model = gtk_string_list_new(NULL);
    GKGTKWidget *native = gkGTKWrap(gtk_drop_down_new(G_LIST_MODEL(model), NULL), GK_GTK_WIDGET_SELECT, handle);
    if (native) {
        g_signal_connect(native->widget, "notify::selected", G_CALLBACK(gkGTKNotifySignal), native);
    }
    return native;
}

void gkGTKSelectionAddItem(void *pointer, const char *text) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        GtkStringList *model = GTK_STRING_LIST(gtk_drop_down_get_model(GTK_DROP_DOWN(native->widget)));
        gtk_string_list_append(model, text ? text : "");
    } else if (native->kind == GK_GTK_WIDGET_RADIO_GROUP) {
        GtkWidget *first = native->items->len ? g_ptr_array_index(native->items, 0) : NULL;
        GtkWidget *button = gtk_check_button_new_with_label(text ? text : "");
        if (first) {
            gtk_check_button_set_group(GTK_CHECK_BUTTON(button), GTK_CHECK_BUTTON(first));
        }
        g_ptr_array_add(native->items, button);
        gtk_box_append(GTK_BOX(native->widget), button);
        g_signal_connect(button, "toggled", G_CALLBACK(gkGTKRadioSignal), native);
    }
}

int gkGTKSelectionIndex(void *pointer) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return -1;
    }
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        guint selected = gtk_drop_down_get_selected(GTK_DROP_DOWN(native->widget));
        return selected == GTK_INVALID_LIST_POSITION ? -1 : (int)selected;
    }
    return native->kind == GK_GTK_WIDGET_RADIO_GROUP ? native->selected : -1;
}

void gkGTKSelectionSetIndex(void *pointer, int index) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    native->suppress = TRUE;
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        gtk_drop_down_set_selected(GTK_DROP_DOWN(native->widget),
                                   index < 0 ? GTK_INVALID_LIST_POSITION : (guint)index);
    } else if (native->kind == GK_GTK_WIDGET_RADIO_GROUP && native->items && index >= 0 &&
               index < (int)native->items->len) {
        native->selected = index;
        gtk_check_button_set_active(GTK_CHECK_BUTTON(g_ptr_array_index(native->items, index)), TRUE);
    }
    native->suppress = FALSE;
}

void *gkGTKCreateSlider(double minimum, double maximum, double value, double step, uintptr_t handle) {
    double increment = step > 0 ? step : (maximum - minimum) / 100.0;
    GtkWidget *scale = gtk_scale_new_with_range(GTK_ORIENTATION_HORIZONTAL, minimum, maximum, increment);
    gtk_range_set_value(GTK_RANGE(scale), value);
    gtk_scale_set_draw_value(GTK_SCALE(scale), FALSE);
    GKGTKWidget *native = gkGTKWrap(scale, GK_GTK_WIDGET_SLIDER, handle);
    if (native) {
        g_signal_connect(native->widget, "value-changed", G_CALLBACK(gkGTKSignal), native);
    }
    return native;
}

void *gkGTKCreateProgressBar(double value) {
    GtkWidget *progress = gtk_progress_bar_new();
    gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(progress), CLAMP(value, 0, 1));
    return gkGTKWrap(progress, GK_GTK_WIDGET_PROGRESS, 0);
}

void *gkGTKCreateActivityIndicator(int running) {
    GtkWidget *spinner = gtk_spinner_new();
    gtk_spinner_set_spinning(GTK_SPINNER(spinner), running != 0);
    GKGTKWidget *native = gkGTKWrap(spinner, GK_GTK_WIDGET_ACTIVITY, 0);
    if (native) {
        native->running = running != 0;
    }
    return native;
}

void *gkGTKCreateSeparator(int direction) {
    return gkGTKWrap(gtk_separator_new(direction == 0 ? GTK_ORIENTATION_HORIZONTAL : GTK_ORIENTATION_VERTICAL),
                     GK_GTK_WIDGET_SEPARATOR, 0);
}

void *gkGTKCreateImage(int scaling) {
    GtkWidget *area = gtk_drawing_area_new();
    GKGTKWidget *native = gkGTKWrap(area, GK_GTK_WIDGET_IMAGE, 0);
    if (native) {
        native->scaling = scaling;
        gtk_drawing_area_set_draw_func(GTK_DRAWING_AREA(area), gkGTKImageDraw, native, NULL);
    }
    return native;
}

void gkGTKImageSet(void *pointer, const unsigned char *pixels, int width, int height, int stride) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_IMAGE || !pixels || width <= 0 || height <= 0 || stride < width * 4) {
        return;
    }
    gsize size = (gsize)stride * height;
    guchar *copy = g_malloc(size);
    memcpy(copy, pixels, size);
    GdkPixbuf *pixbuf =
        gdk_pixbuf_new_from_data(copy, GDK_COLORSPACE_RGB, TRUE, 8, width, height, stride, gkGTKFreePixels, NULL);
    if (!pixbuf) {
        g_free(copy);
        return;
    }
    if (native->pixbuf) {
        g_object_unref(native->pixbuf);
    }
    native->pixbuf = pixbuf;
    gtk_widget_queue_draw(native->widget);
}

void gkGTKImageSetScaling(void *pointer, int scaling) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_IMAGE) {
        native->scaling = scaling;
        gtk_widget_queue_draw(native->widget);
    }
}

void *gkGTKCreateLink(const char *text, const char *url, uintptr_t handle) {
    GtkWidget *link = gtk_link_button_new_with_label(url ? url : "", text ? text : "");
    GKGTKWidget *native = gkGTKWrap(link, GK_GTK_WIDGET_LINK, handle);
    if (native) {
        g_signal_connect(link, "activate-link", G_CALLBACK(gkGTKLinkSignal), native);
    }
    return native;
}

char *gkGTKLinkURL(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *url =
        native && native->kind == GK_GTK_WIDGET_LINK ? gtk_link_button_get_uri(GTK_LINK_BUTTON(native->widget)) : "";
    return strdup(url ? url : "");
}

void gkGTKLinkSetURL(void *pointer, const char *url) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_LINK) {
        gtk_link_button_set_uri(GTK_LINK_BUTTON(native->widget), url ? url : "");
    }
}

void *gkGTKCreateTabs(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_notebook_new(), GK_GTK_WIDGET_TABS, handle);
    if (native) {
        g_signal_connect(native->widget, "switch-page", G_CALLBACK(gkGTKTabSignal), native);
    }
    return native;
}

int gkGTKTabsAdd(void *tabs_pointer, const char *title, void *content_pointer) {
    GKGTKWidget *tabs = tabs_pointer;
    GKGTKWidget *content = content_pointer;
    if (!tabs || tabs->kind != GK_GTK_WIDGET_TABS || !content) {
        return 0;
    }
    gkGTKRemoveFromParent(content->widget);
    GtkWidget *label = gtk_label_new(title ? title : "");
    gtk_notebook_append_page(GTK_NOTEBOOK(tabs->widget), content->widget, label);
    return 1;
}

int gkGTKTabsSelected(void *pointer) {
    GKGTKWidget *tabs = pointer;
    return tabs && tabs->kind == GK_GTK_WIDGET_TABS ? gtk_notebook_get_current_page(GTK_NOTEBOOK(tabs->widget)) : -1;
}

void gkGTKTabsSetSelected(void *pointer, int index) {
    GKGTKWidget *tabs = pointer;
    if (!tabs || tabs->kind != GK_GTK_WIDGET_TABS) {
        return;
    }
    tabs->suppress = TRUE;
    gtk_notebook_set_current_page(GTK_NOTEBOOK(tabs->widget), index);
    tabs->suppress = FALSE;
}

void gkGTKTabsContentSize(void *pointer, int *width, int *height) {
    GKGTKWidget *tabs = pointer;
    int value_width = 0;
    int value_height = 0;
    if (tabs && tabs->kind == GK_GTK_WIDGET_TABS) {
        int index = gtk_notebook_get_current_page(GTK_NOTEBOOK(tabs->widget));
        GtkWidget *page = gtk_notebook_get_nth_page(GTK_NOTEBOOK(tabs->widget), index);
        if (page) {
            value_width = gtk_widget_get_width(page);
            value_height = gtk_widget_get_height(page);
        }
    }
    if (width) {
        *width = value_width;
    }
    if (height) {
        *height = value_height;
    }
}

void *gkGTKCreateScrollView(int axes) {
    GtkWidget *scroll = gtk_scrolled_window_new();
    GtkPolicyType horizontal = axes == 1 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER;
    GtkPolicyType vertical = axes == 0 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER;
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), horizontal, vertical);
    GtkWidget *viewport = gtk_viewport_new(NULL, NULL);
    GtkWidget *content = gtk_fixed_new();
    gtk_viewport_set_child(GTK_VIEWPORT(viewport), content);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), viewport);
    GKGTKWidget *native = gkGTKWrap(scroll, GK_GTK_WIDGET_SCROLL_VIEW, 0);
    if (!native) {
        return NULL;
    }
    native->content = content;
    native->control = viewport;
    return native;
}

void gkGTKScrollViewportSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    int value_width = native && native->control ? gtk_widget_get_width(native->control) : 0;
    int value_height = native && native->control ? gtk_widget_get_height(native->control) : 0;
    if (width) {
        *width = value_width;
    }
    if (height) {
        *height = value_height;
    }
}

void gkGTKScrollSetContentSize(void *pointer, int width, int height) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW && native->content) {
        gtk_widget_set_size_request(native->content, MAX(0, width), MAX(0, height));
    }
}

void gkGTKScrollOffset(void *pointer, int *x, int *y) {
    GKGTKWidget *native = pointer;
    double horizontal = 0;
    double vertical = 0;
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW) {
        horizontal = gtk_adjustment_get_value(gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(native->widget)));
        vertical = gtk_adjustment_get_value(gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(native->widget)));
    }
    if (x) {
        *x = (int)horizontal;
    }
    if (y) {
        *y = (int)vertical;
    }
}

void gkGTKScrollSetOffset(void *pointer, int x, int y) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_SCROLL_VIEW) {
        return;
    }
    gtk_adjustment_set_value(gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(native->widget)), MAX(0, x));
    gtk_adjustment_set_value(gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(native->widget)), MAX(0, y));
}

static void gkGTKCanvasButton(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKGTKWidget *native = data;
    int button = gkGTKPointerButton(gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture)));
    if (!native || button < 0 || button >= GK_GTK_POINTER_BUTTON_COUNT) {
        return;
    }
    native->buttons |= 1u << button;
    if (native->handle) {
        GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
        gkGTKGoCanvasPointerButton(native->handle, button, GK_GTK_POINTER_PRESSED, gkGTKModifiers(state));
    }
}

static void gkGTKCanvasButtonReleased(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKGTKWidget *native = data;
    int button = gkGTKPointerButton(gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture)));
    if (!native || button < 0 || button >= GK_GTK_POINTER_BUTTON_COUNT) {
        return;
    }
    unsigned int mask = 1u << button;
    if (!(native->buttons & mask)) {
        return;
    }
    native->buttons &= ~mask;
    if (native->handle) {
        GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
        gkGTKGoCanvasPointerButton(native->handle, button, GK_GTK_POINTER_RELEASED, gkGTKModifiers(state));
    }
}

static gboolean gkGTKCanvasScroll(GtkEventControllerScroll *controller, double dx, double dy, gpointer data) {
    (void)controller;
    GKGTKWidget *native = data;
    if (native && native->handle) {
        gkGTKGoCanvasScroll(native->handle, dx, -dy);
    }
    return TRUE;
}

static void gkGTKCanvasSyncXWindow(GKGTKWidget *native) {
    if (!native || !native->widget) {
        return;
    }
    GtkNative *gtk_native = gtk_widget_get_native(native->widget);
    GdkSurface *surface = gtk_native ? gtk_native_get_surface(gtk_native) : NULL;
    if (!surface || !GDK_IS_X11_SURFACE(surface)) {
        return;
    }
    GtkWidget *root = GTK_WIDGET(gtk_native);
    graphene_rect_t bounds;
    if (!gtk_widget_compute_bounds(native->widget, root, &bounds)) {
        return;
    }
    int scale = gtk_widget_get_scale_factor(native->widget);
    int x = (int)(bounds.origin.x * scale);
    int y = (int)(bounds.origin.y * scale);
    int width = MAX(1, (int)(bounds.size.width * scale));
    int height = MAX(1, (int)(bounds.size.height * scale));
    Display *display = gdk_x11_display_get_xdisplay(gdk_surface_get_display(surface));
    Window parent = gdk_x11_surface_get_xid(surface);
    if (!native->xwindow) {
        native->xdisplay = display;
        native->xwindow = XCreateSimpleWindow(display, parent, x, y, width, height, 0, 0, 0);
        XSelectInput(display, native->xwindow, 0);
    }
    XMoveResizeWindow(display, native->xwindow, x, y, (unsigned int)width, (unsigned int)height);
    gboolean mapped = gtk_widget_get_mapped(native->widget);
    if (mapped && !native->xwindow_mapped) {
        XMapWindow(display, native->xwindow);
        native->xwindow_mapped = TRUE;
    } else if (!mapped && native->xwindow_mapped) {
        XUnmapWindow(display, native->xwindow);
        native->xwindow_mapped = FALSE;
    }
    XFlush(display);
}

static gboolean gkGTKCanvasTick(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    (void)widget;
    (void)clock;
    gkGTKCanvasSyncXWindow(data);
    return G_SOURCE_CONTINUE;
}

void *gkGTKCreateCanvas(uintptr_t handle) {
    GtkWidget *area = gtk_drawing_area_new();
    GKGTKWidget *native = gkGTKWrap(area, GK_GTK_WIDGET_CANVAS, handle);
    if (!native) {
        return NULL;
    }
    GtkGesture *click = gtk_gesture_click_new();
    gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(click), 0);
    g_signal_connect(click, "pressed", G_CALLBACK(gkGTKCanvasButton), native);
    g_signal_connect(click, "released", G_CALLBACK(gkGTKCanvasButtonReleased), native);
    gtk_widget_add_controller(area, GTK_EVENT_CONTROLLER(click));
    GtkEventController *scroll = gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_BOTH_AXES);
    g_signal_connect(scroll, "scroll", G_CALLBACK(gkGTKCanvasScroll), native);
    gtk_widget_add_controller(area, scroll);
    gtk_widget_add_tick_callback(area, gkGTKCanvasTick, native, NULL);
    return native;
}

void *gkGTKCreateSection(const char *title, int expanded, uintptr_t handle) {
    GtkWidget *expander = gtk_expander_new(title ? title : "");
    GKGTKWidget *native = gkGTKWrap(expander, GK_GTK_WIDGET_SECTION, handle);
    if (!native) {
        return NULL;
    }
    native->content = gtk_fixed_new();
    gtk_expander_set_child(GTK_EXPANDER(native->widget), native->content);
    gtk_expander_set_expanded(GTK_EXPANDER(native->widget), expanded != 0);
    g_signal_connect(native->widget, "notify::expanded", G_CALLBACK(gkGTKNotifySignal), native);
    return native;
}

void gkGTKWidgetDestroy(void *pointer) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    native->handle = 0;
    gkGTKReleaseCanvasButtons(native, 0);
    if (native->xwindow && native->xdisplay) {
        XDestroyWindow(native->xdisplay, native->xwindow);
        XFlush(native->xdisplay);
        native->xwindow = 0;
    }
    gkGTKRemoveFromParent(native->widget);
    g_object_unref(native->widget);
    if (native->items) {
        g_ptr_array_free(native->items, TRUE);
    }
    if (native->pixbuf) {
        g_object_unref(native->pixbuf);
    }
    free(native);
}

void gkGTKWidgetUnparent(void *pointer) {
    GKGTKWidget *native = pointer;
    if (native) {
        gkGTKRemoveFromParent(native->widget);
    }
}

void gkGTKWidgetSetParent(void *widget_pointer, void *parent_pointer) {
    GKGTKWidget *widget = widget_pointer;
    GKGTKWidget *parent = parent_pointer;
    if (!widget || !parent) {
        return;
    }
    gkGTKRemoveFromParent(widget->widget);
    GtkWidget *container = parent->content ? parent->content : parent->widget;
    if (GTK_IS_FIXED(container)) {
        gtk_fixed_put(GTK_FIXED(container), widget->widget, 0, 0);
    } else if (GTK_IS_BOX(container)) {
        gtk_box_append(GTK_BOX(container), widget->widget);
    } else if (GTK_IS_EXPANDER(container)) {
        gtk_expander_set_child(GTK_EXPANDER(container), widget->widget);
    } else if (GTK_IS_SCROLLED_WINDOW(container)) {
        gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(container), widget->widget);
    }
    widget->window = parent->window;
}

void gkGTKWidgetAttachToWindow(void *window, void *widget) {
    gkGTKWindowSetContent(window, widget);
}

void gkGTKWidgetSetFrame(void *pointer, int x, int y, int width, int height) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    GtkWidget *parent = gtk_widget_get_parent(native->widget);
    if (parent && GTK_IS_FIXED(parent)) {
        gtk_fixed_move(GTK_FIXED(parent), native->widget, x, y);
    }
    gtk_widget_set_size_request(native->widget, MAX(0, width), MAX(0, height));
    if (native->kind == GK_GTK_WIDGET_CANVAS) {
        gkGTKCanvasSyncXWindow(native);
    }
}

void gkGTKWidgetSetVisible(void *pointer, int visible) {
    GKGTKWidget *native = pointer;
    if (native) {
        gtk_widget_set_visible(native->widget, visible != 0);
        if (native->kind == GK_GTK_WIDGET_CANVAS) {
            gkGTKCanvasSyncXWindow(native);
        }
    }
}

void gkGTKWidgetSetEnabled(void *pointer, int enabled) {
    GKGTKWidget *native = pointer;
    if (native) {
        gtk_widget_set_sensitive(native->widget, enabled != 0);
    }
}

void gkGTKWidgetPreferredSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    int minimum_width = 0;
    int natural_width = 0;
    int minimum_height = 0;
    int natural_height = 0;
    if (native) {
        gtk_widget_measure(native->widget, GTK_ORIENTATION_HORIZONTAL, -1, &minimum_width, &natural_width, NULL, NULL);
        gtk_widget_measure(native->widget, GTK_ORIENTATION_VERTICAL, natural_width, &minimum_height, &natural_height,
                           NULL, NULL);
    }
    if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        natural_width = MAX(natural_width, 200);
        natural_height = MAX(natural_height, 96);
    }
    if (native && native->kind == GK_GTK_WIDGET_IMAGE && native->pixbuf) {
        natural_width = gdk_pixbuf_get_width(native->pixbuf);
        natural_height = gdk_pixbuf_get_height(native->pixbuf);
    }
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW) {
        natural_width = 320;
        natural_height = 240;
    }
    if (width) {
        *width = natural_width;
    }
    if (height) {
        *height = natural_height;
    }
}

char *gkGTKWidgetText(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *text = "";
    if (!native) {
        return strdup(text);
    }
    if (native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(native->control));
        GtkTextIter start, end;
        gtk_text_buffer_get_bounds(buffer, &start, &end);
        gchar *value = gtk_text_buffer_get_text(buffer, &start, &end, FALSE);
        char *result = strdup(value ? value : "");
        g_free(value);
        return result;
    }
    switch (native->kind) {
    case GK_GTK_WIDGET_LABEL:
        text = gtk_label_get_text(GTK_LABEL(native->widget));
        break;
    case GK_GTK_WIDGET_BUTTON:
    case GK_GTK_WIDGET_LINK:
        text = gtk_button_get_label(GTK_BUTTON(native->widget));
        break;
    case GK_GTK_WIDGET_TEXT_FIELD:
        text = gtk_editable_get_text(GTK_EDITABLE(native->widget));
        break;
    case GK_GTK_WIDGET_CHECK_BOX:
        text = gtk_check_button_get_label(GTK_CHECK_BUTTON(native->widget));
        break;
    case GK_GTK_WIDGET_SECTION:
        text = gtk_expander_get_label(GTK_EXPANDER(native->widget));
        break;
    }
    return strdup(text ? text : "");
}

void gkGTKWidgetSetText(void *pointer, const char *text) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    native->suppress = TRUE;
    switch (native->kind) {
    case GK_GTK_WIDGET_LABEL:
        gtk_label_set_text(GTK_LABEL(native->widget), text ? text : "");
        break;
    case GK_GTK_WIDGET_BUTTON:
    case GK_GTK_WIDGET_LINK:
        gtk_button_set_label(GTK_BUTTON(native->widget), text ? text : "");
        break;
    case GK_GTK_WIDGET_TEXT_FIELD:
        gtk_editable_set_text(GTK_EDITABLE(native->widget), text ? text : "");
        break;
    case GK_GTK_WIDGET_TEXT_AREA: {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(native->control));
        gtk_text_buffer_set_text(buffer, text ? text : "", -1);
        break;
    }
    case GK_GTK_WIDGET_CHECK_BOX:
        gtk_check_button_set_label(GTK_CHECK_BUTTON(native->widget), text ? text : "");
        break;
    case GK_GTK_WIDGET_SECTION:
        gtk_expander_set_label(GTK_EXPANDER(native->widget), text ? text : "");
        break;
    }
    native->suppress = FALSE;
}

char *gkGTKTextFieldPlaceholder(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *text = NULL;
    if (native && native->kind == GK_GTK_WIDGET_TEXT_FIELD) {
        text = gtk_entry_get_placeholder_text(GTK_ENTRY(native->widget));
    } else if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        text = g_object_get_data(G_OBJECT(native->control), "gamekit-placeholder");
    }
    return strdup(text ? text : "");
}

void gkGTKTextFieldSetPlaceholder(void *pointer, const char *placeholder) {
    GKGTKWidget *native = pointer;
    if (!native) {
        return;
    }
    if (native->kind == GK_GTK_WIDGET_TEXT_FIELD) {
        gtk_entry_set_placeholder_text(GTK_ENTRY(native->widget), placeholder ? placeholder : "");
    } else if (native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        g_object_set_data_full(G_OBJECT(native->control), "gamekit-placeholder",
                               g_strdup(placeholder ? placeholder : ""), g_free);
    }
}

int gkGTKWidgetChecked(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_CHECK_BOX &&
           gtk_check_button_get_active(GTK_CHECK_BUTTON(native->widget));
}

void gkGTKWidgetSetChecked(void *pointer, int checked) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_CHECK_BOX) {
        return;
    }
    native->suppress = TRUE;
    gtk_check_button_set_active(GTK_CHECK_BUTTON(native->widget), checked != 0);
    native->suppress = FALSE;
}

int gkGTKTextAreaReadOnly(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_TEXT_AREA &&
           !gtk_text_view_get_editable(GTK_TEXT_VIEW(native->control));
}

void gkGTKTextAreaSetReadOnly(void *pointer, int read_only) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        gtk_text_view_set_editable(GTK_TEXT_VIEW(native->control), read_only == 0);
    }
}

double gkGTKSliderValue(void *pointer) {
    GKGTKWidget *native = pointer;
    return native ? gtk_range_get_value(GTK_RANGE(native->widget)) : 0;
}

void gkGTKSliderSetValue(void *pointer, double value) {
    GKGTKWidget *native = pointer;
    if (native) {
        native->suppress = TRUE;
        gtk_range_set_value(GTK_RANGE(native->widget), value);
        native->suppress = FALSE;
    }
}

double gkGTKProgressValue(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_PROGRESS
               ? gtk_progress_bar_get_fraction(GTK_PROGRESS_BAR(native->widget))
               : 0;
}

void gkGTKProgressSetValue(void *pointer, double value) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_PROGRESS) {
        gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(native->widget), CLAMP(value, 0, 1));
    }
}

int gkGTKActivityRunning(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_ACTIVITY ? native->running : 0;
}

void gkGTKActivitySetRunning(void *pointer, int running) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_ACTIVITY) {
        native->running = running != 0;
        gtk_spinner_set_spinning(GTK_SPINNER(native->widget), native->running);
    }
}

uintptr_t gkGTKCanvasXID(void *pointer) {
    GKGTKWidget *native = pointer;
    gkGTKCanvasSyncXWindow(native);
    return native ? (uintptr_t)native->xwindow : 0;
}

void gkGTKCanvasFramebufferSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    int scale = native ? gtk_widget_get_scale_factor(native->widget) : 1;
    if (width) {
        *width = native ? gtk_widget_get_width(native->widget) * scale : 0;
    }
    if (height) {
        *height = native ? gtk_widget_get_height(native->widget) * scale : 0;
    }
}

void gkGTKCanvasQueueDraw(void *pointer) {
    GKGTKWidget *native = pointer;
    if (native) {
        gtk_widget_queue_draw(native->widget);
    }
}

void gkGTKCanvasPointerPosition(void *pointer, double *x, double *y) {
    GKGTKWidget *native = pointer;
    gkGTKPointerPosition(native ? native->widget : NULL, x, y);
}

int gkGTKSectionExpanded(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && gtk_expander_get_expanded(GTK_EXPANDER(native->widget));
}

void gkGTKSectionSetExpanded(void *pointer, int expanded) {
    GKGTKWidget *native = pointer;
    if (native) {
        native->suppress = TRUE;
        gtk_expander_set_expanded(GTK_EXPANDER(native->widget), expanded != 0);
        native->suppress = FALSE;
    }
}

static void gkGTKConfigureFileChooser(GtkFileChooser *chooser, const char *directory, const char *filename,
                                      const char *extensions, gboolean saving) {
    GFile *folder = directory && directory[0] ? g_file_new_for_path(directory) : NULL;
    if (folder) {
        gtk_file_chooser_set_current_folder(chooser, folder, NULL);
    }
    if (filename && filename[0]) {
        if (saving) {
            gtk_file_chooser_set_current_name(chooser, filename);
        } else {
            GFile *file = g_path_is_absolute(filename) ? g_file_new_for_path(filename)
                                                       : folder ? g_file_get_child(folder, filename) : NULL;
            if (file) {
                gtk_file_chooser_set_file(chooser, file, NULL);
                g_object_unref(file);
            }
        }
    }
    if (folder) {
        g_object_unref(folder);
    }
    if (!extensions || !extensions[0]) {
        return;
    }
    GtkFileFilter *supported = gtk_file_filter_new();
    gtk_file_filter_set_name(supported, "Supported files");
    gchar **values = g_strsplit(extensions, ",", -1);
    for (gchar **next = values; next && *next; next++) {
        if ((*next)[0]) {
            gchar *pattern = g_strdup_printf("*.%s", *next);
            gtk_file_filter_add_pattern(supported, pattern);
            g_free(pattern);
        }
    }
    g_strfreev(values);
    gtk_file_chooser_add_filter(chooser, supported);
    GtkFileFilter *all = gtk_file_filter_new();
    gtk_file_filter_set_name(all, "All files");
    gtk_file_filter_add_pattern(all, "*");
    gtk_file_chooser_add_filter(chooser, all);
}

typedef struct {
    GMainLoop *loop;
    int response;
} GKGTKDialogState;

static void gkGTKDialogResponse(GtkNativeDialog *dialog, int response, gpointer data) {
    (void)dialog;
    GKGTKDialogState *state = data;
    state->response = response;
    g_main_loop_quit(state->loop);
}

static int gkGTKRunDialog(GtkNativeDialog *dialog) {
    GKGTKDialogState state = {g_main_loop_new(NULL, FALSE), GTK_RESPONSE_CANCEL};
    g_signal_connect(dialog, "response", G_CALLBACK(gkGTKDialogResponse), &state);
    gtk_native_dialog_show(dialog);
    g_main_loop_run(state.loop);
    g_main_loop_unref(state.loop);
    return state.response;
}

static char *gkGTKPackFiles(GListModel *files, size_t *result_size) {
    size_t size = 0;
    guint count = files ? g_list_model_get_n_items(files) : 0;
    GPtrArray *paths = g_ptr_array_new_with_free_func(g_free);
    for (guint i = 0; i < count; i++) {
        GFile *file = g_list_model_get_item(files, i);
        char *path = file ? g_file_get_path(file) : NULL;
        if (file) {
            g_object_unref(file);
        }
        if (path) {
            size += strlen(path) + 1;
            g_ptr_array_add(paths, path);
        }
    }
    if (result_size) {
        *result_size = size;
    }
    if (!size) {
        g_ptr_array_free(paths, TRUE);
        return NULL;
    }
    char *result = malloc(size);
    char *next = result;
    for (guint i = 0; i < paths->len; i++) {
        const char *path = g_ptr_array_index(paths, i);
        size_t length = strlen(path) + 1;
        memcpy(next, path, length);
        next += length;
    }
    g_ptr_array_free(paths, TRUE);
    return result;
}

char *gkGTKOpenDialog(void *window_pointer, const char *title, const char *directory, const char *filename,
                      const char *extensions, int multiple, int directories, size_t *result_size, int *cancelled) {
    GKGTKWindow *window = window_pointer;
    if (result_size) {
        *result_size = 0;
    }
    if (cancelled) {
        *cancelled = 1;
    }
    if (!window) {
        return NULL;
    }
    GtkFileChooserAction action = directories ? GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER : GTK_FILE_CHOOSER_ACTION_OPEN;
    GtkFileChooserNative *dialog = gtk_file_chooser_native_new(
        title && title[0] ? title : directories ? "Select Folder" : "Open File", GTK_WINDOW(window->window), action,
        "_Open", "_Cancel");
    GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
    gtk_file_chooser_set_select_multiple(chooser, multiple != 0);
    gkGTKConfigureFileChooser(chooser, directory, filename, extensions, FALSE);
    char *result = NULL;
    if (gkGTKRunDialog(GTK_NATIVE_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
        GListModel *files = gtk_file_chooser_get_files(chooser);
        result = gkGTKPackFiles(files, result_size);
        g_object_unref(files);
        if (cancelled) {
            *cancelled = result == NULL;
        }
    }
    gtk_native_dialog_destroy(GTK_NATIVE_DIALOG(dialog));
    g_object_unref(dialog);
    return result;
}

char *gkGTKSaveDialog(void *window_pointer, const char *title, const char *directory, const char *filename,
                      const char *extensions, int confirm_overwrite, size_t *result_size, int *cancelled) {
    (void)confirm_overwrite;
    GKGTKWindow *window = window_pointer;
    if (result_size) {
        *result_size = 0;
    }
    if (cancelled) {
        *cancelled = 1;
    }
    if (!window) {
        return NULL;
    }
    GtkFileChooserNative *dialog = gtk_file_chooser_native_new(title && title[0] ? title : "Save File",
                                                               GTK_WINDOW(window->window), GTK_FILE_CHOOSER_ACTION_SAVE,
                                                               "_Save", "_Cancel");
    GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
    gkGTKConfigureFileChooser(chooser, directory, filename, extensions, TRUE);
    char *result = NULL;
    if (gkGTKRunDialog(GTK_NATIVE_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
        GFile *file = gtk_file_chooser_get_file(chooser);
        GListStore *files = g_list_store_new(G_TYPE_FILE);
        if (file) {
            g_list_store_append(files, file);
            g_object_unref(file);
        }
        result = gkGTKPackFiles(G_LIST_MODEL(files), result_size);
        g_object_unref(files);
        if (cancelled) {
            *cancelled = result == NULL;
        }
    }
    gtk_native_dialog_destroy(GTK_NATIVE_DIALOG(dialog));
    g_object_unref(dialog);
    return result;
}
