//go:build linux && cgo

#include "gtk_internal.h"

#include <stdio.h>

/* A menu is a GMenu. The root menu owns a GKMenu record with the action
   group that its items and its submenus use; GTK looks the actions up under
   the record's prefix. */

typedef struct GKMenu {
    uintptr_t handle;
    char prefix[24];
    GSimpleActionGroup *actions;
    GHashTable *names;
    GListStore *shortcuts;
    guint next;
    GtkWidget *popover;
} GKMenu;

static guint gkMenuCount;

static GQuark gkMenuQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-menu");
    }
    return quark;
}

static GQuark gkMenuRootQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-menu-root");
    }
    return quark;
}

static void gkMenuFree(gpointer data) {
    GKMenu *menu = data;
    if (menu->popover) {
        if (gtk_widget_get_parent(menu->popover)) {
            gtk_widget_unparent(menu->popover);
        }
        g_object_unref(menu->popover);
    }
    g_object_unref(menu->actions);
    g_hash_table_unref(menu->names);
    g_object_unref(menu->shortcuts);
    g_free(menu);
}

static GKMenu *gkMenuRecord(void *pointer) {
    if (!pointer || !G_IS_MENU(pointer)) {
        return NULL;
    }
    GKMenu *menu = g_object_get_qdata(G_OBJECT(pointer), gkMenuQuark());
    if (menu) {
        return menu;
    }
    return g_object_get_qdata(G_OBJECT(pointer), gkMenuRootQuark());
}

GMenuModel *gkMenuModel(void *menu) {
    return menu && G_IS_MENU_MODEL(menu) ? G_MENU_MODEL(menu) : NULL;
}

GActionGroup *gkMenuActions(void *pointer) {
    GKMenu *menu = gkMenuRecord(pointer);
    return menu ? G_ACTION_GROUP(menu->actions) : NULL;
}

const char *gkMenuPrefix(void *pointer) {
    GKMenu *menu = gkMenuRecord(pointer);
    return menu ? menu->prefix : "";
}

GListModel *gkMenuShortcuts(void *pointer) {
    GKMenu *menu = gkMenuRecord(pointer);
    return menu ? G_LIST_MODEL(menu->shortcuts) : NULL;
}

void *gkGTKMenuCreate(uintptr_t handle, const char *title) {
    (void)title;
    GMenu *model = g_menu_new();
    GKMenu *menu = g_new0(GKMenu, 1);
    menu->handle = handle;
    snprintf(menu->prefix, sizeof(menu->prefix), "gkm%u", ++gkMenuCount);
    menu->actions = g_simple_action_group_new();
    menu->names = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
    menu->shortcuts = g_list_store_new(GTK_TYPE_SHORTCUT);
    g_object_set_qdata_full(G_OBJECT(model), gkMenuQuark(), menu, gkMenuFree);
    return model;
}

void gkGTKMenuSetHandle(void *pointer, uintptr_t handle) {
    GKMenu *menu = gkMenuRecord(pointer);
    if (menu) {
        menu->handle = handle;
    }
}

void gkGTKMenuClear(void *pointer) {
    if (!pointer || !G_IS_MENU(pointer)) {
        return;
    }
    g_menu_remove_all(G_MENU(pointer));
    g_object_set_data(G_OBJECT(pointer), "gk-section", NULL);
    GKMenu *menu = g_object_get_qdata(G_OBJECT(pointer), gkMenuQuark());
    if (menu) {
        /* A root menu drops its actions; submenus are rebuilt with it. */
        char **names = g_action_group_list_actions(G_ACTION_GROUP(menu->actions));
        for (int i = 0; names && names[i]; i++) {
            g_action_map_remove_action(G_ACTION_MAP(menu->actions), names[i]);
        }
        g_strfreev(names);
        g_hash_table_remove_all(menu->names);
        g_list_store_remove_all(menu->shortcuts);
    }
}

static GMenu *gkMenuTarget(void *pointer);

static void gkMenuActivate(GSimpleAction *action, GVariant *parameter, gpointer data) {
    (void)parameter;
    GKMenu *menu = data;
    GVariant *state = g_action_get_state(G_ACTION(action));
    if (state) {
        g_simple_action_set_state(action, g_variant_new_boolean(!g_variant_get_boolean(state)));
        g_variant_unref(state);
    }
    const char *identifier = g_object_get_data(G_OBJECT(action), "gk-id");
    gkSend(menu->handle, GK_EVENT_ACTION, 0, 0, 0, 0, identifier);
}

/* gkMenuAddAction adds an action. Only a checked action has a state, because
   GTK draws a check box for every action with a state. */
static void gkMenuAddAction(GKMenu *menu, const char *name, const char *identifier, int checked) {
    GSimpleAction *action = checked ? g_simple_action_new_stateful(name, NULL, g_variant_new_boolean(TRUE))
                                    : g_simple_action_new(name, NULL);
    g_object_set_data_full(G_OBJECT(action), "gk-id", g_strdup(identifier), g_free);
    g_signal_connect(action, "activate", G_CALLBACK(gkMenuActivate), menu);
    g_action_map_add_action(G_ACTION_MAP(menu->actions), G_ACTION(action));
    g_object_unref(action);
}

static void gkMenuIcon(GMenuItem *item, void *icon) {
    if (!icon) {
        return;
    }
    GIcon *gicon = NULL;
    if (G_IS_ICON(icon)) {
        gicon = g_object_ref(icon);
    } else if (GTK_IS_ICON_PAINTABLE(icon)) {
        const char *name = gtk_icon_paintable_get_icon_name(GTK_ICON_PAINTABLE(icon));
        if (name) {
            gicon = g_themed_icon_new(name);
        }
    }
    if (gicon) {
        g_menu_item_set_icon(item, gicon);
        g_object_unref(gicon);
    }
}

void gkGTKMenuAddItem(void *pointer, const char *action_id, const char *title, int enabled, int checked,
                      const char *key, int modifiers, void *icon, const char *tooltip) {
    (void)modifiers;
    (void)tooltip;
    GKMenu *menu = gkMenuRecord(pointer);
    if (!menu || !G_IS_MENU(pointer)) {
        return;
    }
    const char *identifier = action_id ? action_id : "";
    const char *name = g_hash_table_lookup(menu->names, identifier);
    GSimpleAction *action = NULL;
    if (!name) {
        char *created = g_strdup_printf("a%u", ++menu->next);
        g_hash_table_insert(menu->names, g_strdup(identifier), created);
        name = created;
        gkMenuAddAction(menu, name, identifier, checked != 0);
    }
    action = G_SIMPLE_ACTION(g_action_map_lookup_action(G_ACTION_MAP(menu->actions), name));
    if (checked && !g_action_get_state_type(G_ACTION(action))) {
        gkMenuAddAction(menu, name, identifier, 1);
        action = G_SIMPLE_ACTION(g_action_map_lookup_action(G_ACTION_MAP(menu->actions), name));
    }
    g_simple_action_set_enabled(action, enabled != 0);
    char *detailed = g_strdup_printf("%s.%s", menu->prefix, name);
    GMenuItem *item = g_menu_item_new(title ? title : "", detailed);
    if (key && *key) {
        g_menu_item_set_attribute(item, "accel", "s", key);
        GtkShortcutTrigger *trigger = gtk_shortcut_trigger_parse_string(key);
        if (trigger) {
            GtkShortcut *shortcut = gtk_shortcut_new(trigger, gtk_named_action_new(detailed));
            g_list_store_append(menu->shortcuts, shortcut);
            g_object_unref(shortcut);
        }
    }
    gkMenuIcon(item, icon);
    g_menu_append_item(gkMenuTarget(pointer), item);
    g_object_unref(item);
    g_free(detailed);
}

/* GMenu draws separators between sections, so a menu holds sections and a
   separator starts a new one. */
static GMenu *gkMenuNewSection(void *pointer) {
    GMenu *section = g_menu_new();
    g_menu_append_section(G_MENU(pointer), NULL, G_MENU_MODEL(section));
    g_object_set_data_full(G_OBJECT(pointer), "gk-section", section, g_object_unref);
    return section;
}

static GMenu *gkMenuTarget(void *pointer) {
    GMenu *section = g_object_get_data(G_OBJECT(pointer), "gk-section");
    return section ? section : gkMenuNewSection(pointer);
}

void gkGTKMenuAddSeparator(void *pointer) {
    if (pointer && G_IS_MENU(pointer)) {
        gkMenuNewSection(pointer);
    }
}

void *gkGTKMenuAddSubmenu(void *pointer, const char *title) {
    GKMenu *menu = gkMenuRecord(pointer);
    if (!menu || !G_IS_MENU(pointer)) {
        return NULL;
    }
    GMenu *submenu = g_menu_new();
    g_object_set_qdata(G_OBJECT(submenu), gkMenuRootQuark(), menu);
    g_menu_append_submenu(gkMenuTarget(pointer), title ? title : "", G_MENU_MODEL(submenu));
    /* The parent model keeps the submenu alive. */
    g_object_unref(submenu);
    return submenu;
}

void gkGTKMenuSetActionState(void *pointer, const char *action_id, int enabled, int checked) {
    GKMenu *menu = gkMenuRecord(pointer);
    if (!menu) {
        return;
    }
    const char *name = g_hash_table_lookup(menu->names, action_id ? action_id : "");
    GAction *action = name ? g_action_map_lookup_action(G_ACTION_MAP(menu->actions), name) : NULL;
    if (!action) {
        return;
    }
    if (enabled >= 0) {
        g_simple_action_set_enabled(G_SIMPLE_ACTION(action), enabled != 0);
    }
    if (checked > 0 && !g_action_get_state_type(action)) {
        gboolean was_enabled = g_action_get_enabled(action);
        char *id = g_strdup(g_object_get_data(G_OBJECT(action), "gk-id"));
        gkMenuAddAction(menu, name, id, 1);
        g_free(id);
        action = g_action_map_lookup_action(G_ACTION_MAP(menu->actions), name);
        g_simple_action_set_enabled(G_SIMPLE_ACTION(action), was_enabled);
    } else if (checked >= 0 && g_action_get_state_type(action)) {
        g_simple_action_set_state(G_SIMPLE_ACTION(action), g_variant_new_boolean(checked != 0));
    }
}

/* gkMenuPopupAt shows menu at a point of widget. */
static void gkMenuPopupAt(void *pointer, GtkWidget *widget, double x, double y) {
    GKMenu *menu = gkMenuRecord(pointer);
    if (!menu || !widget) {
        return;
    }
    if (menu->popover && gtk_widget_get_parent(menu->popover) != widget) {
        gtk_widget_unparent(menu->popover);
        g_object_unref(menu->popover);
        menu->popover = NULL;
    }
    if (!menu->popover) {
        menu->popover = gtk_popover_menu_new_from_model(G_MENU_MODEL(pointer));
        g_object_ref_sink(menu->popover);
        gtk_popover_set_has_arrow(GTK_POPOVER(menu->popover), FALSE);
        gtk_widget_set_halign(menu->popover, GTK_ALIGN_START);
        gtk_widget_set_parent(menu->popover, widget);
    }
    gtk_widget_insert_action_group(widget, menu->prefix, G_ACTION_GROUP(menu->actions));
    GdkRectangle rect = {(int)x, (int)y, 1, 1};
    gtk_popover_set_pointing_to(GTK_POPOVER(menu->popover), &rect);
    gtk_popover_popup(GTK_POPOVER(menu->popover));
}

void gkGTKMenuPopup(void *pointer, void *view_pointer, double x, double y) {
    GKView *view = gkView(view_pointer);
    if (view) {
        gkMenuPopupAt(pointer, view->outer, x, y);
    }
}

static void gkContextPressed(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    (void)data;
    GtkWidget *widget = gtk_event_controller_get_widget(GTK_EVENT_CONTROLLER(gesture));
    void *menu = g_object_get_data(G_OBJECT(widget), "gk-context-menu");
    if (menu) {
        gtk_gesture_set_state(GTK_GESTURE(gesture), GTK_EVENT_SEQUENCE_CLAIMED);
        gkMenuPopupAt(menu, widget, x, y);
    }
}

void gkMenuAttach(GtkWidget *widget, void *menu) {
    if (menu) {
        g_object_set_data_full(G_OBJECT(widget), "gk-context-menu", g_object_ref(menu), g_object_unref);
    } else {
        g_object_set_data(G_OBJECT(widget), "gk-context-menu", NULL);
    }
    if (!g_object_get_data(G_OBJECT(widget), "gk-context-gesture")) {
        GtkGesture *gesture = gtk_gesture_click_new();
        gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(gesture), GDK_BUTTON_SECONDARY);
        gtk_event_controller_set_propagation_phase(GTK_EVENT_CONTROLLER(gesture), GTK_PHASE_CAPTURE);
        g_signal_connect(gesture, "pressed", G_CALLBACK(gkContextPressed), NULL);
        gtk_widget_add_controller(widget, GTK_EVENT_CONTROLLER(gesture));
        g_object_set_data(G_OBJECT(widget), "gk-context-gesture", gesture);
    }
}

void gkGTKSetMenu(void *view_pointer, void *menu) {
    GKView *view = gkView(view_pointer);
    if (view) {
        gkMenuAttach(view->outer, menu);
    }
}

/* ---- Menu bar ---- */

typedef struct {
    GKWin *win;
    GMenu *model;
    GtkWidget *bar;
    GtkEventController *shortcuts;
    GPtrArray *menus;
} GKMenuBar;

void *gkGTKMenuBarCreate(void *window, const char *titles, size_t titles_size, void **menus, int count) {
    GKWin *win = window;
    if (!win) {
        return NULL;
    }
    for (int i = 0; i < count; i++) {
        if (!menus[i] || g_object_get_data(G_OBJECT(menus[i]), "gk-in-bar")) {
            return NULL;
        }
    }
    GKMenuBar *bar = g_new0(GKMenuBar, 1);
    bar->win = win;
    bar->model = g_menu_new();
    bar->menus = g_ptr_array_new_with_free_func(g_object_unref);
    int title_count = 0;
    char **names = gkSplit(titles, titles_size, &title_count);
    GListStore *shortcuts = g_list_store_new(G_TYPE_LIST_MODEL);
    for (int i = 0; i < count; i++) {
        g_menu_append_submenu(bar->model, i < title_count ? names[i] : "", G_MENU_MODEL(menus[i]));
        g_object_set_data(G_OBJECT(menus[i]), "gk-in-bar", bar);
        g_ptr_array_add(bar->menus, g_object_ref(menus[i]));
        gtk_widget_insert_action_group(win->window, gkMenuPrefix(menus[i]), gkMenuActions(menus[i]));
        g_list_store_append(shortcuts, gkMenuShortcuts(menus[i]));
    }
    g_strfreev(names);
    bar->bar = gtk_popover_menu_bar_new_from_model(G_MENU_MODEL(bar->model));
    g_object_ref_sink(bar->bar);
    gkWinAddChrome(win, bar->bar, 1);
    gtk_box_reorder_child_after(GTK_BOX(win->top), bar->bar, NULL);
    bar->shortcuts = gtk_shortcut_controller_new_for_model(G_LIST_MODEL(gtk_flatten_list_model_new(G_LIST_MODEL(shortcuts))));
    gtk_shortcut_controller_set_scope(GTK_SHORTCUT_CONTROLLER(bar->shortcuts), GTK_SHORTCUT_SCOPE_GLOBAL);
    gtk_widget_add_controller(win->window, bar->shortcuts);
    win->menubar = bar->bar;
    return bar;
}

void gkGTKMenuBarInstall(void *pointer) {
    GKMenuBar *bar = pointer;
    if (bar && bar->bar) {
        gtk_widget_set_visible(bar->bar, TRUE);
    }
}

void gkGTKMenuBarRelease(void *pointer) {
    GKMenuBar *bar = pointer;
    if (!bar) {
        return;
    }
    if (!bar->win->destroyed) {
        gtk_widget_remove_controller(bar->win->window, bar->shortcuts);
        for (guint i = 0; i < bar->menus->len; i++) {
            gtk_widget_insert_action_group(bar->win->window, gkMenuPrefix(g_ptr_array_index(bar->menus, i)), NULL);
        }
        gkRemoveFromParent(bar->bar);
        if (bar->win->menubar == bar->bar) {
            bar->win->menubar = NULL;
        }
    }
    for (guint i = 0; i < bar->menus->len; i++) {
        g_object_set_data(G_OBJECT(g_ptr_array_index(bar->menus, i)), "gk-in-bar", NULL);
    }
    g_ptr_array_free(bar->menus, TRUE);
    g_object_unref(bar->bar);
    g_object_unref(bar->model);
    g_free(bar);
}
