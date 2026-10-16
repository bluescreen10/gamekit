//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>
#include <stdio.h>

/* ---- GKLayout ---- */

typedef struct {
    int x, y, width, height;
} GKFrame;

struct _GKLayout {
    GtkWidget parent_instance;
    int minimum_width;
    int minimum_height;
    void (*on_allocate)(GtkWidget *layout, int width, int height, gpointer data);
    gpointer on_allocate_data;
    int allocating;
    double zoom;
};

G_DEFINE_FINAL_TYPE(GKLayout, gk_layout, GTK_TYPE_WIDGET)

static GQuark gkFrameQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-frame");
    }
    return quark;
}

static void gk_layout_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum,
                              int *natural, int *minimum_baseline, int *natural_baseline) {
    (void)for_size;
    GKLayout *layout = GK_LAYOUT(widget);
    int value = orientation == GTK_ORIENTATION_HORIZONTAL ? layout->minimum_width : layout->minimum_height;
    value = (int)ceil(value * layout->zoom);
    *minimum = value;
    *natural = value;
    *minimum_baseline = -1;
    *natural_baseline = -1;
}

static void gk_layout_size_allocate(GtkWidget *widget, int width, int height, int baseline) {
    (void)baseline;
    GKLayout *layout = GK_LAYOUT(widget);
    layout->allocating = 1;
    if (layout->on_allocate) {
        layout->on_allocate(widget, width, height, layout->on_allocate_data);
    }
    for (GtkWidget *child = gtk_widget_get_first_child(widget); child; child = gtk_widget_get_next_sibling(child)) {
        if (!gtk_widget_should_layout(child)) {
            continue;
        }
        GKFrame *frame = g_object_get_qdata(G_OBJECT(child), gkFrameQuark());
        GtkAllocation allocation = {0, 0, 0, 0};
        if (frame) {
            allocation.x = frame->x;
            allocation.y = frame->y;
            allocation.width = frame->width;
            allocation.height = frame->height;
        }
        /* GTK warns when a widget gets less than its minimum size. Keep the
           minimum; the snapshot of the layout clips it near the frame. */
        int minimum = 0;
        gtk_widget_measure(child, GTK_ORIENTATION_HORIZONTAL, -1, &minimum, NULL, NULL, NULL);
        int allocated_width = MAX(allocation.width, minimum);
        gtk_widget_measure(child, GTK_ORIENTATION_VERTICAL, allocated_width, &minimum, NULL, NULL, NULL);
        int allocated_height = MAX(allocation.height, minimum);
        if (layout->zoom != 1) {
            graphene_point_t origin = GRAPHENE_POINT_INIT((float)(allocation.x * layout->zoom),
                                                          (float)(allocation.y * layout->zoom));
            GskTransform *transform = gsk_transform_translate(NULL, &origin);
            transform = gsk_transform_scale(transform, (float)layout->zoom, (float)layout->zoom);
            gtk_widget_allocate(child, allocated_width, allocated_height, -1, transform);
            continue;
        }
        GtkAllocation real = {allocation.x, allocation.y, allocated_width, allocated_height};
        gtk_widget_size_allocate(child, &real, -1);
    }
    layout->allocating = 0;
}

/* gkLayoutBleed is the distance in pixels that children can draw outside the
   layout, for example a badge on the corner of a button. */
static const float gkLayoutBleed = 8;

/* gk_layout_snapshot clips the children to the layout and a small border
   around it. A child that GTK cannot make as small as its frame does not
   draw over its neighbors. */
static void gk_layout_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
    graphene_rect_t clip = GRAPHENE_RECT_INIT(-gkLayoutBleed, -gkLayoutBleed,
                                              gtk_widget_get_width(widget) + 2 * gkLayoutBleed,
                                              gtk_widget_get_height(widget) + 2 * gkLayoutBleed);
    gtk_snapshot_push_clip(snapshot, &clip);
    GTK_WIDGET_CLASS(gk_layout_parent_class)->snapshot(widget, snapshot);
    gtk_snapshot_pop(snapshot);
}

static void gk_layout_dispose(GObject *object) {
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(GTK_WIDGET(object)))) {
        gtk_widget_unparent(child);
    }
    G_OBJECT_CLASS(gk_layout_parent_class)->dispose(object);
}

static void gk_layout_class_init(GKLayoutClass *klass) {
    GtkWidgetClass *widget_class = GTK_WIDGET_CLASS(klass);
    widget_class->measure = gk_layout_measure;
    widget_class->size_allocate = gk_layout_size_allocate;
    widget_class->snapshot = gk_layout_snapshot;
    G_OBJECT_CLASS(klass)->dispose = gk_layout_dispose;
    gtk_widget_class_set_accessible_role(widget_class, GTK_ACCESSIBLE_ROLE_GROUP);
}

static void gk_layout_init(GKLayout *layout) {
    layout->zoom = 1;
    /* The snapshot clips the children; see gk_layout_snapshot. */
    gtk_widget_set_overflow(GTK_WIDGET(layout), GTK_OVERFLOW_VISIBLE);
}

void gkLayoutSetZoom(GtkWidget *widget, double zoom) {
    if (!widget || !GK_IS_LAYOUT(widget) || zoom <= 0) {
        return;
    }
    GK_LAYOUT(widget)->zoom = zoom;
    gtk_widget_queue_resize(widget);
}

double gkLayoutZoom(GtkWidget *widget) {
    return widget && GK_IS_LAYOUT(widget) ? GK_LAYOUT(widget)->zoom : 1;
}

GtkWidget *gkDescendant(GtkWidget *widget, GType type) {
    for (GtkWidget *child = widget ? gtk_widget_get_first_child(widget) : NULL; child;
         child = gtk_widget_get_next_sibling(child)) {
        if (G_TYPE_CHECK_INSTANCE_TYPE(child, type)) {
            return child;
        }
        GtkWidget *found = gkDescendant(child, type);
        if (found) {
            return found;
        }
    }
    return NULL;
}

GtkWidget *gkLayoutNew(void) {
    return g_object_new(GK_TYPE_LAYOUT, NULL);
}

GtkWidget *gkLayoutNewWithRole(GtkAccessibleRole role) {
    return g_object_new(GK_TYPE_LAYOUT, "accessible-role", role, NULL);
}

void gkLayoutPut(GtkWidget *layout, GtkWidget *child) {
    if (!layout || !child) {
        return;
    }
    gkRemoveFromParent(child);
    GKFrame *frame = g_object_get_qdata(G_OBJECT(child), gkFrameQuark());
    gtk_widget_set_child_visible(child, frame && frame->width > 0 && frame->height > 0);
    gtk_widget_set_parent(child, layout);
}

void gkLayoutMove(GtkWidget *child, int x, int y, int width, int height) {
    GKFrame *frame = g_object_get_qdata(G_OBJECT(child), gkFrameQuark());
    if (!frame) {
        frame = g_new0(GKFrame, 1);
        g_object_set_qdata_full(G_OBJECT(child), gkFrameQuark(), frame, g_free);
    }
    if (frame->x == x && frame->y == y && frame->width == width && frame->height == height) {
        return;
    }
    frame->x = x;
    frame->y = y;
    frame->width = MAX(0, width);
    frame->height = MAX(0, height);
    gtk_widget_set_child_visible(child, frame->width > 0 && frame->height > 0);
    GtkWidget *parent = gtk_widget_get_parent(child);
    if (parent && !(GK_IS_LAYOUT(parent) && GK_LAYOUT(parent)->allocating)) {
        gtk_widget_queue_allocate(parent);
    }
}

void gkLayoutSetMinimum(GtkWidget *widget, int width, int height) {
    if (!widget || !GK_IS_LAYOUT(widget)) {
        return;
    }
    GKLayout *layout = GK_LAYOUT(widget);
    layout->minimum_width = MAX(0, width);
    layout->minimum_height = MAX(0, height);
    gtk_widget_queue_resize(widget);
}

void gkLayoutOnAllocate(GtkWidget *widget, void (*callback)(GtkWidget *, int, int, gpointer), gpointer data) {
    if (!widget || !GK_IS_LAYOUT(widget)) {
        return;
    }
    GK_LAYOUT(widget)->on_allocate = callback;
    GK_LAYOUT(widget)->on_allocate_data = data;
}

/* ---- Views ---- */

static GQuark gkViewQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-view");
    }
    return quark;
}

static void gkViewFree(gpointer data) {
    GKView *view = data;
    if (view->ext && view->ext_free) {
        view->ext_free(view->ext);
    }
    if (view->items) {
        g_ptr_array_free(view->items, TRUE);
    }
    if (view->buttons) {
        g_ptr_array_free(view->buttons, TRUE);
    }
    if (view->texture) {
        g_object_unref(view->texture);
    }
    free(view->css_class);
    free(view->placeholder);
    free(view->url);
    free(view->accessible_id);
    free(view->accessible_name);
    free(view->accessible_value);
    g_free(view);
}

GKView *gkView(void *object) {
    if (!object || !G_IS_OBJECT(object)) {
        return NULL;
    }
    return g_object_get_qdata(G_OBJECT(object), gkViewQuark());
}

GKView *gkViewNew(GtkWidget *outer, GtkWidget *control, int kind, uintptr_t window, uintptr_t action) {
    if (!outer) {
        return NULL;
    }
    GKView *view = g_new0(GKView, 1);
    view->outer = outer;
    view->control = control ? control : outer;
    view->kind = kind;
    view->window = gkWinFromHandle(window);
    view->action = action;
    view->visible = 1;
    view->selected = -1;
    view->source_scale = 1;
    g_object_ref_sink(outer);
    g_object_set_qdata_full(G_OBJECT(outer), gkViewQuark(), view, gkViewFree);
    return view;
}

GKView *gkViewFromDescendant(GtkWidget *widget) {
    for (; widget; widget = gtk_widget_get_parent(widget)) {
        GKView *view = gkView(widget);
        if (view) {
            return view;
        }
    }
    return NULL;
}

GtkWidget *gkViewContainer(GKView *view) {
    if (!view) {
        return NULL;
    }
    return view->body ? view->body : view->outer;
}

void gkSend(uintptr_t handle, int kind, int a, int b, double x, double y, const char *text) {
    if (!handle) {
        return;
    }
    GKGTKEvent event = {0};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.x = x;
    event.y = y;
    event.text = text;
    gkGTKGoEvent(handle, &event);
}

void gkViewSend(GKView *view, int kind, int a, int b, double x, double y, const char *text) {
    if (view) {
        gkSend(view->handle, kind, a, b, x, y, text);
    }
}

void gkViewAction(GKView *view, int event) {
    if (view && view->action && !view->suppress) {
        gkGTKUIGoEvent(view->action, event);
    }
}

void gkRemoveFromParent(GtkWidget *widget) {
    GtkWidget *parent = widget ? gtk_widget_get_parent(widget) : NULL;
    if (!parent) {
        return;
    }
    if (GK_IS_LAYOUT(parent)) {
        gtk_widget_unparent(widget);
    } else if (GTK_IS_BOX(parent)) {
        gtk_box_remove(GTK_BOX(parent), widget);
    } else if (GTK_IS_OVERLAY(parent)) {
        if (gtk_overlay_get_child(GTK_OVERLAY(parent)) == widget) {
            gtk_overlay_set_child(GTK_OVERLAY(parent), NULL);
        } else {
            gtk_overlay_remove_overlay(GTK_OVERLAY(parent), widget);
        }
    } else if (GTK_IS_SCROLLED_WINDOW(parent)) {
        gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(parent), NULL);
    } else if (GTK_IS_VIEWPORT(parent)) {
        gtk_viewport_set_child(GTK_VIEWPORT(parent), NULL);
    } else if (GTK_IS_FRAME(parent)) {
        gtk_frame_set_child(GTK_FRAME(parent), NULL);
    } else if (GTK_IS_POPOVER(parent)) {
        gtk_popover_set_child(GTK_POPOVER(parent), NULL);
    } else if (GTK_IS_REVEALER(parent)) {
        gtk_revealer_set_child(GTK_REVEALER(parent), NULL);
    } else if (GTK_IS_WINDOW(parent)) {
        gtk_window_set_child(GTK_WINDOW(parent), NULL);
    } else if (GTK_IS_GRID(parent)) {
        gtk_grid_remove(GTK_GRID(parent), widget);
    } else if (GTK_IS_STACK(parent)) {
        gtk_stack_remove(GTK_STACK(parent), widget);
    } else if (GTK_IS_PANED(parent)) {
        if (gtk_paned_get_start_child(GTK_PANED(parent)) == widget) {
            gtk_paned_set_start_child(GTK_PANED(parent), NULL);
        } else {
            gtk_paned_set_end_child(GTK_PANED(parent), NULL);
        }
    } else if (GTK_IS_FLOW_BOX_CHILD(parent)) {
        gtk_flow_box_child_set_child(GTK_FLOW_BOX_CHILD(parent), NULL);
    } else {
        gtk_widget_unparent(widget);
    }
}

/* ---- Strings ---- */

char *gkStrdup(const char *text) {
    return strdup(text ? text : "");
}

char **gkSplit(const char *buffer, size_t size, int *count) {
    GPtrArray *values = g_ptr_array_new();
    size_t start = 0;
    for (size_t i = 0; buffer && i < size; i++) {
        if (buffer[i] == '\0') {
            g_ptr_array_add(values, g_strndup(buffer + start, i - start));
            start = i + 1;
        }
    }
    if (buffer && start < size) {
        g_ptr_array_add(values, g_strndup(buffer + start, size - start));
    }
    if (count) {
        *count = (int)values->len;
    }
    g_ptr_array_add(values, NULL);
    return (char **)g_ptr_array_free(values, FALSE);
}

char *gkJoin(char **values, int count, size_t *size) {
    size_t total = 0;
    for (int i = 0; i < count; i++) {
        total += strlen(values[i] ? values[i] : "") + 1;
    }
    char *buffer = malloc(MAX(total, 1));
    size_t offset = 0;
    for (int i = 0; i < count; i++) {
        const char *value = values[i] ? values[i] : "";
        size_t length = strlen(value) + 1;
        memcpy(buffer + offset, value, length);
        offset += length;
    }
    if (size) {
        *size = total;
    }
    return buffer;
}

GdkTexture *gkTextureFromRGBA(const unsigned char *pixels, int width, int height, int stride) {
    if (!pixels || width <= 0 || height <= 0) {
        return NULL;
    }
    if (stride <= 0) {
        stride = width * 4;
    }
    GBytes *bytes = g_bytes_new(pixels, (gsize)stride * (gsize)height);
    GdkTexture *texture = gdk_memory_texture_new(width, height, GDK_MEMORY_R8G8B8A8, bytes, (gsize)stride);
    g_bytes_unref(bytes);
    return texture;
}

/* ---- Input translation ---- */

int gkModifiers(GdkModifierType state) {
    int modifiers = 0;
    if (state & GDK_SHIFT_MASK) {
        modifiers |= GK_MOD_SHIFT;
    }
    if (state & GDK_CONTROL_MASK) {
        modifiers |= GK_MOD_CONTROL;
    }
    if (state & GDK_ALT_MASK) {
        modifiers |= GK_MOD_ALT;
    }
    if (state & (GDK_SUPER_MASK | GDK_META_MASK)) {
        modifiers |= GK_MOD_SUPER;
    }
    if (state & GDK_LOCK_MASK) {
        modifiers |= GK_MOD_CAPS_LOCK;
    }
    return modifiers;
}

int gkPointerButton(guint button) {
    switch (button) {
    case 1:
        return GK_POINTER_BUTTON_LEFT;
    case 2:
        return GK_POINTER_BUTTON_MIDDLE;
    case 3:
        return GK_POINTER_BUTTON_RIGHT;
    default:
        if (button >= 8 && button < 8 + GK_POINTER_BUTTON_COUNT - 3) {
            return 3 + (int)button - 8;
        }
        return -1;
    }
}

/* gkKeyFromKeycode maps an X11 keycode, which is the evdev code plus 8, to a
   GLFW key code. The mapping uses physical positions, like GLFW. */
int gkKeyFromKeycode(guint keycode) {
    static const short evdev[] = {
        [1] = 256,  [2] = 49,   [3] = 50,   [4] = 51,   [5] = 52,   [6] = 53,   [7] = 54,   [8] = 55,
        [9] = 56,   [10] = 57,  [11] = 48,  [12] = 45,  [13] = 61,  [14] = 259, [15] = 258, [16] = 81,
        [17] = 87,  [18] = 69,  [19] = 82,  [20] = 84,  [21] = 89,  [22] = 85,  [23] = 73,  [24] = 79,
        [25] = 80,  [26] = 91,  [27] = 93,  [28] = 257, [29] = 341, [30] = 65,  [31] = 83,  [32] = 68,
        [33] = 70,  [34] = 71,  [35] = 72,  [36] = 74,  [37] = 75,  [38] = 76,  [39] = 59,  [40] = 39,
        [41] = 96,  [42] = 340, [43] = 92,  [44] = 90,  [45] = 88,  [46] = 67,  [47] = 86,  [48] = 66,
        [49] = 78,  [50] = 77,  [51] = 44,  [52] = 46,  [53] = 47,  [54] = 344, [55] = 332, [56] = 342,
        [57] = 32,  [58] = 280, [59] = 290, [60] = 291, [61] = 292, [62] = 293, [63] = 294, [64] = 295,
        [65] = 296, [66] = 297, [67] = 298, [68] = 299, [69] = 282, [70] = 281, [71] = 327, [72] = 328,
        [73] = 329, [74] = 333, [75] = 324, [76] = 325, [77] = 326, [78] = 334, [79] = 321, [80] = 322,
        [81] = 323, [82] = 320, [83] = 330, [86] = 161, [87] = 300, [88] = 301, [96] = 335, [97] = 345,
        [98] = 331, [99] = 283, [100] = 346, [102] = 268, [103] = 265, [104] = 266, [105] = 263, [106] = 262,
        [107] = 269, [108] = 264, [109] = 267, [110] = 260, [111] = 261, [117] = 336, [119] = 284, [125] = 343,
        [126] = 347, [127] = 348, [183] = 302, [184] = 303, [185] = 304, [186] = 305, [187] = 306, [188] = 307,
        [189] = 308, [190] = 309, [191] = 310, [192] = 311, [193] = 312, [194] = 313,
    };
    if (keycode < 8) {
        return -1;
    }
    guint code = keycode - 8;
    if (code >= G_N_ELEMENTS(evdev) || evdev[code] == 0) {
        return -1;
    }
    return evdev[code];
}

/* ---- Styles ----

   Widget styles are CSS rules for a class that only one widget has. One
   provider holds every rule, so no per-widget provider is needed. */

static GtkCssProvider *gkProvider;
static GHashTable *gkRules;
static guint gkNextClass;

void gkApplyCSS(void) {
    if (!gkRules) {
        return;
    }
    GString *css = g_string_new("");
    GHashTableIter iter;
    gpointer key, value;
    g_hash_table_iter_init(&iter, gkRules);
    while (g_hash_table_iter_next(&iter, &key, &value)) {
        g_string_append_printf(css, "%s { %s }\n", (const char *)key, (const char *)value);
    }
    if (!gkProvider) {
        gkProvider = gtk_css_provider_new();
        GdkDisplay *display = gdk_display_get_default();
        if (display) {
            gtk_style_context_add_provider_for_display(display, GTK_STYLE_PROVIDER(gkProvider),
                                                       GTK_STYLE_PROVIDER_PRIORITY_APPLICATION + 1);
        }
    }
    gtk_css_provider_load_from_string(gkProvider, css->str);
    g_string_free(css, TRUE);
}

void gkSetCSSRule(const char *selector, const char *rule) {
    if (!gkRules) {
        gkRules = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
    }
    if (rule && *rule) {
        g_hash_table_insert(gkRules, g_strdup(selector), g_strdup(rule));
    } else {
        g_hash_table_remove(gkRules, selector);
    }
    gkApplyCSS();
}

const char *gkWidgetCSSClass(GKView *view) {
    if (!view) {
        return "";
    }
    if (!view->css_class) {
        char name[32];
        snprintf(name, sizeof(name), "gk-w%u", ++gkNextClass);
        view->css_class = strdup(name);
        gtk_widget_add_css_class(view->outer, name);
        if (view->control && view->control != view->outer) {
            gtk_widget_add_css_class(view->control, name);
        }
    }
    return view->css_class;
}

void gkRunPending(void) {
    while (g_main_context_iteration(NULL, FALSE)) {
    }
}

/* ---- Objects ---- */

static GQuark gkHandleQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-handle");
    }
    return quark;
}

void gkGTKSetHandle(void *object, uintptr_t handle) {
    GKView *view = gkView(object);
    if (view) {
        view->handle = handle;
        return;
    }
    if (object && G_IS_OBJECT(object)) {
        g_object_set_qdata(G_OBJECT(object), gkHandleQuark(), (gpointer)handle);
    }
}

uintptr_t gkGTKHandle(void *object) {
    GKView *view = gkView(object);
    if (view) {
        return view->handle;
    }
    if (object && G_IS_OBJECT(object)) {
        return (uintptr_t)g_object_get_qdata(G_OBJECT(object), gkHandleQuark());
    }
    return 0;
}

void gkGTKRelease(void *object) {
    if (object && G_IS_OBJECT(object)) {
        g_object_unref(object);
    }
}

void gkGTKRetain(void *object) {
    if (object && G_IS_OBJECT(object)) {
        g_object_ref(object);
    }
}

void gkGTKFree(void *pointer) {
    free(pointer);
}
