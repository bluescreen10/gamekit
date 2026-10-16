//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>
#include <stdio.h>

/* ---- Icons ---- */

void *gkGTKIcon(const char *symbol, const unsigned char *pixels, int width, int height, double point_size) {
    if (pixels && width > 0 && height > 0) {
        return gkTextureFromRGBA(pixels, width, height, width * 4);
    }
    if (!symbol || !*symbol) {
        return NULL;
    }
    GdkDisplay *display = gdk_display_get_default();
    GtkIconTheme *theme = display ? gtk_icon_theme_get_for_display(display) : NULL;
    if (!theme) {
        return NULL;
    }
    /* symbol is a list of names separated by line feeds. The first name that
       the theme has wins; the others are fallbacks. */
    char **names = g_strsplit(symbol, "\n", -1);
    const char *name = names[0];
    for (char **candidate = names; *candidate; candidate++) {
        if (gtk_icon_theme_has_icon(theme, *candidate)) {
            name = *candidate;
            break;
        }
    }
    int size = point_size > 0 ? (int)ceil(point_size) : 16;
    GtkIconPaintable *icon = gtk_icon_theme_lookup_icon(theme, name, NULL, size, 1, GTK_TEXT_DIR_NONE, 0);
    g_strfreev(names);
    return icon;
}

GtkWidget *gkIconImage(void *icon, int size) {
    if (!icon) {
        return NULL;
    }
    GtkWidget *image = gtk_image_new_from_paintable(GDK_PAINTABLE(icon));
    if (size > 0) {
        gtk_image_set_pixel_size(GTK_IMAGE(image), size);
    }
    return image;
}

GdkPaintable *gkIconPaintable(void *icon) {
    return icon ? GDK_PAINTABLE(icon) : NULL;
}

/* ---- Labels ---- */

void gkGTKLabelConfigure(void *pointer, int alignment, int wrap, int truncation, int selectable, int style,
                         int maximum_lines) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_LABEL(view->outer)) {
        return;
    }
    GtkLabel *label = GTK_LABEL(view->outer);
    static const float alignments[] = {0, 0.5f, 1, 0};
    int index = alignment >= 0 && alignment < 4 ? alignment : 0;
    gtk_label_set_xalign(label, alignments[index]);
    gtk_label_set_justify(label, index == 1   ? GTK_JUSTIFY_CENTER
                                 : index == 2 ? GTK_JUSTIFY_RIGHT
                                 : index == 3 ? GTK_JUSTIFY_FILL
                                              : GTK_JUSTIFY_LEFT);
    gtk_label_set_selectable(label, selectable != 0);
    gtk_label_set_wrap(label, wrap != 0);
    gtk_label_set_wrap_mode(label, wrap == 2 ? PANGO_WRAP_CHAR : PANGO_WRAP_WORD_CHAR);
    gtk_label_set_ellipsize(label, truncation == 1   ? PANGO_ELLIPSIZE_START
                                   : truncation == 2 ? PANGO_ELLIPSIZE_MIDDLE
                                   : truncation == 3 ? PANGO_ELLIPSIZE_END
                                                     : PANGO_ELLIPSIZE_NONE);
    gtk_label_set_lines(label, wrap && maximum_lines > 0 ? maximum_lines : -1);
    gtk_label_set_single_line_mode(label, wrap == 0);
    static const char *styles[] = {"body", "caption", "heading", "title-2", "monospace"};
    for (size_t i = 0; i < G_N_ELEMENTS(styles); i++) {
        gtk_widget_remove_css_class(view->outer, styles[i]);
    }
    if (style >= 0 && style < (int)G_N_ELEMENTS(styles)) {
        gtk_widget_add_css_class(view->outer, styles[style]);
    }
}

void gkGTKLabelPreferredWidth(void *pointer, int width) {
    GKView *view = gkView(pointer);
    if (view) {
        view->preferred_width = width > 0 ? width : 0;
        if (GTK_IS_LABEL(view->outer)) {
            gtk_label_set_max_width_chars(GTK_LABEL(view->outer), -1);
        }
    }
}

/* ---- Buttons ---- */

typedef struct {
    GtkWidget *box;
    GtkWidget *icon;
    GtkWidget *label;
    GtkWidget *spinner;
    GtkWidget *badge;
    GtkEventController *shortcuts;
} GKButtonParts;

/* gkButtonBadgePosition centers the badge near the top trailing corner of
   the button, so that the badge extends past the edges of the button. The
   overlay fills only the content box of the button, so the position comes from
   the bounds of the button, which include its padding and border. */
static gboolean gkButtonBadgePosition(GtkOverlay *overlay, GtkWidget *widget, GdkRectangle *allocation,
                                      gpointer data) {
    (void)data;
    if (!gtk_widget_has_css_class(widget, "gk-badge")) {
        return FALSE;
    }
    int minimum = 0, width = 0, height = 0;
    gtk_widget_measure(widget, GTK_ORIENTATION_HORIZONTAL, -1, &minimum, &width, NULL, NULL);
    gtk_widget_measure(widget, GTK_ORIENTATION_VERTICAL, width, &minimum, &height, NULL, NULL);
    GtkWidget *button = gtk_widget_get_parent(GTK_WIDGET(overlay));
    graphene_rect_t bounds;
    if (!button || !gtk_widget_compute_bounds(button, GTK_WIDGET(overlay), &bounds)) {
        graphene_rect_init(&bounds, 0, 0, gtk_widget_get_width(GTK_WIDGET(overlay)),
                           gtk_widget_get_height(GTK_WIDGET(overlay)));
    }
    allocation->width = width;
    allocation->height = height;
    allocation->x = (int)(bounds.origin.x + bounds.size.width) - width / 2 - 4;
    if (gtk_widget_get_direction(GTK_WIDGET(overlay)) == GTK_TEXT_DIR_RTL) {
        allocation->x = (int)bounds.origin.x - width / 2 + 4;
    }
    allocation->y = (int)bounds.origin.y - height / 2 + 2;
    return TRUE;
}

GtkWidget *gkButtonContent(GKView *view) {
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    if (parts) {
        return parts->label;
    }
    parts = g_new0(GKButtonParts, 1);
    const char *text = "";
    if (GTK_IS_BUTTON(view->control) && gtk_button_get_label(GTK_BUTTON(view->control))) {
        text = gtk_button_get_label(GTK_BUTTON(view->control));
    }
    char *saved = g_strdup(text);
    GtkWidget *overlay = gtk_overlay_new();
    parts->box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    gtk_widget_set_halign(parts->box, GTK_ALIGN_CENTER);
    parts->icon = gtk_image_new();
    gtk_widget_set_visible(parts->icon, FALSE);
    parts->label = gtk_label_new(saved);
    gtk_box_append(GTK_BOX(parts->box), parts->icon);
    gtk_box_append(GTK_BOX(parts->box), parts->label);
    gtk_overlay_set_child(GTK_OVERLAY(overlay), parts->box);
    parts->spinner = gtk_spinner_new();
    gtk_widget_set_halign(parts->spinner, GTK_ALIGN_CENTER);
    gtk_widget_set_valign(parts->spinner, GTK_ALIGN_CENTER);
    gtk_widget_set_visible(parts->spinner, FALSE);
    gtk_overlay_add_overlay(GTK_OVERLAY(overlay), parts->spinner);
    parts->badge = gtk_label_new("");
    gtk_widget_add_css_class(parts->badge, "gk-badge");
    gtk_widget_set_halign(parts->badge, GTK_ALIGN_END);
    gtk_widget_set_valign(parts->badge, GTK_ALIGN_START);
    gtk_widget_set_visible(parts->badge, FALSE);
    gtk_widget_set_can_target(parts->badge, FALSE);
    gtk_overlay_add_overlay(GTK_OVERLAY(overlay), parts->badge);
    g_signal_connect(overlay, "get-child-position", G_CALLBACK(gkButtonBadgePosition), NULL);
    /* The badge draws outside the button. */
    gtk_widget_set_overflow(overlay, GTK_OVERFLOW_VISIBLE);
    gtk_widget_set_overflow(view->control, GTK_OVERFLOW_VISIBLE);
    gkSetCSSRule("label.gk-badge",
                 "background-color: @error_color; color: white; border-radius: 8px; padding: 0 5px; "
                 "font-size: smaller; font-weight: bold; min-width: 6px; min-height: 16px;");
    gtk_button_set_child(GTK_BUTTON(view->control), overlay);
    g_object_set_data_full(G_OBJECT(view->outer), "gk-button", parts, g_free);
    g_object_set_data_full(G_OBJECT(view->outer), "gk-text", saved, g_free);
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL, saved, -1);
    return parts->label;
}

void gkButtonSetText(GKView *view, const char *text) {
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    g_object_set_data_full(G_OBJECT(view->outer), "gk-text", g_strdup(text ? text : ""), g_free);
    if (parts) {
        gtk_label_set_text(GTK_LABEL(parts->label), text ? text : "");
        if (!view->accessible_name || !*view->accessible_name) {
            gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL,
                                           text ? text : "", -1);
        }
    }
}

static void gkButtonSetShortcut(GKView *view, GKButtonParts *parts, const char *accelerator) {
    if (parts->shortcuts) {
        gtk_widget_remove_controller(view->control, parts->shortcuts);
        parts->shortcuts = NULL;
    }
    if (!accelerator || !*accelerator) {
        return;
    }
    GtkShortcutTrigger *trigger = gtk_shortcut_trigger_parse_string(accelerator);
    if (!trigger) {
        return;
    }
    GtkEventController *controller = gtk_shortcut_controller_new();
    gtk_shortcut_controller_set_scope(GTK_SHORTCUT_CONTROLLER(controller), GTK_SHORTCUT_SCOPE_MANAGED);
    gtk_shortcut_controller_add_shortcut(GTK_SHORTCUT_CONTROLLER(controller),
                                         gtk_shortcut_new(trigger, g_object_ref(gtk_activate_action_get())));
    gtk_widget_add_controller(view->control, controller);
    parts->shortcuts = controller;
}

void gkGTKButtonConfigure(void *pointer, void *icon, int placement, int role, const char *key, int modifiers) {
    (void)modifiers;
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_BUTTON(view->control)) {
        return;
    }
    gkButtonContent(view);
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    if (icon) {
        gtk_image_set_from_paintable(GTK_IMAGE(parts->icon), icon);
    } else {
        gtk_image_clear(GTK_IMAGE(parts->icon));
    }
    gtk_widget_set_visible(parts->icon, icon != NULL);
    gtk_widget_set_visible(parts->label, !(icon && placement == 4));
    gtk_orientable_set_orientation(GTK_ORIENTABLE(parts->box),
                                   placement == 2 || placement == 3 ? GTK_ORIENTATION_VERTICAL
                                                                    : GTK_ORIENTATION_HORIZONTAL);
    gtk_box_reorder_child_after(GTK_BOX(parts->box), parts->icon, placement == 1 || placement == 3 ? parts->label
                                                                                                     : NULL);
    gtk_widget_remove_css_class(view->control, "suggested-action");
    gtk_widget_remove_css_class(view->control, "destructive-action");
    const char *accelerator = key;
    if (!key || !*key) {
        accelerator = role == 1 ? "Return" : role == 2 ? "Escape" : NULL;
    }
    if (role == 1) {
        gtk_widget_add_css_class(view->control, "suggested-action");
    } else if (role == 3) {
        gtk_widget_add_css_class(view->control, "destructive-action");
    }
    gkButtonSetShortcut(view, parts, accelerator);
}

void gkGTKButtonSetBusy(void *pointer, int busy) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_BUTTON(view->control)) {
        return;
    }
    gkButtonContent(view);
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    gtk_widget_set_visible(parts->spinner, busy != 0);
    gtk_spinner_set_spinning(GTK_SPINNER(parts->spinner), busy != 0);
    /* The content keeps its size, so that the button keeps its size, but the
       spinner draws in its place. */
    gtk_widget_set_opacity(parts->box, busy ? 0 : 1);
    gtk_accessible_update_state(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_STATE_BUSY, busy != 0, -1);
}

void gkGTKButtonSetBadge(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_BUTTON(view->control)) {
        return;
    }
    gkButtonContent(view);
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    gtk_label_set_text(GTK_LABEL(parts->badge), text ? text : "");
    gtk_widget_set_visible(parts->badge, text && *text);
    if (text && *text) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_DESCRIPTION, text, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_DESCRIPTION);
    }
}

/* ---- Text ---- */

static int gkUTF16ToChars(const char *text, int units) {
    int characters = 0;
    int count = 0;
    for (const char *p = text; p && *p && count < units; p = g_utf8_next_char(p)) {
        count += g_utf8_get_char(p) >= 0x10000 ? 2 : 1;
        characters++;
    }
    return characters;
}

static int gkCharsToUTF16(const char *text, int characters) {
    int units = 0;
    int index = 0;
    for (const char *p = text; p && *p && index < characters; p = g_utf8_next_char(p), index++) {
        units += g_utf8_get_char(p) >= 0x10000 ? 2 : 1;
    }
    return units;
}

void gkGTKTextFieldConfigure(void *pointer, int read_only, int maximum_length, int purpose, int alignment,
                             const char *prefix, const char *suffix, int spell_check) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_EDITABLE(view->control)) {
        return;
    }
    gtk_editable_set_editable(GTK_EDITABLE(view->control), read_only == 0);
    if (GTK_IS_ENTRY(view->control)) {
        GtkEntry *entry = GTK_ENTRY(view->control);
        gtk_entry_set_max_length(entry, MAX(0, maximum_length));
        static const GtkInputPurpose purposes[] = {GTK_INPUT_PURPOSE_FREE_FORM, GTK_INPUT_PURPOSE_NAME,
                                                   GTK_INPUT_PURPOSE_FREE_FORM, GTK_INPUT_PURPOSE_EMAIL,
                                                   GTK_INPUT_PURPOSE_URL,       GTK_INPUT_PURPOSE_PHONE};
        gtk_entry_set_input_purpose(entry, purposes[purpose >= 0 && purpose < 6 ? purpose : 0]);
        gtk_entry_set_input_hints(entry, spell_check ? GTK_INPUT_HINT_SPELLCHECK : GTK_INPUT_HINT_NO_SPELLCHECK);
    }
    static const float alignments[] = {0, 0.5f, 1, 0};
    gtk_editable_set_alignment(GTK_EDITABLE(view->control), alignments[alignment >= 0 && alignment < 4 ? alignment : 0]);
    GtkWidget *prefix_label = g_object_get_data(G_OBJECT(view->outer), "gk-prefix");
    GtkWidget *suffix_label = g_object_get_data(G_OBJECT(view->outer), "gk-suffix");
    if (prefix_label) {
        gtk_label_set_text(GTK_LABEL(prefix_label), prefix ? prefix : "");
        gtk_widget_set_visible(prefix_label, prefix && *prefix);
    }
    if (suffix_label) {
        gtk_label_set_text(GTK_LABEL(suffix_label), suffix ? suffix : "");
        gtk_widget_set_visible(suffix_label, suffix && *suffix);
    }
}

void gkGTKTextSelection(void *pointer, int *start, int *length) {
    GKView *view = gkView(pointer);
    int s = 0, e = 0;
    char *text = NULL;
    if (view && view->kind == GK_KIND_TEXT_AREA) {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control));
        GtkTextIter first, last, begin, end;
        gtk_text_buffer_get_selection_bounds(buffer, &first, &last);
        s = gtk_text_iter_get_offset(&first);
        e = gtk_text_iter_get_offset(&last);
        gtk_text_buffer_get_bounds(buffer, &begin, &end);
        text = gtk_text_buffer_get_text(buffer, &begin, &end, FALSE);
    } else if (view && GTK_IS_EDITABLE(view->control)) {
        gtk_editable_get_selection_bounds(GTK_EDITABLE(view->control), &s, &e);
        if (s == e) {
            s = e = gtk_editable_get_position(GTK_EDITABLE(view->control));
        }
        text = g_strdup(gtk_editable_get_text(GTK_EDITABLE(view->control)));
    }
    int from = gkCharsToUTF16(text, MIN(s, e));
    int to = gkCharsToUTF16(text, MAX(s, e));
    g_free(text);
    *start = from;
    *length = to - from;
}

void gkGTKSetTextSelection(void *pointer, int start, int length) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    if (view->kind == GK_KIND_TEXT_AREA) {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control));
        GtkTextIter begin, end;
        gtk_text_buffer_get_bounds(buffer, &begin, &end);
        char *text = gtk_text_buffer_get_text(buffer, &begin, &end, FALSE);
        int from = gkUTF16ToChars(text, MAX(0, start));
        int to = gkUTF16ToChars(text, MAX(0, start) + MAX(0, length));
        g_free(text);
        GtkTextIter first, last;
        gtk_text_buffer_get_iter_at_offset(buffer, &first, from);
        gtk_text_buffer_get_iter_at_offset(buffer, &last, to);
        gtk_text_buffer_select_range(buffer, &last, &first);
    } else if (GTK_IS_EDITABLE(view->control)) {
        const char *text = gtk_editable_get_text(GTK_EDITABLE(view->control));
        int from = gkUTF16ToChars(text, MAX(0, start));
        int to = gkUTF16ToChars(text, MAX(0, start) + MAX(0, length));
        gtk_editable_select_region(GTK_EDITABLE(view->control), from, to);
    }
}

static void gkTextAreaInsert(GtkTextBuffer *buffer, GtkTextIter *location, char *text, int length, gpointer data) {
    (void)location;
    GKView *view = data;
    if (view->maximum_length <= 0) {
        return;
    }
    int existing = gtk_text_buffer_get_char_count(buffer);
    long added = g_utf8_strlen(text, length);
    if (existing + added > view->maximum_length) {
        g_signal_stop_emission_by_name(buffer, "insert-text");
    }
}

void gkGTKTextAreaConfigure(void *pointer, int wrap, int maximum_length, int tab_moves_focus, int spell_check) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_TEXT_AREA) {
        return;
    }
    GtkTextView *editor = GTK_TEXT_VIEW(view->control);
    gtk_text_view_set_wrap_mode(editor, wrap == 0 ? GTK_WRAP_NONE : wrap == 2 ? GTK_WRAP_CHAR : GTK_WRAP_WORD_CHAR);
    GtkWidget *scroll = g_object_get_data(G_OBJECT(view->outer), "gk-scroll");
    if (scroll) {
        gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), wrap ? GTK_POLICY_NEVER : GTK_POLICY_AUTOMATIC,
                                       GTK_POLICY_AUTOMATIC);
    }
    if (!g_object_get_data(G_OBJECT(view->outer), "gk-limit")) {
        g_signal_connect(gtk_text_view_get_buffer(editor), "insert-text", G_CALLBACK(gkTextAreaInsert), view);
        g_object_set_data(G_OBJECT(view->outer), "gk-limit", GINT_TO_POINTER(1));
    }
    view->maximum_length = MAX(0, maximum_length);
    gtk_text_view_set_accepts_tab(editor, tab_moves_focus == 0);
    gtk_text_view_set_input_hints(editor, spell_check ? GTK_INPUT_HINT_SPELLCHECK : GTK_INPUT_HINT_NO_SPELLCHECK);
}

/* ---- Scrolling ---- */

static GtkScrolledWindow *gkScrolledWindow(GKView *view) {
    if (!view) {
        return NULL;
    }
    if (GTK_IS_SCROLLED_WINDOW(view->outer)) {
        return GTK_SCROLLED_WINDOW(view->outer);
    }
    GtkWidget *scroll = g_object_get_data(G_OBJECT(view->outer), "gk-scroll");
    return scroll ? GTK_SCROLLED_WINDOW(scroll) : NULL;
}

void gkGTKScrollPosition(void *pointer, int *x, int *y) {
    GtkScrolledWindow *scroll = gkScrolledWindow(gkView(pointer));
    *x = scroll ? (int)floor(gtk_adjustment_get_value(gtk_scrolled_window_get_hadjustment(scroll))) : 0;
    *y = scroll ? (int)floor(gtk_adjustment_get_value(gtk_scrolled_window_get_vadjustment(scroll))) : 0;
}

typedef struct {
    GtkAdjustment *horizontal;
    GtkAdjustment *vertical;
    double from_x, from_y, to_x, to_y;
    gint64 start;
} GKScrollAnimation;

static gboolean gkScrollAnimate(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    (void)widget;
    GKScrollAnimation *animation = data;
    double t = (gdk_frame_clock_get_frame_time(clock) - animation->start) / 200000.0;
    if (t >= 1) {
        gtk_adjustment_set_value(animation->horizontal, animation->to_x);
        gtk_adjustment_set_value(animation->vertical, animation->to_y);
        return G_SOURCE_REMOVE;
    }
    double eased = 1 - pow(1 - MAX(0, t), 3);
    gtk_adjustment_set_value(animation->horizontal, animation->from_x + (animation->to_x - animation->from_x) * eased);
    gtk_adjustment_set_value(animation->vertical, animation->from_y + (animation->to_y - animation->from_y) * eased);
    return G_SOURCE_CONTINUE;
}

static void gkScrollAdjust(GtkAdjustment *adjustment, double value) {
    double upper = gtk_adjustment_get_upper(adjustment);
    double page = gtk_adjustment_get_page_size(adjustment);
    if (value > upper - page) {
        /* The scrolled window has not been allocated yet: keep the value
           until it configures the adjustment. */
        gtk_adjustment_configure(adjustment, value, gtk_adjustment_get_lower(adjustment), value + page,
                                 gtk_adjustment_get_step_increment(adjustment),
                                 gtk_adjustment_get_page_increment(adjustment), page);
    }
    gtk_adjustment_set_value(adjustment, value);
}

static void gkScrollTo(GtkScrolledWindow *scroll, double x, double y, int animated) {
    GtkAdjustment *horizontal = gtk_scrolled_window_get_hadjustment(scroll);
    GtkAdjustment *vertical = gtk_scrolled_window_get_vadjustment(scroll);
    x = MAX(0, x);
    y = MAX(0, y);
    if (!animated || !gtk_widget_get_mapped(GTK_WIDGET(scroll))) {
        gkScrollAdjust(horizontal, x);
        gkScrollAdjust(vertical, y);
        return;
    }
    x = MIN(x, MAX(0, gtk_adjustment_get_upper(horizontal) - gtk_adjustment_get_page_size(horizontal)));
    y = MIN(y, MAX(0, gtk_adjustment_get_upper(vertical) - gtk_adjustment_get_page_size(vertical)));
    GKScrollAnimation *animation = g_new0(GKScrollAnimation, 1);
    animation->horizontal = horizontal;
    animation->vertical = vertical;
    animation->from_x = gtk_adjustment_get_value(horizontal);
    animation->from_y = gtk_adjustment_get_value(vertical);
    animation->to_x = x;
    animation->to_y = y;
    GdkFrameClock *clock = gtk_widget_get_frame_clock(GTK_WIDGET(scroll));
    animation->start = clock ? gdk_frame_clock_get_frame_time(clock) : g_get_monotonic_time();
    gtk_widget_add_tick_callback(GTK_WIDGET(scroll), gkScrollAnimate, animation, g_free);
}

void gkGTKSetScrollPosition(void *pointer, int x, int y, int animated) {
    GtkScrolledWindow *scroll = gkScrolledWindow(gkView(pointer));
    if (scroll) {
        gkScrollTo(scroll, x, y, animated);
    }
}

/* ---- Check boxes ---- */

typedef struct {
    GtkWidget *box;
    GtkWidget *title;
    GtkWidget *detail;
} GKCheckParts;

void gkCheckBoxSetText(GKView *view, const char *text) {
    GKCheckParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-check");
    g_object_set_data_full(G_OBJECT(view->outer), "gk-text", g_strdup(text ? text : ""), g_free);
    if (parts) {
        gtk_label_set_text(GTK_LABEL(parts->title), text ? text : "");
    } else {
        gtk_check_button_set_label(GTK_CHECK_BUTTON(view->control), text ? text : "");
    }
}

void gkGTKCheckBoxConfigure(void *pointer, int allows_mixed, int state, int label_before, const char *text,
                            const char *description) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_CHECK_BUTTON(view->control)) {
        return;
    }
    GtkCheckButton *box = GTK_CHECK_BUTTON(view->control);
    view->suppress++;
    gtk_check_button_set_active(box, state == 1);
    gtk_check_button_set_inconsistent(box, state == 2 && allows_mixed);
    view->suppress--;
    gtk_widget_set_direction(view->control, label_before ? GTK_TEXT_DIR_RTL : GTK_TEXT_DIR_NONE);
    GKCheckParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-check");
    if (description && *description) {
        if (!parts) {
            parts = g_new0(GKCheckParts, 1);
            parts->box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
            parts->title = gtk_label_new("");
            parts->detail = gtk_label_new("");
            gtk_label_set_xalign(GTK_LABEL(parts->title), 0);
            gtk_label_set_xalign(GTK_LABEL(parts->detail), 0);
            gtk_widget_add_css_class(parts->detail, "dim-label");
            gtk_widget_add_css_class(parts->detail, "caption");
            gtk_box_append(GTK_BOX(parts->box), parts->title);
            gtk_box_append(GTK_BOX(parts->box), parts->detail);
            gtk_check_button_set_child(box, parts->box);
            g_object_set_data_full(G_OBJECT(view->outer), "gk-check", parts, g_free);
        }
        gtk_label_set_text(GTK_LABEL(parts->title), text ? text : "");
        gtk_label_set_text(GTK_LABEL(parts->detail), description);
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL,
                                       text ? text : "", GTK_ACCESSIBLE_PROPERTY_DESCRIPTION, description, -1);
    } else {
        if (parts) {
            gtk_check_button_set_child(box, NULL);
            g_object_set_data(G_OBJECT(view->outer), "gk-check", NULL);
        }
        gtk_check_button_set_label(box, text ? text : "");
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_DESCRIPTION);
    }
    g_object_set_data_full(G_OBJECT(view->outer), "gk-text", g_strdup(text ? text : ""), g_free);
}

int gkGTKCheckBoxState(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_CHECK_BUTTON(view->control)) {
        return 0;
    }
    if (gtk_check_button_get_inconsistent(GTK_CHECK_BUTTON(view->control))) {
        return 2;
    }
    return gtk_check_button_get_active(GTK_CHECK_BUTTON(view->control));
}

/* ---- Choices ---- */

extern GtkWidget *gkRadioAdd(GKView *view, const char *title, int enabled);
extern void gkRadioSelect(GKView *view, int index);

void gkGTKSetChoices(void *pointer, const char *titles, size_t titles_size, const int *enabled, int count) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    int title_count = 0;
    char **values = gkSplit(titles, titles_size, &title_count);
    if (view->kind == GK_KIND_RADIO_GROUP) {
        int selected = view->selected;
        view->suppress++;
        while (view->buttons->len > 0) {
            GtkWidget *button = g_ptr_array_index(view->buttons, view->buttons->len - 1);
            gtk_box_remove(GTK_BOX(view->control), button);
            g_ptr_array_remove_index(view->buttons, view->buttons->len - 1);
        }
        for (int i = 0; i < title_count; i++) {
            gkRadioAdd(view, values[i], i < count ? enabled[i] : 1);
        }
        view->suppress--;
        gkRadioSelect(view, selected < title_count ? selected : -1);
    } else if (view->kind == GK_KIND_SELECT) {
        GArray *flags = g_array_new(FALSE, FALSE, sizeof(int));
        for (int i = 0; i < title_count; i++) {
            int value = i < count ? enabled[i] != 0 : 1;
            g_array_append_val(flags, value);
        }
        g_object_set_data_full(G_OBJECT(view->outer), "gk-enabled", flags, (GDestroyNotify)g_array_unref);
        GtkDropDown *dropdown = GTK_DROP_DOWN(view->control);
        guint selected = gtk_drop_down_get_selected(dropdown);
        GtkStringList *model = gtk_string_list_new((const char *const *)values);
        view->suppress++;
        gtk_drop_down_set_model(dropdown, G_LIST_MODEL(model));
        gtk_drop_down_set_selected(dropdown, selected < (guint)title_count ? selected : GTK_INVALID_LIST_POSITION);
        view->suppress--;
        g_object_unref(model);
    }
    g_strfreev(values);
}

void gkGTKRadioGroupConfigure(void *pointer, int horizontal, const char *label) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_RADIO_GROUP) {
        return;
    }
    gtk_orientable_set_orientation(GTK_ORIENTABLE(view->control),
                                   horizontal ? GTK_ORIENTATION_HORIZONTAL : GTK_ORIENTATION_VERTICAL);
    gtk_box_set_spacing(GTK_BOX(view->control), horizontal ? 12 : 2);
    gtk_label_set_text(GTK_LABEL(view->label), label ? label : "");
    gtk_widget_add_css_class(view->label, "heading");
    gtk_widget_set_visible(view->label, label && *label);
    if (label && *label) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL);
    }
}

void gkGTKRadioGroupPreferredSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0;
    if (view) {
        gtk_widget_measure(view->outer, GTK_ORIENTATION_HORIZONTAL, -1, NULL, &w, NULL, NULL);
        gtk_widget_measure(view->outer, GTK_ORIENTATION_VERTICAL, w, NULL, &h, NULL, NULL);
    }
    *width = MAX(120, w);
    *height = MAX(24, h);
}

static void gkSelectPopupSize(GKView *view) {
    GtkWidget *popover = gkDescendant(view->control, GTK_TYPE_POPOVER);
    if (!popover) {
        return;
    }
    int width = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-popup-width"));
    gtk_widget_set_size_request(popover, width > 0 ? width : -1, -1);
    GtkWidget *scroll = gkDescendant(popover, GTK_TYPE_SCROLLED_WINDOW);
    int rows = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-rows"));
    if (scroll) {
        /* A row of the pop-up list is one line of text with padding. */
        gtk_scrolled_window_set_max_content_height(GTK_SCROLLED_WINDOW(scroll), rows > 0 ? rows * 34 : 400);
        gtk_scrolled_window_set_propagate_natural_height(GTK_SCROLLED_WINDOW(scroll), TRUE);
    }
}

void gkGTKSelectConfigure(void *pointer, const char *placeholder, int popup_width) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SELECT) {
        return;
    }
    if (placeholder && *placeholder) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_PLACEHOLDER,
                                       placeholder, -1);
        gtk_widget_set_tooltip_text(view->control, placeholder);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_PLACEHOLDER);
    }
    g_object_set_data(G_OBJECT(view->outer), "gk-popup-width", GINT_TO_POINTER(MAX(0, popup_width)));
    gkSelectPopupSize(view);
}

void gkGTKSelectSetVisibleRows(void *pointer, int rows) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SELECT) {
        return;
    }
    g_object_set_data(G_OBJECT(view->outer), "gk-rows", GINT_TO_POINTER(MAX(0, rows)));
    gkSelectPopupSize(view);
}

/* ---- Value controls ---- */

static void gkSliderApplyDirection(GKView *view) {
    /* A vertical GTK scale has its minimum at the top; GameKit puts it at the
       bottom, like the other platforms. */
    gtk_range_set_inverted(GTK_RANGE(view->control), view->vertical != view->inverted);
}

void gkGTKSliderConfigure(void *pointer, int vertical, double minimum, double maximum, double frequency, int snap) {
    GKView *view = gkView(pointer);
    if (!view || !GTK_IS_SCALE(view->control)) {
        return;
    }
    GtkScale *scale = GTK_SCALE(view->control);
    view->vertical = vertical != 0;
    gtk_orientable_set_orientation(GTK_ORIENTABLE(scale), vertical ? GTK_ORIENTATION_VERTICAL
                                                                   : GTK_ORIENTATION_HORIZONTAL);
    gtk_widget_set_size_request(view->control, vertical ? -1 : 96, vertical ? 96 : -1);
    gkSliderApplyDirection(view);
    gtk_scale_clear_marks(scale);
    if (frequency > 0 && maximum > minimum) {
        int count = (int)floor((maximum - minimum) / frequency) + 1;
        for (int i = 0; i < count && i < 1000; i++) {
            gtk_scale_add_mark(scale, minimum + i * frequency, vertical ? GTK_POS_RIGHT : GTK_POS_BOTTOM, NULL);
        }
    }
    view->value = frequency > 0 && snap ? frequency : 0;
}

void gkGTKSliderSetInverted(void *pointer, int inverted) {
    GKView *view = gkView(pointer);
    if (view && GTK_IS_RANGE(view->control)) {
        view->inverted = inverted != 0;
        gkSliderApplyDirection(view);
    }
}

static gboolean gkProgressPulse(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    GKView *view = data;
    gint64 now = gdk_frame_clock_get_frame_time(clock);
    gint64 *last = g_object_get_data(G_OBJECT(widget), "gk-pulse-time");
    if (!view->running) {
        g_object_set_data(G_OBJECT(widget), "gk-pulse", NULL);
        return G_SOURCE_REMOVE;
    }
    if (!last) {
        last = g_new0(gint64, 1);
        g_object_set_data_full(G_OBJECT(widget), "gk-pulse-time", last, g_free);
    }
    if (now - *last > 100000) {
        *last = now;
        gtk_progress_bar_pulse(GTK_PROGRESS_BAR(widget));
    }
    return G_SOURCE_CONTINUE;
}

void gkGTKProgressConfigure(void *pointer, int indeterminate, int vertical, const char *text, int state) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_PROGRESS) {
        return;
    }
    GtkProgressBar *progress = GTK_PROGRESS_BAR(view->control);
    view->running = indeterminate != 0;
    if (indeterminate && !g_object_get_data(G_OBJECT(view->control), "gk-pulse")) {
        g_object_set_data(G_OBJECT(view->control), "gk-pulse", GINT_TO_POINTER(1));
        gtk_progress_bar_set_pulse_step(progress, 0.1);
        gtk_widget_add_tick_callback(view->control, gkProgressPulse, view, NULL);
    } else if (!indeterminate) {
        gtk_progress_bar_set_fraction(progress, view->value);
    }
    gtk_orientable_set_orientation(GTK_ORIENTABLE(progress), vertical ? GTK_ORIENTATION_VERTICAL
                                                                      : GTK_ORIENTATION_HORIZONTAL);
    gtk_progress_bar_set_inverted(progress, vertical != 0);
    gtk_widget_set_valign(view->control, vertical ? GTK_ALIGN_FILL : GTK_ALIGN_CENTER);
    gtk_widget_set_halign(view->control, vertical ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    gtk_progress_bar_set_text(progress, text && *text ? text : NULL);
    gtk_progress_bar_set_show_text(progress, text && *text);
    if (text && *text) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(progress), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, text, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(progress), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT);
    }
    static const char *classes[] = {NULL, "gk-paused", "gk-warning", "gk-error"};
    for (int i = 1; i < 4; i++) {
        gtk_widget_remove_css_class(view->control, classes[i]);
    }
    if (state > 0 && state < 4) {
        gtk_widget_add_css_class(view->control, classes[state]);
    }
    gkSetCSSRule("progressbar.gk-paused > trough > progress", "background-color: alpha(@theme_fg_color, 0.4);");
    gkSetCSSRule("progressbar.gk-warning > trough > progress", "background-color: @warning_color;");
    gkSetCSSRule("progressbar.gk-error > trough > progress", "background-color: @error_color;");
}

void gkGTKActivityConfigure(void *pointer, int size, const char *label) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ACTIVITY) {
        return;
    }
    static const int sizes[] = {24, 12, 16, 48};
    int pixels = sizes[size >= 0 && size < 4 ? size : 0];
    gtk_widget_set_size_request(view->control, pixels, pixels);
    gtk_widget_set_tooltip_text(view->control, label && *label ? label : NULL);
    if (label && *label) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL);
    }
}

void gkGTKSeparatorConfigure(void *pointer, int vertical, const char *label, int inset, int thickness) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SEPARATOR) {
        return;
    }
    GtkWidget *end = g_object_get_data(G_OBJECT(view->outer), "gk-end");
    GtkOrientation orientation = vertical ? GTK_ORIENTATION_VERTICAL : GTK_ORIENTATION_HORIZONTAL;
    view->vertical = vertical != 0;
    gtk_orientable_set_orientation(GTK_ORIENTABLE(view->outer), orientation);
    GtkWidget *lines[] = {view->control, end};
    for (int i = 0; i < 2; i++) {
        gtk_orientable_set_orientation(GTK_ORIENTABLE(lines[i]), orientation);
        gtk_widget_set_hexpand(lines[i], !vertical);
        gtk_widget_set_vexpand(lines[i], vertical);
        gtk_widget_set_valign(lines[i], vertical ? GTK_ALIGN_FILL : GTK_ALIGN_CENTER);
        gtk_widget_set_halign(lines[i], vertical ? GTK_ALIGN_CENTER : GTK_ALIGN_FILL);
    }
    gtk_label_set_text(GTK_LABEL(view->label), label ? label : "");
    gtk_widget_set_visible(view->label, label && *label);
    gtk_widget_set_visible(end, label && *label);
    if (label && *label) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
    }
    int margin = inset ? 16 : 0;
    if (vertical) {
        gtk_widget_set_margin_top(view->outer, margin);
        gtk_widget_set_margin_bottom(view->outer, margin);
        gtk_widget_set_margin_start(view->outer, 0);
        gtk_widget_set_margin_end(view->outer, 0);
    } else {
        gtk_widget_set_margin_start(view->outer, margin);
        gtk_widget_set_margin_end(view->outer, margin);
        gtk_widget_set_margin_top(view->outer, 0);
        gtk_widget_set_margin_bottom(view->outer, 0);
    }
    const char *name = gkWidgetCSSClass(view);
    char selector[96], rule[96];
    snprintf(selector, sizeof(selector), ".%s > separator", name);
    snprintf(rule, sizeof(rule), "%s: %dpx;", vertical ? "min-width" : "min-height", MAX(1, thickness));
    gkSetCSSRule(selector, rule);
}

void gkGTKImageConfigure(void *pointer, int alignment, int nearest, const char *alternative, double scale) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_IMAGE) {
        return;
    }
    view->alignment = alignment;
    view->nearest = nearest != 0;
    view->source_scale = scale > 0 ? scale : 1;
    if (alternative && *alternative) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, alternative, -1);
        gtk_accessible_update_state(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_STATE_HIDDEN, FALSE, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL);
        gtk_accessible_update_state(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_STATE_HIDDEN, TRUE, -1);
    }
    gtk_widget_queue_resize(view->outer);
    gtk_widget_queue_draw(view->outer);
}

void gkGTKLinkConfigure(void *pointer, int visited, void *icon) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_LINK) {
        return;
    }
    gtk_link_button_set_visited(GTK_LINK_BUTTON(view->control), visited != 0);
    gkButtonContent(view);
    GKButtonParts *parts = g_object_get_data(G_OBJECT(view->outer), "gk-button");
    gtk_widget_set_halign(parts->box, GTK_ALIGN_START);
    if (icon) {
        gtk_image_set_from_paintable(GTK_IMAGE(parts->icon), icon);
    } else {
        gtk_image_clear(GTK_IMAGE(parts->icon));
    }
    gtk_widget_set_visible(parts->icon, icon != NULL);
}

int gkGTKOpenURL(const char *url, int background) {
    (void)background;
    if (!url || !*url) {
        return 0;
    }
    GError *error = NULL;
    gboolean opened = g_app_info_launch_default_for_uri(url, NULL, &error);
    if (error) {
        g_error_free(error);
    }
    return opened;
}

void gkGTKTabsConfigure(void *pointer, int placement) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_TABS) {
        return;
    }
    static const GtkPositionType positions[] = {GTK_POS_TOP, GTK_POS_BOTTOM, GTK_POS_LEFT, GTK_POS_RIGHT};
    gtk_notebook_set_tab_pos(GTK_NOTEBOOK(view->outer), positions[placement >= 0 && placement < 4 ? placement : 0]);
}

void gkGTKTabsSetOverflow(void *pointer, int policy) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_TABS) {
        return;
    }
    GtkNotebook *notebook = GTK_NOTEBOOK(view->outer);
    gtk_notebook_set_scrollable(notebook, TRUE);
    if (policy == 0) {
        gtk_notebook_popup_enable(notebook);
    } else {
        gtk_notebook_popup_disable(notebook);
    }
}

/* ---- Scroll views ---- */

void gkGTKScrollConfigure(void *pointer, int axes, int policy, int top, int right, int bottom, int left,
                          int overscroll) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SCROLL) {
        return;
    }
    GtkScrolledWindow *scroll = GTK_SCROLLED_WINDOW(view->outer);
    GtkPolicyType shown = policy == 1 ? GTK_POLICY_ALWAYS : policy == 2 ? GTK_POLICY_EXTERNAL : GTK_POLICY_AUTOMATIC;
    int vertical = axes == 0 || axes == 2;
    int horizontal = axes == 1 || axes == 2;
    gtk_scrolled_window_set_policy(scroll, horizontal ? shown : GTK_POLICY_NEVER, vertical ? shown : GTK_POLICY_NEVER);
    gtk_scrolled_window_set_overlay_scrolling(scroll, policy == 0);
    gtk_scrolled_window_set_kinetic_scrolling(scroll, overscroll == 0);
    gtk_widget_set_margin_top(view->body, top);
    gtk_widget_set_margin_end(view->body, right);
    gtk_widget_set_margin_bottom(view->body, bottom);
    gtk_widget_set_margin_start(view->body, left);
}

void gkGTKScrollSetZoom(void *pointer, double zoom) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_SCROLL) {
        gkLayoutSetZoom(view->body, zoom);
    }
}

double gkGTKScrollZoom(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_SCROLL ? gkLayoutZoom(view->body) : 1;
}

void gkGTKScrollViewport(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0;
    if (view && view->kind == GK_KIND_SCROLL) {
        GtkWidget *viewport = gtk_scrolled_window_get_child(GTK_SCROLLED_WINDOW(view->outer));
        if (viewport && gtk_widget_get_width(viewport) > 1 && gtk_widget_get_width(view->outer) == view->width) {
            w = gtk_widget_get_width(viewport);
            h = gtk_widget_get_height(viewport);
        } else {
            gkGTKUIScrollViewportSize(pointer, &w, &h);
        }
    }
    *width = w;
    *height = h;
}

int gkGTKScrollToView(void *scroll_pointer, void *view_pointer, int animated) {
    GKView *scroll = gkView(scroll_pointer);
    GKView *target = gkView(view_pointer);
    if (!scroll || !target || scroll->kind != GK_KIND_SCROLL || !gtk_widget_is_ancestor(target->outer, scroll->body)) {
        return 0;
    }
    /* The frames of the Go layout are in the coordinates of the parent, so
       add them up to the body. */
    double x = 0, y = 0;
    for (GtkWidget *widget = target->outer; widget && widget != scroll->body; widget = gtk_widget_get_parent(widget)) {
        GKView *view = gkView(widget);
        if (view) {
            x += view->x;
            y += view->y;
        }
    }
    double zoom = gkLayoutZoom(scroll->body);
    x *= zoom;
    y *= zoom;
    double width = target->width * zoom;
    double height = target->height * zoom;
    GtkScrolledWindow *window = GTK_SCROLLED_WINDOW(scroll->outer);
    double origin_x = gtk_adjustment_get_value(gtk_scrolled_window_get_hadjustment(window));
    double origin_y = gtk_adjustment_get_value(gtk_scrolled_window_get_vadjustment(window));
    int viewport_width = 0, viewport_height = 0;
    gkGTKScrollViewport(scroll_pointer, &viewport_width, &viewport_height);
    if (x + width > origin_x + viewport_width) {
        origin_x = x + width - viewport_width;
    }
    if (x < origin_x) {
        origin_x = x;
    }
    if (y + height > origin_y + viewport_height) {
        origin_y = y + height - viewport_height;
    }
    if (y < origin_y) {
        origin_y = y;
    }
    gkScrollTo(window, origin_x, origin_y, animated);
    return 1;
}
