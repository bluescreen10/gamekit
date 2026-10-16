//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>

/* ---- GKImage ----

   GKImage draws a texture with the fit, fill, or stretch scaling of
   ui.ImageScaling and the anchor of ui.Anchor. */

#define GK_TYPE_IMAGE (gk_image_get_type())
G_DECLARE_FINAL_TYPE(GKImage, gk_image, GK, IMAGE, GtkWidget)

struct _GKImage {
    GtkWidget parent_instance;
};

G_DEFINE_FINAL_TYPE(GKImage, gk_image, GTK_TYPE_WIDGET)

static void gk_image_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum, int *natural,
                             int *minimum_baseline, int *natural_baseline) {
    (void)for_size;
    GKView *view = gkView(widget);
    int value = 0;
    if (view && view->texture) {
        double scale = view->source_scale > 0 ? view->source_scale : 1;
        value = (int)ceil((orientation == GTK_ORIENTATION_HORIZONTAL ? gdk_texture_get_width(view->texture)
                                                                     : gdk_texture_get_height(view->texture)) /
                          scale);
    }
    *minimum = 0;
    *natural = value;
    *minimum_baseline = -1;
    *natural_baseline = -1;
}

static void gk_image_snapshot(GtkWidget *widget, GtkSnapshot *snapshot) {
    GKView *view = gkView(widget);
    if (!view || !view->texture) {
        return;
    }
    double width = gtk_widget_get_width(widget);
    double height = gtk_widget_get_height(widget);
    double source_width = gdk_texture_get_width(view->texture);
    double source_height = gdk_texture_get_height(view->texture);
    graphene_rect_t destination = GRAPHENE_RECT_INIT(0, 0, (float)width, (float)height);
    if (view->scaling != 2 && source_width > 0 && source_height > 0) {
        double sx = width / source_width;
        double sy = height / source_height;
        double scale = view->scaling == 1 ? MAX(sx, sy) : MIN(sx, sy);
        double w = source_width * scale;
        double h = source_height * scale;
        /* Anchor order matches ui.Anchor: center, then clockwise from top-left. */
        static const double horizontal[] = {0.5, 0, 0.5, 1, 0, 1, 0, 0.5, 1};
        static const double vertical[] = {0.5, 0, 0, 0, 0.5, 0.5, 1, 1, 1};
        int anchor = view->alignment >= 0 && view->alignment < 9 ? view->alignment : 0;
        destination = GRAPHENE_RECT_INIT((float)((width - w) * horizontal[anchor]),
                                         (float)((height - h) * vertical[anchor]), (float)w, (float)h);
    }
    graphene_rect_t clip = GRAPHENE_RECT_INIT(0, 0, (float)width, (float)height);
    gtk_snapshot_push_clip(snapshot, &clip);
    gtk_snapshot_append_scaled_texture(snapshot, view->texture,
                                       view->nearest ? GSK_SCALING_FILTER_NEAREST : GSK_SCALING_FILTER_TRILINEAR,
                                       &destination);
    gtk_snapshot_pop(snapshot);
}

static void gk_image_class_init(GKImageClass *klass) {
    GtkWidgetClass *widget_class = GTK_WIDGET_CLASS(klass);
    widget_class->measure = gk_image_measure;
    widget_class->snapshot = gk_image_snapshot;
    gtk_widget_class_set_accessible_role(widget_class, GTK_ACCESSIBLE_ROLE_IMG);
}

static void gk_image_init(GKImage *image) {
    (void)image;
}

/* ---- Creation helpers ---- */

static void gkAddToWindow(uintptr_t window, GtkWidget *widget) {
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, widget);
    }
}

static GKView *gkCreate(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind, uintptr_t action) {
    GKView *view = gkViewNew(outer, control, kind, window, action);
    gkAddToWindow(window, outer);
    return view;
}

static void gkActionSignal(gpointer object, gpointer data) {
    (void)object;
    GKView *view = data;
    gkViewAction(view, GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-event")));
}

static void gkInstallAction(GKView *view, GtkWidget *source, const char *signal, int event) {
    g_object_set_data(G_OBJECT(view->outer), "gk-event", GINT_TO_POINTER(event));
    g_signal_connect(source, signal, G_CALLBACK(gkActionSignal), view);
}

uintptr_t gkGTKUIControlHandle(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return 0;
    }
    if (view->kind == GK_KIND_CANVAS) {
        extern Window gkCanvasXWindow(GKView * view);
        return (uintptr_t)gkCanvasXWindow(view);
    }
    return (uintptr_t)control;
}

/* ---- Simple controls ---- */

void *gkGTKUICreateLayout(uintptr_t window) {
    GtkWidget *layout = gkLayoutNew();
    GKView *view = gkCreate(window, layout, NULL, GK_KIND_LAYOUT, 0);
    view->body = layout;
    return layout;
}

void *gkGTKUICreateLabel(uintptr_t window, const char *text) {
    GtkWidget *label = gtk_label_new(text ? text : "");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gkCreate(window, label, NULL, GK_KIND_LABEL, 0);
    return label;
}

void *gkGTKUICreateButton(uintptr_t window, const char *text, uintptr_t handle) {
    GtkWidget *button = gtk_button_new_with_label(text ? text : "");
    GKView *view = gkCreate(window, button, NULL, GK_KIND_BUTTON, handle);
    gkInstallAction(view, button, "clicked", GK_UI_EVENT_CLICK);
    return button;
}

static void gkEntryChanged(GtkEditable *editable, gpointer data) {
    (void)editable;
    gkViewAction(data, GK_UI_EVENT_TEXT_CHANGE);
}

static void gkEntryActivate(GtkEntry *entry, gpointer data) {
    (void)entry;
    gkViewAction(data, GK_UI_EVENT_SUBMIT);
}

/* gkAffixBox puts entry between a prefix and a suffix label, which stay
   hidden while they are empty. */
GtkWidget *gkAffixBox(GtkWidget *entry) {
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
    GtkWidget *prefix = gtk_label_new("");
    GtkWidget *suffix = gtk_label_new("");
    gtk_widget_add_css_class(prefix, "dim-label");
    gtk_widget_add_css_class(suffix, "dim-label");
    gtk_widget_set_visible(prefix, FALSE);
    gtk_widget_set_visible(suffix, FALSE);
    gtk_widget_set_hexpand(entry, TRUE);
    gtk_widget_set_valign(entry, GTK_ALIGN_CENTER);
    gtk_box_append(GTK_BOX(box), prefix);
    gtk_box_append(GTK_BOX(box), entry);
    gtk_box_append(GTK_BOX(box), suffix);
    g_object_set_data(G_OBJECT(box), "gk-prefix", prefix);
    g_object_set_data(G_OBJECT(box), "gk-suffix", suffix);
    return box;
}

void *gkGTKUICreateTextField(uintptr_t window, uintptr_t handle) {
    GtkWidget *entry = gtk_entry_new();
    GtkWidget *box = gkAffixBox(entry);
    GKView *view = gkCreate(window, box, entry, GK_KIND_TEXT_FIELD, handle);
    g_signal_connect(entry, "changed", G_CALLBACK(gkEntryChanged), view);
    g_signal_connect(entry, "activate", G_CALLBACK(gkEntryActivate), view);
    return box;
}

static void gkTextAreaUpdatePlaceholder(GKView *view) {
    if (!view->label) {
        return;
    }
    GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control));
    gboolean empty = gtk_text_buffer_get_char_count(buffer) == 0;
    gtk_label_set_text(GTK_LABEL(view->label), view->placeholder ? view->placeholder : "");
    gtk_widget_set_visible(view->label, empty && view->placeholder && *view->placeholder);
}

static void gkTextBufferChanged(GtkTextBuffer *buffer, gpointer data) {
    (void)buffer;
    GKView *view = data;
    gkTextAreaUpdatePlaceholder(view);
    gkViewAction(view, GK_UI_EVENT_TEXT_CHANGE);
}

void *gkGTKUICreateTextArea(uintptr_t window, const char *text, const char *placeholder, int read_only, int wrap,
                            uintptr_t handle) {
    GtkWidget *overlay = gtk_overlay_new();
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_has_frame(GTK_SCROLLED_WINDOW(scroll), TRUE);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), wrap ? GTK_POLICY_NEVER : GTK_POLICY_AUTOMATIC,
                                   GTK_POLICY_AUTOMATIC);
    GtkWidget *editor = gtk_text_view_new();
    gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(editor), wrap ? GTK_WRAP_WORD_CHAR : GTK_WRAP_NONE);
    gtk_text_view_set_editable(GTK_TEXT_VIEW(editor), !read_only);
    gtk_text_view_set_cursor_visible(GTK_TEXT_VIEW(editor), !read_only);
    gtk_text_view_set_left_margin(GTK_TEXT_VIEW(editor), 4);
    gtk_text_view_set_top_margin(GTK_TEXT_VIEW(editor), 4);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), editor);
    gtk_overlay_set_child(GTK_OVERLAY(overlay), scroll);
    GtkWidget *hint = gtk_label_new("");
    gtk_widget_add_css_class(hint, "dim-label");
    gtk_widget_set_halign(hint, GTK_ALIGN_START);
    gtk_widget_set_valign(hint, GTK_ALIGN_START);
    gtk_widget_set_margin_start(hint, 6);
    gtk_widget_set_margin_top(hint, 5);
    gtk_widget_set_can_target(hint, FALSE);
    gtk_overlay_add_overlay(GTK_OVERLAY(overlay), hint);
    GKView *view = gkCreate(window, overlay, editor, GK_KIND_TEXT_AREA, handle);
    view->body = NULL;
    view->label = hint;
    view->placeholder = gkStrdup(placeholder);
    view->read_only = read_only != 0;
    g_object_set_data(G_OBJECT(overlay), "gk-scroll", scroll);
    GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(editor));
    view->suppress = 1;
    gtk_text_buffer_set_text(buffer, text ? text : "", -1);
    view->suppress = 0;
    g_signal_connect(buffer, "changed", G_CALLBACK(gkTextBufferChanged), view);
    gkTextAreaUpdatePlaceholder(view);
    return overlay;
}

static void gkCheckToggled(GtkCheckButton *button, gpointer data) {
    GKView *view = data;
    if (!view->suppress) {
        gtk_check_button_set_inconsistent(button, FALSE);
    }
    gkViewAction(view, GK_UI_EVENT_TOGGLE_CHANGE);
}

void *gkGTKUICreateCheckBox(uintptr_t window, const char *text, int checked, uintptr_t handle) {
    GtkWidget *check = gtk_check_button_new_with_label(text ? text : "");
    gtk_check_button_set_active(GTK_CHECK_BUTTON(check), checked != 0);
    GKView *view = gkCreate(window, check, NULL, GK_KIND_CHECK_BOX, handle);
    g_signal_connect(check, "toggled", G_CALLBACK(gkCheckToggled), view);
    return check;
}

/* ---- Radio groups ---- */

static void gkRadioToggled(GtkCheckButton *button, gpointer data) {
    GKView *view = data;
    if (!gtk_check_button_get_active(button) || view->suppress) {
        return;
    }
    for (guint i = 0; view->buttons && i < view->buttons->len; i++) {
        if (g_ptr_array_index(view->buttons, i) == button) {
            view->selected = (int)i;
        }
    }
    gkViewAction(view, GK_UI_EVENT_SELECTION_CHANGE);
}

GtkWidget *gkRadioAdd(GKView *view, const char *title, int enabled) {
    GtkWidget *button = gtk_check_button_new_with_label(title ? title : "");
    if (view->buttons->len > 0) {
        gtk_check_button_set_group(GTK_CHECK_BUTTON(button), g_ptr_array_index(view->buttons, 0));
    }
    gtk_widget_set_sensitive(button, enabled);
    g_ptr_array_add(view->buttons, button);
    gtk_box_append(GTK_BOX(view->control), button);
    g_signal_connect(button, "toggled", G_CALLBACK(gkRadioToggled), view);
    return button;
}

void gkRadioSelect(GKView *view, int index) {
    view->suppress++;
    view->selected = index;
    for (guint i = 0; i < view->buttons->len; i++) {
        GtkCheckButton *button = g_ptr_array_index(view->buttons, i);
        if ((int)i == index) {
            gtk_check_button_set_active(button, TRUE);
        } else if (gtk_check_button_get_active(button)) {
            gtk_check_button_set_active(button, FALSE);
        }
    }
    view->suppress--;
}

void *gkGTKUICreateRadioGroup(uintptr_t window, uintptr_t handle) {
    GtkWidget *outer = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_VERTICAL, "spacing", 4,
                                    "accessible-role", GTK_ACCESSIBLE_ROLE_RADIO_GROUP, NULL);
    GtkWidget *title = gtk_label_new("");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_widget_set_visible(title, FALSE);
    GtkWidget *buttons = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    gtk_box_append(GTK_BOX(outer), title);
    gtk_box_append(GTK_BOX(outer), buttons);
    GKView *view = gkCreate(window, outer, buttons, GK_KIND_RADIO_GROUP, handle);
    view->label = title;
    view->buttons = g_ptr_array_new();
    return outer;
}

/* ---- Selects ---- */

static void gkDropDownSelected(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkViewAction(data, GK_UI_EVENT_SELECTION_CHANGE);
}

static int gkChoiceEnabled(GKView *view, guint position) {
    GArray *enabled = g_object_get_data(G_OBJECT(view->outer), "gk-enabled");
    return !enabled || position >= enabled->len || g_array_index(enabled, int, position);
}

static void gkChoiceSetup(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    (void)data;
    GtkWidget *label = gtk_label_new("");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_list_item_set_child(item, label);
}

static void gkChoiceBind(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    GKView *view = data;
    GtkStringObject *string = gtk_list_item_get_item(item);
    GtkWidget *label = gtk_list_item_get_child(item);
    gtk_label_set_text(GTK_LABEL(label), string ? gtk_string_object_get_string(string) : "");
    int enabled = gkChoiceEnabled(view, gtk_list_item_get_position(item));
    gtk_widget_set_sensitive(label, enabled);
    gtk_list_item_set_selectable(item, enabled);
    gtk_list_item_set_activatable(item, enabled);
}

void *gkGTKUICreateSelect(uintptr_t window, uintptr_t handle) {
    GtkStringList *model = gtk_string_list_new(NULL);
    GtkWidget *dropdown = gtk_drop_down_new(G_LIST_MODEL(model), NULL);
    gtk_drop_down_set_selected(GTK_DROP_DOWN(dropdown), GTK_INVALID_LIST_POSITION);
    GKView *view = gkCreate(window, dropdown, NULL, GK_KIND_SELECT, handle);
    GtkListItemFactory *factory = gtk_signal_list_item_factory_new();
    g_signal_connect(factory, "setup", G_CALLBACK(gkChoiceSetup), view);
    g_signal_connect(factory, "bind", G_CALLBACK(gkChoiceBind), view);
    gtk_drop_down_set_list_factory(GTK_DROP_DOWN(dropdown), factory);
    g_object_unref(factory);
    g_signal_connect(dropdown, "notify::selected", G_CALLBACK(gkDropDownSelected), view);
    return dropdown;
}

void gkGTKUISelectionAddItem(void *control, const char *text) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_RADIO_GROUP) {
        gkRadioAdd(view, text, 1);
    } else if (view->kind == GK_KIND_SELECT) {
        GListModel *model = gtk_drop_down_get_model(GTK_DROP_DOWN(view->control));
        view->suppress++;
        guint selected = gtk_drop_down_get_selected(GTK_DROP_DOWN(view->control));
        gtk_string_list_append(GTK_STRING_LIST(model), text ? text : "");
        gtk_drop_down_set_selected(GTK_DROP_DOWN(view->control), selected);
        view->suppress--;
    }
}

int gkGTKUISelectionIndex(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return -1;
    }
    if (view->kind == GK_KIND_RADIO_GROUP) {
        return view->selected;
    }
    if (view->kind == GK_KIND_SELECT) {
        guint selected = gtk_drop_down_get_selected(GTK_DROP_DOWN(view->control));
        return selected == GTK_INVALID_LIST_POSITION ? -1 : (int)selected;
    }
    return -1;
}

void gkGTKUISetSelectionIndex(void *control, int index) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_RADIO_GROUP) {
        gkRadioSelect(view, index);
    } else if (view->kind == GK_KIND_SELECT) {
        view->suppress++;
        gtk_drop_down_set_selected(GTK_DROP_DOWN(view->control), index < 0 ? GTK_INVALID_LIST_POSITION : (guint)index);
        view->suppress--;
    }
}

/* ---- Sliders and indicators ---- */

static void gkRangeChanged(GtkRange *range, gpointer data) {
    (void)range;
    gkViewAction(data, GK_UI_EVENT_VALUE_CHANGE);
}

static gboolean gkRangeChangeValue(GtkRange *range, GtkScrollType scroll, double value, gpointer data) {
    (void)scroll;
    GKView *view = data;
    double step = view->value;
    if (step <= 0) {
        return FALSE;
    }
    GtkAdjustment *adjustment = gtk_range_get_adjustment(range);
    double minimum = gtk_adjustment_get_lower(adjustment);
    double maximum = gtk_adjustment_get_upper(adjustment);
    double snapped = minimum + round((value - minimum) / step) * step;
    snapped = MAX(minimum, MIN(maximum, snapped));
    gtk_range_set_value(range, snapped);
    return TRUE;
}

void *gkGTKUICreateSlider(uintptr_t window, double minimum, double maximum, double value, double step,
                          uintptr_t handle) {
    GtkAdjustment *adjustment =
        gtk_adjustment_new(value, minimum, maximum, step > 0 ? step : (maximum - minimum) / 100, 0, 0);
    GtkWidget *scale = gtk_scale_new(GTK_ORIENTATION_HORIZONTAL, adjustment);
    gtk_scale_set_draw_value(GTK_SCALE(scale), FALSE);
    gtk_widget_set_size_request(scale, 96, -1);
    GKView *view = gkCreate(window, scale, NULL, GK_KIND_SLIDER, handle);
    view->value = step;
    g_signal_connect(scale, "value-changed", G_CALLBACK(gkRangeChanged), view);
    g_signal_connect(scale, "change-value", G_CALLBACK(gkRangeChangeValue), view);
    return scale;
}

void *gkGTKUICreateProgressBar(uintptr_t window, double value) {
    GtkWidget *progress = gtk_progress_bar_new();
    gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(progress), MAX(0, MIN(1, value)));
    gtk_widget_set_valign(progress, GTK_ALIGN_CENTER);
    GKView *view = gkCreate(window, progress, NULL, GK_KIND_PROGRESS, 0);
    view->value = MAX(0, MIN(1, value));
    return progress;
}

void *gkGTKUICreateActivityIndicator(uintptr_t window, int running) {
    GtkWidget *spinner = gtk_spinner_new();
    gtk_spinner_set_spinning(GTK_SPINNER(spinner), running != 0);
    gtk_widget_set_size_request(spinner, 16, 16);
    GKView *view = gkCreate(window, spinner, NULL, GK_KIND_ACTIVITY, 0);
    view->running = running != 0;
    return spinner;
}

void *gkGTKUICreateSeparator(uintptr_t window, int direction) {
    GtkOrientation orientation = direction == 1 ? GTK_ORIENTATION_VERTICAL : GTK_ORIENTATION_HORIZONTAL;
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", orientation, "spacing", 6, "accessible-role",
                                  GTK_ACCESSIBLE_ROLE_SEPARATOR, NULL);
    GtkWidget *start = gtk_separator_new(orientation);
    GtkWidget *label = gtk_label_new("");
    GtkWidget *end = gtk_separator_new(orientation);
    gtk_widget_add_css_class(label, "dim-label");
    gtk_widget_add_css_class(label, "caption");
    gtk_widget_set_visible(label, FALSE);
    gtk_widget_set_visible(end, FALSE);
    gtk_widget_set_hexpand(start, orientation == GTK_ORIENTATION_HORIZONTAL);
    gtk_widget_set_vexpand(start, orientation == GTK_ORIENTATION_VERTICAL);
    gtk_widget_set_hexpand(end, orientation == GTK_ORIENTATION_HORIZONTAL);
    gtk_widget_set_vexpand(end, orientation == GTK_ORIENTATION_VERTICAL);
    gtk_widget_set_valign(start, orientation == GTK_ORIENTATION_HORIZONTAL ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    gtk_widget_set_halign(start, orientation == GTK_ORIENTATION_VERTICAL ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    gtk_widget_set_valign(end, orientation == GTK_ORIENTATION_HORIZONTAL ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    gtk_widget_set_halign(end, orientation == GTK_ORIENTATION_VERTICAL ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    gtk_box_append(GTK_BOX(box), start);
    gtk_box_append(GTK_BOX(box), label);
    gtk_box_append(GTK_BOX(box), end);
    GKView *view = gkCreate(window, box, start, GK_KIND_SEPARATOR, 0);
    view->label = label;
    view->vertical = direction == 1;
    g_object_set_data(G_OBJECT(box), "gk-end", end);
    return box;
}

/* ---- Images ---- */

void *gkGTKUICreateImage(uintptr_t window, int scaling) {
    GtkWidget *image = g_object_new(GK_TYPE_IMAGE, NULL);
    GKView *view = gkCreate(window, image, NULL, GK_KIND_IMAGE, 0);
    view->scaling = scaling;
    return image;
}

void gkGTKUIImageSet(void *control, const unsigned char *pixels, int width, int height, int stride) {
    GKView *view = gkView(control);
    if (!view || !pixels || width <= 0 || height <= 0) {
        return;
    }
    GdkTexture *texture = gkTextureFromRGBA(pixels, width, height, stride);
    if (view->texture) {
        g_object_unref(view->texture);
    }
    view->texture = texture;
    gtk_widget_queue_resize(view->outer);
    gtk_widget_queue_draw(view->outer);
}

int gkGTKUIImageScaling(void *control) {
    GKView *view = gkView(control);
    return view ? view->scaling : 0;
}

void gkGTKUIImageSetScaling(void *control, int scaling) {
    GKView *view = gkView(control);
    if (view) {
        view->scaling = scaling;
        gtk_widget_queue_draw(view->outer);
    }
}

/* ---- Links ---- */

static gboolean gkLinkActivate(GtkLinkButton *button, gpointer data) {
    (void)button;
    gkViewAction(data, GK_UI_EVENT_CLICK);
    return TRUE;
}

void *gkGTKUICreateLink(uintptr_t window, const char *text, const char *url, uintptr_t handle) {
    GtkWidget *link = gtk_link_button_new_with_label(url && *url ? url : "about:blank", text ? text : "");
    gtk_widget_set_halign(link, GTK_ALIGN_START);
    GKView *view = gkCreate(window, link, NULL, GK_KIND_LINK, handle);
    view->url = gkStrdup(url);
    g_signal_connect(link, "activate-link", G_CALLBACK(gkLinkActivate), view);
    return link;
}

char *gkGTKUILinkURL(void *control) {
    GKView *view = gkView(control);
    return gkStrdup(view ? view->url : "");
}

void gkGTKUISetLinkURL(void *control, const char *url) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    free(view->url);
    view->url = gkStrdup(url);
    gtk_link_button_set_uri(GTK_LINK_BUTTON(view->outer), url && *url ? url : "about:blank");
}

/* ---- Tabs ---- */

static void gkTabSwitched(GtkNotebook *notebook, GtkWidget *page, guint index, gpointer data) {
    (void)notebook;
    (void)page;
    (void)index;
    gkViewAction(data, GK_UI_EVENT_TAB_CHANGE);
}

void *gkGTKUICreateTabs(uintptr_t window, uintptr_t handle) {
    GtkWidget *notebook = gtk_notebook_new();
    GKView *view = gkCreate(window, notebook, NULL, GK_KIND_TABS, handle);
    g_signal_connect_after(notebook, "switch-page", G_CALLBACK(gkTabSwitched), view);
    return notebook;
}

int gkGTKUITabsAdd(void *tabs, const char *title, void *content) {
    GKView *view = gkView(tabs);
    if (!view || !content) {
        return 0;
    }
    GtkWidget *page = gkLayoutNew();
    gkLayoutPut(page, content);
    view->suppress++;
    int index = gtk_notebook_append_page(GTK_NOTEBOOK(view->outer), page, gtk_label_new(title ? title : ""));
    view->suppress--;
    return index >= 0;
}

int gkGTKUITabsSelected(void *tabs) {
    GKView *view = gkView(tabs);
    return view ? gtk_notebook_get_current_page(GTK_NOTEBOOK(view->outer)) : -1;
}

void gkGTKUITabsSetSelected(void *tabs, int index) {
    GKView *view = gkView(tabs);
    if (!view || index < 0 || index >= gtk_notebook_get_n_pages(GTK_NOTEBOOK(view->outer))) {
        return;
    }
    view->suppress++;
    gtk_notebook_set_current_page(GTK_NOTEBOOK(view->outer), index);
    view->suppress--;
}

/* gkChromeSize returns how much of the widget's frame its own decoration
   takes: the minimum size of the widget when its pages are empty. */
static void gkChromeSize(GtkWidget *widget, int *width, int *height) {
    int min_width = 0, min_height = 0;
    gtk_widget_measure(widget, GTK_ORIENTATION_HORIZONTAL, -1, &min_width, NULL, NULL, NULL);
    gtk_widget_measure(widget, GTK_ORIENTATION_VERTICAL, -1, &min_height, NULL, NULL, NULL);
    *width = min_width;
    *height = min_height;
}

void gkGTKUITabsContentSize(void *tabs, int *width, int *height) {
    GKView *view = gkView(tabs);
    int w = 0, h = 0;
    if (view) {
        int page_index = gtk_notebook_get_current_page(GTK_NOTEBOOK(view->outer));
        GtkWidget *page = page_index >= 0 ? gtk_notebook_get_nth_page(GTK_NOTEBOOK(view->outer), page_index) : NULL;
        int allocated_width = gtk_widget_get_width(view->outer);
        int allocated_height = gtk_widget_get_height(view->outer);
        if (page && allocated_width == view->width && allocated_height == view->height &&
            gtk_widget_get_width(page) > 0) {
            w = gtk_widget_get_width(page);
            h = gtk_widget_get_height(page);
        } else {
            int chrome_width, chrome_height;
            gkChromeSize(view->outer, &chrome_width, &chrome_height);
            int position = gtk_notebook_get_tab_pos(GTK_NOTEBOOK(view->outer));
            int horizontal_tabs = position == GTK_POS_TOP || position == GTK_POS_BOTTOM;
            w = MAX(0, view->width - (horizontal_tabs ? 2 : chrome_width));
            h = MAX(0, view->height - (horizontal_tabs ? chrome_height : 2));
        }
    }
    *width = w;
    *height = h;
}

/* ---- Scroll views ---- */

void *gkGTKUICreateScrollView(uintptr_t window, int axes) {
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_has_frame(GTK_SCROLLED_WINDOW(scroll), TRUE);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll),
                                   axes == 1 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER,
                                   axes == 0 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER);
    GtkWidget *body = gkLayoutNew();
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), body);
    GKView *view = gkCreate(window, scroll, NULL, GK_KIND_SCROLL, 0);
    view->body = body;
    view->axes = axes;
    return scroll;
}

void gkGTKUIScrollViewportSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0;
    if (view) {
        int chrome_width, chrome_height;
        gkChromeSize(view->outer, &chrome_width, &chrome_height);
        /* A frame of 1 pixel on each side surrounds the viewport. */
        w = MAX(0, view->width - 2);
        h = MAX(0, view->height - 2);
    }
    *width = w;
    *height = h;
}

static void gkScrollConfigure(GKView *view, GtkAdjustment *adjustment, double content, double page) {
    double value = gtk_adjustment_get_value(adjustment);
    gtk_adjustment_configure(adjustment, MAX(0, MIN(value, content - page)), 0, MAX(content, page), page * 0.1,
                             page * 0.9, page);
    (void)view;
}

void gkGTKUIScrollSetContentSize(void *pointer, int width, int height) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    gkLayoutSetMinimum(view->body, width, height);
    int viewport_width, viewport_height;
    gkGTKUIScrollViewportSize(pointer, &viewport_width, &viewport_height);
    GtkScrolledWindow *scroll = GTK_SCROLLED_WINDOW(view->outer);
    gkScrollConfigure(view, gtk_scrolled_window_get_hadjustment(scroll), width, viewport_width);
    gkScrollConfigure(view, gtk_scrolled_window_get_vadjustment(scroll), height, viewport_height);
}

void gkGTKUIScrollOffset(void *pointer, int *x, int *y) {
    GKView *view = gkView(pointer);
    double ox = 0, oy = 0;
    if (view) {
        GtkScrolledWindow *scroll = GTK_SCROLLED_WINDOW(view->outer);
        ox = gtk_adjustment_get_value(gtk_scrolled_window_get_hadjustment(scroll));
        oy = gtk_adjustment_get_value(gtk_scrolled_window_get_vadjustment(scroll));
    }
    *x = (int)floor(ox);
    *y = (int)floor(oy);
}

/* ---- Canvas ---- */

typedef struct {
    GKView *view;
} GKCanvas;

Window gkCanvasXWindow(GKView *view);

static void gkCanvasSyncXWindow(GKView *view) {
    if (!view || !view->xwindow) {
        return;
    }
    GtkNative *native = gtk_widget_get_native(view->outer);
    GdkSurface *surface = native ? gtk_native_get_surface(native) : NULL;
    if (!surface || !GDK_IS_X11_SURFACE(surface)) {
        return;
    }
    graphene_rect_t bounds;
    int x = 0, y = 0, width = 1, height = 1;
    int scale = gtk_widget_get_scale_factor(view->outer);
    if (gtk_widget_compute_bounds(view->outer, GTK_WIDGET(native), &bounds)) {
        double offset_x = 0, offset_y = 0;
        gtk_native_get_surface_transform(native, &offset_x, &offset_y);
        x = (int)((bounds.origin.x + offset_x) * scale);
        y = (int)((bounds.origin.y + offset_y) * scale);
        width = MAX(1, (int)(bounds.size.width * scale));
        height = MAX(1, (int)(bounds.size.height * scale));
    }
    XMoveResizeWindow(view->xdisplay, view->xwindow, x, y, (unsigned)width, (unsigned)height);
    gboolean mapped = gtk_widget_get_mapped(view->outer) && gtk_widget_get_width(view->outer) > 0;
    if (mapped && !view->xmapped) {
        XMapWindow(view->xdisplay, view->xwindow);
        view->xmapped = 1;
    } else if (!mapped && view->xmapped) {
        XUnmapWindow(view->xdisplay, view->xwindow);
        view->xmapped = 0;
    }
    XFlush(view->xdisplay);
}

Window gkCanvasXWindow(GKView *view) {
    if (!view) {
        return 0;
    }
    if (!view->xwindow) {
        GKWin *win = gkWinFromWidget(view->outer);
        if (!win) {
            win = view->window;
        }
        Display *display = gkWinXDisplay(win);
        Window parent = gkWinXWindow(win);
        if (!display || !parent) {
            return 0;
        }
        view->xdisplay = display;
        view->xwindow = XCreateSimpleWindow(display, parent, 0, 0, 1, 1, 0, 0, 0);
        XSelectInput(display, view->xwindow, 0);
        gkCanvasSyncXWindow(view);
    }
    return view->xwindow;
}

uintptr_t gkGTKUICanvasXDisplay(void *canvas) {
    GKView *view = gkView(canvas);
    if (!view) {
        return 0;
    }
    gkCanvasXWindow(view);
    return (uintptr_t)view->xdisplay;
}

static void gkCanvasSend(GKView *view, int kind, int a, int b, int c, double x, double y, double z, double w) {
    if (!view->action) {
        return;
    }
    GKGTKEvent event = {0};
    event.kind = kind;
    event.a = a;
    event.b = b;
    event.c = c;
    event.x = x;
    event.y = y;
    event.z = z;
    event.w = w;
    gkGTKGoEvent(view->action, &event);
}

static void gkCanvasButton(GKView *view, guint native_button, int action, GdkModifierType state) {
    int button = gkPointerButton(native_button);
    if (!view->action || button < 0 || button >= GK_POINTER_BUTTON_COUNT) {
        return;
    }
    unsigned int mask = 1u << button;
    if (action == GK_POINTER_PRESSED) {
        view->canvas_buttons |= (int)mask;
    } else {
        if (!(view->canvas_buttons & (int)mask)) {
            return;
        }
        view->canvas_buttons &= ~(int)mask;
    }
    gkGTKUIGoCanvasPointerButton(view->action, button, action, gkModifiers(state));
}

static void gkCanvasPressed(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKView *view = data;
    guint button = gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture));
    GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
    if (gtk_widget_get_focusable(view->outer)) {
        gtk_widget_grab_focus(view->outer);
    }
    gkCanvasButton(view, button, GK_POINTER_PRESSED, state);
}

static void gkCanvasReleased(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)x;
    (void)y;
    GKView *view = data;
    guint button = gtk_gesture_single_get_current_button(GTK_GESTURE_SINGLE(gesture));
    GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(gesture));
    gkCanvasButton(view, button, GK_POINTER_RELEASED, state);
}

static void gkCanvasCancelled(GtkGesture *gesture, GdkEventSequence *sequence, gpointer data) {
    (void)gesture;
    (void)sequence;
    GKView *view = data;
    for (int button = 0; button < GK_POINTER_BUTTON_COUNT; button++) {
        if ((view->canvas_buttons & (1 << button)) && view->action) {
            gkGTKUIGoCanvasPointerButton(view->action, button, GK_POINTER_RELEASED, 0);
        }
    }
    view->canvas_buttons = 0;
}

static gboolean gkCanvasScroll(GtkEventControllerScroll *controller, double dx, double dy, gpointer data) {
    (void)controller;
    GKView *view = data;
    if (view->action) {
        gkGTKUIGoCanvasScroll(view->action, -dx, -dy);
    }
    return TRUE;
}

static void gkCanvasMotion(GtkEventControllerMotion *controller, double x, double y, gpointer data) {
    GKView *view = data;
    GdkModifierType state = gtk_event_controller_get_current_event_state(GTK_EVENT_CONTROLLER(controller));
    double *last = g_object_get_data(G_OBJECT(view->outer), "gk-last");
    double delta_x = last ? x - last[0] : 0, delta_y = last ? y - last[1] : 0;
    if (!last) {
        last = g_new0(double, 2);
        g_object_set_data_full(G_OBJECT(view->outer), "gk-last", last, g_free);
    }
    last[0] = x;
    last[1] = y;
    gkCanvasSend(view, GK_EVENT_POINTER_MOVE, gkModifiers(state), 0, 0, x, y, delta_x, delta_y);
}

static void gkCanvasEnter(GtkEventControllerMotion *controller, double x, double y, gpointer data) {
    (void)controller;
    gkCanvasSend(data, GK_EVENT_POINTER_ENTER, 0, 0, 0, x, y, 0, 0);
}

static void gkCanvasLeave(GtkEventControllerMotion *controller, gpointer data) {
    (void)controller;
    GKView *view = data;
    double x = 0, y = 0;
    gkGTKUICanvasPointerPosition(view->outer, &x, &y);
    g_object_set_data(G_OBJECT(view->outer), "gk-last", NULL);
    gkCanvasSend(view, GK_EVENT_POINTER_LEAVE, 0, 0, 0, x, y, 0, 0);
}

static gboolean gkCanvasKeyPressed(GtkEventControllerKey *controller, guint keyval, guint keycode,
                                   GdkModifierType state, gpointer data) {
    (void)controller;
    GKView *view = data;
    int key = gkKeyFromKeycode(keycode);
    if (key >= 0) {
        gkCanvasSend(view, GK_EVENT_KEY, key, (int)keycode, GK_KEY_PRESSED, gkModifiers(state), 0, 0, 0);
    }
    gunichar c = gdk_keyval_to_unicode(keyval);
    if (c >= 0x20 && c != 0x7f && !(state & (GDK_CONTROL_MASK | GDK_ALT_MASK | GDK_SUPER_MASK))) {
        gkCanvasSend(view, GK_EVENT_TEXT, (int)c, 0, 0, 0, 0, 0, 0);
    }
    return TRUE;
}

static void gkCanvasKeyReleased(GtkEventControllerKey *controller, guint keyval, guint keycode,
                                GdkModifierType state, gpointer data) {
    (void)controller;
    (void)keyval;
    GKView *view = data;
    int key = gkKeyFromKeycode(keycode);
    if (key >= 0) {
        gkCanvasSend(view, GK_EVENT_KEY, key, (int)keycode, GK_KEY_RELEASED, gkModifiers(state), 0, 0, 0);
    }
}

static gboolean gkCanvasTick(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    (void)widget;
    GKView *view = data;
    gkCanvasSyncXWindow(view);
    if (view->frames & 1) {
        gint64 now = g_get_monotonic_time();
        gint64 interval = 0, presentation = 0;
        gdk_frame_clock_get_refresh_info(clock, gdk_frame_clock_get_frame_time(clock), &interval, &presentation);
        double until = presentation > now ? (presentation - now) / 1e6 : 0;
        double period = interval > 0 ? interval / 1e6 : 1.0 / 60;
        gkCanvasSend(view, GK_EVENT_FRAME, 0, 0, 0, until, period, 0, 0);
        gtk_widget_queue_draw(view->outer);
    }
    return G_SOURCE_CONTINUE;
}

static void gkCanvasReportSurface(GKView *view, int kind) {
    int scale = gtk_widget_get_scale_factor(view->outer);
    gkCanvasSend(view, GK_EVENT_SURFACE, kind, 1, 0, view->width * scale, view->height * scale, scale, 0);
}

static void gkCanvasScaleChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkCanvasReportSurface(data, 1);
}

static void gkCanvasMap(GtkWidget *widget, gpointer data) {
    (void)widget;
    GKView *view = data;
    gkCanvasSyncXWindow(view);
    gkCanvasSend(view, GK_EVENT_WINDOW_OCCLUSION, 0, 0, 0, 0, 0, 0, 0);
}

static void gkCanvasUnmap(GtkWidget *widget, gpointer data) {
    (void)widget;
    GKView *view = data;
    gkCanvasSyncXWindow(view);
    gkCanvasSend(view, GK_EVENT_WINDOW_OCCLUSION, 1, 0, 0, 0, 0, 0, 0);
}

void *gkGTKUICreateCanvas(uintptr_t window, uintptr_t handle) {
    GtkWidget *area = gkLayoutNewWithRole(GTK_ACCESSIBLE_ROLE_APPLICATION);
    gtk_widget_set_focusable(area, TRUE);
    GKView *view = gkCreate(window, area, NULL, GK_KIND_CANVAS, handle);
    GtkGesture *click = gtk_gesture_click_new();
    gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(click), 0);
    g_signal_connect(click, "pressed", G_CALLBACK(gkCanvasPressed), view);
    g_signal_connect(click, "released", G_CALLBACK(gkCanvasReleased), view);
    g_signal_connect(click, "cancel", G_CALLBACK(gkCanvasCancelled), view);
    gtk_widget_add_controller(area, GTK_EVENT_CONTROLLER(click));
    GtkEventController *scroll = gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_BOTH_AXES);
    g_signal_connect(scroll, "scroll", G_CALLBACK(gkCanvasScroll), view);
    gtk_widget_add_controller(area, scroll);
    GtkEventController *motion = gtk_event_controller_motion_new();
    g_signal_connect(motion, "motion", G_CALLBACK(gkCanvasMotion), view);
    g_signal_connect(motion, "enter", G_CALLBACK(gkCanvasEnter), view);
    g_signal_connect(motion, "leave", G_CALLBACK(gkCanvasLeave), view);
    gtk_widget_add_controller(area, motion);
    GtkEventController *keys = gtk_event_controller_key_new();
    g_signal_connect(keys, "key-pressed", G_CALLBACK(gkCanvasKeyPressed), view);
    g_signal_connect(keys, "key-released", G_CALLBACK(gkCanvasKeyReleased), view);
    gtk_widget_add_controller(area, keys);
    view->tick = gtk_widget_add_tick_callback(area, gkCanvasTick, view, NULL);
    g_signal_connect(area, "notify::scale-factor", G_CALLBACK(gkCanvasScaleChanged), view);
    g_signal_connect(area, "map", G_CALLBACK(gkCanvasMap), view);
    g_signal_connect(area, "unmap", G_CALLBACK(gkCanvasUnmap), view);
    return area;
}

void gkGTKUICanvasPointerPosition(void *canvas, double *x, double *y) {
    GKView *view = gkView(canvas);
    double px = 0, py = 0;
    GtkNative *native = view ? gtk_widget_get_native(view->outer) : NULL;
    GdkSurface *surface = native ? gtk_native_get_surface(native) : NULL;
    if (surface) {
        GdkDisplay *display = gdk_surface_get_display(surface);
        GdkSeat *seat = gdk_display_get_default_seat(display);
        GdkDevice *device = seat ? gdk_seat_get_pointer(seat) : NULL;
        double sx = 0, sy = 0;
        if (device && gdk_surface_get_device_position(surface, device, &sx, &sy, NULL)) {
            double offset_x = 0, offset_y = 0;
            gtk_native_get_surface_transform(native, &offset_x, &offset_y);
            graphene_point_t source = GRAPHENE_POINT_INIT((float)(sx - offset_x), (float)(sy - offset_y));
            graphene_point_t destination = source;
            if (gtk_widget_compute_point(GTK_WIDGET(native), view->outer, &source, &destination)) {
                px = destination.x;
                py = destination.y;
            }
        }
    }
    *x = px;
    *y = py;
}

int gkGTKCanvasStartFrames(void *pointer, int start) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_CANVAS) {
        return 0;
    }
    if (start) {
        view->frames |= 1;
        gtk_widget_queue_draw(view->outer);
    } else {
        view->frames &= ~1;
    }
    return 1;
}

void gkGTKCanvasAttach(void *pointer, uintptr_t handle) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_CANVAS) {
        view->action = handle;
    }
}

gboolean gkGTKCaptureDeliver(GtkWidget *canvas, GKWin *win, GdkEvent *event) {
    (void)win;
    GKView *view = gkView(canvas);
    if (!view || !view->action) {
        return FALSE;
    }
    GdkEventType type = gdk_event_get_event_type(event);
    GtkNative *native = gtk_widget_get_native(canvas);
    double sx = 0, sy = 0;
    gdk_event_get_position(event, &sx, &sy);
    double offset_x = 0, offset_y = 0;
    if (native) {
        gtk_native_get_surface_transform(native, &offset_x, &offset_y);
    }
    graphene_point_t source = GRAPHENE_POINT_INIT((float)(sx - offset_x), (float)(sy - offset_y));
    graphene_point_t point = source;
    if (native) {
        (void)!gtk_widget_compute_point(GTK_WIDGET(native), canvas, &source, &point);
    }
    GdkModifierType state = gdk_event_get_modifier_state(event);
    graphene_rect_t bounds = GRAPHENE_RECT_INIT(0, 0, (float)gtk_widget_get_width(canvas),
                                                (float)gtk_widget_get_height(canvas));
    gboolean inside = graphene_rect_contains_point(&bounds, &point);
    switch (type) {
    case GDK_BUTTON_PRESS:
    case GDK_BUTTON_RELEASE:
        if (inside) {
            /* The canvas's own controllers report events inside it. */
            return FALSE;
        }
        gkCanvasButton(view, gdk_button_event_get_button(event),
                       type == GDK_BUTTON_PRESS ? GK_POINTER_PRESSED : GK_POINTER_RELEASED, state);
        return TRUE;
    case GDK_MOTION_NOTIFY:
        if (inside) {
            return FALSE;
        }
        gkCanvasSend(view, GK_EVENT_POINTER_MOVE, gkModifiers(state), 0, 0, point.x, point.y, 0, 0);
        return TRUE;
    case GDK_SCROLL:
        if (inside) {
            return FALSE;
        }
        {
            double dx = 0, dy = 0;
            gdk_scroll_event_get_deltas(event, &dx, &dy);
            gkGTKUIGoCanvasScroll(view->action, -dx, -dy);
        }
        return TRUE;
    default:
        return FALSE;
    }
}

/* ---- Sections ---- */

typedef struct {
    GtkWidget *header;
    GtkWidget *revealer;
    GtkWidget *arrow;
    GtkWidget *title;
    GtkWidget *subtitle;
    GtkWidget *badge;
    GtkWidget *icon;
    int expanded;
} GKSection;

static void gkSectionUpdate(GKView *view) {
    GKSection *section = view->ext;
    gtk_image_set_from_icon_name(GTK_IMAGE(section->arrow), section->expanded ? "pan-down-symbolic" : "pan-end-symbolic");
    gtk_revealer_set_transition_duration(GTK_REVEALER(section->revealer), view->animated ? 200 : 0);
    gtk_revealer_set_reveal_child(GTK_REVEALER(section->revealer), section->expanded);
    gtk_accessible_update_state(GTK_ACCESSIBLE(section->header), GTK_ACCESSIBLE_STATE_EXPANDED, section->expanded, -1);
}

static void gkSectionArrange(GtkWidget *layout, int width, int height, gpointer data) {
    (void)layout;
    GKView *view = data;
    GKSection *section = view->ext;
    gkLayoutMove(section->header, 0, 0, width, 28);
    gkLayoutMove(section->revealer, 0, 28, width, MAX(0, height - 28));
}

static void gkSectionToggle(GtkButton *button, gpointer data) {
    (void)button;
    GKView *view = data;
    GKSection *section = view->ext;
    section->expanded = !section->expanded;
    gkSectionUpdate(view);
    gkViewAction(view, GK_UI_EVENT_SECTION_CHANGE);
}

void *gkGTKUICreateSection(uintptr_t window, const char *title, int expanded, uintptr_t handle) {
    GtkWidget *outer = gkLayoutNew();
    GKSection *section = g_new0(GKSection, 1);
    section->header = gtk_button_new();
    gtk_widget_add_css_class(section->header, "flat");
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    section->arrow = gtk_image_new();
    section->icon = gtk_image_new();
    gtk_widget_set_visible(section->icon, FALSE);
    section->title = gtk_label_new(title ? title : "");
    gtk_label_set_xalign(GTK_LABEL(section->title), 0);
    section->subtitle = gtk_label_new("");
    gtk_widget_add_css_class(section->subtitle, "dim-label");
    gtk_widget_set_visible(section->subtitle, FALSE);
    section->badge = gtk_label_new("");
    gtk_widget_set_hexpand(section->subtitle, TRUE);
    gtk_label_set_xalign(GTK_LABEL(section->subtitle), 0);
    gtk_widget_set_visible(section->badge, FALSE);
    gtk_widget_add_css_class(section->badge, "dim-label");
    gtk_box_append(GTK_BOX(row), section->arrow);
    gtk_box_append(GTK_BOX(row), section->icon);
    gtk_box_append(GTK_BOX(row), section->title);
    gtk_box_append(GTK_BOX(row), section->subtitle);
    gtk_box_append(GTK_BOX(row), section->badge);
    gtk_button_set_child(GTK_BUTTON(section->header), row);
    gtk_widget_set_parent(section->header, outer);
    gkLayoutMove(section->header, 0, 0, 160, 28);
    GtkWidget *body = gkLayoutNew();
    section->revealer = gtk_revealer_new();
    gtk_revealer_set_transition_type(GTK_REVEALER(section->revealer), GTK_REVEALER_TRANSITION_TYPE_SLIDE_DOWN);
    gtk_revealer_set_child(GTK_REVEALER(section->revealer), body);
    gtk_widget_set_parent(section->revealer, outer);
    GKView *view = gkCreate(window, outer, section->header, GK_KIND_SECTION, handle);
    view->body = body;
    view->ext = section;
    view->ext_free = g_free;
    section->expanded = expanded != 0;
    gkLayoutOnAllocate(outer, gkSectionArrange, view);
    g_signal_connect(section->header, "clicked", G_CALLBACK(gkSectionToggle), view);
    gkSectionUpdate(view);
    return outer;
}

GtkWidget *gkSectionPart(void *control, int part) {
    GKView *view = gkView(control);
    if (!view || view->kind != GK_KIND_SECTION) {
        return NULL;
    }
    GKSection *section = view->ext;
    switch (part) {
    case 0:
        return section->title;
    case 1:
        return section->subtitle;
    case 2:
        return section->badge;
    case 3:
        return section->icon;
    default:
        return section->header;
    }
}

int gkGTKUISectionExpanded(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_SECTION && ((GKSection *)view->ext)->expanded;
}

void gkGTKSectionSetAnimated(void *control, int animated) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_SECTION) {
        view->animated = animated != 0;
        gkSectionUpdate(view);
    }
}

void gkGTKSectionConfigure(void *control, const char *subtitle, void *icon, const char *badge) {
    GKView *view = gkView(control);
    if (!view || view->kind != GK_KIND_SECTION) {
        return;
    }
    GKSection *section = view->ext;
    gtk_label_set_text(GTK_LABEL(section->subtitle), subtitle ? subtitle : "");
    gtk_widget_set_visible(section->subtitle, subtitle && *subtitle);
    gtk_label_set_text(GTK_LABEL(section->badge), badge ? badge : "");
    gtk_widget_set_visible(section->badge, badge && *badge);
    if (icon) {
        gtk_image_set_from_paintable(GTK_IMAGE(section->icon), icon);
    } else {
        gtk_image_clear(GTK_IMAGE(section->icon));
    }
    gtk_widget_set_visible(section->icon, icon != NULL);
}

void gkGTKUISectionSetExpanded(void *control, int expanded) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_SECTION) {
        ((GKSection *)view->ext)->expanded = expanded != 0;
        gkSectionUpdate(view);
    }
}

/* ---- Common operations ---- */

static void gkCanvasDestroy(GKView *view) {
    if (view->tick) {
        gtk_widget_remove_tick_callback(view->outer, view->tick);
        view->tick = 0;
    }
    if (view->xwindow && view->xdisplay) {
        XDestroyWindow(view->xdisplay, view->xwindow);
        XFlush(view->xdisplay);
        view->xwindow = 0;
    }
}

void gkGTKUIDestroy(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_CANVAS) {
        gkCanvasDestroy(view);
    }
    GtkWidget *outer = view->outer;
    gkRemoveFromParent(outer);
    g_object_unref(outer);
}

void gkGTKUIClearAction(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->action = 0;
    if (view->kind == GK_KIND_CANVAS) {
        view->frames = 0;
        gkGTKReleaseCaptureFor(control);
    }
}

void gkGTKUIAttachToWindow(uintptr_t window, void *control) {
    if (window && control) {
        gkAddToWindow(window, control);
    }
}

void gkGTKUISetParent(void *control, void *parent) {
    GKView *view = gkView(control);
    GKView *container = gkView(parent);
    if (!view || !container) {
        return;
    }
    GtkWidget *target = gkViewContainer(container);
    if (target && GK_IS_LAYOUT(target)) {
        gkLayoutPut(target, view->outer);
    }
}

void gkGTKUIBringToFront(void *control) {
    GKView *view = gkView(control);
    GtkWidget *parent = view ? gtk_widget_get_parent(view->outer) : NULL;
    if (parent) {
        gtk_widget_insert_before(view->outer, parent, NULL);
    }
}

void gkGTKUISetFrame(void *control, int x, int y, int width, int height) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->x = x;
    view->y = y;
    view->width = MAX(0, width);
    view->height = MAX(0, height);
    gkLayoutMove(view->outer, x, y, width, height);
    if (view->kind == GK_KIND_CANVAS) {
        gkCanvasSyncXWindow(view);
        gkCanvasReportSurface(view, 0);
    }
}

void gkGTKUISetVisible(void *control, int visible) {
    GKView *view = gkView(control);
    if (view) {
        view->visible = visible != 0;
        gtk_widget_set_visible(view->outer, visible != 0);
        if (view->kind == GK_KIND_CANVAS) {
            gkCanvasSyncXWindow(view);
        }
    }
}

void gkGTKUISetEnabled(void *control, int enabled) {
    GKView *view = gkView(control);
    if (view) {
        gtk_widget_set_sensitive(view->outer, enabled != 0);
    }
}

void gkGTKRadioGroupPreferredSize(void *group, int *width, int *height);

void gkGTKUIPreferredSize(void *control, int *width, int *height) {
    GKView *view = gkView(control);
    int w = 0, h = 0;
    if (view) {
        gtk_widget_measure(view->outer, GTK_ORIENTATION_HORIZONTAL, -1, NULL, &w, NULL, NULL);
        if (view->kind == GK_KIND_LABEL && view->preferred_width > 0 &&
            gtk_label_get_wrap(GTK_LABEL(view->outer))) {
            int minimum = 0;
            gtk_widget_measure(view->outer, GTK_ORIENTATION_HORIZONTAL, -1, &minimum, NULL, NULL, NULL);
            w = MAX(minimum, MIN(w, view->preferred_width));
        }
        gtk_widget_measure(view->outer, GTK_ORIENTATION_VERTICAL, w, NULL, &h, NULL, NULL);
        switch (view->kind) {
        case GK_KIND_TEXT_AREA:
            w = 200;
            h = 96;
            break;
        case GK_KIND_RADIO_GROUP:
            gkGTKRadioGroupPreferredSize(control, &w, &h);
            break;
        case GK_KIND_TABS:
        case GK_KIND_SCROLL:
            w = 320;
            h = 240;
            break;
        case GK_KIND_COMMAND_BAR:
            w = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-natural-width"));
            h = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-natural-height"));
            break;
        default:
            break;
        }
    }
    *width = w;
    *height = h;
}

void gkGTKUIFramebufferSize(void *control, int *width, int *height) {
    GKView *view = gkView(control);
    int scale = view ? gtk_widget_get_scale_factor(view->outer) : 1;
    *width = view ? view->width * scale : 0;
    *height = view ? view->height * scale : 0;
}

char *gkGTKUIText(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return gkStrdup("");
    }
    switch (view->kind) {
    case GK_KIND_LABEL:
        return gkStrdup(gtk_label_get_text(GTK_LABEL(view->outer)));
    case GK_KIND_BUTTON:
    case GK_KIND_LINK:
    case GK_KIND_TOGGLE: {
        const char *label = g_object_get_data(G_OBJECT(view->outer), "gk-text");
        if (label) {
            return gkStrdup(label);
        }
        return gkStrdup(gtk_button_get_label(GTK_BUTTON(view->control)));
    }
    case GK_KIND_CHECK_BOX: {
        const char *label = g_object_get_data(G_OBJECT(view->outer), "gk-text");
        if (label) {
            return gkStrdup(label);
        }
        return gkStrdup(gtk_check_button_get_label(GTK_CHECK_BUTTON(view->control)));
    }
    case GK_KIND_TEXT_FIELD:
    case GK_KIND_SEARCH:
    case GK_KIND_COMBO:
        return gkStrdup(gtk_editable_get_text(GTK_EDITABLE(view->control)));
    case GK_KIND_TEXT_AREA: {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control));
        GtkTextIter start, end;
        gtk_text_buffer_get_bounds(buffer, &start, &end);
        char *value = gtk_text_buffer_get_text(buffer, &start, &end, FALSE);
        char *result = gkStrdup(value);
        g_free(value);
        return result;
    }
    case GK_KIND_SECTION:
        return gkStrdup(gtk_label_get_text(GTK_LABEL(((GKSection *)view->ext)->title)));
    default:
        if (GTK_IS_LABEL(view->control)) {
            return gkStrdup(gtk_label_get_text(GTK_LABEL(view->control)));
        }
        if (GTK_IS_EDITABLE(view->control)) {
            return gkStrdup(gtk_editable_get_text(GTK_EDITABLE(view->control)));
        }
        return gkStrdup("");
    }
}

void gkGTKUISetText(void *control, const char *text) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    const char *value = text ? text : "";
    view->suppress++;
    switch (view->kind) {
    case GK_KIND_LABEL:
        gtk_label_set_text(GTK_LABEL(view->outer), value);
        break;
    case GK_KIND_BUTTON:
    case GK_KIND_LINK:
    case GK_KIND_TOGGLE:
        if (g_object_get_data(G_OBJECT(view->outer), "gk-text")) {
            gkButtonSetText(view, value);
        } else {
            gtk_button_set_label(GTK_BUTTON(view->control), value);
        }
        break;
    case GK_KIND_CHECK_BOX:
        if (g_object_get_data(G_OBJECT(view->outer), "gk-text")) {
            extern void gkCheckBoxSetText(GKView * view, const char *text);
            gkCheckBoxSetText(view, value);
        } else {
            gtk_check_button_set_label(GTK_CHECK_BUTTON(view->control), value);
        }
        break;
    case GK_KIND_TEXT_FIELD:
    case GK_KIND_SEARCH:
    case GK_KIND_COMBO:
        gtk_editable_set_text(GTK_EDITABLE(view->control), value);
        break;
    case GK_KIND_TEXT_AREA:
        gtk_text_buffer_set_text(gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control)), value, -1);
        break;
    case GK_KIND_SECTION:
        gtk_label_set_text(GTK_LABEL(((GKSection *)view->ext)->title), value);
        break;
    default:
        if (GTK_IS_LABEL(view->control)) {
            gtk_label_set_text(GTK_LABEL(view->control), value);
        } else if (GTK_IS_EDITABLE(view->control)) {
            gtk_editable_set_text(GTK_EDITABLE(view->control), value);
        }
        break;
    }
    view->suppress--;
}

void gkGTKUISetPlaceholder(void *control, const char *text) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_TEXT_AREA) {
        free(view->placeholder);
        view->placeholder = gkStrdup(text);
        gkTextAreaUpdatePlaceholder(view);
    } else if (GTK_IS_ENTRY(view->control)) {
        gtk_entry_set_placeholder_text(GTK_ENTRY(view->control), text ? text : "");
    } else if (GTK_IS_SEARCH_ENTRY(view->control)) {
        gtk_search_entry_set_placeholder_text(GTK_SEARCH_ENTRY(view->control), text ? text : "");
    } else if (GTK_IS_PASSWORD_ENTRY(view->control) || GTK_IS_TEXT(view->control)) {
        g_object_set(view->control, "placeholder-text", text ? text : "", NULL);
    }
}

char *gkGTKUIPlaceholder(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return gkStrdup("");
    }
    if (view->kind == GK_KIND_TEXT_AREA) {
        return gkStrdup(view->placeholder);
    }
    if (GTK_IS_ENTRY(view->control)) {
        return gkStrdup(gtk_entry_get_placeholder_text(GTK_ENTRY(view->control)));
    }
    if (GTK_IS_SEARCH_ENTRY(view->control)) {
        return gkStrdup(gtk_search_entry_get_placeholder_text(GTK_SEARCH_ENTRY(view->control)));
    }
    if (GTK_IS_PASSWORD_ENTRY(view->control) || GTK_IS_TEXT(view->control)) {
        char *value = NULL;
        g_object_get(view->control, "placeholder-text", &value, NULL);
        char *result = gkStrdup(value);
        g_free(value);
        return result;
    }
    return gkStrdup("");
}

int gkGTKUIChecked(void *control) {
    GKView *view = gkView(control);
    if (!view) {
        return 0;
    }
    if (GTK_IS_CHECK_BUTTON(view->control)) {
        return gtk_check_button_get_active(GTK_CHECK_BUTTON(view->control));
    }
    if (GTK_IS_TOGGLE_BUTTON(view->control)) {
        return gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(view->control));
    }
    if (GTK_IS_SWITCH(view->control)) {
        return gtk_switch_get_active(GTK_SWITCH(view->control));
    }
    return 0;
}

void gkGTKUISetChecked(void *control, int checked) {
    GKView *view = gkView(control);
    if (!view) {
        return;
    }
    view->suppress++;
    if (GTK_IS_CHECK_BUTTON(view->control)) {
        gtk_check_button_set_inconsistent(GTK_CHECK_BUTTON(view->control), FALSE);
        gtk_check_button_set_active(GTK_CHECK_BUTTON(view->control), checked != 0);
    } else if (GTK_IS_TOGGLE_BUTTON(view->control)) {
        gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(view->control), checked != 0);
    } else if (GTK_IS_SWITCH(view->control)) {
        gtk_switch_set_active(GTK_SWITCH(view->control), checked != 0);
    }
    view->suppress--;
}

int gkGTKUIReadOnly(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_TEXT_AREA && !gtk_text_view_get_editable(GTK_TEXT_VIEW(view->control));
}

void gkGTKUISetReadOnly(void *control, int read_only) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_TEXT_AREA) {
        view->read_only = read_only != 0;
        gtk_text_view_set_editable(GTK_TEXT_VIEW(view->control), !read_only);
        gtk_text_view_set_cursor_visible(GTK_TEXT_VIEW(view->control), !read_only);
    }
}

double gkGTKUISliderValue(void *control) {
    GKView *view = gkView(control);
    return view && GTK_IS_RANGE(view->control) ? gtk_range_get_value(GTK_RANGE(view->control)) : 0;
}

void gkGTKUISetSliderValue(void *control, double value) {
    GKView *view = gkView(control);
    if (view && GTK_IS_RANGE(view->control)) {
        view->suppress++;
        gtk_range_set_value(GTK_RANGE(view->control), value);
        view->suppress--;
    }
}

double gkGTKUIProgressValue(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_PROGRESS ? view->value : 0;
}

void gkGTKUISetProgressValue(void *control, double value) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_PROGRESS) {
        view->value = MAX(0, MIN(1, value));
        if (GTK_IS_PROGRESS_BAR(view->control)) {
            gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(view->control), view->value);
        } else if (GTK_IS_LEVEL_BAR(view->control)) {
            gtk_level_bar_set_value(GTK_LEVEL_BAR(view->control), view->value);
        }
    }
}

int gkGTKUIActivityRunning(void *control) {
    GKView *view = gkView(control);
    return view && view->kind == GK_KIND_ACTIVITY && view->running;
}

void gkGTKUISetActivityRunning(void *control, int running) {
    GKView *view = gkView(control);
    if (view && view->kind == GK_KIND_ACTIVITY) {
        view->running = running != 0;
        gtk_spinner_set_spinning(GTK_SPINNER(view->control), running != 0);
    }
}

/* ---- File dialogs ---- */

typedef struct {
    int done;
    GListModel *files;
    GFile *file;
    GError *error;
} GKDialogResult;

static gboolean gkDialogDone(gpointer data) {
    return ((GKDialogResult *)data)->done;
}

static void gkOpenMultipleDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKDialogResult *state = data;
    state->files = gtk_file_dialog_open_multiple_finish(GTK_FILE_DIALOG(source), result, &state->error);
    state->done = 1;
}

static void gkOpenDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKDialogResult *state = data;
    state->file = gtk_file_dialog_open_finish(GTK_FILE_DIALOG(source), result, &state->error);
    state->done = 1;
}

static void gkFolderDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKDialogResult *state = data;
    state->file = gtk_file_dialog_select_folder_finish(GTK_FILE_DIALOG(source), result, &state->error);
    state->done = 1;
}

static void gkFoldersDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKDialogResult *state = data;
    state->files = gtk_file_dialog_select_multiple_folders_finish(GTK_FILE_DIALOG(source), result, &state->error);
    state->done = 1;
}

static void gkSaveDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKDialogResult *state = data;
    state->file = gtk_file_dialog_save_finish(GTK_FILE_DIALOG(source), result, &state->error);
    state->done = 1;
}

static GtkFileDialog *gkFileDialog(const char *title, const char *directory, const char *filename,
                                   const char *extensions) {
    GtkFileDialog *dialog = gtk_file_dialog_new();
    gtk_file_dialog_set_modal(dialog, TRUE);
    if (title && *title) {
        gtk_file_dialog_set_title(dialog, title);
    }
    if (directory && *directory) {
        GFile *folder = g_file_new_for_path(directory);
        gtk_file_dialog_set_initial_folder(dialog, folder);
        g_object_unref(folder);
    }
    if (filename && *filename) {
        gtk_file_dialog_set_initial_name(dialog, filename);
    }
    if (extensions && *extensions) {
        GtkFileFilter *filter = gtk_file_filter_new();
        char **parts = g_strsplit(extensions, ",", -1);
        GString *name = g_string_new("");
        for (int i = 0; parts[i]; i++) {
            char *extension = g_strstrip(parts[i]);
            if (!*extension) {
                continue;
            }
            if (*extension == '.') {
                extension++;
            }
            gtk_file_filter_add_suffix(filter, extension);
            g_string_append_printf(name, "%s*.%s", name->len ? ", " : "", extension);
        }
        gtk_file_filter_set_name(filter, name->str);
        g_string_free(name, TRUE);
        g_strfreev(parts);
        GListStore *filters = g_list_store_new(GTK_TYPE_FILE_FILTER);
        g_list_store_append(filters, filter);
        gtk_file_dialog_set_filters(dialog, G_LIST_MODEL(filters));
        gtk_file_dialog_set_default_filter(dialog, filter);
        g_object_unref(filters);
        g_object_unref(filter);
    }
    return dialog;
}

static char *gkPackFiles(GListModel *files, GFile *file, size_t *result_size) {
    GPtrArray *paths = g_ptr_array_new_with_free_func(g_free);
    if (files) {
        for (guint i = 0; i < g_list_model_get_n_items(files); i++) {
            GFile *item = g_list_model_get_item(files, i);
            char *path = g_file_get_path(item);
            if (path) {
                g_ptr_array_add(paths, path);
            }
            g_object_unref(item);
        }
    }
    if (file) {
        char *path = g_file_get_path(file);
        if (path) {
            g_ptr_array_add(paths, path);
        }
    }
    size_t size = 0;
    char *buffer = gkJoin((char **)paths->pdata, (int)paths->len, &size);
    g_ptr_array_free(paths, TRUE);
    if (result_size) {
        *result_size = size;
    }
    return buffer;
}

static GtkWindow *gkDialogParent(uintptr_t window) {
    GKWin *win = gkWinFromHandle(window);
    return win && gtk_widget_get_visible(win->window) ? GTK_WINDOW(win->window) : NULL;
}

static char *gkFinishDialog(GKDialogResult *state, size_t *result_size, int *cancelled) {
    char *result = NULL;
    if (state->error || (!state->file && !state->files)) {
        if (cancelled) {
            *cancelled = 1;
        }
        if (result_size) {
            *result_size = 0;
        }
    } else {
        if (cancelled) {
            *cancelled = 0;
        }
        result = gkPackFiles(state->files, state->file, result_size);
    }
    if (state->error) {
        g_error_free(state->error);
    }
    if (state->files) {
        g_object_unref(state->files);
    }
    if (state->file) {
        g_object_unref(state->file);
    }
    return result;
}

char *gkGTKUIOpenPanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                       const char *extensions, int multiple, int directories, int hidden, int packages,
                       size_t *result_size, int *cancelled) {
    (void)hidden;
    (void)packages;
    GtkFileDialog *dialog = gkFileDialog(title, directory, filename, directories ? NULL : extensions);
    GKDialogResult state = {0};
    GtkWindow *parent = gkDialogParent(window);
    if (directories && multiple) {
        gtk_file_dialog_select_multiple_folders(dialog, parent, NULL, gkFoldersDone, &state);
    } else if (directories) {
        gtk_file_dialog_select_folder(dialog, parent, NULL, gkFolderDone, &state);
    } else if (multiple) {
        gtk_file_dialog_open_multiple(dialog, parent, NULL, gkOpenMultipleDone, &state);
    } else {
        gtk_file_dialog_open(dialog, parent, NULL, gkOpenDone, &state);
    }
    while (!state.done) {
        g_main_context_iteration(NULL, TRUE);
    }
    g_object_unref(dialog);
    return gkFinishDialog(&state, result_size, cancelled);
}

char *gkGTKUISavePanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                       const char *extensions, int hidden, const char *default_extension, size_t *result_size,
                       int *cancelled) {
    (void)hidden;
    char *name = NULL;
    if (filename && *filename && default_extension && *default_extension && !strchr(filename, '.')) {
        name = g_strdup_printf("%s.%s", filename, default_extension[0] == '.' ? default_extension + 1
                                                                               : default_extension);
    }
    GtkFileDialog *dialog = gkFileDialog(title, directory, name ? name : filename, extensions);
    g_free(name);
    GKDialogResult state = {0};
    gtk_file_dialog_save(dialog, gkDialogParent(window), NULL, gkSaveDone, &state);
    while (!state.done) {
        g_main_context_iteration(NULL, TRUE);
    }
    g_object_unref(dialog);
    (void)gkDialogDone;
    return gkFinishDialog(&state, result_size, cancelled);
}
