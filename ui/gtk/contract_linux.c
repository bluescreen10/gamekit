//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>
#include <stdio.h>

extern void gkGTKGoDrop(uintptr_t handle, int operation, double x, double y, char *text, char *files,
                        size_t filesSize, unsigned char *pixels, int width, int height, char *types,
                        size_t typesSize, unsigned char *data, size_t *sizes, int count);

/* ---- Accessibility ---- */

static GtkWidget *gkAccessibleWidget(GKView *view) {
    return view->control ? view->control : view->outer;
}

void gkGTKSetAccessibility(void *pointer, const char *identifier, const char *name, const char *description,
                           const char *value, int required, int hidden, int invalid, void *labeled_by,
                           void **described_by, int described_by_count) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    GtkAccessible *element = GTK_ACCESSIBLE(gkAccessibleWidget(view));
    free(view->accessible_id);
    free(view->accessible_name);
    free(view->accessible_value);
    view->accessible_id = gkStrdup(identifier);
    view->accessible_name = gkStrdup(name);
    view->accessible_value = gkStrdup(value);
    if (name && *name) {
        gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_LABEL, name, -1);
    } else {
        gtk_accessible_reset_property(element, GTK_ACCESSIBLE_PROPERTY_LABEL);
    }
    if (description && *description) {
        gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_DESCRIPTION, description, -1);
    } else {
        gtk_accessible_reset_property(element, GTK_ACCESSIBLE_PROPERTY_DESCRIPTION);
    }
    if (value && *value) {
        gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, value, -1);
    } else {
        gtk_accessible_reset_property(element, GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT);
    }
    gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_REQUIRED, required != 0, -1);
    gtk_accessible_update_state(element, GTK_ACCESSIBLE_STATE_HIDDEN, hidden != 0, -1);
    gtk_accessible_update_state(element, GTK_ACCESSIBLE_STATE_INVALID,
                                invalid ? GTK_ACCESSIBLE_INVALID_TRUE : GTK_ACCESSIBLE_INVALID_FALSE, -1);
    GKView *label = gkView(labeled_by);
    if (label) {
        gtk_accessible_update_relation(element, GTK_ACCESSIBLE_RELATION_LABELLED_BY, gkAccessibleWidget(label), NULL,
                                       -1);
    } else {
        gtk_accessible_reset_relation(element, GTK_ACCESSIBLE_RELATION_LABELLED_BY);
    }
    GList *described = NULL;
    for (int i = described_by_count - 1; i >= 0; i--) {
        GKView *related = gkView(described_by[i]);
        if (related) {
            described = g_list_prepend(described, gkAccessibleWidget(related));
        }
    }
    if (described) {
        GValue list = G_VALUE_INIT;
        gtk_accessible_relation_init_value(GTK_ACCESSIBLE_RELATION_DESCRIBED_BY, &list);
        g_value_set_pointer(&list, described);
        GtkAccessibleRelation relations[] = {GTK_ACCESSIBLE_RELATION_DESCRIBED_BY};
        gtk_accessible_update_relation_value(element, 1, relations, &list);
        g_value_unset(&list);
    } else {
        gtk_accessible_reset_relation(element, GTK_ACCESSIBLE_RELATION_DESCRIBED_BY);
    }
}

void gkGTKAnnounceText(GtkWidget *widget, const char *text, int priority) {
    if (!widget || !text || !*text) {
        return;
    }
    gtk_accessible_announce(GTK_ACCESSIBLE(widget), text,
                            priority >= 2 ? GTK_ACCESSIBLE_ANNOUNCEMENT_PRIORITY_HIGH
                                          : GTK_ACCESSIBLE_ANNOUNCEMENT_PRIORITY_MEDIUM);
}

void gkGTKAnnounce(void *element, const char *text, int priority) {
    GKView *view = gkView(element);
    gkGTKAnnounceText(view ? gkAccessibleWidget(view) : element, text, priority);
}

/* ---- Accessibility tree ---- */

static const char *gkRoleNick(GtkAccessibleRole role) {
    static GEnumClass *roles;
    if (!roles) {
        roles = g_type_class_ref(GTK_TYPE_ACCESSIBLE_ROLE);
    }
    GEnumValue *value = g_enum_get_value(roles, role);
    return value ? value->value_nick : "";
}

/* gkSubrole names the GameKit component behind a widget when its GTK role
   cannot tell assistive technologies what it is. */
static const char *gkSubrole(GtkWidget *widget) {
    GKView *view = gkView(widget);
    if (!view) {
        GtkWidget *parent = gtk_widget_get_parent(widget);
        GKView *owner = gkView(parent);
        if (owner && owner->control == widget) {
            view = owner;
        } else {
            return "";
        }
    }
    switch (view->kind) {
    case GK_KIND_PASSWORD:
        return "password";
    case GK_KIND_TEXT_AREA:
        return "textarea";
    case GK_KIND_ACTIVITY:
        return "busy";
    case GK_KIND_SECTION:
        return "disclosure";
    case GK_KIND_SCROLL:
        return "scroll";
    case GK_KIND_CANVAS:
        return "canvas";
    case GK_KIND_DATE:
        return "date";
    case GK_KIND_CALENDAR:
        return "calendar";
    case GK_KIND_COLOR:
        return "color";
    case GK_KIND_FONT:
        return "font";
    case GK_KIND_SPLIT:
        return "split";
    case GK_KIND_WEB:
        return "web";
    case GK_KIND_VIDEO:
        return "video";
    case GK_KIND_DOCUMENT:
        return "document";
    case GK_KIND_STATUS_BAR:
        return "status";
    default:
        return "";
    }
}

static void gkAppendClean(GString *out, const char *value) {
    for (const char *p = value ? value : ""; *p; p++) {
        g_string_append_c(out, *p == '\x1e' || *p == '\x1f' ? ' ' : *p);
    }
}

static char *gkWidgetName(GtkWidget *widget) {
    GKView *view = gkView(widget);
    if (!view) {
        GKView *owner = gkView(gtk_widget_get_parent(widget));
        if (owner && owner->control == widget) {
            view = owner;
        }
    }
    if (view && view->accessible_name && *view->accessible_name) {
        return g_strdup(view->accessible_name);
    }
    if (GTK_IS_LABEL(widget)) {
        return g_strdup(gtk_label_get_text(GTK_LABEL(widget)));
    }
    if (GTK_IS_BUTTON(widget) && gtk_button_get_label(GTK_BUTTON(widget))) {
        return g_strdup(gtk_button_get_label(GTK_BUTTON(widget)));
    }
    if (GTK_IS_CHECK_BUTTON(widget) && gtk_check_button_get_label(GTK_CHECK_BUTTON(widget))) {
        return g_strdup(gtk_check_button_get_label(GTK_CHECK_BUTTON(widget)));
    }
    if (GTK_IS_WINDOW(widget)) {
        return g_strdup(gtk_window_get_title(GTK_WINDOW(widget)));
    }
    if (view && view->kind == GK_KIND_SECTION) {
        extern GtkWidget *gkSectionPart(void *control, int part);
        return g_strdup(gtk_label_get_text(GTK_LABEL(gkSectionPart(view->outer, 0))));
    }
    return g_strdup("");
}

static char *gkWidgetValue(GtkWidget *widget) {
    GKView *view = gkView(widget);
    if (view && view->accessible_value && *view->accessible_value) {
        return g_strdup(view->accessible_value);
    }
    if (GTK_IS_EDITABLE(widget) && !GTK_IS_PASSWORD_ENTRY(widget)) {
        return g_strdup(gtk_editable_get_text(GTK_EDITABLE(widget)));
    }
    if (GTK_IS_RANGE(widget)) {
        return g_strdup_printf("%g", gtk_range_get_value(GTK_RANGE(widget)));
    }
    if (GTK_IS_CHECK_BUTTON(widget)) {
        return g_strdup(gtk_check_button_get_active(GTK_CHECK_BUTTON(widget)) ? "1" : "0");
    }
    return g_strdup("");
}

static void gkAppendNode(GString *out, GtkAccessible *element, GtkWidget *origin, int depth, int *count) {
    if (!element || depth > 64 || *count > 20000) {
        return;
    }
    if (GTK_IS_WIDGET(element) && depth > 0) {
        GtkWidget *widget = GTK_WIDGET(element);
        if (!gtk_widget_get_visible(widget) || !gtk_widget_get_child_visible(widget)) {
            return;
        }
    }
    (*count)++;
    GtkWidget *widget = GTK_IS_WIDGET(element) ? GTK_WIDGET(element) : NULL;
    GKView *view = widget ? gkView(widget) : NULL;
    const char *identifier = view && view->accessible_id ? view->accessible_id : "";
    char *name = widget ? gkWidgetName(widget) : g_strdup("");
    char *value = widget ? gkWidgetValue(widget) : g_strdup("");
    graphene_rect_t bounds = GRAPHENE_RECT_INIT(0, 0, 0, 0);
    if (widget && widget != origin) {
        if (!gtk_widget_compute_bounds(widget, origin, &bounds)) {
            bounds = GRAPHENE_RECT_INIT(0, 0, 0, 0);
        }
    } else if (widget) {
        bounds = GRAPHENE_RECT_INIT(0, 0, (float)gtk_widget_get_width(widget), (float)gtk_widget_get_height(widget));
    }
    g_string_append_printf(out, "%d\x1f", depth);
    gkAppendClean(out, gkRoleNick(gtk_accessible_get_accessible_role(element)));
    g_string_append_c(out, '\x1f');
    gkAppendClean(out, widget ? gkSubrole(widget) : "");
    g_string_append_c(out, '\x1f');
    gkAppendClean(out, identifier);
    g_string_append_c(out, '\x1f');
    gkAppendClean(out, name);
    g_string_append_c(out, '\x1f');
    gkAppendClean(out, value);
    g_string_append_printf(out, "\x1f%.0f\x1f%.0f\x1f%.0f\x1f%.0f\x1e", bounds.origin.x, bounds.origin.y,
                           bounds.size.width, bounds.size.height);
    g_free(name);
    g_free(value);
    for (GtkAccessible *child = gtk_accessible_get_first_accessible_child(element); child;) {
        gkAppendNode(out, child, origin, depth + 1, count);
        GtkAccessible *next = gtk_accessible_get_next_accessible_sibling(child);
        g_object_unref(child);
        child = next;
    }
}

char *gkGTKAccessibilityTree(void *pointer, size_t *size) {
    GKWin *win = pointer;
    if (!win) {
        *size = 0;
        return NULL;
    }
    GString *out = g_string_new("");
    int count = 0;
    gkAppendNode(out, GTK_ACCESSIBLE(win->window), win->root, 0, &count);
    *size = out->len;
    char *result = malloc(out->len + 1);
    memcpy(result, out->str, out->len + 1);
    g_string_free(out, TRUE);
    return result;
}

/* ---- Focus ---- */

int gkGTKFocus(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view || !gtk_widget_get_root(view->outer)) {
        return 0;
    }
    GtkWidget *target = view->control ? view->control : view->outer;
    if (!gtk_widget_get_focusable(target) && !GTK_IS_EDITABLE(target) && !GTK_IS_BUTTON(target) &&
        !GTK_IS_CHECK_BUTTON(target)) {
        if (!gtk_widget_get_focusable(view->outer)) {
            return 0;
        }
        target = view->outer;
    }
    gtk_widget_set_can_focus(target, TRUE);
    if (!gtk_widget_grab_focus(target)) {
        return 0;
    }
    return 1;
}

int gkGTKIsFocused(void *pointer) {
    GKView *view = gkView(pointer);
    GtkRoot *root = view ? gtk_widget_get_root(view->outer) : NULL;
    if (!root) {
        return 0;
    }
    GtkWidget *focus = gtk_root_get_focus(root);
    return focus && (focus == view->outer || gtk_widget_is_ancestor(focus, view->outer));
}

void gkGTKAfterPoll(void) {
    GList *toplevels = gtk_window_list_toplevels();
    for (GList *item = toplevels; item; item = item->next) {
        GKWin *win = g_object_get_data(G_OBJECT(item->data), "gk-win");
        if (win) {
            gkGTKTrackFocus(win);
        }
    }
    g_list_free(toplevels);
}

void *gkGTKViewWindow(void *view) {
    GKView *record = gkView(view);
    return record ? gkWinFromWidget(record->outer) : NULL;
}

int gkGTKHandleDragEvent(void *event) {
    (void)event;
    return 0;
}

void gkGTKDisconnect(void *view) {
    GKView *record = gkView(view);
    if (record) {
        record->handle = 0;
    }
}

/* ---- Presentation ---- */

void gkGTKSetTooltip(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (view) {
        gtk_widget_set_tooltip_text(view->outer, text && *text ? text : NULL);
    }
}

static const char *gkCursorName(int cursor) {
    static const char *names[] = {NULL,        "pointer",     "text",        "crosshair", "move", "ew-resize",
                                  "ns-resize", "not-allowed", "wait",        "grab",      "grabbing"};
    return cursor >= 0 && cursor < (int)G_N_ELEMENTS(names) ? names[cursor] : NULL;
}

void gkGTKSetCursor(void *pointer, int cursor) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    const char *name = gkCursorName(cursor);
    gtk_widget_set_cursor_from_name(view->outer, name);
    if (view->control && view->control != view->outer) {
        gtk_widget_set_cursor_from_name(view->control, name);
    }
}

/* gkSemanticColor returns a CSS color for a semantic color. The names are
   the ones that the GTK theme defines. Names such as @window_fg_color come
   from libadwaita, and GTK draws an undefined name as white. */
const char *gkSemanticColor(int color) {
    switch (color) {
    case 1:
        return "@theme_fg_color";
    case 2:
        return "alpha(@theme_fg_color, 0.7)";
    case 3:
        return "alpha(@theme_fg_color, 0.5)";
    case 4:
        return "@theme_base_color";
    case 5:
        return "@theme_selected_bg_color";
    case 6:
        return "@success_color";
    case 7:
        return "@warning_color";
    case 8:
        return "@error_color";
    case 9:
        return "@accent_color";
    default:
        return NULL;
    }
}

int gkFontFamilyInstalled(const char *family) {
    if (!family || !*family || strcmp(family, "System") == 0 || strcmp(family, "Monospace") == 0) {
        return 1;
    }
    PangoFontMap *map = pango_cairo_font_map_get_default();
    PangoFontFamily **families = NULL;
    int count = 0;
    pango_font_map_list_families(map, &families, &count);
    int found = 0;
    for (int i = 0; i < count && !found; i++) {
        found = g_ascii_strcasecmp(pango_font_family_get_name(families[i]), family) == 0;
    }
    g_free(families);
    return found;
}

/* gkFontCSS appends CSS declarations for a font request. */
void gkFontCSS(GString *css, const char *family, double size, int weight, int italic) {
    if (family && *family && strcmp(family, "System") != 0) {
        if (strcmp(family, "Monospace") == 0) {
            g_string_append(css, "font-family: monospace; ");
        } else {
            g_string_append_printf(css, "font-family: \"%s\"; ", family);
        }
    }
    if (size > 0) {
        g_string_append_printf(css, "font-size: %gpx; ", size);
    }
    if (weight > 0) {
        g_string_append_printf(css, "font-weight: %d; ", weight);
    }
    if (italic) {
        g_string_append(css, "font-style: italic; ");
    }
}

static void gkColorCSS(GString *css, const char *property, int semantic, const double *rgba) {
    if (rgba) {
        g_string_append_printf(css, "%s: rgba(%d, %d, %d, %g); ", property, (int)round(rgba[0] * 255),
                               (int)round(rgba[1] * 255), (int)round(rgba[2] * 255), rgba[3]);
        return;
    }
    const char *color = gkSemanticColor(semantic);
    if (color) {
        g_string_append_printf(css, "%s: %s; ", property, color);
    }
}

int gkGTKSetStyle(void *pointer, const char *family, double size, int weight, int italic, int foreground,
                  int background, const double *foreground_rgba, const double *background_rgba,
                  double corner_radius) {
    GKView *view = gkView(pointer);
    if (!view) {
        return 0;
    }
    if (!gkFontFamilyInstalled(family)) {
        return 0;
    }
    const char *name = gkWidgetCSSClass(view);
    GString *css = g_string_new("");
    gkFontCSS(css, family, size, weight, italic);
    gkColorCSS(css, "color", foreground, foreground_rgba);
    int has_background = background_rgba || gkSemanticColor(background);
    if (has_background) {
        gkColorCSS(css, "background-color", background, background_rgba);
        g_string_append(css, "background-image: none; ");
        if (GTK_IS_BUTTON(view->control)) {
            g_string_append(css, "border: none; box-shadow: none; ");
        }
    }
    if (corner_radius > 0) {
        g_string_append_printf(css, "border-radius: %gpx; ", corner_radius);
    }
    char selector[96];
    snprintf(selector, sizeof(selector), ".%s", name);
    gkSetCSSRule(selector, css->str);
    snprintf(selector, sizeof(selector), ".%s:active", name);
    gkSetCSSRule(selector, has_background && GTK_IS_BUTTON(view->control) ? "filter: brightness(1.3);" : NULL);
    /* Text in an entry or a label inherits the color; a text view draws its
       own background. */
    snprintf(selector, sizeof(selector), ".%s text", name);
    GString *text = g_string_new("");
    gkColorCSS(text, "color", foreground, foreground_rgba);
    if (has_background && GTK_IS_TEXT_VIEW(view->control)) {
        gkColorCSS(text, "background-color", background, background_rgba);
    }
    gkSetCSSRule(selector, text->str);
    g_string_free(text, TRUE);
    view->style_set = 1;
    g_string_free(css, TRUE);
    return 1;
}

/* ---- Drag and drop ---- */

typedef struct {
    GdkDragAction actions;
    char *text;
    GPtrArray *files;
    GdkTexture *image;
    GPtrArray *types;
    GPtrArray *data;
} GKDragSource;

static void gkDragSourceFree(gpointer data) {
    GKDragSource *source = data;
    g_free(source->text);
    if (source->files) {
        g_ptr_array_free(source->files, TRUE);
    }
    if (source->image) {
        g_object_unref(source->image);
    }
    if (source->types) {
        g_ptr_array_free(source->types, TRUE);
    }
    if (source->data) {
        g_ptr_array_free(source->data, TRUE);
    }
    g_free(source);
}

static GdkDragAction gkDragActions(int operations) {
    GdkDragAction actions = 0;
    if (operations & 1) {
        actions |= GDK_ACTION_COPY;
    }
    if (operations & 2) {
        actions |= GDK_ACTION_MOVE;
    }
    if (operations & 4) {
        actions |= GDK_ACTION_LINK;
    }
    return actions;
}

static int gkOperationBits(GdkDragAction action) {
    if (action & GDK_ACTION_COPY) {
        return 1;
    }
    if (action & GDK_ACTION_MOVE) {
        return 2;
    }
    if (action & GDK_ACTION_LINK) {
        return 4;
    }
    return 0;
}

static GdkContentProvider *gkDragPrepare(GtkDragSource *controller, double x, double y, gpointer data) {
    (void)controller;
    (void)x;
    (void)y;
    GKDragSource *source = data;
    GPtrArray *providers = g_ptr_array_new();
    if (source->text) {
        g_ptr_array_add(providers, gdk_content_provider_new_typed(G_TYPE_STRING, source->text));
    }
    if (source->files && source->files->len) {
        GSList *files = NULL;
        for (guint i = source->files->len; i > 0; i--) {
            files = g_slist_prepend(files, g_file_new_for_path(g_ptr_array_index(source->files, i - 1)));
        }
        GdkFileList *list = gdk_file_list_new_from_list(files);
        g_ptr_array_add(providers, gdk_content_provider_new_typed(GDK_TYPE_FILE_LIST, list));
        g_boxed_free(GDK_TYPE_FILE_LIST, list);
        g_slist_free_full(files, g_object_unref);
    }
    if (source->image) {
        g_ptr_array_add(providers, gdk_content_provider_new_typed(GDK_TYPE_TEXTURE, source->image));
    }
    for (guint i = 0; source->types && i < source->types->len; i++) {
        g_ptr_array_add(providers, gdk_content_provider_new_for_bytes(g_ptr_array_index(source->types, i),
                                                                      g_ptr_array_index(source->data, i)));
    }
    if (!providers->len) {
        g_ptr_array_free(providers, TRUE);
        return NULL;
    }
    GdkContentProvider *result =
        gdk_content_provider_new_union((GdkContentProvider **)providers->pdata, providers->len);
    g_ptr_array_free(providers, FALSE);
    return result;
}

static void gkDragBegin(GtkDragSource *controller, GdkDrag *drag, gpointer data) {
    (void)drag;
    GtkWidget *widget = gtk_event_controller_get_widget(GTK_EVENT_CONTROLLER(controller));
    GdkPaintable *paintable = gtk_widget_paintable_new(widget);
    gtk_drag_source_set_icon(controller, paintable, 0, 0);
    g_object_unref(paintable);
    (void)data;
}

void gkGTKSetDragSource(void *pointer, int operations, const char *text, const char *files, size_t files_size,
                        const unsigned char *pixels, int width, int height, const char *custom_types,
                        size_t custom_types_size, const unsigned char *custom_data, const size_t *custom_sizes,
                        int custom_count) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    if (view->drag_source) {
        gtk_widget_remove_controller(view->outer, view->drag_source);
        view->drag_source = NULL;
    }
    if (!operations) {
        return;
    }
    GKDragSource *source = g_new0(GKDragSource, 1);
    source->actions = gkDragActions(operations);
    source->text = text && *text ? g_strdup(text) : NULL;
    int count = 0;
    char **paths = gkSplit(files, files_size, &count);
    source->files = g_ptr_array_new_with_free_func(g_free);
    for (int i = 0; i < count; i++) {
        g_ptr_array_add(source->files, g_strdup(paths[i]));
    }
    g_strfreev(paths);
    source->image = gkTextureFromRGBA(pixels, width, height, width * 4);
    int type_count = 0;
    char **types = gkSplit(custom_types, custom_types_size, &type_count);
    source->types = g_ptr_array_new_with_free_func(g_free);
    source->data = g_ptr_array_new_with_free_func((GDestroyNotify)g_bytes_unref);
    size_t offset = 0;
    for (int i = 0; i < custom_count && i < type_count; i++) {
        g_ptr_array_add(source->types, g_strdup(types[i]));
        g_ptr_array_add(source->data, g_bytes_new(custom_data + offset, custom_sizes[i]));
        offset += custom_sizes[i];
    }
    g_strfreev(types);
    GtkDragSource *controller = gtk_drag_source_new();
    gtk_drag_source_set_actions(controller, source->actions);
    g_object_set_data_full(G_OBJECT(controller), "gk-source", source, gkDragSourceFree);
    g_signal_connect(controller, "prepare", G_CALLBACK(gkDragPrepare), source);
    g_signal_connect(controller, "drag-begin", G_CALLBACK(gkDragBegin), source);
    gtk_event_controller_set_propagation_phase(GTK_EVENT_CONTROLLER(controller), GTK_PHASE_CAPTURE);
    gtk_widget_add_controller(view->outer, GTK_EVENT_CONTROLLER(controller));
    view->drag_source = GTK_EVENT_CONTROLLER(controller);
}

typedef struct {
    GKView *view;
    GdkDrop *drop;
    GdkDragAction action;
    double x, y;
    int pending;
    char *text;
    GString *files;
    GdkTexture *image;
    GString *types;
    GByteArray *data;
    GArray *sizes;
    char **custom;
} GKDropRead;

typedef struct {
    GdkDragAction actions;
    char **formats;
} GKDropTarget;

static void gkDropTargetFree(gpointer data) {
    GKDropTarget *target = data;
    g_strfreev(target->formats);
    g_free(target);
}

static void gkDropComplete(GKDropRead *read) {
    if (--read->pending > 0) {
        return;
    }
    uintptr_t handle = read->view ? read->view->handle : 0;
    int width = 0, height = 0;
    unsigned char *pixels = NULL;
    if (read->image) {
        width = gdk_texture_get_width(read->image);
        height = gdk_texture_get_height(read->image);
        pixels = g_malloc((size_t)width * height * 4);
        GdkTextureDownloader *downloader = gdk_texture_downloader_new(read->image);
        gdk_texture_downloader_set_format(downloader, GDK_MEMORY_R8G8B8A8);
        gdk_texture_downloader_download_into(downloader, pixels, (gsize)width * 4);
        gdk_texture_downloader_free(downloader);
    }
    if (handle) {
        gkGTKGoDrop(handle, gkOperationBits(read->action), read->x, read->y, read->text, read->files->str,
                    read->files->len, pixels, width, height, read->types->str, read->types->len, read->data->data,
                    (size_t *)read->sizes->data, (int)read->sizes->len);
    }
    gdk_drop_finish(read->drop, read->action);
    g_object_unref(read->drop);
    g_free(pixels);
    g_free(read->text);
    g_string_free(read->files, TRUE);
    if (read->image) {
        g_object_unref(read->image);
    }
    g_string_free(read->types, TRUE);
    g_byte_array_free(read->data, TRUE);
    g_array_free(read->sizes, TRUE);
    g_strfreev(read->custom);
    g_free(read);
}

static void gkDropTextRead(GObject *source, GAsyncResult *result, gpointer data) {
    GKDropRead *read = data;
    const GValue *value = gdk_drop_read_value_finish(GDK_DROP(source), result, NULL);
    if (value && G_VALUE_HOLDS_STRING(value)) {
        read->text = g_value_dup_string(value);
    }
    gkDropComplete(read);
}

static void gkDropFilesRead(GObject *source, GAsyncResult *result, gpointer data) {
    GKDropRead *read = data;
    const GValue *value = gdk_drop_read_value_finish(GDK_DROP(source), result, NULL);
    if (value && G_VALUE_HOLDS(value, GDK_TYPE_FILE_LIST)) {
        GSList *files = gdk_file_list_get_files(g_value_get_boxed(value));
        for (GSList *item = files; item; item = item->next) {
            char *path = g_file_get_path(item->data);
            if (path) {
                g_string_append_len(read->files, path, (gssize)strlen(path) + 1);
                g_free(path);
            }
        }
        g_slist_free(files);
    }
    gkDropComplete(read);
}

static void gkDropImageRead(GObject *source, GAsyncResult *result, gpointer data) {
    GKDropRead *read = data;
    const GValue *value = gdk_drop_read_value_finish(GDK_DROP(source), result, NULL);
    if (value && G_VALUE_HOLDS(value, GDK_TYPE_TEXTURE)) {
        read->image = g_value_dup_object(value);
    }
    gkDropComplete(read);
}

typedef struct {
    GKDropRead *read;
    char *mime;
    GByteArray *bytes;
    GInputStream *stream;
} GKDropStream;

static void gkDropStreamChunk(GObject *source, GAsyncResult *result, gpointer data) {
    GKDropStream *stream = data;
    GBytes *chunk = g_input_stream_read_bytes_finish(G_INPUT_STREAM(source), result, NULL);
    if (chunk && g_bytes_get_size(chunk) > 0) {
        gsize size = 0;
        const guint8 *bytes = g_bytes_get_data(chunk, &size);
        g_byte_array_append(stream->bytes, bytes, (guint)size);
        g_bytes_unref(chunk);
        g_input_stream_read_bytes_async(stream->stream, 65536, G_PRIORITY_DEFAULT, NULL, gkDropStreamChunk, stream);
        return;
    }
    if (chunk) {
        g_bytes_unref(chunk);
    }
    GKDropRead *read = stream->read;
    g_string_append_len(read->types, stream->mime, (gssize)strlen(stream->mime) + 1);
    g_byte_array_append(read->data, stream->bytes->data, stream->bytes->len);
    size_t length = stream->bytes->len;
    g_array_append_val(read->sizes, length);
    g_object_unref(stream->stream);
    g_byte_array_free(stream->bytes, TRUE);
    g_free(stream->mime);
    g_free(stream);
    gkDropComplete(read);
}

static void gkDropCustomRead(GObject *source, GAsyncResult *result, gpointer data) {
    GKDropStream *stream = data;
    const char *mime = NULL;
    stream->stream = gdk_drop_read_finish(GDK_DROP(source), result, &mime, NULL);
    if (!stream->stream) {
        GKDropRead *read = stream->read;
        g_free(stream->mime);
        g_byte_array_free(stream->bytes, TRUE);
        g_free(stream);
        gkDropComplete(read);
        return;
    }
    g_input_stream_read_bytes_async(stream->stream, 65536, G_PRIORITY_DEFAULT, NULL, gkDropStreamChunk, stream);
}

static GdkDragAction gkDropChoose(GKDropTarget *target, GdkDrop *drop) {
    GdkDragAction allowed = gdk_drop_get_actions(drop) & target->actions;
    if (allowed & GDK_ACTION_COPY) {
        return GDK_ACTION_COPY;
    }
    if (allowed & GDK_ACTION_MOVE) {
        return GDK_ACTION_MOVE;
    }
    if (allowed & GDK_ACTION_LINK) {
        return GDK_ACTION_LINK;
    }
    return 0;
}

static GdkDragAction gkDropMotion(GtkDropTargetAsync *controller, GdkDrop *drop, double x, double y,
                                  gpointer data) {
    (void)controller;
    (void)x;
    (void)y;
    return gkDropChoose(data, drop);
}

static gboolean gkDropPerform(GtkDropTargetAsync *controller, GdkDrop *drop, double x, double y, gpointer data) {
    GKDropTarget *target = data;
    GtkWidget *widget = gtk_event_controller_get_widget(GTK_EVENT_CONTROLLER(controller));
    GdkDragAction action = gkDropChoose(target, drop);
    GKView *view = gkView(widget);
    if (!action || !view || !view->handle) {
        return FALSE;
    }
    GKDropRead *read = g_new0(GKDropRead, 1);
    read->view = view;
    read->drop = g_object_ref(drop);
    read->action = action;
    read->x = x;
    read->y = y;
    read->files = g_string_new("");
    read->types = g_string_new("");
    read->data = g_byte_array_new();
    read->sizes = g_array_new(FALSE, FALSE, sizeof(size_t));
    read->pending = 1;
    GdkContentFormats *formats = gdk_drop_get_formats(drop);
    if (gdk_content_formats_contain_gtype(formats, G_TYPE_STRING)) {
        read->pending++;
        gdk_drop_read_value_async(drop, G_TYPE_STRING, G_PRIORITY_DEFAULT, NULL, gkDropTextRead, read);
    }
    if (gdk_content_formats_contain_gtype(formats, GDK_TYPE_FILE_LIST)) {
        read->pending++;
        gdk_drop_read_value_async(drop, GDK_TYPE_FILE_LIST, G_PRIORITY_DEFAULT, NULL, gkDropFilesRead, read);
    } else if (gdk_content_formats_contain_gtype(formats, GDK_TYPE_TEXTURE)) {
        read->pending++;
        gdk_drop_read_value_async(drop, GDK_TYPE_TEXTURE, G_PRIORITY_DEFAULT, NULL, gkDropImageRead, read);
    }
    for (int i = 0; target->formats && target->formats[i]; i++) {
        const char *mime = target->formats[i];
        if (strcmp(mime, "text/plain") == 0 || strcmp(mime, "text/uri-list") == 0 ||
            !gdk_content_formats_contain_mime_type(formats, mime)) {
            continue;
        }
        GKDropStream *stream = g_new0(GKDropStream, 1);
        stream->read = read;
        stream->mime = g_strdup(mime);
        stream->bytes = g_byte_array_new();
        const char *mimes[] = {mime, NULL};
        read->pending++;
        gdk_drop_read_async(drop, mimes, G_PRIORITY_DEFAULT, NULL, gkDropCustomRead, stream);
    }
    gkDropComplete(read);
    return TRUE;
}

static gboolean gkDropAccept(GtkDropTargetAsync *controller, GdkDrop *drop, gpointer data) {
    (void)controller;
    return gkDropChoose(data, drop) != 0;
}

void gkGTKSetDropTarget(void *pointer, int operations, const char *types, size_t types_size) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    if (view->drop_target) {
        gtk_widget_remove_controller(view->outer, view->drop_target);
        view->drop_target = NULL;
    }
    if (!operations) {
        return;
    }
    GKDropTarget *target = g_new0(GKDropTarget, 1);
    target->actions = gkDragActions(operations);
    int count = 0;
    target->formats = gkSplit(types, types_size, &count);
    GdkContentFormatsBuilder *builder = gdk_content_formats_builder_new();
    for (int i = 0; i < count; i++) {
        if (strcmp(target->formats[i], "text/plain") == 0) {
            gdk_content_formats_builder_add_gtype(builder, G_TYPE_STRING);
        } else if (strcmp(target->formats[i], "text/uri-list") == 0) {
            gdk_content_formats_builder_add_gtype(builder, GDK_TYPE_FILE_LIST);
        }
        gdk_content_formats_builder_add_mime_type(builder, target->formats[i]);
    }
    if (count == 0) {
        gdk_content_formats_builder_add_gtype(builder, G_TYPE_STRING);
        gdk_content_formats_builder_add_gtype(builder, GDK_TYPE_FILE_LIST);
    }
    GdkContentFormats *formats = gdk_content_formats_builder_free_to_formats(builder);
    GtkDropTargetAsync *controller = gtk_drop_target_async_new(formats, target->actions);
    g_object_set_data_full(G_OBJECT(controller), "gk-target", target, gkDropTargetFree);
    g_signal_connect(controller, "accept", G_CALLBACK(gkDropAccept), target);
    g_signal_connect(controller, "drag-enter", G_CALLBACK(gkDropMotion), target);
    g_signal_connect(controller, "drag-motion", G_CALLBACK(gkDropMotion), target);
    g_signal_connect(controller, "drop", G_CALLBACK(gkDropPerform), target);
    gtk_widget_add_controller(view->outer, GTK_EVENT_CONTROLLER(controller));
    view->drop_target = GTK_EVENT_CONTROLLER(controller);
}
