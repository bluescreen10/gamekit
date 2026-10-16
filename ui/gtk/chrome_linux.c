//go:build linux && cgo

#include "gtk_internal.h"

/* ---- Toolbar ---- */

typedef struct {
    GKWin *win;
    GtkWidget *bar;
    GHashTable *buttons;
    uintptr_t handle;
} GKToolbar;

static void gkToolbarActivate(GtkButton *button, gpointer data) {
    GKToolbar *toolbar = data;
    gkSend(toolbar->handle, GK_EVENT_ACTION, 0, 0, 0, 0, g_object_get_data(G_OBJECT(button), "gk-id"));
}

/* Item kinds: 0 action, 1 separator, 2 space, 3 flexible space. */
void *gkGTKToolbarCreate(uintptr_t handle, void *window, const int *kinds, const char *ids, size_t ids_size,
                         const char *labels, size_t labels_size, void **icons, const int *enabled, int count,
                         int shows_labels) {
    GKWin *win = window;
    if (!win) {
        return NULL;
    }
    GKToolbar *toolbar = g_new0(GKToolbar, 1);
    toolbar->win = win;
    toolbar->handle = handle;
    toolbar->buttons = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, NULL);
    toolbar->bar = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 4,
                                "accessible-role", GTK_ACCESSIBLE_ROLE_TOOLBAR, NULL);
    g_object_ref_sink(toolbar->bar);
    gtk_widget_add_css_class(toolbar->bar, "toolbar");
    int id_count = 0, label_count = 0;
    char **identifiers = gkSplit(ids, ids_size, &id_count);
    char **names = gkSplit(labels, labels_size, &label_count);
    for (int i = 0; i < count; i++) {
        switch (kinds[i]) {
        case 1:
            gtk_box_append(GTK_BOX(toolbar->bar), gtk_separator_new(GTK_ORIENTATION_VERTICAL));
            break;
        case 2: {
            GtkWidget *space = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
            gtk_widget_set_size_request(space, 12, -1);
            gtk_box_append(GTK_BOX(toolbar->bar), space);
            break;
        }
        case 3: {
            GtkWidget *space = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
            gtk_widget_set_hexpand(space, TRUE);
            gtk_box_append(GTK_BOX(toolbar->bar), space);
            break;
        }
        default: {
            const char *identifier = i < id_count ? identifiers[i] : "";
            const char *label = i < label_count ? names[i] : identifier;
            GtkWidget *button = gtk_button_new();
            gtk_widget_add_css_class(button, "flat");
            GtkWidget *content = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
            if (icons[i]) {
                gtk_box_append(GTK_BOX(content), gkIconImage(icons[i], 16));
            }
            if (shows_labels || !icons[i]) {
                GtkWidget *text = gtk_label_new(label);
                gtk_widget_add_css_class(text, "caption");
                gtk_box_append(GTK_BOX(content), text);
            }
            gtk_button_set_child(GTK_BUTTON(button), content);
            gtk_widget_set_tooltip_text(button, label);
            gtk_accessible_update_property(GTK_ACCESSIBLE(button), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
            gtk_widget_set_sensitive(button, enabled[i] != 0);
            g_object_set_data_full(G_OBJECT(button), "gk-id", g_strdup(identifier), g_free);
            g_signal_connect(button, "clicked", G_CALLBACK(gkToolbarActivate), toolbar);
            g_hash_table_insert(toolbar->buttons, g_strdup(identifier), button);
            gtk_box_append(GTK_BOX(toolbar->bar), button);
            break;
        }
        }
    }
    g_strfreev(identifiers);
    g_strfreev(names);
    gkWinAddChrome(win, toolbar->bar, 1);
    if (win->toolbar && win->toolbar != toolbar->bar) {
        gkRemoveFromParent(win->toolbar);
    }
    win->toolbar = toolbar->bar;
    return toolbar;
}

void gkGTKToolbarSetEnabled(void *pointer, const char *identifier, int enabled) {
    GKToolbar *toolbar = pointer;
    GtkWidget *button = toolbar ? g_hash_table_lookup(toolbar->buttons, identifier ? identifier : "") : NULL;
    if (button) {
        gtk_widget_set_sensitive(button, enabled != 0);
    }
}

void gkGTKToolbarRelease(void *pointer) {
    GKToolbar *toolbar = pointer;
    if (!toolbar) {
        return;
    }
    toolbar->handle = 0;
    if (!toolbar->win->destroyed) {
        if (toolbar->win->toolbar == toolbar->bar) {
            toolbar->win->toolbar = NULL;
        }
        gkRemoveFromParent(toolbar->bar);
    }
    g_hash_table_unref(toolbar->buttons);
    g_object_unref(toolbar->bar);
    g_free(toolbar);
}

/* ---- Status bar ---- */

void *gkGTKStatusBarCreate(uintptr_t window, const char *text) {
    GtkWidget *bar = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "accessible-role",
                                  GTK_ACCESSIBLE_ROLE_STATUS, NULL);
    gtk_widget_add_css_class(bar, "gk-status-bar");
    gkSetCSSRule("box.gk-status-bar", "border-top: 1px solid alpha(currentColor, 0.15); padding: 2px 8px; "
                                      "min-height: 18px;");
    GtkWidget *label = gtk_label_new(text ? text : "");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_label_set_ellipsize(GTK_LABEL(label), PANGO_ELLIPSIZE_END);
    gtk_widget_set_hexpand(label, TRUE);
    gtk_widget_add_css_class(label, "dim-label");
    gtk_widget_add_css_class(label, "caption");
    gtk_box_append(GTK_BOX(bar), label);
    GKView *view = gkViewNew(bar, label, GK_KIND_STATUS_BAR, window, 0);
    view->label = label;
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, bar);
    }
    return bar;
}

void gkGTKStatusBarSet(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_STATUS_BAR) {
        gtk_label_set_text(GTK_LABEL(view->label), text ? text : "");
    }
}

/* ---- Command bar ----

   A command bar shows one button for each action and moves the actions that
   do not fit into an overflow menu. */

typedef struct {
    GtkWidget *overflow;
    GtkWidget *list;
    GPtrArray *items;
} GKCommandBar;

static void gkCommandActivate(GtkButton *button, gpointer data) {
    GKView *view = data;
    GKCommandBar *bar = view->ext;
    if (gtk_widget_get_ancestor(GTK_WIDGET(button), GTK_TYPE_POPOVER)) {
        gtk_menu_button_popdown(GTK_MENU_BUTTON(bar->overflow));
    }
    gkSend(view->handle, GK_EVENT_ACTION, 0, 0, 0, 0, g_object_get_data(G_OBJECT(button), "gk-id"));
}

static void gkCommandArrange(GtkWidget *layout, int width, int height, gpointer data) {
    (void)layout;
    GKView *view = data;
    GKCommandBar *bar = view->ext;
    int overflow_width = 0, overflow_height = 0;
    gtk_widget_measure(bar->overflow, GTK_ORIENTATION_HORIZONTAL, -1, NULL, &overflow_width, NULL, NULL);
    gtk_widget_measure(bar->overflow, GTK_ORIENTATION_VERTICAL, -1, NULL, &overflow_height, NULL, NULL);
    int x = 0;
    int overflowing = 0;
    for (guint i = 0; i < view->buttons->len; i++) {
        GtkWidget *button = g_ptr_array_index(view->buttons, i);
        GtkWidget *item = g_ptr_array_index(bar->items, i);
        int button_width = 0, button_height = 0;
        gtk_widget_measure(button, GTK_ORIENTATION_HORIZONTAL, -1, NULL, &button_width, NULL, NULL);
        gtk_widget_measure(button, GTK_ORIENTATION_VERTICAL, -1, NULL, &button_height, NULL, NULL);
        int last = i == view->buttons->len - 1;
        int reserve = last ? 0 : overflow_width + 4;
        if (overflowing || x + button_width + reserve > width) {
            overflowing = 1;
            gkLayoutMove(button, 0, 0, 0, 0);
            gtk_widget_set_visible(item, TRUE);
            continue;
        }
        gtk_widget_set_visible(item, FALSE);
        gkLayoutMove(button, x, (height - button_height) / 2, button_width, button_height);
        x += button_width + 4;
    }
    if (overflowing) {
        gkLayoutMove(bar->overflow, MIN(x, width - overflow_width), (height - overflow_height) / 2, overflow_width,
                     overflow_height);
    } else {
        gkLayoutMove(bar->overflow, 0, 0, 0, 0);
    }
}

static void gkCommandBarFree(gpointer data) {
    GKCommandBar *bar = data;
    g_ptr_array_free(bar->items, TRUE);
    g_free(bar);
}

void *gkGTKCommandBarCreate(uintptr_t window) {
    GtkWidget *layout = gkLayoutNewWithRole(GTK_ACCESSIBLE_ROLE_TOOLBAR);
    GKCommandBar *bar = g_new0(GKCommandBar, 1);
    bar->items = g_ptr_array_new();
    bar->overflow = gtk_menu_button_new();
    gtk_menu_button_set_icon_name(GTK_MENU_BUTTON(bar->overflow), "view-more-symbolic");
    gtk_widget_add_css_class(bar->overflow, "flat");
    gtk_accessible_update_property(GTK_ACCESSIBLE(bar->overflow), GTK_ACCESSIBLE_PROPERTY_LABEL, "More", -1);
    GtkWidget *popover = gtk_popover_new();
    bar->list = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_popover_set_child(GTK_POPOVER(popover), bar->list);
    gtk_menu_button_set_popover(GTK_MENU_BUTTON(bar->overflow), popover);
    gtk_widget_set_parent(bar->overflow, layout);
    gkLayoutMove(bar->overflow, 0, 0, 0, 0);
    GKView *view = gkViewNew(layout, NULL, GK_KIND_COMMAND_BAR, window, 0);
    view->ext = bar;
    view->ext_free = gkCommandBarFree;
    view->buttons = g_ptr_array_new();
    gkLayoutOnAllocate(layout, gkCommandArrange, view);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, layout);
    }
    return layout;
}

static GtkWidget *gkCommandButton(const char *identifier, const char *title, void *icon, int compact, int flat) {
    GtkWidget *button = gtk_button_new();
    if (flat) {
        gtk_widget_add_css_class(button, "flat");
    }
    GtkWidget *content = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    if (icon) {
        gtk_box_append(GTK_BOX(content), gkIconImage(icon, 16));
    }
    if (!compact || !icon || flat) {
        GtkWidget *label = gtk_label_new(title);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_box_append(GTK_BOX(content), label);
    }
    gtk_button_set_child(GTK_BUTTON(button), content);
    gtk_widget_set_tooltip_text(button, title);
    gtk_accessible_update_property(GTK_ACCESSIBLE(button), GTK_ACCESSIBLE_PROPERTY_LABEL, title, -1);
    g_object_set_data_full(G_OBJECT(button), "gk-id", g_strdup(identifier), g_free);
    return button;
}

void gkGTKCommandBarSetActions(void *pointer, const char *ids, size_t ids_size, const char *titles,
                               size_t titles_size, void **icons, const int *enabled, int count, int compact) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_COMMAND_BAR) {
        return;
    }
    GKCommandBar *bar = view->ext;
    while (view->buttons->len > 0) {
        gtk_widget_unparent(g_ptr_array_index(view->buttons, view->buttons->len - 1));
        g_ptr_array_remove_index(view->buttons, view->buttons->len - 1);
    }
    while (bar->items->len > 0) {
        gtk_box_remove(GTK_BOX(bar->list), g_ptr_array_index(bar->items, bar->items->len - 1));
        g_ptr_array_remove_index(bar->items, bar->items->len - 1);
    }
    int id_count = 0, title_count = 0;
    char **identifiers = gkSplit(ids, ids_size, &id_count);
    char **names = gkSplit(titles, titles_size, &title_count);
    for (int i = 0; i < count; i++) {
        const char *identifier = i < id_count ? identifiers[i] : "";
        const char *title = i < title_count ? names[i] : "";
        GtkWidget *button = gkCommandButton(identifier, title, icons[i], compact, FALSE);
        gtk_widget_set_sensitive(button, enabled[i] != 0);
        g_signal_connect(button, "clicked", G_CALLBACK(gkCommandActivate), view);
        gtk_widget_insert_before(button, view->outer, bar->overflow);
        gkLayoutMove(button, 0, 0, 0, 0);
        g_ptr_array_add(view->buttons, button);
        GtkWidget *item = gkCommandButton(identifier, title, icons[i], compact, TRUE);
        gtk_widget_set_sensitive(item, enabled[i] != 0);
        gtk_widget_set_visible(item, FALSE);
        g_signal_connect(item, "clicked", G_CALLBACK(gkCommandActivate), view);
        gtk_box_append(GTK_BOX(bar->list), item);
        g_ptr_array_add(bar->items, item);
    }
    g_strfreev(identifiers);
    g_strfreev(names);
    int width = 0, height = 0;
    for (guint i = 0; i < view->buttons->len; i++) {
        int button_width = 0, button_height = 0;
        GtkWidget *button = g_ptr_array_index(view->buttons, i);
        gtk_widget_measure(button, GTK_ORIENTATION_HORIZONTAL, -1, NULL, &button_width, NULL, NULL);
        gtk_widget_measure(button, GTK_ORIENTATION_VERTICAL, -1, NULL, &button_height, NULL, NULL);
        width += button_width + 4;
        height = MAX(height, button_height);
    }
    g_object_set_data(G_OBJECT(view->outer), "gk-natural-width", GINT_TO_POINTER(MAX(0, width - 4)));
    g_object_set_data(G_OBJECT(view->outer), "gk-natural-height", GINT_TO_POINTER(height));
    gtk_widget_queue_allocate(view->outer);
}
