//go:build linux && cgo

#include "gtk_internal.h"

#include <stdio.h>
#include <unistd.h>

/* A system tray item is a StatusNotifierItem on the session bus. Its menu is
   exported with the com.canonical.dbusmenu interface. The item registers
   with the StatusNotifierWatcher whenever one runs, and a tray host such as
   the KDE panel or the GNOME AppIndicator extension shows it. */

extern GMenuModel *gkMenuModel(void *menu);
extern GActionGroup *gkMenuActions(void *menu);

static const char gkItemXML[] =
    "<node><interface name='org.kde.StatusNotifierItem'>"
    "<property name='Category' type='s' access='read'/>"
    "<property name='Id' type='s' access='read'/>"
    "<property name='Title' type='s' access='read'/>"
    "<property name='Status' type='s' access='read'/>"
    "<property name='WindowId' type='i' access='read'/>"
    "<property name='IconName' type='s' access='read'/>"
    "<property name='IconPixmap' type='a(iiay)' access='read'/>"
    "<property name='AttentionIconName' type='s' access='read'/>"
    "<property name='ToolTip' type='(sa(iiay)ss)' access='read'/>"
    "<property name='ItemIsMenu' type='b' access='read'/>"
    "<property name='Menu' type='o' access='read'/>"
    "<method name='ContextMenu'><arg type='i' direction='in'/><arg type='i' direction='in'/></method>"
    "<method name='Activate'><arg type='i' direction='in'/><arg type='i' direction='in'/></method>"
    "<method name='SecondaryActivate'><arg type='i' direction='in'/><arg type='i' direction='in'/></method>"
    "<method name='Scroll'><arg type='i' direction='in'/><arg type='s' direction='in'/></method>"
    "<signal name='NewTitle'/><signal name='NewIcon'/><signal name='NewAttentionIcon'/>"
    "<signal name='NewToolTip'/><signal name='NewStatus'><arg type='s'/></signal>"
    "</interface></node>";

static const char gkMenuXML[] =
    "<node><interface name='com.canonical.dbusmenu'>"
    "<property name='Version' type='u' access='read'/>"
    "<property name='TextDirection' type='s' access='read'/>"
    "<property name='Status' type='s' access='read'/>"
    "<property name='IconThemePath' type='as' access='read'/>"
    "<method name='GetLayout'><arg type='i' direction='in'/><arg type='i' direction='in'/>"
    "<arg type='as' direction='in'/><arg type='u' direction='out'/><arg type='(ia{sv}av)' direction='out'/></method>"
    "<method name='GetGroupProperties'><arg type='ai' direction='in'/><arg type='as' direction='in'/>"
    "<arg type='a(ia{sv})' direction='out'/></method>"
    "<method name='GetProperty'><arg type='i' direction='in'/><arg type='s' direction='in'/>"
    "<arg type='v' direction='out'/></method>"
    "<method name='Event'><arg type='i' direction='in'/><arg type='s' direction='in'/>"
    "<arg type='v' direction='in'/><arg type='u' direction='in'/></method>"
    "<method name='EventGroup'><arg type='a(isvu)' direction='in'/><arg type='ai' direction='out'/></method>"
    "<method name='AboutToShow'><arg type='i' direction='in'/><arg type='b' direction='out'/></method>"
    "<method name='AboutToShowGroup'><arg type='ai' direction='in'/><arg type='ai' direction='out'/>"
    "<arg type='ai' direction='out'/></method>"
    "<signal name='ItemsPropertiesUpdated'><arg type='a(ia{sv})'/><arg type='a(ias)'/></signal>"
    "<signal name='LayoutUpdated'><arg type='u'/><arg type='i'/></signal>"
    "</interface></node>";

typedef struct {
    char *action;
    GVariant *properties;
    GArray *children;
} GKMenuEntry;

typedef struct {
    uintptr_t handle;
    GDBusConnection *bus;
    char *item_path;
    char *menu_path;
    guint item_registration;
    guint menu_registration;
    guint watch;
    int registered;
    char *title;
    char *tooltip;
    char *icon_name;
    GVariant *pixmap;
    int visible;
    int attention;
    GMenuModel *menu;
    GActionGroup *actions;
    gulong menu_changed;
    gulong actions_changed[2];
    guint revision;
    GPtrArray *entries;
} GKTray;

static guint gkTrayCount;

/* ---- Menu layout ---- */

static void gkEntryFree(gpointer data) {
    GKMenuEntry *entry = data;
    g_free(entry->action);
    if (entry->properties) {
        g_variant_unref(entry->properties);
    }
    g_array_free(entry->children, TRUE);
    g_free(entry);
}

static char *gkMnemonic(const char *label) {
    GString *text = g_string_new("");
    for (const char *p = label ? label : ""; *p; p++) {
        if (*p == '_') {
            g_string_append_c(text, '_');
        }
        g_string_append_c(text, *p);
    }
    return g_string_free(text, FALSE);
}

static int gkTrayAddEntry(GKTray *tray, GVariant *properties, const char *action) {
    GKMenuEntry *entry = g_new0(GKMenuEntry, 1);
    entry->properties = g_variant_ref_sink(properties);
    entry->action = g_strdup(action);
    entry->children = g_array_new(FALSE, FALSE, sizeof(int));
    g_ptr_array_add(tray->entries, entry);
    return (int)tray->entries->len - 1;
}

static void gkTrayBuild(GKTray *tray, GMenuModel *model, int parent);

static void gkTrayAddSeparator(GKTray *tray, int parent) {
    GVariantBuilder properties;
    g_variant_builder_init(&properties, G_VARIANT_TYPE("a{sv}"));
    g_variant_builder_add(&properties, "{sv}", "type", g_variant_new_string("separator"));
    int id = gkTrayAddEntry(tray, g_variant_builder_end(&properties), NULL);
    GKMenuEntry *owner = g_ptr_array_index(tray->entries, parent);
    g_array_append_val(owner->children, id);
}

static void gkTrayBuild(GKTray *tray, GMenuModel *model, int parent) {
    int count = g_menu_model_get_n_items(model);
    for (int i = 0; i < count; i++) {
        GMenuModel *section = g_menu_model_get_item_link(model, i, G_MENU_LINK_SECTION);
        if (section) {
            GKMenuEntry *owner = g_ptr_array_index(tray->entries, parent);
            if (owner->children->len > 0 && g_menu_model_get_n_items(section) > 0) {
                gkTrayAddSeparator(tray, parent);
            }
            gkTrayBuild(tray, section, parent);
            g_object_unref(section);
            continue;
        }
        char *label = NULL;
        char *action = NULL;
        g_menu_model_get_item_attribute(model, i, G_MENU_ATTRIBUTE_LABEL, "s", &label);
        g_menu_model_get_item_attribute(model, i, G_MENU_ATTRIBUTE_ACTION, "s", &action);
        const char *name = action ? strchr(action, '.') : NULL;
        name = name ? name + 1 : action;
        GVariantBuilder properties;
        g_variant_builder_init(&properties, G_VARIANT_TYPE("a{sv}"));
        char *text = gkMnemonic(label);
        g_variant_builder_add(&properties, "{sv}", "label", g_variant_new_string(text));
        g_free(text);
        GMenuModel *submenu = g_menu_model_get_item_link(model, i, G_MENU_LINK_SUBMENU);
        if (submenu) {
            g_variant_builder_add(&properties, "{sv}", "children-display", g_variant_new_string("submenu"));
        }
        if (name && tray->actions && g_action_group_has_action(tray->actions, name)) {
            g_variant_builder_add(&properties, "{sv}", "enabled",
                                  g_variant_new_boolean(g_action_group_get_action_enabled(tray->actions, name)));
            GVariant *state = g_action_group_get_action_state(tray->actions, name);
            if (state && g_variant_is_of_type(state, G_VARIANT_TYPE_BOOLEAN)) {
                g_variant_builder_add(&properties, "{sv}", "toggle-type", g_variant_new_string("checkmark"));
                g_variant_builder_add(&properties, "{sv}", "toggle-state",
                                      g_variant_new_int32(g_variant_get_boolean(state) ? 1 : 0));
            }
            if (state) {
                g_variant_unref(state);
            }
        }
        int id = gkTrayAddEntry(tray, g_variant_builder_end(&properties), name);
        GKMenuEntry *owner = g_ptr_array_index(tray->entries, parent);
        g_array_append_val(owner->children, id);
        if (submenu) {
            gkTrayBuild(tray, submenu, id);
            g_object_unref(submenu);
        }
        g_free(label);
        g_free(action);
    }
}

static void gkTrayRebuild(GKTray *tray) {
    if (tray->entries) {
        g_ptr_array_free(tray->entries, TRUE);
    }
    tray->entries = g_ptr_array_new_with_free_func(gkEntryFree);
    GVariantBuilder root;
    g_variant_builder_init(&root, G_VARIANT_TYPE("a{sv}"));
    g_variant_builder_add(&root, "{sv}", "children-display", g_variant_new_string("submenu"));
    gkTrayAddEntry(tray, g_variant_builder_end(&root), NULL);
    if (tray->menu) {
        gkTrayBuild(tray, tray->menu, 0);
    }
}

static GVariant *gkTrayLayout(GKTray *tray, int id, int depth) {
    GKMenuEntry *entry = g_ptr_array_index(tray->entries, id);
    GVariantBuilder children;
    g_variant_builder_init(&children, G_VARIANT_TYPE("av"));
    if (depth != 0) {
        for (guint i = 0; i < entry->children->len; i++) {
            int child = g_array_index(entry->children, int, i);
            g_variant_builder_add(&children, "v", gkTrayLayout(tray, child, depth < 0 ? -1 : depth - 1));
        }
    }
    return g_variant_new("(i@a{sv}av)", id, entry->properties, &children);
}

static void gkTrayLayoutChanged(GKTray *tray) {
    tray->revision++;
    gkTrayRebuild(tray);
    if (tray->bus) {
        g_dbus_connection_emit_signal(tray->bus, NULL, tray->menu_path, "com.canonical.dbusmenu", "LayoutUpdated",
                                      g_variant_new("(ui)", tray->revision, 0), NULL);
    }
}

static void gkTrayMenuChanged(GMenuModel *model, int position, int removed, int added, gpointer data) {
    (void)model;
    (void)position;
    (void)removed;
    (void)added;
    gkTrayLayoutChanged(data);
}

static void gkTrayActionChanged(GActionGroup *group, const char *name, GVariant *value, gpointer data) {
    (void)group;
    (void)name;
    (void)value;
    gkTrayLayoutChanged(data);
}

static void gkTrayMenuCall(GDBusConnection *connection, const char *sender, const char *path, const char *interface,
                           const char *method, GVariant *parameters, GDBusMethodInvocation *invocation,
                           gpointer data) {
    (void)connection;
    (void)sender;
    (void)path;
    (void)interface;
    GKTray *tray = data;
    if (!tray->entries) {
        gkTrayRebuild(tray);
    }
    if (strcmp(method, "GetLayout") == 0) {
        int parent = 0, depth = -1;
        g_variant_get(parameters, "(ii@as)", &parent, &depth, NULL);
        if (parent < 0 || parent >= (int)tray->entries->len) {
            parent = 0;
        }
        g_dbus_method_invocation_return_value(invocation,
                                              g_variant_new("(u@(ia{sv}av))", tray->revision,
                                                            gkTrayLayout(tray, parent, depth)));
    } else if (strcmp(method, "GetGroupProperties") == 0) {
        GVariantIter *ids = NULL;
        g_variant_get(parameters, "(aias)", &ids, NULL);
        GVariantBuilder result;
        g_variant_builder_init(&result, G_VARIANT_TYPE("a(ia{sv})"));
        int id;
        while (g_variant_iter_next(ids, "i", &id)) {
            if (id >= 0 && id < (int)tray->entries->len) {
                GKMenuEntry *entry = g_ptr_array_index(tray->entries, id);
                g_variant_builder_add(&result, "(i@a{sv})", id, entry->properties);
            }
        }
        g_variant_iter_free(ids);
        g_dbus_method_invocation_return_value(invocation, g_variant_new("(a(ia{sv}))", &result));
    } else if (strcmp(method, "GetProperty") == 0) {
        int id = 0;
        const char *name = NULL;
        g_variant_get(parameters, "(i&s)", &id, &name);
        GVariant *value = NULL;
        if (id >= 0 && id < (int)tray->entries->len) {
            GKMenuEntry *entry = g_ptr_array_index(tray->entries, id);
            value = g_variant_lookup_value(entry->properties, name, NULL);
        }
        g_dbus_method_invocation_return_value(invocation,
                                              g_variant_new("(v)", value ? value : g_variant_new_string("")));
        if (value) {
            g_variant_unref(value);
        }
    } else if (strcmp(method, "Event") == 0 || strcmp(method, "EventGroup") == 0) {
        GVariantIter *events = NULL;
        GVariantBuilder errors;
        g_variant_builder_init(&errors, G_VARIANT_TYPE("ai"));
        if (strcmp(method, "Event") == 0) {
            int id = 0;
            const char *kind = NULL;
            g_variant_get(parameters, "(i&svu)", &id, &kind, NULL, NULL);
            if (strcmp(kind, "clicked") == 0 && id > 0 && id < (int)tray->entries->len) {
                GKMenuEntry *entry = g_ptr_array_index(tray->entries, id);
                if (entry->action && tray->actions) {
                    g_action_group_activate_action(tray->actions, entry->action, NULL);
                }
            }
            g_dbus_method_invocation_return_value(invocation, NULL);
            return;
        }
        g_variant_get(parameters, "(a(isvu))", &events);
        int id = 0;
        const char *kind = NULL;
        while (g_variant_iter_next(events, "(i&svu)", &id, &kind, NULL, NULL)) {
            if (id <= 0 || id >= (int)tray->entries->len) {
                g_variant_builder_add(&errors, "i", id);
                continue;
            }
            GKMenuEntry *entry = g_ptr_array_index(tray->entries, id);
            if (strcmp(kind, "clicked") == 0 && entry->action && tray->actions) {
                g_action_group_activate_action(tray->actions, entry->action, NULL);
            }
        }
        g_variant_iter_free(events);
        g_dbus_method_invocation_return_value(invocation, g_variant_new("(ai)", &errors));
    } else if (strcmp(method, "AboutToShow") == 0) {
        g_dbus_method_invocation_return_value(invocation, g_variant_new("(b)", FALSE));
    } else if (strcmp(method, "AboutToShowGroup") == 0) {
        g_dbus_method_invocation_return_value(invocation, g_variant_new("(@ai@ai)", g_variant_new("ai", NULL),
                                                                        g_variant_new("ai", NULL)));
    } else {
        g_dbus_method_invocation_return_error(invocation, G_DBUS_ERROR, G_DBUS_ERROR_UNKNOWN_METHOD,
                                              "unknown method %s", method);
    }
}

static GVariant *gkTrayMenuProperty(GDBusConnection *connection, const char *sender, const char *path,
                                    const char *interface, const char *name, GError **error, gpointer data) {
    (void)connection;
    (void)sender;
    (void)path;
    (void)interface;
    (void)error;
    (void)data;
    if (strcmp(name, "Version") == 0) {
        return g_variant_new_uint32(3);
    }
    if (strcmp(name, "TextDirection") == 0) {
        return g_variant_new_string(gtk_widget_get_default_direction() == GTK_TEXT_DIR_RTL ? "rtl" : "ltr");
    }
    if (strcmp(name, "Status") == 0) {
        return g_variant_new_string("normal");
    }
    return g_variant_new_strv(NULL, 0);
}

/* ---- Item ---- */

static GVariant *gkTrayPixmaps(GKTray *tray) {
    if (tray->pixmap) {
        return g_variant_ref(tray->pixmap);
    }
    return g_variant_new("a(iiay)", NULL);
}

static void gkTrayItemCall(GDBusConnection *connection, const char *sender, const char *path, const char *interface,
                           const char *method, GVariant *parameters, GDBusMethodInvocation *invocation,
                           gpointer data) {
    (void)connection;
    (void)sender;
    (void)path;
    (void)interface;
    (void)parameters;
    GKTray *tray = data;
    if (strcmp(method, "Activate") == 0 || strcmp(method, "SecondaryActivate") == 0) {
        gkSend(tray->handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, NULL);
    }
    g_dbus_method_invocation_return_value(invocation, NULL);
}

static GVariant *gkTrayItemProperty(GDBusConnection *connection, const char *sender, const char *path,
                                    const char *interface, const char *name, GError **error, gpointer data) {
    (void)connection;
    (void)sender;
    (void)path;
    (void)interface;
    (void)error;
    GKTray *tray = data;
    if (strcmp(name, "Category") == 0) {
        return g_variant_new_string("ApplicationStatus");
    }
    if (strcmp(name, "Id") == 0) {
        const char *prgname = g_get_prgname();
        return g_variant_new_string(prgname ? prgname : "gamekit");
    }
    if (strcmp(name, "Title") == 0) {
        return g_variant_new_string(tray->title ? tray->title : "");
    }
    if (strcmp(name, "Status") == 0) {
        return g_variant_new_string(!tray->visible ? "Passive" : tray->attention ? "NeedsAttention" : "Active");
    }
    if (strcmp(name, "WindowId") == 0) {
        return g_variant_new_int32(0);
    }
    if (strcmp(name, "IconName") == 0 || strcmp(name, "AttentionIconName") == 0) {
        return g_variant_new_string(tray->icon_name ? tray->icon_name : "");
    }
    if (strcmp(name, "IconPixmap") == 0) {
        return gkTrayPixmaps(tray);
    }
    if (strcmp(name, "ToolTip") == 0) {
        return g_variant_new("(s@a(iiay)ss)", tray->icon_name ? tray->icon_name : "", g_variant_new("a(iiay)", NULL),
                             tray->tooltip ? tray->tooltip : "", "");
    }
    if (strcmp(name, "ItemIsMenu") == 0) {
        return g_variant_new_boolean(FALSE);
    }
    if (strcmp(name, "Menu") == 0) {
        return g_variant_new_object_path(tray->menu_path);
    }
    return NULL;
}

static void gkTraySignal(GKTray *tray, const char *signal, GVariant *parameters) {
    if (tray->bus) {
        g_dbus_connection_emit_signal(tray->bus, NULL, tray->item_path, "org.kde.StatusNotifierItem", signal,
                                      parameters, NULL);
    }
}

static void gkTrayRegister(GDBusConnection *connection, const char *name, const char *owner, gpointer data) {
    (void)name;
    (void)owner;
    GKTray *tray = data;
    GVariant *reply = g_dbus_connection_call_sync(
        connection, "org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher", "org.kde.StatusNotifierWatcher",
        "RegisterStatusNotifierItem", g_variant_new("(s)", tray->item_path), NULL, G_DBUS_CALL_FLAGS_NONE, 2000, NULL,
        NULL);
    tray->registered = reply != NULL;
    if (reply) {
        g_variant_unref(reply);
    }
}

static void gkTrayLost(GDBusConnection *connection, const char *name, gpointer data) {
    (void)connection;
    (void)name;
    ((GKTray *)data)->registered = 0;
}

/* gkTrayConnection opens a private connection to the session bus. The
   StatusNotifierWatcher removes an item only when the bus name of its
   connection goes away, so each item has its own connection, and closes it
   when the item is removed. */
static GDBusConnection *gkTrayConnection(void) {
    char *address = g_dbus_address_get_for_bus_sync(G_BUS_TYPE_SESSION, NULL, NULL);
    if (!address) {
        return NULL;
    }
    GDBusConnection *bus = g_dbus_connection_new_for_address_sync(
        address, G_DBUS_CONNECTION_FLAGS_AUTHENTICATION_CLIENT | G_DBUS_CONNECTION_FLAGS_MESSAGE_BUS_CONNECTION, NULL,
        NULL, NULL);
    g_free(address);
    if (bus) {
        g_dbus_connection_set_exit_on_close(bus, FALSE);
    }
    return bus;
}

void *gkGTKTrayCreate(uintptr_t handle) {
    GDBusConnection *bus = gkTrayConnection();
    if (!bus) {
        return NULL;
    }
    static GDBusNodeInfo *item_info, *menu_info;
    if (!item_info) {
        item_info = g_dbus_node_info_new_for_xml(gkItemXML, NULL);
        menu_info = g_dbus_node_info_new_for_xml(gkMenuXML, NULL);
    }
    GKTray *tray = g_new0(GKTray, 1);
    tray->handle = handle;
    tray->bus = bus;
    tray->visible = 1;
    guint index = ++gkTrayCount;
    tray->item_path = g_strdup_printf("/StatusNotifierItem/gamekit%u", index);
    tray->menu_path = g_strdup_printf("/MenuBar/gamekit%u", index);
    static const GDBusInterfaceVTable item_table = {gkTrayItemCall, gkTrayItemProperty, NULL, {0}};
    static const GDBusInterfaceVTable menu_table = {gkTrayMenuCall, gkTrayMenuProperty, NULL, {0}};
    tray->item_registration = g_dbus_connection_register_object(bus, tray->item_path, item_info->interfaces[0],
                                                                &item_table, tray, NULL, NULL);
    tray->menu_registration = g_dbus_connection_register_object(bus, tray->menu_path, menu_info->interfaces[0],
                                                                &menu_table, tray, NULL, NULL);
    if (!tray->item_registration || !tray->menu_registration) {
        gkGTKTrayRemove(tray);
        return NULL;
    }
    gkTrayRebuild(tray);
    tray->watch = g_bus_watch_name_on_connection(bus, "org.kde.StatusNotifierWatcher", G_BUS_NAME_WATCHER_FLAGS_NONE,
                                                 gkTrayRegister, gkTrayLost, tray, NULL);
    return tray;
}

static GVariant *gkTrayPixmapFor(GdkTexture *texture) {
    int width = gdk_texture_get_width(texture), height = gdk_texture_get_height(texture);
    guchar *rgba = g_malloc((size_t)width * height * 4);
    GdkTextureDownloader *downloader = gdk_texture_downloader_new(texture);
    gdk_texture_downloader_set_format(downloader, GDK_MEMORY_R8G8B8A8);
    gdk_texture_downloader_download_into(downloader, rgba, (gsize)width * 4);
    gdk_texture_downloader_free(downloader);
    /* StatusNotifierItem pixmaps are ARGB32 in network byte order. */
    guchar *argb = g_malloc((size_t)width * height * 4);
    for (int i = 0; i < width * height; i++) {
        argb[i * 4] = rgba[i * 4 + 3];
        argb[i * 4 + 1] = rgba[i * 4];
        argb[i * 4 + 2] = rgba[i * 4 + 1];
        argb[i * 4 + 3] = rgba[i * 4 + 2];
    }
    g_free(rgba);
    GVariant *bytes =
        g_variant_new_from_data(G_VARIANT_TYPE("ay"), argb, (gsize)width * height * 4, TRUE, g_free, argb);
    GVariantBuilder pixmaps;
    g_variant_builder_init(&pixmaps, G_VARIANT_TYPE("a(iiay)"));
    g_variant_builder_add(&pixmaps, "(ii@ay)", width, height, bytes);
    return g_variant_ref_sink(g_variant_builder_end(&pixmaps));
}

void gkGTKTrayConfigure(void *pointer, void *icon, int is_template, const char *title, const char *tooltip) {
    (void)is_template;
    GKTray *tray = pointer;
    if (!tray) {
        return;
    }
    g_free(tray->title);
    g_free(tray->tooltip);
    g_free(tray->icon_name);
    tray->title = g_strdup(title ? title : "");
    tray->tooltip = g_strdup(tooltip ? tooltip : "");
    tray->icon_name = NULL;
    if (tray->pixmap) {
        g_variant_unref(tray->pixmap);
        tray->pixmap = NULL;
    }
    if (icon && GTK_IS_ICON_PAINTABLE(icon)) {
        tray->icon_name = g_strdup(gtk_icon_paintable_get_icon_name(GTK_ICON_PAINTABLE(icon)));
    } else if (icon && GDK_IS_TEXTURE(icon)) {
        tray->pixmap = gkTrayPixmapFor(GDK_TEXTURE(icon));
    }
    gkTraySignal(tray, "NewTitle", NULL);
    gkTraySignal(tray, "NewIcon", NULL);
    gkTraySignal(tray, "NewToolTip", NULL);
}

void gkGTKTraySetVisible(void *pointer, int visible) {
    GKTray *tray = pointer;
    if (tray) {
        tray->visible = visible != 0;
        gkTraySignal(tray, "NewStatus", g_variant_new("(s)", !tray->visible   ? "Passive"
                                                             : tray->attention ? "NeedsAttention"
                                                                               : "Active"));
    }
}

void gkGTKTraySetAttention(void *pointer, int attention) {
    GKTray *tray = pointer;
    if (tray) {
        tray->attention = attention != 0;
        gkGTKTraySetVisible(tray, tray->visible);
    }
}

static void gkTrayDropMenu(GKTray *tray) {
    if (tray->menu) {
        g_signal_handler_disconnect(tray->menu, tray->menu_changed);
        g_object_unref(tray->menu);
        tray->menu = NULL;
    }
    if (tray->actions) {
        g_signal_handler_disconnect(tray->actions, tray->actions_changed[0]);
        g_signal_handler_disconnect(tray->actions, tray->actions_changed[1]);
        g_object_unref(tray->actions);
        tray->actions = NULL;
    }
}

void gkGTKTraySetMenu(void *pointer, void *menu) {
    GKTray *tray = pointer;
    if (!tray) {
        return;
    }
    gkTrayDropMenu(tray);
    GMenuModel *model = gkMenuModel(menu);
    if (model) {
        tray->menu = g_object_ref(model);
        tray->menu_changed = g_signal_connect(model, "items-changed", G_CALLBACK(gkTrayMenuChanged), tray);
        GActionGroup *actions = gkMenuActions(menu);
        if (actions) {
            tray->actions = g_object_ref(actions);
            tray->actions_changed[0] =
                g_signal_connect(actions, "action-enabled-changed", G_CALLBACK(gkTrayActionChanged), tray);
            tray->actions_changed[1] =
                g_signal_connect(actions, "action-state-changed", G_CALLBACK(gkTrayActionChanged), tray);
        }
    }
    gkTrayLayoutChanged(tray);
}

int gkGTKTrayBounds(void *pointer, double *x, double *y, double *width, double *height) {
    (void)x;
    (void)y;
    (void)width;
    (void)height;
    (void)pointer;
    /* The StatusNotifierItem protocol does not tell an item where the host
       shows it. */
    return 0;
}

void gkGTKTrayRemove(void *pointer) {
    GKTray *tray = pointer;
    if (!tray) {
        return;
    }
    gkTrayDropMenu(tray);
    if (tray->watch) {
        g_bus_unwatch_name(tray->watch);
    }
    if (tray->item_registration) {
        g_dbus_connection_unregister_object(tray->bus, tray->item_registration);
    }
    if (tray->menu_registration) {
        g_dbus_connection_unregister_object(tray->bus, tray->menu_registration);
    }
    if (tray->entries) {
        g_ptr_array_free(tray->entries, TRUE);
    }
    if (tray->pixmap) {
        g_variant_unref(tray->pixmap);
    }
    /* The watcher removes the item when the name of the connection goes
       away. */
    g_dbus_connection_close_sync(tray->bus, NULL, NULL);
    g_object_unref(tray->bus);
    g_free(tray->item_path);
    g_free(tray->menu_path);
    g_free(tray->title);
    g_free(tray->tooltip);
    g_free(tray->icon_name);
    g_free(tray);
}
