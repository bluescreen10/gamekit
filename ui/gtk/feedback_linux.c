//go:build linux && cgo

#include "gtk_internal.h"

#include <stdio.h>

/* Severity: 0 information, 1 success, 2 warning, 3 error. */
static const char *gkSeverityIcon(int severity) {
    switch (severity) {
    case 1:
        /* The current Adwaita theme has no "emblem-ok-symbolic" icon. */
        return "object-select-symbolic";
    case 2:
        return "dialog-warning-symbolic";
    case 3:
        return "dialog-error-symbolic";
    default:
        return "dialog-information-symbolic";
    }
}

static const char *gkSeverityClass(int severity) {
    switch (severity) {
    case 1:
        return "gk-success";
    case 2:
        return "gk-warning";
    case 3:
        return "gk-error";
    default:
        return "gk-info";
    }
}

static void gkSeverityStyles(void) {
    static int installed;
    if (installed) {
        return;
    }
    installed = 1;
    gkSetCSSRule(".gk-info-bar", "border-radius: 6px; padding: 6px 8px; border: 1px solid;");
    gkSetCSSRule(".gk-info-bar.gk-info", "background-color: alpha(@theme_selected_bg_color, 0.12); border-color: "
                                         "alpha(@theme_selected_bg_color, 0.45);");
    gkSetCSSRule(".gk-info-bar.gk-success", "background-color: alpha(@success_color, 0.12); border-color: "
                                            "alpha(@success_color, 0.45);");
    gkSetCSSRule(".gk-info-bar.gk-warning", "background-color: alpha(@warning_color, 0.12); border-color: "
                                            "alpha(@warning_color, 0.45);");
    gkSetCSSRule(".gk-info-bar.gk-error", "background-color: alpha(@error_color, 0.12); border-color: "
                                          "alpha(@error_color, 0.45);");
    gkSetCSSRule("image.gk-info, label.gk-info", "color: @accent_color;");
    gkSetCSSRule("image.gk-success, label.gk-success", "color: @success_color;");
    gkSetCSSRule("image.gk-warning, label.gk-warning", "color: @warning_color;");
    gkSetCSSRule("image.gk-error, label.gk-error", "color: @error_color;");
    gkSetCSSRule("label.gk-pill", "border-radius: 9px; padding: 0 6px; min-width: 6px; min-height: 18px; "
                                  "font-size: smaller; font-weight: bold; color: white;");
    gkSetCSSRule("label.gk-pill.gk-info", "background-color: @theme_selected_bg_color; color: white;");
    gkSetCSSRule("label.gk-pill.gk-success", "background-color: @success_color; color: white;");
    gkSetCSSRule("label.gk-pill.gk-warning", "background-color: @warning_color; color: white;");
    gkSetCSSRule("label.gk-pill.gk-error", "background-color: @error_color; color: white;");
    gkSetCSSRule(".gk-toast", "border-radius: 10px; padding: 6px 12px;");
}

static void gkSetSeverityClass(GtkWidget *widget, int severity) {
    for (int i = 0; i < 4; i++) {
        gtk_widget_remove_css_class(widget, gkSeverityClass(i));
    }
    gtk_widget_add_css_class(widget, gkSeverityClass(severity));
}

static GKView *gkFeedback(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind) {
    GKView *view = gkViewNew(outer, control, kind, window, 0);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, outer);
    }
    return view;
}

/* gkActionButtons appends one button for each action to box. */
static void gkActionButtons(GtkWidget *box, GCallback callback, gpointer data, const char *ids, size_t ids_size,
                            const char *titles, size_t titles_size, const int *enabled, int count) {
    int id_count = 0, title_count = 0;
    char **identifiers = gkSplit(ids, ids_size, &id_count);
    char **names = gkSplit(titles, titles_size, &title_count);
    for (int i = 0; i < count; i++) {
        GtkWidget *button = gtk_button_new_with_label(i < title_count ? names[i] : "");
        gtk_widget_set_valign(button, GTK_ALIGN_CENTER);
        gtk_widget_set_sensitive(button, enabled[i] != 0);
        g_object_set_data_full(G_OBJECT(button), "gk-id", g_strdup(i < id_count ? identifiers[i] : ""), g_free);
        g_signal_connect(button, "clicked", callback, data);
        gtk_box_append(GTK_BOX(box), button);
    }
    g_strfreev(identifiers);
    g_strfreev(names);
}

static void gkViewActionClicked(GtkButton *button, gpointer data) {
    GKView *view = data;
    gkSend(view->handle, GK_EVENT_ACTION, 0, 0, 0, 0, g_object_get_data(G_OBJECT(button), "gk-id"));
}

/* ---- Info bar ---- */

typedef struct {
    GtkWidget *icon;
    GtkWidget *message;
} GKInfoBar;

static void gkInfoBarClose(GtkButton *button, gpointer data) {
    (void)button;
    GKView *view = data;
    gkSend(view->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, NULL);
}

void *gkGTKInfoBarCreate(uintptr_t window, int closable, const char *ids, size_t ids_size, const char *titles,
                         size_t titles_size, const int *enabled, int count) {
    gkSeverityStyles();
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 8,
                                  "accessible-role", GTK_ACCESSIBLE_ROLE_ALERT, NULL);
    gtk_widget_add_css_class(box, "gk-info-bar");
    GKInfoBar *bar = g_new0(GKInfoBar, 1);
    bar->icon = gtk_image_new();
    gtk_widget_set_valign(bar->icon, GTK_ALIGN_CENTER);
    bar->message = gtk_label_new("");
    gtk_label_set_wrap(GTK_LABEL(bar->message), TRUE);
    gtk_label_set_xalign(GTK_LABEL(bar->message), 0);
    gtk_widget_set_hexpand(bar->message, TRUE);
    gtk_box_append(GTK_BOX(box), bar->icon);
    gtk_box_append(GTK_BOX(box), bar->message);
    GKView *view = gkFeedback(window, box, bar->message, GK_KIND_INFO_BAR);
    view->ext = bar;
    view->ext_free = g_free;
    gkActionButtons(box, G_CALLBACK(gkViewActionClicked), view, ids, ids_size, titles, titles_size, enabled, count);
    if (closable) {
        GtkWidget *close = gtk_button_new_from_icon_name("window-close-symbolic");
        gtk_widget_add_css_class(close, "flat");
        gtk_widget_set_valign(close, GTK_ALIGN_CENTER);
        gtk_accessible_update_property(GTK_ACCESSIBLE(close), GTK_ACCESSIBLE_PROPERTY_LABEL, "Close", -1);
        g_signal_connect(close, "clicked", G_CALLBACK(gkInfoBarClose), view);
        gtk_box_append(GTK_BOX(box), close);
    }
    return box;
}

void gkGTKInfoBarSet(void *pointer, int severity, const char *title, const char *detail) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_INFO_BAR) {
        return;
    }
    GKInfoBar *bar = view->ext;
    gtk_image_set_from_icon_name(GTK_IMAGE(bar->icon), gkSeverityIcon(severity));
    gkSetSeverityClass(bar->icon, severity);
    gkSetSeverityClass(view->outer, severity);
    char *markup = detail && *detail ? g_markup_printf_escaped("<b>%s</b>  %s", title ? title : "", detail)
                                     : g_markup_printf_escaped("<b>%s</b>", title ? title : "");
    gtk_label_set_markup(GTK_LABEL(bar->message), markup);
    g_free(markup);
    char *text = g_strdup_printf("%s%s%s", title ? title : "", detail && *detail ? " " : "", detail ? detail : "");
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, text, -1);
    g_free(text);
}

/* ---- Tooltip ---- */

typedef struct {
    GtkWidget *popover;
    GtkWidget *label;
    guint timer;
} GKTooltip;

static GQuark gkTooltipQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-tooltip");
    }
    return quark;
}

static void gkTooltipFree(gpointer data) {
    GKTooltip *tooltip = data;
    if (tooltip->timer) {
        g_source_remove(tooltip->timer);
    }
    if (gtk_widget_get_parent(tooltip->popover)) {
        gtk_widget_unparent(tooltip->popover);
    }
    g_object_unref(tooltip->popover);
    g_free(tooltip);
}

void *gkGTKTooltipCreate(void) {
    GKTooltip *tooltip = g_new0(GKTooltip, 1);
    tooltip->popover = g_object_new(GTK_TYPE_POPOVER, "accessible-role", GTK_ACCESSIBLE_ROLE_TOOLTIP, NULL);
    g_object_ref_sink(tooltip->popover);
    gtk_popover_set_autohide(GTK_POPOVER(tooltip->popover), FALSE);
    gtk_popover_set_has_arrow(GTK_POPOVER(tooltip->popover), FALSE);
    gtk_widget_set_can_target(tooltip->popover, FALSE);
    gtk_widget_set_can_focus(tooltip->popover, FALSE);
    gtk_widget_add_css_class(tooltip->popover, "tooltip");
    tooltip->label = gtk_label_new("");
    gtk_popover_set_child(GTK_POPOVER(tooltip->popover), tooltip->label);
    GObject *object = g_object_new(G_TYPE_OBJECT, NULL);
    g_object_set_qdata_full(object, gkTooltipQuark(), tooltip, gkTooltipFree);
    return object;
}

static GKTooltip *gkTooltip(void *pointer) {
    return pointer ? g_object_get_qdata(G_OBJECT(pointer), gkTooltipQuark()) : NULL;
}

void gkGTKTooltipSetText(void *pointer, const char *text) {
    GKTooltip *tooltip = gkTooltip(pointer);
    if (tooltip) {
        gtk_label_set_text(GTK_LABEL(tooltip->label), text ? text : "");
    }
}

static gboolean gkTooltipShowNow(gpointer data) {
    GKTooltip *tooltip = data;
    tooltip->timer = 0;
    gtk_popover_popup(GTK_POPOVER(tooltip->popover));
    return G_SOURCE_REMOVE;
}

int gkGTKTooltipShow(void *pointer, void *anchor_pointer, int edge, double delay) {
    GKTooltip *tooltip = gkTooltip(pointer);
    GKView *anchor = gkView(anchor_pointer);
    GKWin *win = anchor ? gkWinFromWidget(anchor->outer) : NULL;
    if (!tooltip || !anchor || !win || !gtk_widget_get_visible(win->window) || !gtk_widget_get_mapped(anchor->outer)) {
        return 0;
    }
    if (tooltip->timer) {
        g_source_remove(tooltip->timer);
        tooltip->timer = 0;
    }
    if (gtk_widget_get_parent(tooltip->popover) != anchor->outer) {
        if (gtk_widget_get_parent(tooltip->popover)) {
            gtk_widget_unparent(tooltip->popover);
        }
        gtk_widget_set_parent(tooltip->popover, anchor->outer);
    }
    static const GtkPositionType positions[] = {GTK_POS_TOP, GTK_POS_BOTTOM, GTK_POS_LEFT, GTK_POS_RIGHT};
    gtk_popover_set_position(GTK_POPOVER(tooltip->popover), positions[edge >= 0 && edge < 4 ? edge : 1]);
    if (delay <= 0) {
        gtk_popover_popup(GTK_POPOVER(tooltip->popover));
    } else {
        tooltip->timer = g_timeout_add((guint)(delay * 1000), gkTooltipShowNow, tooltip);
    }
    return 1;
}

void gkGTKTooltipHide(void *pointer) {
    GKTooltip *tooltip = gkTooltip(pointer);
    if (!tooltip) {
        return;
    }
    if (tooltip->timer) {
        g_source_remove(tooltip->timer);
        tooltip->timer = 0;
    }
    gtk_popover_popdown(GTK_POPOVER(tooltip->popover));
}

int gkGTKTooltipShown(void *pointer) {
    GKTooltip *tooltip = gkTooltip(pointer);
    return tooltip && (gtk_widget_get_visible(tooltip->popover) || tooltip->timer);
}

/* ---- Toast ---- */

typedef struct {
    GtkWidget *box;
    GtkWidget *message;
    GtkWidget *overlay;
    double timeout;
    guint timer;
    uintptr_t handle;
} GKToast;

static GQuark gkToastQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-toast");
    }
    return quark;
}

static void gkToastDismiss(GKToast *toast) {
    if (toast->timer) {
        g_source_remove(toast->timer);
        toast->timer = 0;
    }
    if (toast->overlay && gtk_widget_get_parent(toast->box) == toast->overlay) {
        gtk_overlay_remove_overlay(GTK_OVERLAY(toast->overlay), toast->box);
    }
    toast->overlay = NULL;
}

static void gkToastFree(gpointer data) {
    GKToast *toast = data;
    gkToastDismiss(toast);
    g_object_unref(toast->box);
    g_free(toast);
}

static GKToast *gkToast(void *pointer) {
    return pointer ? g_object_get_qdata(G_OBJECT(pointer), gkToastQuark()) : NULL;
}

static void gkToastAction(GtkButton *button, gpointer data) {
    GKToast *toast = data;
    gkSend(toast->handle, GK_EVENT_ACTION, 0, 0, 0, 0, g_object_get_data(G_OBJECT(button), "gk-id"));
    gkToastDismiss(toast);
}

void *gkGTKToastCreate(uintptr_t handle, const char *message, const char *ids, size_t ids_size, const char *titles,
                       size_t titles_size, const int *enabled, int count, double timeout) {
    gkSeverityStyles();
    GKToast *toast = g_new0(GKToast, 1);
    toast->handle = handle;
    toast->timeout = timeout;
    toast->box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 8,
                              "accessible-role", GTK_ACCESSIBLE_ROLE_STATUS, NULL);
    g_object_ref_sink(toast->box);
    gtk_widget_add_css_class(toast->box, "osd");
    gtk_widget_add_css_class(toast->box, "gk-toast");
    gtk_widget_set_halign(toast->box, GTK_ALIGN_CENTER);
    gtk_widget_set_valign(toast->box, GTK_ALIGN_END);
    gtk_widget_set_margin_bottom(toast->box, 20);
    toast->message = gtk_label_new(message ? message : "");
    gtk_label_set_ellipsize(GTK_LABEL(toast->message), PANGO_ELLIPSIZE_END);
    gtk_label_set_max_width_chars(GTK_LABEL(toast->message), 60);
    gtk_box_append(GTK_BOX(toast->box), toast->message);
    gkActionButtons(toast->box, G_CALLBACK(gkToastAction), toast, ids, ids_size, titles, titles_size, enabled, count);
    gtk_accessible_update_property(GTK_ACCESSIBLE(toast->box), GTK_ACCESSIBLE_PROPERTY_LABEL, message ? message : "",
                                   -1);
    GObject *object = g_object_new(G_TYPE_OBJECT, NULL);
    g_object_set_qdata_full(object, gkToastQuark(), toast, gkToastFree);
    return object;
}

static gboolean gkToastExpired(gpointer data) {
    GKToast *toast = data;
    toast->timer = 0;
    gkToastDismiss(toast);
    return G_SOURCE_REMOVE;
}

int gkGTKToastShow(void *pointer, void *window) {
    GKToast *toast = gkToast(pointer);
    GKWin *win = window;
    if (!toast || !win || !win->overlay) {
        return 0;
    }
    gkToastDismiss(toast);
    gtk_overlay_add_overlay(GTK_OVERLAY(win->overlay), toast->box);
    toast->overlay = win->overlay;
    if (toast->timeout > 0) {
        toast->timer = g_timeout_add((guint)(toast->timeout * 1000), gkToastExpired, toast);
    }
    gkGTKAnnounceText(toast->box, gtk_label_get_text(GTK_LABEL(toast->message)), 1);
    return 1;
}

void gkGTKToastDismiss(void *pointer) {
    GKToast *toast = gkToast(pointer);
    if (toast) {
        gkToastDismiss(toast);
    }
}

int gkGTKToastShown(void *pointer) {
    GKToast *toast = gkToast(pointer);
    return toast && toast->overlay != NULL;
}

void gkGTKToastRelease(void *pointer) {
    GKToast *toast = gkToast(pointer);
    if (toast) {
        toast->handle = 0;
    }
    if (pointer) {
        g_object_unref(pointer);
    }
}

/* ---- Badge ---- */

void *gkGTKBadgeCreate(uintptr_t window) {
    gkSeverityStyles();
    GtkWidget *label = g_object_new(GTK_TYPE_LABEL, "accessible-role", GTK_ACCESSIBLE_ROLE_STATUS, NULL);
    gtk_widget_add_css_class(label, "gk-pill");
    gtk_widget_set_valign(label, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(label, GTK_ALIGN_CENTER);
    gkFeedback(window, label, NULL, GK_KIND_BADGE);
    return label;
}

void gkGTKBadgeSet(void *pointer, const char *text, int severity, const char *accessible) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_BADGE) {
        return;
    }
    gtk_label_set_text(GTK_LABEL(view->outer), text ? text : "");
    gkSetSeverityClass(view->outer, severity);
    const char *value = accessible && *accessible ? accessible : text ? text : "";
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, value, -1);
}

/* ---- Progress dialog ---- */

typedef struct {
    GtkWidget *window;
    GtkWidget *message;
    GtkWidget *progress;
    uintptr_t handle;
    int indeterminate;
} GKProgressDialog;

static void gkProgressCancel(GtkButton *button, gpointer data) {
    (void)button;
    GKProgressDialog *dialog = data;
    gkSend(dialog->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, NULL);
}

static gboolean gkProgressDialogClose(GtkWindow *window, gpointer data) {
    (void)window;
    (void)data;
    return TRUE;
}

static gboolean gkProgressDialogPulse(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    GKProgressDialog *dialog = data;
    gint64 now = gdk_frame_clock_get_frame_time(clock);
    gint64 last = (gint64)GPOINTER_TO_SIZE(g_object_get_data(G_OBJECT(widget), "gk-last"));
    if (dialog->indeterminate && now - last > 100000) {
        g_object_set_data(G_OBJECT(widget), "gk-last", GSIZE_TO_POINTER((gsize)now));
        gtk_progress_bar_pulse(GTK_PROGRESS_BAR(widget));
    }
    return G_SOURCE_CONTINUE;
}

void *gkGTKProgressDialogCreate(uintptr_t handle, const char *title, int cancellable) {
    GKProgressDialog *dialog = g_new0(GKProgressDialog, 1);
    dialog->handle = handle;
    dialog->window = gtk_window_new();
    g_object_ref_sink(dialog->window);
    gtk_window_set_title(GTK_WINDOW(dialog->window), title ? title : "");
    gtk_window_set_modal(GTK_WINDOW(dialog->window), TRUE);
    gtk_window_set_resizable(GTK_WINDOW(dialog->window), FALSE);
    gtk_window_set_default_size(GTK_WINDOW(dialog->window), 360, -1);
    gtk_window_set_deletable(GTK_WINDOW(dialog->window), FALSE);
    g_signal_connect(dialog->window, "close-request", G_CALLBACK(gkProgressDialogClose), dialog);
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
    gtk_widget_set_margin_start(box, 20);
    gtk_widget_set_margin_end(box, 20);
    gtk_widget_set_margin_top(box, 16);
    gtk_widget_set_margin_bottom(box, 12);
    GtkWidget *heading = gtk_label_new(title ? title : "");
    gtk_widget_add_css_class(heading, "heading");
    gtk_label_set_xalign(GTK_LABEL(heading), 0);
    dialog->message = gtk_label_new("");
    gtk_label_set_xalign(GTK_LABEL(dialog->message), 0);
    gtk_label_set_ellipsize(GTK_LABEL(dialog->message), PANGO_ELLIPSIZE_END);
    dialog->progress = gtk_progress_bar_new();
    gtk_box_append(GTK_BOX(box), heading);
    gtk_box_append(GTK_BOX(box), dialog->message);
    gtk_box_append(GTK_BOX(box), dialog->progress);
    if (cancellable) {
        GtkWidget *cancel = gtk_button_new_with_label("Cancel");
        gtk_widget_set_halign(cancel, GTK_ALIGN_END);
        g_signal_connect(cancel, "clicked", G_CALLBACK(gkProgressCancel), dialog);
        GtkEventController *shortcuts = gtk_shortcut_controller_new();
        gtk_shortcut_controller_add_shortcut(
            GTK_SHORTCUT_CONTROLLER(shortcuts),
            gtk_shortcut_new(gtk_keyval_trigger_new(GDK_KEY_Escape, 0), g_object_ref(gtk_activate_action_get())));
        gtk_shortcut_controller_set_scope(GTK_SHORTCUT_CONTROLLER(shortcuts), GTK_SHORTCUT_SCOPE_MANAGED);
        gtk_widget_add_controller(cancel, shortcuts);
        gtk_box_append(GTK_BOX(box), cancel);
    }
    gtk_window_set_child(GTK_WINDOW(dialog->window), box);
    gtk_widget_add_tick_callback(dialog->progress, gkProgressDialogPulse, dialog, NULL);
    return dialog;
}

void gkGTKProgressDialogSet(void *pointer, const char *message, double value) {
    GKProgressDialog *dialog = pointer;
    if (!dialog) {
        return;
    }
    gtk_label_set_text(GTK_LABEL(dialog->message), message ? message : "");
    dialog->indeterminate = value < 0;
    if (value >= 0) {
        gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(dialog->progress), MIN(1, value));
    }
}

int gkGTKProgressDialogShow(void *pointer, void *parent) {
    GKProgressDialog *dialog = pointer;
    GKWin *win = parent;
    if (!dialog) {
        return 0;
    }
    if (win && gtk_widget_get_visible(win->window)) {
        gtk_window_set_transient_for(GTK_WINDOW(dialog->window), GTK_WINDOW(win->window));
    }
    gtk_window_present(GTK_WINDOW(dialog->window));
    return 1;
}

void gkGTKProgressDialogRelease(void *pointer) {
    GKProgressDialog *dialog = pointer;
    if (!dialog) {
        return;
    }
    dialog->handle = 0;
    g_signal_handlers_disconnect_by_data(dialog->window, dialog);
    gtk_window_destroy(GTK_WINDOW(dialog->window));
    g_object_unref(dialog->window);
    g_free(dialog);
}

/* ---- Empty state ---- */

typedef struct {
    GtkWidget *title;
    GtkWidget *detail;
} GKEmptyState;

void *gkGTKEmptyStateCreate(uintptr_t window, void *icon, const char *ids, size_t ids_size, const char *titles,
                            size_t titles_size, const int *enabled, int count) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_VERTICAL, "spacing", 6,
                                  "accessible-role", GTK_ACCESSIBLE_ROLE_GROUP, NULL);
    gtk_widget_set_valign(box, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(box, GTK_ALIGN_CENTER);
    if (icon) {
        GtkWidget *image = gkIconImage(icon, 48);
        gtk_widget_add_css_class(image, "dim-label");
        gtk_widget_set_margin_bottom(image, 8);
        gtk_box_append(GTK_BOX(box), image);
    }
    GKEmptyState *state = g_new0(GKEmptyState, 1);
    state->title = gtk_label_new("");
    gtk_widget_add_css_class(state->title, "title-3");
    gtk_label_set_wrap(GTK_LABEL(state->title), TRUE);
    gtk_label_set_justify(GTK_LABEL(state->title), GTK_JUSTIFY_CENTER);
    gtk_label_set_max_width_chars(GTK_LABEL(state->title), 40);
    state->detail = gtk_label_new("");
    gtk_widget_add_css_class(state->detail, "dim-label");
    gtk_label_set_wrap(GTK_LABEL(state->detail), TRUE);
    gtk_label_set_justify(GTK_LABEL(state->detail), GTK_JUSTIFY_CENTER);
    gtk_label_set_max_width_chars(GTK_LABEL(state->detail), 40);
    gtk_box_append(GTK_BOX(box), state->title);
    gtk_box_append(GTK_BOX(box), state->detail);
    GKView *view = gkFeedback(window, box, state->title, GK_KIND_EMPTY_STATE);
    view->ext = state;
    view->ext_free = g_free;
    if (count > 0) {
        GtkWidget *buttons = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_halign(buttons, GTK_ALIGN_CENTER);
        gtk_widget_set_margin_top(buttons, 6);
        gkActionButtons(buttons, G_CALLBACK(gkViewActionClicked), view, ids, ids_size, titles, titles_size, enabled,
                        count);
        gtk_box_append(GTK_BOX(box), buttons);
    }
    return box;
}

void gkGTKEmptyStateSet(void *pointer, const char *title, const char *detail) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_EMPTY_STATE) {
        return;
    }
    GKEmptyState *state = view->ext;
    gtk_label_set_text(GTK_LABEL(state->title), title ? title : "");
    gtk_label_set_text(GTK_LABEL(state->detail), detail ? detail : "");
    gtk_widget_set_visible(state->detail, detail && *detail);
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, title ? title : "",
                                   -1);
}

/* ---- Inline validation ---- */

void *gkGTKInlineValidationCreate(uintptr_t window) {
    gkSeverityStyles();
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
    GtkWidget *icon = gtk_image_new();
    gtk_image_set_pixel_size(GTK_IMAGE(icon), 14);
    gtk_widget_set_valign(icon, GTK_ALIGN_START);
    GtkWidget *label = gtk_label_new("");
    gtk_label_set_wrap(GTK_LABEL(label), TRUE);
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_widget_add_css_class(label, "caption");
    gtk_widget_set_hexpand(label, TRUE);
    gtk_box_append(GTK_BOX(box), icon);
    gtk_box_append(GTK_BOX(box), label);
    GKView *view = gkFeedback(window, box, label, GK_KIND_VALIDATION);
    view->label = icon;
    return box;
}

void gkGTKInlineValidationSet(void *pointer, int severity, const char *text) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_VALIDATION) {
        return;
    }
    gtk_image_set_from_icon_name(GTK_IMAGE(view->label), gkSeverityIcon(severity));
    gkSetSeverityClass(view->label, severity);
    gtk_label_set_text(GTK_LABEL(view->control), text ? text : "");
    for (int i = 0; i < 4; i++) {
        gtk_widget_remove_css_class(view->control, gkSeverityClass(i));
    }
    if (severity >= 2) {
        gtk_widget_add_css_class(view->control, gkSeverityClass(severity));
        gtk_widget_remove_css_class(view->control, "dim-label");
    } else {
        gtk_widget_add_css_class(view->control, "dim-label");
    }
}
