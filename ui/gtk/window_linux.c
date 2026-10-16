//go:build linux && cgo

#include "gtk_internal.h"

#include <locale.h>
#include <math.h>
#include <stdio.h>

static gboolean gkInitialized;
static int gkSystemDark;
static GList *gkWindows;
static uintptr_t gkAppHandle;
static int gkAppActive = -1;
static GtkWidget *gkCaptureView;

static void gkSetError(char *error, size_t size, const char *message) {
    if (error && size) {
        snprintf(error, size, "%s", message);
    }
}

/* ---- Lookup ---- */

GKWin *gkWinFromWidget(GtkWidget *widget) {
    GtkRoot *root = widget ? gtk_widget_get_root(widget) : NULL;
    if (!root || !GTK_IS_WINDOW(root)) {
        return NULL;
    }
    return g_object_get_data(G_OBJECT(root), "gk-win");
}

GKWin *gkWinFromHandle(uintptr_t window) {
    return (GKWin *)window;
}

GdkSurface *gkWinSurface(GKWin *win) {
    if (!win || !win->window) {
        return NULL;
    }
    gtk_widget_realize(win->window);
    GtkNative *native = gtk_widget_get_native(win->window);
    return native ? gtk_native_get_surface(native) : NULL;
}

Display *gkWinXDisplay(GKWin *win) {
    GdkSurface *surface = gkWinSurface(win);
    if (!surface || !GDK_IS_X11_SURFACE(surface)) {
        return NULL;
    }
    return gdk_x11_display_get_xdisplay(gdk_surface_get_display(surface));
}

Window gkWinXWindow(GKWin *win) {
    GdkSurface *surface = gkWinSurface(win);
    return surface && GDK_IS_X11_SURFACE(surface) ? gdk_x11_surface_get_xid(surface) : 0;
}

void gkWinNotify(GKWin *win, int kind, int a, double x, double y, const char *text) {
    if (win && win->handle && !win->destroyed) {
        gkSend(win->handle, kind, a, 0, x, y, text);
    }
}

typedef struct {
    gboolean (*done)(gpointer data);
    gpointer data;
} GKPump;

void gkWinPump(gboolean (*done)(gpointer data), gpointer data, double timeout) {
    gint64 deadline = g_get_monotonic_time() + (gint64)(timeout * G_USEC_PER_SEC);
    while (!done(data) && g_get_monotonic_time() < deadline) {
        if (!g_main_context_iteration(NULL, FALSE)) {
            g_usleep(2000);
        }
    }
}

/* ---- X11 helpers ---- */

static void gkWinFrameExtents(GKWin *win, long *left, long *right, long *top, long *bottom) {
    *left = *right = *top = *bottom = 0;
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window || win->borderless) {
        return;
    }
    Atom property = XInternAtom(display, "_NET_FRAME_EXTENTS", True);
    if (property == None) {
        return;
    }
    Atom type;
    int format;
    unsigned long count, remaining;
    unsigned char *data = NULL;
    if (XGetWindowProperty(display, window, property, 0, 4, False, XA_CARDINAL, &type, &format, &count, &remaining,
                           &data) == Success &&
        data && count == 4 && format == 32) {
        long *values = (long *)data;
        *left = values[0];
        *right = values[1];
        *top = values[2];
        *bottom = values[3];
    }
    if (data) {
        XFree(data);
    }
}

/* gkWinContentOrigin returns the root coordinates of the content area. */
void gkWinContentOrigin(GKWin *win, int *x, int *y) {
    int origin_x = 0, origin_y = 0;
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (display && window) {
        Window child = 0;
        XTranslateCoordinates(display, window, DefaultRootWindow(display), 0, 0, &origin_x, &origin_y, &child);
    }
    if (win && win->root && gtk_widget_get_mapped(win->root)) {
        graphene_point_t source = GRAPHENE_POINT_INIT(0, 0), destination;
        if (gtk_widget_compute_point(win->root, win->window, &source, &destination)) {
            origin_x += (int)destination.x;
            origin_y += (int)destination.y;
        }
    }
    *x = origin_x;
    *y = origin_y;
}

/* gkWinFrameOrigin returns the root coordinates of the top-left corner of
   the frame. */
static void gkWinFrameOrigin(GKWin *win, int *x, int *y) {
    int origin_x = 0, origin_y = 0;
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (display && window) {
        Window child = 0;
        XTranslateCoordinates(display, window, DefaultRootWindow(display), 0, 0, &origin_x, &origin_y, &child);
    }
    long left, right, top, bottom;
    gkWinFrameExtents(win, &left, &right, &top, &bottom);
    *x = origin_x - (int)left;
    *y = origin_y - (int)top;
}

static void gkWinPositionValues(GKWin *win, int *x, int *y) {
    if (!win) {
        *x = *y = 0;
        return;
    }
    if (!gtk_widget_get_mapped(win->window)) {
        *x = win->x;
        *y = win->y;
        return;
    }
    int frame_x, frame_y;
    gkWinFrameOrigin(win, &frame_x, &frame_y);
    int owner_x = 0, owner_y = 0;
    if (win->owner) {
        gkWinContentOrigin(win->owner, &owner_x, &owner_y);
    }
    *x = frame_x - owner_x;
    *y = frame_y - owner_y;
}

static void gkWinSendState(GKWin *win, int add, const char *name1, const char *name2) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window) {
        return;
    }
    Atom state = XInternAtom(display, "_NET_WM_STATE", False);
    Atom first = XInternAtom(display, name1, False);
    Atom second = name2 ? XInternAtom(display, name2, False) : 0;
    if (!gtk_widget_get_mapped(win->window)) {
        /* Before mapping, the window manager reads the property. */
        Atom values[8];
        int count = 0;
        Atom type;
        int format;
        unsigned long items, remaining;
        unsigned char *data = NULL;
        if (XGetWindowProperty(display, window, state, 0, 8, False, XA_ATOM, &type, &format, &items, &remaining,
                               &data) == Success &&
            data) {
            for (unsigned long i = 0; i < items && count < 6; i++) {
                Atom value = ((Atom *)data)[i];
                if (value != first && value != second) {
                    values[count++] = value;
                }
            }
            XFree(data);
        }
        if (add) {
            values[count++] = first;
            if (second) {
                values[count++] = second;
            }
        }
        XChangeProperty(display, window, state, XA_ATOM, 32, PropModeReplace, (unsigned char *)values, count);
        XFlush(display);
        return;
    }
    XEvent event = {0};
    event.xclient.type = ClientMessage;
    event.xclient.window = window;
    event.xclient.message_type = state;
    event.xclient.format = 32;
    event.xclient.data.l[0] = add ? 1 : 0;
    event.xclient.data.l[1] = (long)first;
    event.xclient.data.l[2] = (long)second;
    event.xclient.data.l[3] = 1;
    XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
    XFlush(display);
}

static void gkWinApplyHints(GKWin *win) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window) {
        return;
    }
    XSizeHints *hints = XAllocSizeHints();
    long supplied = 0;
    XGetWMNormalHints(display, window, hints, &supplied);
    hints->flags &= ~(PMaxSize);
    if (win->fullscreen || win->fullscreen_target) {
        /* A maximum size would stop the window from filling the screen. */
    } else if (!win->resizable) {
        int width = win->reported_width > 0 ? win->reported_width : win->width;
        int height = win->reported_height > 0 ? win->reported_height : win->height;
        hints->flags |= PMinSize | PMaxSize;
        hints->min_width = hints->max_width = width;
        hints->min_height = hints->max_height = height;
    } else if (win->max_width > 0 || win->max_height > 0) {
        hints->flags |= PMaxSize;
        hints->max_width = win->max_width > 0 ? win->max_width : 32767;
        hints->max_height = win->max_height > 0 ? win->max_height : 32767;
    }
    if (win->have_position) {
        hints->flags |= USPosition;
        hints->x = win->x;
        hints->y = win->y;
    }
    XSetWMNormalHints(display, window, hints);
    XFree(hints);
    XFlush(display);
}

static void gkWinMoveFrame(GKWin *win, int x, int y) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window) {
        return;
    }
    int owner_x = 0, owner_y = 0;
    if (win->owner) {
        gkWinContentOrigin(win->owner, &owner_x, &owner_y);
    }
    /* With the default north-west gravity, the window manager puts the top-left
       corner of the frame at the requested position. */
    XMoveWindow(display, window, owner_x + x, owner_y + y);
    XFlush(display);
}

static void gkWinSetType(GKWin *win) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window || !win->utility) {
        return;
    }
    Atom property = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
    Atom utility = XInternAtom(display, "_NET_WM_WINDOW_TYPE_UTILITY", False);
    XChangeProperty(display, window, property, XA_ATOM, 32, PropModeReplace, (unsigned char *)&utility, 1);
    gkWinSendState(win, 1, "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_STATE_SKIP_PAGER");
}

/* ---- Theme ---- */

static int gkSettingsDark(void) {
    GSettingsSchemaSource *source = g_settings_schema_source_get_default();
    GSettingsSchema *schema =
        source ? g_settings_schema_source_lookup(source, "org.gnome.desktop.interface", TRUE) : NULL;
    if (schema) {
        int dark = gkSystemDark;
        if (g_settings_schema_has_key(schema, "color-scheme")) {
            GSettings *settings = g_settings_new("org.gnome.desktop.interface");
            gchar *scheme = g_settings_get_string(settings, "color-scheme");
            dark = scheme && strcmp(scheme, "prefer-dark") == 0;
            g_free(scheme);
            g_object_unref(settings);
        }
        g_settings_schema_unref(schema);
        return dark;
    }
    return gkSystemDark;
}

static int gkRequestedTheme(void) {
    int theme = 0;
    for (GList *item = gkWindows; item; item = item->next) {
        GKWin *win = item->data;
        if (win->theme) {
            theme = win->theme;
        }
    }
    return theme;
}

int gkResolvedTheme(void) {
    int theme = gkRequestedTheme();
    if (theme) {
        return theme;
    }
    return gkSettingsDark() ? 2 : 1;
}

static void gkApplyTheme(void) {
    GtkSettings *settings = gtk_settings_get_default();
    if (!settings) {
        return;
    }
    int theme = gkResolvedTheme();
    int dark = theme == 2;
    g_object_set(settings, "gtk-application-prefer-dark-theme", dark, NULL);
    if (g_object_class_find_property(G_OBJECT_GET_CLASS(settings), "gtk-interface-color-scheme")) {
        /* GTK_INTERFACE_COLOR_SCHEME_DEFAULT is 1, DARK is 2, LIGHT is 3. */
        int requested = gkRequestedTheme();
        int scheme = requested == 2 ? 2 : requested == 1 ? 3 : 1;
        g_object_set(settings, "gtk-interface-color-scheme", scheme, NULL);
    }
}

static void gkNotifyThemes(void) {
    for (GList *item = gkWindows; item; item = item->next) {
        GKWin *win = item->data;
        int resolved = win->theme ? win->theme : gkResolvedTheme();
        if (resolved != win->resolved_theme) {
            win->resolved_theme = resolved;
            gkWinNotify(win, GK_EVENT_WINDOW_THEME, resolved, 0, 0, NULL);
        }
    }
}

static void gkSystemSettingChanged(GSettings *settings, gchar *key, gpointer data) {
    (void)settings;
    (void)data;
    if (strcmp(key, "color-scheme") == 0) {
        gkApplyTheme();
        gkNotifyThemes();
    }
    gkSend(gkAppHandle, GK_EVENT_PREFERENCES, 0, 0, 0, 0, NULL);
}

static void gkWatchSystemSettings(void) {
    GSettingsSchemaSource *source = g_settings_schema_source_get_default();
    GSettingsSchema *schema =
        source ? g_settings_schema_source_lookup(source, "org.gnome.desktop.interface", TRUE) : NULL;
    if (!schema) {
        return;
    }
    g_settings_schema_unref(schema);
    static GSettings *settings;
    settings = g_settings_new("org.gnome.desktop.interface");
    g_signal_connect(settings, "changed", G_CALLBACK(gkSystemSettingChanged), NULL);
}

/* ---- Input ---- */

static void gkWinPointToRoot(GKWin *win, GdkEvent *event, double *x, double *y) {
    double surface_x = 0, surface_y = 0;
    gdk_event_get_position(event, &surface_x, &surface_y);
    double offset_x = 0, offset_y = 0;
    GtkNative *native = gtk_widget_get_native(win->window);
    if (native) {
        gtk_native_get_surface_transform(native, &offset_x, &offset_y);
    }
    graphene_point_t source = GRAPHENE_POINT_INIT((float)(surface_x - offset_x), (float)(surface_y - offset_y));
    graphene_point_t destination = source;
    if (win->root) {
        (void)!gtk_widget_compute_point(win->window, win->root, &source, &destination);
    }
    *x = destination.x;
    *y = destination.y;
}

static int gkFocusIsEditable(GKWin *win) {
    GtkWidget *focus = gtk_root_get_focus(GTK_ROOT(win->window));
    for (GtkWidget *widget = focus; widget; widget = gtk_widget_get_parent(widget)) {
        if (GTK_IS_EDITABLE(widget) || GTK_IS_TEXT_VIEW(widget)) {
            return 1;
        }
    }
    return 0;
}

static void gkEmitText(GKWin *win, const char *text) {
    if (!win->handle || !text) {
        return;
    }
    for (const char *p = text; *p; p = g_utf8_next_char(p)) {
        gunichar c = g_utf8_get_char(p);
        if (c < 0x20 || c == 0x7f) {
            continue;
        }
        gkGTKGoCharEvent(win->handle, c);
    }
}

static int gkUTF16Length(const char *text, int characters) {
    int units = 0;
    int index = 0;
    for (const char *p = text; p && *p && index < characters; p = g_utf8_next_char(p), index++) {
        units += g_utf8_get_char(p) >= 0x10000 ? 2 : 1;
    }
    return units;
}

static void gkWinIME(GKWin *win, int kind, const char *text, int start, int length) {
    if (!win->handle) {
        return;
    }
    GKGTKEvent event = {0};
    event.kind = GK_EVENT_IME;
    event.a = kind;
    event.b = start;
    event.c = length;
    event.text = text ? text : "";
    gkGTKGoEvent(win->handle, &event);
}

static void gkIMCommit(GtkIMContext *context, const char *text, gpointer data) {
    (void)context;
    GKWin *win = data;
    if (win->composing) {
        win->composing = 0;
        int length = gkUTF16Length(text, (int)g_utf8_strlen(text, -1));
        gkWinIME(win, 2, text, length, 0);
    }
    gkEmitText(win, text);
}

static void gkIMPreeditStart(GtkIMContext *context, gpointer data) {
    (void)context;
    GKWin *win = data;
    if (!win->composing) {
        win->composing = 1;
        gkWinIME(win, 0, "", 0, 0);
    }
}

static void gkIMPreeditChanged(GtkIMContext *context, gpointer data) {
    GKWin *win = data;
    char *text = NULL;
    int cursor = 0;
    gtk_im_context_get_preedit_string(context, &text, NULL, &cursor);
    if (text && *text) {
        if (!win->composing) {
            win->composing = 1;
            gkWinIME(win, 0, "", 0, 0);
        }
        gkWinIME(win, 1, text, gkUTF16Length(text, cursor), 0);
    }
    g_free(text);
}

static void gkIMPreeditEnd(GtkIMContext *context, gpointer data) {
    (void)context;
    GKWin *win = data;
    if (win->composing) {
        win->composing = 0;
        gkWinIME(win, 3, "", 0, 0);
    }
}

static void gkWinCenter(GKWin *win, int *x, int *y) {
    int width = win->root ? gtk_widget_get_width(win->root) : win->width;
    int height = win->root ? gtk_widget_get_height(win->root) : win->height;
    *x = width / 2;
    *y = height / 2;
}

static void gkWinWarpToCenter(GKWin *win) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window) {
        return;
    }
    int x, y;
    gkWinCenter(win, &x, &y);
    int scale = gtk_widget_get_scale_factor(win->window);
    graphene_point_t source = GRAPHENE_POINT_INIT((float)x, (float)y), destination = source;
    if (win->root) {
        (void)!gtk_widget_compute_point(win->root, win->window, &source, &destination);
    }
    XWarpPointer(display, None, window, 0, 0, 0, 0, (int)(destination.x * scale), (int)(destination.y * scale));
    XFlush(display);
    win->warping = 1;
}

static void gkSendTouch(GKWin *win, GdkEvent *event, int phase) {
    double x, y;
    gkWinPointToRoot(win, event, &x, &y);
    GKGTKEvent report = {0};
    report.kind = GK_EVENT_TOUCH;
    report.a = (int)(GPOINTER_TO_UINT(gdk_event_get_event_sequence(event)) & 0x7fffffff);
    report.b = phase;
    report.x = x;
    report.y = y;
    report.z = 1;
    gkGTKGoEvent(win->handle, &report);
}

static void gkSendPen(GKWin *win, GdkEvent *event, int phase) {
    GdkDevice *device = gdk_event_get_device(event);
    double x, y;
    gkWinPointToRoot(win, event, &x, &y);
    double pressure = 0, tilt_x = 0, tilt_y = 0, rotation = 0;
    gdk_event_get_axis(event, GDK_AXIS_PRESSURE, &pressure);
    gdk_event_get_axis(event, GDK_AXIS_XTILT, &tilt_x);
    gdk_event_get_axis(event, GDK_AXIS_YTILT, &tilt_y);
    gdk_event_get_axis(event, GDK_AXIS_ROTATION, &rotation);
    GdkModifierType state = gdk_event_get_modifier_state(event);
    GKGTKEvent report = {0};
    report.kind = GK_EVENT_PEN;
    report.a = phase;
    report.b = (int)(GPOINTER_TO_UINT(device) & 0x7fffffff);
    GdkDeviceTool *tool = gdk_event_get_device_tool(event);
    int eraser = tool && gdk_device_tool_get_tool_type(tool) == GDK_DEVICE_TOOL_TYPE_ERASER;
    report.c = (eraser ? 1 : 0) | ((state & GDK_BUTTON2_MASK) ? 2 : 0);
    report.x = x;
    report.y = y;
    report.z = pressure;
    report.w = rotation * 2 * G_PI;
    char tilt[64];
    snprintf(tilt, sizeof(tilt), "%f %f", tilt_x * G_PI / 2, tilt_y * G_PI / 2);
    report.text = tilt;
    gkGTKGoEvent(win->handle, &report);
}

static gboolean gkIsPen(GdkEvent *event) {
    GdkDevice *device = gdk_event_get_device(event);
    if (!device) {
        return FALSE;
    }
    return gdk_device_get_source(device) == GDK_SOURCE_PEN;
}

/* gkCaptureDeliver routes a pointer event to the canvas that captured the
   pointer. It returns TRUE when the event is consumed. */
gboolean gkGTKCaptureDeliver(GtkWidget *canvas, GKWin *win, GdkEvent *event);

static double gkLastPinchScale;

static gboolean gkWinEvent(GtkEventControllerLegacy *controller, GdkEvent *event, gpointer data) {
    (void)controller;
    GKWin *win = data;
    if (!win || win->destroyed) {
        return FALSE;
    }
    GdkEventType type = gdk_event_get_event_type(event);
    GdkModifierType state = gdk_event_get_modifier_state(event);
    int mods = gkModifiers(state);
    switch (type) {
    case GDK_KEY_PRESS:
    case GDK_KEY_RELEASE: {
        guint keycode = gdk_key_event_get_keycode(event);
        int key = gkKeyFromKeycode(keycode);
        int pressed = type == GDK_KEY_PRESS;
        if (key >= 0 && win->handle) {
            gkGTKGoKeyEvent(win->handle, key, (int)keycode, pressed ? GK_KEY_PRESSED : GK_KEY_RELEASED, mods);
        }
        if (gkFocusIsEditable(win)) {
            if (pressed && !(state & (GDK_CONTROL_MASK | GDK_ALT_MASK | GDK_SUPER_MASK))) {
                gunichar c = gdk_keyval_to_unicode(gdk_key_event_get_keyval(event));
                char text[8] = {0};
                if (c) {
                    g_unichar_to_utf8(c, text);
                    gkEmitText(win, text);
                }
            }
            return FALSE;
        }
        if (win->im && gtk_im_context_filter_keypress(win->im, event)) {
            return FALSE;
        }
        if (pressed && !(state & (GDK_CONTROL_MASK | GDK_ALT_MASK | GDK_SUPER_MASK))) {
            gunichar c = gdk_keyval_to_unicode(gdk_key_event_get_keyval(event));
            char text[8] = {0};
            if (c) {
                g_unichar_to_utf8(c, text);
                gkEmitText(win, text);
            }
        }
        return FALSE;
    }
    case GDK_BUTTON_PRESS:
    case GDK_BUTTON_RELEASE: {
        int button = gkPointerButton(gdk_button_event_get_button(event));
        int pressed = type == GDK_BUTTON_PRESS;
        if (button >= 0 && button < GK_POINTER_BUTTON_COUNT && win->handle) {
            unsigned int mask = 1u << button;
            if (pressed) {
                win->buttons |= mask;
            } else {
                win->buttons &= ~mask;
            }
            gkGTKGoPointerButtonEvent(win->handle, button, pressed ? GK_POINTER_PRESSED : GK_POINTER_RELEASED, mods);
        }
        if (gkIsPen(event) && button == GK_POINTER_BUTTON_LEFT && win->handle) {
            gkSendPen(win, event, pressed ? 0 : 2);
        }
        break;
    }
    case GDK_MOTION_NOTIFY: {
        double x, y;
        gkWinPointToRoot(win, event, &x, &y);
        double dx = win->have_last ? x - win->last_x : 0;
        double dy = win->have_last ? y - win->last_y : 0;
        if (win->cursor_mode == GK_CURSOR_DISABLED) {
            int cx, cy;
            gkWinCenter(win, &cx, &cy);
            if (win->warping && fabs(x - cx) < 1 && fabs(y - cy) < 1) {
                win->warping = 0;
                win->last_x = x;
                win->last_y = y;
                win->have_last = 1;
                break;
            }
            dx = x - cx;
            dy = y - cy;
            win->virtual_x += dx;
            win->virtual_y += dy;
            gkWinWarpToCenter(win);
            x = win->virtual_x;
            y = win->virtual_y;
        } else {
            win->last_x = x;
            win->last_y = y;
            win->have_last = 1;
        }
        if (win->handle) {
            GKGTKEvent report = {0};
            report.kind = GK_EVENT_POINTER_MOVE;
            report.a = mods;
            report.x = x;
            report.y = y;
            report.z = dx;
            report.w = dy;
            gkGTKGoEvent(win->handle, &report);
            if (gkIsPen(event)) {
                gkSendPen(win, event, 1);
            }
        }
        break;
    }
    case GDK_ENTER_NOTIFY:
    case GDK_LEAVE_NOTIFY: {
        if (gdk_crossing_event_get_detail(event) == GDK_NOTIFY_INFERIOR) {
            break;
        }
        double x, y;
        gkWinPointToRoot(win, event, &x, &y);
        gkWinNotify(win, type == GDK_ENTER_NOTIFY ? GK_EVENT_POINTER_ENTER : GK_EVENT_POINTER_LEAVE, 0, x, y, NULL);
        if (type == GDK_LEAVE_NOTIFY) {
            win->have_last = 0;
        }
        break;
    }
    case GDK_SCROLL: {
        double dx = 0, dy = 0;
        GdkScrollDirection direction = gdk_scroll_event_get_direction(event);
        switch (direction) {
        case GDK_SCROLL_UP:
            dy = -1;
            break;
        case GDK_SCROLL_DOWN:
            dy = 1;
            break;
        case GDK_SCROLL_LEFT:
            dx = -1;
            break;
        case GDK_SCROLL_RIGHT:
            dx = 1;
            break;
        default:
            gdk_scroll_event_get_deltas(event, &dx, &dy);
            break;
        }
        /* GLFW reports positive values for scrolling up and to the left. */
        win->scroll_x += -dx;
        win->scroll_y += -dy;
        if (win->handle && (dx != 0 || dy != 0)) {
            gkGTKGoScrollEvent(win->handle, -dx, -dy);
        }
        break;
    }
    case GDK_TOUCH_BEGIN:
        gkSendTouch(win, event, 0);
        break;
    case GDK_TOUCH_UPDATE:
        gkSendTouch(win, event, 1);
        break;
    case GDK_TOUCH_END:
        gkSendTouch(win, event, 2);
        break;
    case GDK_TOUCH_CANCEL:
        gkSendTouch(win, event, 3);
        break;
    case GDK_TOUCHPAD_PINCH:
    case GDK_TOUCHPAD_SWIPE: {
        GdkTouchpadGesturePhase phase = gdk_touchpad_event_get_gesture_phase(event);
        int report_phase = phase == GDK_TOUCHPAD_GESTURE_PHASE_BEGIN    ? 0
                           : phase == GDK_TOUCHPAD_GESTURE_PHASE_END    ? 2
                           : phase == GDK_TOUCHPAD_GESTURE_PHASE_CANCEL ? 3
                                                                        : 1;
        double x, y;
        gkWinPointToRoot(win, event, &x, &y);
        GKGTKEvent report = {0};
        report.kind = GK_EVENT_GESTURE;
        report.b = report_phase;
        report.x = x;
        report.y = y;
        if (type == GDK_TOUCHPAD_SWIPE) {
            double dx = 0, dy = 0;
            gdk_touchpad_event_get_deltas(event, &dx, &dy);
            report.a = 2;
            report.z = dx;
            report.w = dy;
            gkGTKGoEvent(win->handle, &report);
            break;
        }
        double scale = gdk_touchpad_event_get_pinch_scale(event);
        if (report_phase == 0) {
            gkLastPinchScale = 1;
        }
        report.a = 0;
        report.z = scale - gkLastPinchScale;
        gkLastPinchScale = scale;
        gkGTKGoEvent(win->handle, &report);
        double angle = gdk_touchpad_event_get_pinch_angle_delta(event);
        if (angle != 0) {
            report.a = 1;
            report.z = angle;
            gkGTKGoEvent(win->handle, &report);
        }
        break;
    }
    default:
        break;
    }
    if (gkCaptureView && gkWinFromWidget(gkCaptureView) == win) {
        return gkGTKCaptureDeliver(gkCaptureView, win, event);
    }
    return FALSE;
}

/* ---- Signals ---- */

static void gkWinUpdateApp(void) {
    int active = 0;
    for (GList *item = gkWindows; item; item = item->next) {
        GKWin *win = item->data;
        if (gtk_window_is_active(GTK_WINDOW(win->window))) {
            active = 1;
        }
    }
    if (active != gkAppActive) {
        gkAppActive = active;
        gkSend(gkAppHandle, active ? GK_EVENT_APP_ACTIVE : GK_EVENT_APP_INACTIVE, 0, 0, 0, 0, NULL);
    }
}

static gboolean gkWinUpdateAppIdle(gpointer data) {
    (void)data;
    gkWinUpdateApp();
    return G_SOURCE_REMOVE;
}

static void gkWinActive(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    GKWin *win = data;
    gboolean active = gtk_window_is_active(GTK_WINDOW(object));
    if (win->im) {
        if (active) {
            gtk_im_context_focus_in(win->im);
        } else {
            gtk_im_context_focus_out(win->im);
        }
    }
    if (!active && win->handle) {
        for (int button = 0; button < GK_POINTER_BUTTON_COUNT; button++) {
            if (win->buttons & (1u << button)) {
                gkGTKGoPointerButtonEvent(win->handle, button, GK_POINTER_RELEASED, 0);
            }
        }
        win->buttons = 0;
    }
    gkWinNotify(win, active ? GK_EVENT_WINDOW_FOCUS : GK_EVENT_WINDOW_BLUR, 0, 0, 0, NULL);
    g_idle_add(gkWinUpdateAppIdle, NULL);
}

static gboolean gkWinCloseRequest(GtkWindow *window, gpointer data) {
    GKWin *win = data;
    if (win->handle) {
        GKGTKEvent event = {0};
        event.kind = GK_EVENT_WINDOW_CLOSE_REQUEST;
        if (!gkGTKGoQuery(win->handle, &event)) {
            return TRUE;
        }
    }
    win->should_close = 1;
    if (win->utility) {
        gtk_widget_set_visible(GTK_WIDGET(window), FALSE);
    }
    return TRUE;
}

/* gkWinFullscreenChanged records a completed fullscreen transition. The
   toplevel state of the surface changes only after the window manager
   applies the request, unlike the property of GtkWindow. */
static void gkWinFullscreenChanged(GKWin *win, int fullscreen) {
    if (win->fullscreen_timer) {
        g_source_remove(win->fullscreen_timer);
        win->fullscreen_timer = 0;
    }
    if (fullscreen == win->fullscreen) {
        return;
    }
    win->fullscreen = fullscreen;
    if (!fullscreen) {
        win->fullscreen_target = 0;
        gkWinApplyHints(win);
    }
    gkWinNotify(win, GK_EVENT_WINDOW_FULLSCREEN, fullscreen, 0, 0, NULL);
}

enum {
    GK_STATE_MINIMIZED = 1,
    GK_STATE_MAXIMIZED = 2,
    GK_STATE_FULLSCREEN = 4,
};

static int gkChromeHeight(GKWin *win);
static int gkWinState(GKWin *win);

/* gkWinMessage sends an EWMH client message about the window to the root
   window, where the window manager handles it. */
static void gkWinMessage(GKWin *win, const char *type, long a, long b, long c, long d, long e) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window) {
        return;
    }
    XEvent event = {0};
    event.xclient.type = ClientMessage;
    event.xclient.window = window;
    event.xclient.message_type = XInternAtom(display, type, False);
    event.xclient.format = 32;
    event.xclient.data.l[0] = a;
    event.xclient.data.l[1] = b;
    event.xclient.data.l[2] = c;
    event.xclient.data.l[3] = d;
    event.xclient.data.l[4] = e;
    XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
    XFlush(display);
}

static gboolean gkWinRestoreWindowed(gpointer data) {
    GKWin *win = data;
    int width = gtk_widget_get_width(win->root), height = gtk_widget_get_height(win->root);
    if (win->destroyed || win->fullscreen_state || --win->restore_tries <= 0 ||
        (width == win->restore_width && height == win->restore_height)) {
        win->restore_timer = 0;
        if (!(width == win->restore_width && height == win->restore_height)) {
            win->restore_width = win->restore_height = 0;
        }
        return G_SOURCE_REMOVE;
    }
    int scale = gtk_widget_get_scale_factor(win->window);
    gtk_window_set_default_size(GTK_WINDOW(win->window), win->restore_width,
                                win->restore_height + gkChromeHeight(win));
    /* Flags: north-west gravity, width and height present, sent by an
       application. */
    long flags = 1 | (1L << 10) | (1L << 11) | (1L << 12);
    gkWinMessage(win, "_NET_MOVERESIZE_WINDOW", flags, 0, 0, win->restore_width * scale,
                 (win->restore_height + gkChromeHeight(win)) * scale);
    return G_SOURCE_CONTINUE;
}

static gboolean gkWinFullscreenTimeout(gpointer data) {
    GKWin *win = data;
    win->fullscreen_timer = 0;
    gkWinFullscreenChanged(win, win->fullscreen_state);
    return G_SOURCE_REMOVE;
}

/* gkWinFullscreenSettled reports whether the content has the size of the
   new state: the size of the screen in fullscreen, another size after it. */
static int gkWinFullscreenSettled(GKWin *win, int width, int height) {
    GdkMonitor *monitor = gkWinMonitor(win);
    GdkRectangle geometry = {0, 0, 0, 0};
    if (monitor) {
        gdk_monitor_get_geometry(monitor, &geometry);
    }
    int screen = width >= geometry.width && height >= geometry.height - 1;
    if (win->fullscreen_state) {
        return screen;
    }
    if (win->restore_timer || win->restore_width > 0) {
        /* The windowed size is being restored. */
        return width == win->restore_width && height == win->restore_height;
    }
    return !screen;
}

/* gkWinFullscreenObserved starts a transition that completes when the
   content gets its new size, or after one second. */
static void gkWinFullscreenObserved(GKWin *win, int fullscreen) {
    if (fullscreen == win->fullscreen_state) {
        return;
    }
    win->fullscreen_state = fullscreen;
    if (!fullscreen && win->windowed_width > 0 && !win->destroyed) {
        /* A window that was fullscreen when first shown has no windowed
           geometry for the window manager to restore, so restore it after
           the window manager leaves fullscreen. */
        win->restore_width = win->windowed_width;
        win->restore_height = win->windowed_height;
        win->windowed_width = win->windowed_height = 0;
        win->restore_tries = 20;
        if (!win->restore_timer) {
            win->restore_timer = g_timeout_add(50, gkWinRestoreWindowed, win);
        }
    }
    int width = gtk_widget_get_width(win->root), height = gtk_widget_get_height(win->root);
    if (gkWinFullscreenSettled(win, width, height)) {
        gkWinFullscreenChanged(win, fullscreen);
        return;
    }
    if (win->fullscreen_timer) {
        g_source_remove(win->fullscreen_timer);
    }
    win->fullscreen_timer = g_timeout_add(1000, gkWinFullscreenTimeout, win);
}

/* gkWinPollState follows the fullscreen state of the window manager. */
static void gkWinPollState(GKWin *win) {
    if (!win->destroyed) {
        gkWinFullscreenObserved(win, (gkWinState(win) & GK_STATE_FULLSCREEN) != 0);
    }
}

static void gkWinRootAllocated(GtkWidget *layout, int width, int height, gpointer data) {
    (void)layout;
    GKWin *win = data;
    if (win->fullscreen_state != win->fullscreen && gkWinFullscreenSettled(win, width, height)) {
        win->restore_width = win->restore_height = 0;
        gkWinFullscreenChanged(win, win->fullscreen_state);
    }
    if (width == win->reported_width && height == win->reported_height) {
        return;
    }
    win->reported_width = width;
    win->reported_height = height;
    win->width = width;
    win->height = height;
    gkWinNotify(win, GK_EVENT_WINDOW_RESIZE, 0, width, height, NULL);
}

static void gkWinSurfaceState(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    GKWin *win = data;
    GdkToplevelState state = gdk_toplevel_get_state(GDK_TOPLEVEL(object));
    gkWinPollState(win);
    int occluded = (state & GDK_TOPLEVEL_STATE_MINIMIZED) != 0;
    if (occluded != win->occluded) {
        win->occluded = occluded;
        gkWinNotify(win, GK_EVENT_WINDOW_OCCLUSION, occluded, 0, 0, NULL);
    }
}

static void gkWinMapped(GtkWidget *widget, gpointer data) {
    (void)widget;
    GKWin *win = data;
    gkWinSetType(win);
    gkWinApplyHints(win);
    if (win->have_position) {
        gkWinMoveFrame(win, win->x, win->y);
    }
    if (win->always_on_top) {
        gkWinSendState(win, 1, "_NET_WM_STATE_ABOVE", NULL);
    }
    GdkSurface *surface = gkWinSurface(win);
    if (surface && !g_object_get_data(G_OBJECT(surface), "gk-state")) {
        g_object_set_data(G_OBJECT(surface), "gk-state", win);
        g_signal_connect(surface, "notify::state", G_CALLBACK(gkWinSurfaceState), win);
    }
}

static GdkDragAction gkWinDropEnter(GtkDropTarget *target, double x, double y, gpointer data) {
    (void)target;
    (void)x;
    (void)y;
    GKWin *win = data;
    return win->accept_files ? GDK_ACTION_COPY : 0;
}

static gboolean gkWinDrop(GtkDropTarget *target, const GValue *value, double x, double y, gpointer data) {
    (void)target;
    (void)x;
    (void)y;
    GKWin *win = data;
    if (!win->accept_files || !G_VALUE_HOLDS(value, GDK_TYPE_FILE_LIST)) {
        return FALSE;
    }
    GSList *files = gdk_file_list_get_files(g_value_get_boxed(value));
    GString *paths = g_string_new("");
    int count = 0;
    for (GSList *item = files; item; item = item->next) {
        char *path = g_file_get_path(item->data);
        if (path) {
            if (count++) {
                g_string_append_c(paths, '\x1e');
            }
            g_string_append(paths, path);
            g_free(path);
        }
    }
    g_slist_free(files);
    if (count) {
        gkWinNotify(win, GK_EVENT_WINDOW_FILE_DROP, count, 0, 0, paths->str);
    }
    g_string_free(paths, TRUE);
    return count > 0;
}

static void gkWinFocusWidget(GObject *object, GParamSpec *spec, gpointer data);

/* ---- Initialization ---- */

static void gkDeviceChanged(GdkSeat *seat, GdkDevice *device, gpointer data) {
    (void)seat;
    int connected = GPOINTER_TO_INT(data);
    int kind = 1;
    switch (gdk_device_get_source(device)) {
    case GDK_SOURCE_KEYBOARD:
        kind = 0;
        break;
    case GDK_SOURCE_TOUCHSCREEN:
        kind = 2;
        break;
    case GDK_SOURCE_PEN:
        kind = 3;
        break;
    default:
        kind = 1;
        break;
    }
    GKGTKEvent event = {0};
    event.kind = GK_EVENT_DEVICE;
    event.a = kind;
    event.b = (int)(GPOINTER_TO_UINT(device) & 0x7fffffff);
    event.c = connected;
    event.text = gdk_device_get_name(device);
    gkGTKGoEvent(gkAppHandle, &event);
}

static void gkSettingChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    (void)data;
    gkNotifyThemes();
    gkSend(gkAppHandle, GK_EVENT_PREFERENCES, 0, 0, 0, 0, NULL);
}

static gboolean gkInitialize(char *error, size_t error_size) {
    if (gkInitialized) {
        return TRUE;
    }
    /* GameKit exposes windows as Xlib surfaces. */
    gdk_set_allowed_backends("x11");
    if (!gtk_init_check()) {
        gkSetError(error, error_size, "GTK 4 could not open an X11 display");
        return FALSE;
    }
    GtkSettings *settings = gtk_settings_get_default();
    if (settings) {
        g_object_get(settings, "gtk-application-prefer-dark-theme", &gkSystemDark, NULL);
        g_signal_connect(settings, "notify::gtk-enable-animations", G_CALLBACK(gkSettingChanged), NULL);
        g_signal_connect(settings, "notify::gtk-xft-dpi", G_CALLBACK(gkSettingChanged), NULL);
        if (g_object_class_find_property(G_OBJECT_GET_CLASS(settings), "gtk-interface-contrast")) {
            g_signal_connect(settings, "notify::gtk-interface-contrast", G_CALLBACK(gkSettingChanged), NULL);
        }
    }
    gkWatchSystemSettings();
    GdkDisplay *display = gdk_display_get_default();
    GdkSeat *seat = display ? gdk_display_get_default_seat(display) : NULL;
    if (seat) {
        g_signal_connect(seat, "device-added", G_CALLBACK(gkDeviceChanged), GINT_TO_POINTER(1));
        g_signal_connect(seat, "device-removed", G_CALLBACK(gkDeviceChanged), GINT_TO_POINTER(0));
    }
    gkInitialized = TRUE;
    return TRUE;
}

void gkWinAddChrome(GKWin *win, GtkWidget *widget, int top) {
    if (!win || !widget) {
        return;
    }
    gkRemoveFromParent(widget);
    if (top) {
        gtk_box_append(GTK_BOX(win->top), widget);
    } else {
        gtk_box_append(GTK_BOX(win->bottom), widget);
    }
}

void *gkGTKWindowCreate(const char *title, int width, int height, uint32_t flags, void *owner, int x, int y,
                        char *error, size_t error_size) {
    if (!gkInitialize(error, error_size)) {
        return NULL;
    }
    GKWin *win = g_new0(GKWin, 1);
    win->owner = owner;
    win->utility = (flags & GK_WINDOW_UTILITY) != 0;
    win->resizable = (flags & GK_WINDOW_RESIZABLE) != 0;
    win->borderless = (flags & GK_WINDOW_BORDERLESS) != 0;
    win->always_on_top = (flags & GK_WINDOW_ALWAYS_ON_TOP) != 0;
    win->transparent = (flags & GK_WINDOW_TRANSPARENT) != 0;
    win->width = MAX(1, width);
    win->height = MAX(1, height);
    win->x = x;
    win->y = y;
    win->have_position = owner != NULL;

    win->window = gtk_window_new();
    g_object_ref_sink(win->window);
    g_object_set_data(G_OBJECT(win->window), "gk-win", win);
    win->overlay = gtk_overlay_new();
    win->box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    win->top = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    win->bottom = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    win->root = gkLayoutNew();
    gtk_widget_set_vexpand(win->root, TRUE);
    gtk_widget_set_hexpand(win->root, TRUE);
    gtk_widget_set_focusable(win->root, TRUE);
    gkLayoutOnAllocate(win->root, gkWinRootAllocated, win);
    gtk_box_append(GTK_BOX(win->box), win->top);
    gtk_box_append(GTK_BOX(win->box), win->root);
    gtk_box_append(GTK_BOX(win->box), win->bottom);
    gtk_overlay_set_child(GTK_OVERLAY(win->overlay), win->box);
    gtk_window_set_child(GTK_WINDOW(win->window), win->overlay);
    gtk_window_set_title(GTK_WINDOW(win->window), title ? title : "");
    gtk_window_set_default_size(GTK_WINDOW(win->window), win->width, win->height);
    gtk_window_set_resizable(GTK_WINDOW(win->window), win->resizable);
    gtk_window_set_decorated(GTK_WINDOW(win->window), !win->borderless);
    if (win->transparent) {
        gtk_widget_add_css_class(win->window, "gk-transparent");
        gkSetCSSRule("window.gk-transparent, window.gk-transparent > overlay", "background: transparent;");
    }
    if (win->owner) {
        gtk_window_set_transient_for(GTK_WINDOW(win->window), GTK_WINDOW(win->owner->window));
    }
    g_signal_connect(win->window, "close-request", G_CALLBACK(gkWinCloseRequest), win);
    g_signal_connect(win->window, "notify::is-active", G_CALLBACK(gkWinActive), win);
    g_signal_connect(win->window, "notify::focus-widget", G_CALLBACK(gkWinFocusWidget), win);
    g_signal_connect(win->window, "map", G_CALLBACK(gkWinMapped), win);

    GtkEventController *legacy = gtk_event_controller_legacy_new();
    gtk_event_controller_set_propagation_phase(legacy, GTK_PHASE_CAPTURE);
    g_signal_connect(legacy, "event", G_CALLBACK(gkWinEvent), win);
    gtk_widget_add_controller(win->window, legacy);

    GtkDropTarget *drop = gtk_drop_target_new(GDK_TYPE_FILE_LIST, GDK_ACTION_COPY);
    g_signal_connect(drop, "enter", G_CALLBACK(gkWinDropEnter), win);
    g_signal_connect(drop, "motion", G_CALLBACK(gkWinDropEnter), win);
    g_signal_connect(drop, "drop", G_CALLBACK(gkWinDrop), win);
    gtk_widget_add_controller(win->root, GTK_EVENT_CONTROLLER(drop));

    win->im = gtk_im_multicontext_new();
    gtk_im_context_set_client_widget(win->im, win->root);
    g_signal_connect(win->im, "commit", G_CALLBACK(gkIMCommit), win);
    g_signal_connect(win->im, "preedit-start", G_CALLBACK(gkIMPreeditStart), win);
    g_signal_connect(win->im, "preedit-changed", G_CALLBACK(gkIMPreeditChanged), win);
    g_signal_connect(win->im, "preedit-end", G_CALLBACK(gkIMPreeditEnd), win);

    gkWindows = g_list_append(gkWindows, win);
    gtk_widget_realize(win->window);
    GdkSurface *surface = gkWinSurface(win);
    if (surface) {
        g_object_set_data(G_OBJECT(surface), "gk-state", win);
        g_signal_connect(surface, "notify::state", G_CALLBACK(gkWinSurfaceState), win);
    }
    gkWinSetType(win);
    if (win->always_on_top) {
        gkWinSendState(win, 1, "_NET_WM_STATE_ABOVE", NULL);
    }
    if (win->owner) {
        gkWinApplyHints(win);
        gkWinMoveFrame(win, x, y);
    }
    if (flags & GK_WINDOW_MAXIMIZED) {
        gtk_window_maximize(GTK_WINDOW(win->window));
    }
    win->resolved_theme = gkResolvedTheme();
    if (!(flags & GK_WINDOW_HIDDEN)) {
        gtk_window_present(GTK_WINDOW(win->window));
    }
    return win;
}

void gkGTKWindowDestroy(void *pointer) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    win->destroyed = 1;
    win->handle = 0;
    if (win->fullscreen_timer) {
        g_source_remove(win->fullscreen_timer);
        win->fullscreen_timer = 0;
    }
    if (win->restore_timer) {
        g_source_remove(win->restore_timer);
        win->restore_timer = 0;
    }
    if (gkCaptureView && gkWinFromWidget(gkCaptureView) == win) {
        gkCaptureView = NULL;
    }
    if (win->cursor_mode != GK_CURSOR_NORMAL) {
        Display *display = gkWinXDisplay(win);
        if (display) {
            XUngrabPointer(display, CurrentTime);
        }
    }
    gkWindows = g_list_remove(gkWindows, win);
    if (win->im) {
        gtk_im_context_set_client_widget(win->im, NULL);
        g_object_unref(win->im);
    }
    g_signal_handlers_disconnect_by_data(win->window, win);
    g_object_set_data(G_OBJECT(win->window), "gk-win", NULL);
    gtk_window_destroy(GTK_WINDOW(win->window));
    g_object_unref(win->window);
    g_free(win);
    gkApplyTheme();
}

void gkGTKWindowPoll(void *pointer) {
    GKWin *win = pointer;
    gkRunPending();
    if (win && gtk_widget_get_mapped(win->window)) {
        gkWinPollState(win);
    }
    if (win && !win->destroyed && win->handle && gtk_widget_get_mapped(win->window)) {
        int x, y;
        gkWinPositionValues(win, &x, &y);
        if (!win->have_position || x != win->x || y != win->y) {
            int moved = win->have_position;
            win->x = x;
            win->y = y;
            win->have_position = 1;
            if (moved) {
                gkGTKGoWindowMove(win->handle, x, y);
            }
        }
    }
    gkGTKAfterPoll();
}

int gkGTKWindowShouldClose(void *pointer) {
    GKWin *win = pointer;
    return !win || win->should_close;
}

void gkGTKWindowSetShouldClose(void *pointer, int close) {
    GKWin *win = pointer;
    if (win) {
        win->should_close = close != 0;
    }
}

void gkGTKWindowSize(void *pointer, int *width, int *height) {
    GKWin *win = pointer;
    int w = win ? win->width : 0;
    int h = win ? win->height : 0;
    if (win && gtk_widget_get_mapped(win->window)) {
        int allocated_width = gtk_widget_get_width(win->root);
        int allocated_height = gtk_widget_get_height(win->root);
        if (allocated_width > 0 && allocated_height > 0) {
            w = allocated_width;
            h = allocated_height;
        }
    }
    *width = w;
    *height = h;
}

void gkGTKWindowFramebufferSize(void *pointer, int *width, int *height) {
    GKWin *win = pointer;
    int w = 0, h = 0;
    gkGTKWindowSize(pointer, &w, &h);
    int scale = win ? gtk_widget_get_scale_factor(win->window) : 1;
    *width = w * scale;
    *height = h * scale;
}

static int gkChromeHeight(GKWin *win) {
    int top = 0, bottom = 0;
    gtk_widget_measure(win->top, GTK_ORIENTATION_VERTICAL, -1, NULL, &top, NULL, NULL);
    gtk_widget_measure(win->bottom, GTK_ORIENTATION_VERTICAL, -1, NULL, &bottom, NULL, NULL);
    return top + bottom;
}

typedef struct {
    GKWin *win;
    int width, height;
} GKSizeWait;

static gboolean gkSizeReached(gpointer data) {
    GKSizeWait *wait = data;
    return gtk_widget_get_width(wait->win->root) == wait->width &&
           gtk_widget_get_height(wait->win->root) == wait->height;
}

void gkGTKWindowSetSize(void *pointer, int width, int height) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    width = MAX(1, width);
    height = MAX(1, height);
    if (win->min_width > 0) {
        width = MAX(width, win->min_width);
    }
    if (win->min_height > 0) {
        height = MAX(height, win->min_height);
    }
    if (win->max_width > 0) {
        width = MIN(width, win->max_width);
    }
    if (win->max_height > 0) {
        height = MIN(height, win->max_height);
    }
    win->width = width;
    win->height = height;
    gtk_window_set_default_size(GTK_WINDOW(win->window), width, height + gkChromeHeight(win));
    if (gtk_widget_get_mapped(win->window)) {
        if (!win->resizable) {
            win->reported_width = width;
            win->reported_height = height;
            gkWinApplyHints(win);
        }
        Display *display = gkWinXDisplay(win);
        Window window = gkWinXWindow(win);
        int scale = gtk_widget_get_scale_factor(win->window);
        if (display && window) {
            XResizeWindow(display, window, (unsigned)(width * scale),
                          (unsigned)((height + gkChromeHeight(win)) * scale));
            XFlush(display);
        }
        GKSizeWait wait = {win, width, height};
        gkWinPump(gkSizeReached, &wait, 1.0);
    }
}

void gkGTKWindowPosition(void *pointer, int *x, int *y) {
    gkWinPositionValues(pointer, x, y);
}

typedef struct {
    GKWin *win;
    int x, y;
} GKMoveWait;

static gboolean gkMoveReached(gpointer data) {
    GKMoveWait *wait = data;
    int x, y;
    gkWinPositionValues(wait->win, &x, &y);
    return x == wait->x && y == wait->y;
}

void gkGTKWindowSetPosition(void *pointer, int x, int y) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    int before_x, before_y;
    gkWinPositionValues(win, &before_x, &before_y);
    win->have_position = 1;
    if (!gtk_widget_get_mapped(win->window)) {
        win->x = x;
        win->y = y;
        gkWinApplyHints(win);
        return;
    }
    gkWinMoveFrame(win, x, y);
    GKMoveWait wait = {win, x, y};
    gkWinPump(gkMoveReached, &wait, 1.0);
    int after_x, after_y;
    gkWinPositionValues(win, &after_x, &after_y);
    win->x = after_x;
    win->y = after_y;
    if ((after_x != before_x || after_y != before_y) && win->handle) {
        gkGTKGoWindowMove(win->handle, after_x, after_y);
    }
}

int gkGTKWindowVisible(void *pointer) {
    GKWin *win = pointer;
    return win && gtk_widget_get_visible(win->window);
}

/* gkMapped reports whether the window manager shows the window, which is
   later than GTK maps it. */
static gboolean gkMapped(gpointer data) {
    GKWin *win = data;
    if (!gtk_widget_get_mapped(win->window)) {
        return FALSE;
    }
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    XWindowAttributes attributes;
    return !display || !window ||
           (XGetWindowAttributes(display, window, &attributes) && attributes.map_state == IsViewable);
}

static gboolean gkUnmapped(gpointer data) {
    GKWin *win = data;
    return !gtk_widget_get_mapped(win->window);
}

void gkGTKWindowSetVisible(void *pointer, int visible) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    if (visible) {
        if (win->fullscreen_target) {
            gtk_window_fullscreen(GTK_WINDOW(win->window));
        }
        gtk_window_present(GTK_WINDOW(win->window));
        gkWinPump(gkMapped, win, 1.0);
    } else {
        gtk_widget_set_visible(win->window, FALSE);
        gkWinPump(gkUnmapped, win, 0.2);
    }
}

void gkGTKWindowBringToFront(void *pointer) {
    GKWin *win = pointer;
    if (!win || !gtk_widget_get_visible(win->window)) {
        return;
    }
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (display && window) {
        XRaiseWindow(display, window);
        XFlush(display);
    }
}

void gkGTKWindowSetTitle(void *pointer, const char *title) {
    GKWin *win = pointer;
    if (win) {
        gtk_window_set_title(GTK_WINDOW(win->window), title ? title : "");
    }
}

void gkGTKWindowSetTheme(void *pointer, int theme) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    win->theme = theme;
    gkApplyTheme();
    gkNotifyThemes();
}

uintptr_t gkGTKWindowNativeHandle(void *pointer) {
    return (uintptr_t)gkWinXWindow(pointer);
}

uintptr_t gkGTKWindowNativeDisplayHandle(void *pointer) {
    return (uintptr_t)gkWinXDisplay(pointer);
}

void *gkGTKWindowNativePointer(void *pointer) {
    return pointer;
}

void *gkGTKWindowNativeDisplay(void *pointer) {
    return gkWinXDisplay(pointer);
}

void gkGTKWindowCursorPosition(void *pointer, double *x, double *y) {
    GKWin *win = pointer;
    double px = 0, py = 0;
    if (win && win->cursor_mode == GK_CURSOR_DISABLED) {
        px = win->virtual_x;
        py = win->virtual_y;
    } else if (win) {
        GdkSurface *surface = gkWinSurface(win);
        GdkDisplay *display = surface ? gdk_surface_get_display(surface) : NULL;
        GdkSeat *seat = display ? gdk_display_get_default_seat(display) : NULL;
        GdkDevice *device = seat ? gdk_seat_get_pointer(seat) : NULL;
        double sx = 0, sy = 0;
        if (device && gdk_surface_get_device_position(surface, device, &sx, &sy, NULL)) {
            double offset_x = 0, offset_y = 0;
            gtk_native_get_surface_transform(GTK_NATIVE(win->window), &offset_x, &offset_y);
            graphene_point_t source = GRAPHENE_POINT_INIT((float)(sx - offset_x), (float)(sy - offset_y));
            graphene_point_t destination = source;
            (void)!gtk_widget_compute_point(win->window, win->root, &source, &destination);
            px = destination.x;
            py = destination.y;
        }
    }
    if (x) {
        *x = px;
    }
    if (y) {
        *y = py;
    }
}

void gkGTKWindowSetHandle(void *pointer, uintptr_t handle) {
    GKWin *win = pointer;
    if (win) {
        win->handle = handle;
    }
}

void gkGTKWindowGetScroll(void *pointer, double *x, double *y) {
    GKWin *win = pointer;
    *x = win ? win->scroll_x : 0;
    *y = win ? win->scroll_y : 0;
}

void gkGTKWindowSetCursorMode(void *pointer, int mode) {
    GKWin *win = pointer;
    if (!win || win->cursor_mode == mode) {
        return;
    }
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (mode == GK_CURSOR_DISABLED) {
        double x = 0, y = 0;
        gkGTKWindowCursorPosition(win, &x, &y);
        win->virtual_x = x;
        win->virtual_y = y;
        gtk_window_present(GTK_WINDOW(win->window));
    }
    if (win->cursor_mode == GK_CURSOR_DISABLED && display) {
        XUngrabPointer(display, CurrentTime);
        XFlush(display);
    }
    win->cursor_mode = mode;
    gtk_widget_set_cursor_from_name(win->window, mode == GK_CURSOR_NORMAL ? NULL : "none");
    if (mode == GK_CURSOR_DISABLED && display && window && gtk_widget_get_mapped(win->window)) {
        XGrabPointer(display, window, True, ButtonPressMask | ButtonReleaseMask | PointerMotionMask, GrabModeAsync,
                     GrabModeAsync, window, None, CurrentTime);
        gkWinWarpToCenter(win);
    }
}

int gkGTKWindowGetCursorMode(void *pointer) {
    GKWin *win = pointer;
    return win ? win->cursor_mode : GK_CURSOR_NORMAL;
}

/* ---- Extended window contract ---- */

void gkGTKWindowAttach(void *pointer, uintptr_t handle) {
    GKWin *win = pointer;
    if (win) {
        win->handle = handle;
    }
}

void gkGTKWindowDetach(void *pointer) {
    GKWin *win = pointer;
    if (win) {
        win->handle = 0;
    }
}

void gkGTKWindowNotify(void *window, int kind, int a, double x, double y, const char *text) {
    gkWinNotify(window, kind, a, x, y, text);
}

int gkGTKWindowAllowClose(void *window) {
    GKWin *win = window;
    if (!win || !win->handle) {
        return 1;
    }
    GKGTKEvent event = {0};
    event.kind = GK_EVENT_WINDOW_CLOSE_REQUEST;
    return gkGTKGoQuery(win->handle, &event);
}

void gkGTKWindowDidResize(void *window) {
    (void)window;
}

void gkGTKWindowDidChangeKey(void *window, int key) {
    gkWinNotify(window, key ? GK_EVENT_WINDOW_FOCUS : GK_EVENT_WINDOW_BLUR, 0, 0, 0, NULL);
}

void gkGTKWindowAcceptFiles(void *window, int accept) {
    GKWin *win = window;
    if (win) {
        win->accept_files = accept != 0;
    }
}

void gkGTKWindowDidShow(void *window) {
    (void)window;
}

int gkGTKWindowCanBecomeKey(void *window) {
    GKWin *win = window;
    return win && gtk_widget_get_can_focus(win->window);
}

void gkGTKWindowFullscreenPhase(void *window, int phase) {
    (void)window;
    (void)phase;
}

void gkGTKWindowSetSizeLimits(void *pointer, int min_width, int min_height, int max_width, int max_height) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    win->min_width = MAX(0, min_width);
    win->min_height = MAX(0, min_height);
    win->max_width = MAX(0, max_width);
    win->max_height = MAX(0, max_height);
    gkLayoutSetMinimum(win->root, win->min_width, win->min_height);
    gkWinApplyHints(win);
    int width = win->width, height = win->height;
    int clamped_width = width, clamped_height = height;
    if (win->min_width > 0) {
        clamped_width = MAX(clamped_width, win->min_width);
    }
    if (win->min_height > 0) {
        clamped_height = MAX(clamped_height, win->min_height);
    }
    if (win->max_width > 0) {
        clamped_width = MIN(clamped_width, win->max_width);
    }
    if (win->max_height > 0) {
        clamped_height = MIN(clamped_height, win->max_height);
    }
    if (clamped_width != width || clamped_height != height) {
        gkGTKWindowSetSize(win, clamped_width, clamped_height);
    }
}

void gkGTKWindowSetStyle(void *pointer, int resizable, int borderless) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    win->resizable = resizable != 0;
    win->borderless = borderless != 0;
    gtk_window_set_resizable(GTK_WINDOW(win->window), win->resizable);
    gtk_window_set_decorated(GTK_WINDOW(win->window), !win->borderless);
    gkWinApplyHints(win);
}

void gkGTKWindowSetAlwaysOnTop(void *pointer, int always_on_top) {
    GKWin *win = pointer;
    if (!win) {
        return;
    }
    win->always_on_top = always_on_top != 0;
    gkWinSendState(win, win->always_on_top, "_NET_WM_STATE_ABOVE", NULL);
}

/* gkWinState reads the state that the window manager sets on the window. GDK
   caches the state and can miss a change, so the properties are read from
   the X server. */
static int gkWinState(GKWin *win) {
    Display *display = gkWinXDisplay(win);
    Window window = gkWinXWindow(win);
    if (!display || !window || !gtk_widget_get_mapped(win->window)) {
        return 0;
    }
    int state = 0;
    Atom type;
    int format;
    unsigned long count, remaining;
    unsigned char *data = NULL;
    Atom wm_state = XInternAtom(display, "WM_STATE", False);
    if (XGetWindowProperty(display, window, wm_state, 0, 2, False, wm_state, &type, &format, &count, &remaining,
                           &data) == Success &&
        data && count >= 1 && format == 32) {
        if (((long *)data)[0] == IconicState) {
            state |= GK_STATE_MINIMIZED;
        }
    }
    if (data) {
        XFree(data);
        data = NULL;
    }
    Atom net_state = XInternAtom(display, "_NET_WM_STATE", False);
    if (XGetWindowProperty(display, window, net_state, 0, 32, False, XA_ATOM, &type, &format, &count, &remaining,
                           &data) == Success &&
        data) {
        Atom hidden = XInternAtom(display, "_NET_WM_STATE_HIDDEN", False);
        Atom vertical = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_VERT", False);
        Atom horizontal = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_HORZ", False);
        Atom fullscreen = XInternAtom(display, "_NET_WM_STATE_FULLSCREEN", False);
        int maximized = 0;
        for (unsigned long i = 0; i < count; i++) {
            Atom value = ((Atom *)data)[i];
            if (value == hidden) {
                state |= GK_STATE_MINIMIZED;
            } else if (value == vertical || value == horizontal) {
                maximized++;
            } else if (value == fullscreen) {
                state |= GK_STATE_FULLSCREEN;
            }
        }
        if (maximized == 2) {
            state |= GK_STATE_MAXIMIZED;
        }
        XFree(data);
    }
    return state;
}

static gboolean gkIsMinimizedNow(gpointer data) {
    return (gkWinState(data) & GK_STATE_MINIMIZED) != 0;
}

static gboolean gkIsNotMinimized(gpointer data) {
    return !gkIsMinimizedNow(data);
}

static gboolean gkIsMaximizedNow(gpointer data) {
    return (gkWinState(data) & GK_STATE_MAXIMIZED) != 0;
}

static gboolean gkIsNotMaximized(gpointer data) {
    return !gkIsMaximizedNow(data);
}

/* gkWinUnminimize restores a minimized window. It asks the window manager
   to activate the window, as a task bar does, which also maps it. */
static void gkWinUnminimize(GKWin *win) {
    if (!gkIsMinimizedNow(win)) {
        return;
    }
    gtk_window_unminimize(GTK_WINDOW(win->window));
    gtk_window_present(GTK_WINDOW(win->window));
    gkWinPump(gkIsNotMinimized, win, 2.0);
}

/* gkScreenLocked reports whether the session's screen shield is up. The
   window manager does not show or activate windows then. */
static int gkScreenLocked(void) {
    GDBusConnection *bus = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, NULL);
    if (!bus) {
        return 0;
    }
    static const char *const services[][2] = {{"org.gnome.ScreenSaver", "/org/gnome/ScreenSaver"},
                                              {"org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver"}};
    int locked = 0;
    for (size_t i = 0; i < G_N_ELEMENTS(services) && !locked; i++) {
        GVariant *reply = g_dbus_connection_call_sync(bus, services[i][0], services[i][1], services[i][0],
                                                      "GetActive", NULL, G_VARIANT_TYPE("(b)"),
                                                      G_DBUS_CALL_FLAGS_NO_AUTO_START, 1000, NULL, NULL);
        if (reply) {
            gboolean active = FALSE;
            g_variant_get(reply, "(b)", &active);
            locked = active;
            g_variant_unref(reply);
        }
    }
    g_object_unref(bus);
    return locked;
}

int gkGTKWindowIsMinimized(void *pointer) {
    GKWin *win = pointer;
    return win && gkIsMinimizedNow(win);
}

int gkGTKWindowMinimize(void *pointer) {
    GKWin *win = pointer;
    if (!win) {
        return 0;
    }
    if (gkIsMinimizedNow(win)) {
        return 1;
    }
    if (win->fullscreen) {
        return 0;
    }
    if (!gtk_widget_get_mapped(win->window)) {
        gtk_window_present(GTK_WINDOW(win->window));
        gkWinPump(gkMapped, win, 1.0);
    }
    gtk_window_minimize(GTK_WINDOW(win->window));
    gkWinPump(gkIsMinimizedNow, win, 1.0);
    return gkIsMinimizedNow(win);
}

int gkGTKWindowIsMaximized(void *pointer) {
    GKWin *win = pointer;
    if (!win || win->fullscreen || gkIsMinimizedNow(win)) {
        return 0;
    }
    return gkIsMaximizedNow(win);
}

int gkGTKWindowMaximize(void *pointer) {
    GKWin *win = pointer;
    if (!win || win->fullscreen) {
        return 0;
    }
    gkWinUnminimize(win);
    gtk_window_maximize(GTK_WINDOW(win->window));
    if (gtk_widget_get_mapped(win->window)) {
        gkWinPump(gkIsMaximizedNow, win, 1.0);
        if (!gkIsMaximizedNow(win)) {
            /* A window manager can drop a request that comes while it
               restores the window from the minimized state. */
            gtk_window_unmaximize(GTK_WINDOW(win->window));
            gtk_window_maximize(GTK_WINDOW(win->window));
            gkWinPump(gkIsMaximizedNow, win, 2.0);
        }
    }
    return 1;
}

int gkGTKWindowRestore(void *pointer) {
    GKWin *win = pointer;
    if (!win) {
        return 0;
    }
    if (win->fullscreen || win->fullscreen_target) {
        win->fullscreen_target = 0;
        gtk_window_unfullscreen(GTK_WINDOW(win->window));
    }
    gkWinUnminimize(win);
    if (gkIsMaximizedNow(win) || gtk_window_is_maximized(GTK_WINDOW(win->window))) {
        gtk_window_unmaximize(GTK_WINDOW(win->window));
        gkWinPump(gkIsNotMaximized, win, 1.0);
    }
    return !gkIsMinimizedNow(win);
}

int gkGTKWindowIsFullscreen(void *pointer) {
    GKWin *win = pointer;
    return win && win->fullscreen;
}

int gkGTKWindowSetFullscreen(void *pointer, int fullscreen) {
    GKWin *win = pointer;
    if (!win) {
        return 0;
    }
    if (gtk_widget_get_mapped(win->window) && (fullscreen != 0) != win->fullscreen && gkScreenLocked()) {
        return 0;
    }
    win->fullscreen_target = fullscreen != 0;
    if (fullscreen && !win->fullscreen && !win->fullscreen_state) {
        int width = 0, height = 0;
        gkGTKWindowSize(win, &width, &height);
        win->windowed_width = width;
        win->windowed_height = height;
    }
    if (fullscreen) {
        /* A maximum size would stop the window from filling the screen. */
        gkWinApplyHints(win);
        gtk_window_fullscreen(GTK_WINDOW(win->window));
    } else {
        gtk_window_unfullscreen(GTK_WINDOW(win->window));
    }
    return 1;
}

GdkMonitor *gkWinMonitor(GKWin *win) {
    GdkDisplay *display = gdk_display_get_default();
    GdkSurface *surface = win ? gkWinSurface(win) : NULL;
    GdkMonitor *monitor = NULL;
    if (surface && gtk_widget_get_mapped(win->window)) {
        monitor = gdk_display_get_monitor_at_surface(display, surface);
    }
    if (!monitor && display) {
        GListModel *monitors = gdk_display_get_monitors(display);
        if (g_list_model_get_n_items(monitors) > 0) {
            monitor = g_list_model_get_item(monitors, 0);
            g_object_unref(monitor);
        }
    }
    return monitor;
}

void gkGTKWindowScreen(void *pointer, double *values, char **name) {
    GdkMonitor *monitor = gkWinMonitor(pointer);
    GdkRectangle geometry = {0, 0, 1, 1};
    GdkRectangle workarea = {0, 0, 1, 1};
    double scale = 1, dpi = 96, refresh = 60;
    const char *label = "";
    if (monitor) {
        gdk_monitor_get_geometry(monitor, &geometry);
        workarea = geometry;
        if (GDK_IS_X11_MONITOR(monitor)) {
            gdk_x11_monitor_get_workarea(monitor, &workarea);
        }
        scale = gdk_monitor_get_scale(monitor);
        int width_mm = gdk_monitor_get_width_mm(monitor);
        if (width_mm > 0) {
            dpi = geometry.width * scale / (width_mm / 25.4);
        } else {
            dpi = 96 * scale;
        }
        int rate = gdk_monitor_get_refresh_rate(monitor);
        if (rate > 0) {
            refresh = rate / 1000.0;
        }
        label = gdk_monitor_get_description(monitor);
        if (!label) {
            label = gdk_monitor_get_connector(monitor);
        }
    }
    values[0] = geometry.x;
    values[1] = geometry.y;
    values[2] = geometry.width;
    values[3] = geometry.height;
    values[4] = workarea.x;
    values[5] = workarea.y;
    values[6] = workarea.width;
    values[7] = workarea.height;
    values[8] = scale > 0 ? scale : 1;
    values[9] = dpi > 0 ? dpi : 96;
    values[10] = refresh;
    *name = strdup(label ? label : "");
}

int gkGTKWindowResolvedTheme(void *pointer) {
    GKWin *win = pointer;
    if (win && win->theme) {
        return win->theme;
    }
    return gkResolvedTheme();
}

static void gkAccentColor(double *rgba) {
    /* GNOME accent colors, from the Adwaita palette. */
    static const struct {
        const char *name;
        double r, g, b;
    } accents[] = {
        {"blue", 0x35, 0x84, 0xe4},   {"teal", 0x21, 0x90, 0xa4},  {"green", 0x3a, 0x94, 0x4a},
        {"yellow", 0xc8, 0x88, 0x00}, {"orange", 0xed, 0x5b, 0x00}, {"red", 0xe6, 0x2d, 0x42},
        {"pink", 0xd5, 0x61, 0x99},   {"purple", 0x91, 0x41, 0xac}, {"slate", 0x6f, 0x83, 0x96},
    };
    rgba[0] = 0x35 / 255.0;
    rgba[1] = 0x84 / 255.0;
    rgba[2] = 0xe4 / 255.0;
    rgba[3] = 1;
    GSettingsSchemaSource *source = g_settings_schema_source_get_default();
    GSettingsSchema *schema =
        source ? g_settings_schema_source_lookup(source, "org.gnome.desktop.interface", TRUE) : NULL;
    if (!schema) {
        return;
    }
    if (g_settings_schema_has_key(schema, "accent-color")) {
        GSettings *settings = g_settings_new("org.gnome.desktop.interface");
        gchar *name = g_settings_get_string(settings, "accent-color");
        for (size_t i = 0; name && i < G_N_ELEMENTS(accents); i++) {
            if (strcmp(name, accents[i].name) == 0) {
                rgba[0] = accents[i].r / 255.0;
                rgba[1] = accents[i].g / 255.0;
                rgba[2] = accents[i].b / 255.0;
            }
        }
        g_free(name);
        g_object_unref(settings);
    }
    g_settings_schema_unref(schema);
}

void gkGTKSystemPreferences(void *pointer, double *values, char **locale) {
    values[0] = gkGTKWindowResolvedTheme(pointer);
    double scale = 1;
    gboolean animations = TRUE;
    int contrast = 0;
    GtkSettings *settings = gtk_settings_get_default();
    if (settings) {
        int dpi = 0;
        g_object_get(settings, "gtk-xft-dpi", &dpi, "gtk-enable-animations", &animations, NULL);
        if (dpi > 0) {
            scale = dpi / 1024.0 / 96.0;
        }
        if (g_object_class_find_property(G_OBJECT_GET_CLASS(settings), "gtk-interface-contrast")) {
            /* GTK_INTERFACE_CONTRAST_MORE is 2. */
            g_object_get(settings, "gtk-interface-contrast", &contrast, NULL);
        }
    }
    values[1] = scale > 0 ? scale : 1;
    values[2] = contrast == 2;
    values[3] = !animations;
    values[4] = 0;
    gkAccentColor(values + 5);
    values[9] = gtk_widget_get_default_direction() == GTK_TEXT_DIR_RTL;
    const char *name = setlocale(LC_MESSAGES, NULL);
    if (!name || strcmp(name, "C") == 0 || strcmp(name, "POSIX") == 0) {
        const char *const *languages = g_get_language_names();
        name = languages && languages[0] ? languages[0] : "en";
    }
    char *tag = g_strdup(name);
    char *cut = strpbrk(tag, ".@");
    if (cut) {
        *cut = '\0';
    }
    for (char *p = tag; *p; p++) {
        if (*p == '_') {
            *p = '-';
        }
    }
    if (strcmp(tag, "C") == 0 || strcmp(tag, "POSIX") == 0 || !*tag) {
        g_free(tag);
        tag = g_strdup("en");
    }
    *locale = strdup(tag);
    g_free(tag);
}

int gkGTKWindowColorSpace(void *window) {
    (void)window;
    return 1;
}

/* ---- Application ---- */

void gkGTKAppAttach(uintptr_t handle) {
    gkAppHandle = handle;
}

/* ---- Focus tracking ---- */

static void gkWinFocusWidget(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkGTKTrackFocus(data);
}

void gkGTKTrackFocus(void *window) {
    GKWin *win = window;
    if (!win || win->destroyed) {
        return;
    }
    GtkWidget *focus = gtk_root_get_focus(GTK_ROOT(win->window));
    GKView *view = gkViewFromDescendant(focus);
    GtkWidget *current = view ? view->outer : NULL;
    if (current == win->focus) {
        return;
    }
    GtkWidget *previous = win->focus;
    win->focus = current;
    GKView *old = gkView(previous);
    if (old) {
        gkViewSend(old, GK_EVENT_BLUR, 0, 0, 0, 0, NULL);
    }
    if (view) {
        gkViewSend(view, GK_EVENT_FOCUS, 0, 0, 0, 0, NULL);
    }
}

void gkGTKForgetFocus(void *window, uintptr_t handle) {
    GKWin *win = window;
    if (!win) {
        return;
    }
    GKView *view = gkView(win->focus);
    if (!view || view->handle == handle || view->handle == 0) {
        win->focus = NULL;
    }
}

/* ---- Canvas capture and scale ---- */

int gkGTKCanvasCapture(void *pointer, int capture) {
    GtkWidget *view = pointer;
    if (capture) {
        if (!gkWinFromWidget(view)) {
            return 0;
        }
        gkCaptureView = view;
    } else if (gkCaptureView == view) {
        gkCaptureView = NULL;
    }
    return 1;
}

int gkGTKCanvasHasCapture(void *pointer) {
    return pointer && gkCaptureView == pointer;
}

void *gkGTKCaptureTarget(void *event) {
    (void)event;
    return gkCaptureView;
}

void gkGTKDeliverCaptured(void *view, void *event) {
    (void)view;
    (void)event;
}

void gkGTKReleaseCaptureFor(void *view) {
    if (gkCaptureView == view) {
        gkCaptureView = NULL;
    }
}

double gkGTKViewScale(void *pointer) {
    GtkWidget *widget = pointer;
    if (widget && GTK_IS_WIDGET(widget) && gtk_widget_get_root(widget)) {
        return gtk_widget_get_scale_factor(widget);
    }
    GdkMonitor *monitor = gkWinMonitor(NULL);
    return monitor ? gdk_monitor_get_scale(monitor) : 1;
}

int gkGTKViewColorSpace(void *view) {
    (void)view;
    return 1;
}
