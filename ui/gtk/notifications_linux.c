//go:build linux && cgo

#include "gtk_internal.h"

#include <stdio.h>

/* Notifications use the freedesktop notification service on the session
   bus. The Go layer waits for each result without running the GTK main
   loop, so the calls are synchronous and report before they return. */

extern void gkGTKGoNotificationResult(uintptr_t token, int status, char *ids, size_t size, char *message);

typedef struct {
    char *identifier;
    char *title;
    char *body;
    char *category;
    char *attachment;
    int sound;
    char **action_ids;
    char **action_titles;
    int *action_flags;
    int action_count;
    guint timer;
} GKPendingNotification;

static GDBusConnection *gkBus;
static uintptr_t gkNotificationHandle;
static GHashTable *gkDelivered;   /* identifier -> server ID */
static GHashTable *gkPendingByID; /* identifier -> GKPendingNotification */
static int gkCapabilityReply = -1;
static char *gkBadgeText;

static GDBusConnection *gkSessionBus(void) {
    if (!gkBus) {
        gkBus = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, NULL);
    }
    return gkBus;
}

static void gkPendingFree(gpointer data) {
    GKPendingNotification *pending = data;
    if (pending->timer) {
        g_source_remove(pending->timer);
    }
    g_free(pending->identifier);
    g_free(pending->title);
    g_free(pending->body);
    g_free(pending->category);
    g_free(pending->attachment);
    g_strfreev(pending->action_ids);
    g_strfreev(pending->action_titles);
    g_free(pending->action_flags);
    g_free(pending);
}

static void gkNotificationTables(void) {
    if (!gkDelivered) {
        gkDelivered = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, NULL);
        gkPendingByID = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, gkPendingFree);
    }
}

int gkGTKNotificationsAvailable(void) {
    GDBusConnection *bus = gkSessionBus();
    if (!bus) {
        return 0;
    }
    GVariant *reply = g_dbus_connection_call_sync(bus, "org.freedesktop.DBus", "/org/freedesktop/DBus",
                                                  "org.freedesktop.DBus", "NameHasOwner",
                                                  g_variant_new("(s)", "org.freedesktop.Notifications"),
                                                  G_VARIANT_TYPE("(b)"), G_DBUS_CALL_FLAGS_NONE, 2000, NULL, NULL);
    gboolean owned = FALSE;
    if (reply) {
        g_variant_get(reply, "(b)", &owned);
        g_variant_unref(reply);
    }
    return owned;
}

static int gkSupportsReply(void) {
    if (gkCapabilityReply >= 0) {
        return gkCapabilityReply;
    }
    gkCapabilityReply = 0;
    GVariant *reply = g_dbus_connection_call_sync(
        gkSessionBus(), "org.freedesktop.Notifications", "/org/freedesktop/Notifications",
        "org.freedesktop.Notifications", "GetCapabilities", NULL, G_VARIANT_TYPE("(as)"), G_DBUS_CALL_FLAGS_NONE,
        2000, NULL, NULL);
    if (reply) {
        char **capabilities = NULL;
        g_variant_get(reply, "(^as)", &capabilities);
        gkCapabilityReply = capabilities && g_strv_contains((const char *const *)capabilities, "inline-reply");
        g_strfreev(capabilities);
        g_variant_unref(reply);
    }
    return gkCapabilityReply;
}

static const char *gkIdentifierFor(guint32 server) {
    GHashTableIter iter;
    gpointer key, value;
    g_hash_table_iter_init(&iter, gkDelivered);
    while (g_hash_table_iter_next(&iter, &key, &value)) {
        if (GPOINTER_TO_UINT(value) == server) {
            return key;
        }
    }
    return NULL;
}

static void gkNotificationSignal(GDBusConnection *connection, const char *sender, const char *path,
                                 const char *interface, const char *signal, GVariant *parameters, gpointer data) {
    (void)connection;
    (void)sender;
    (void)path;
    (void)interface;
    (void)data;
    gkNotificationTables();
    if (strcmp(signal, "NotificationClosed") == 0) {
        guint32 server = 0, reason = 0;
        g_variant_get(parameters, "(uu)", &server, &reason);
        const char *identifier = gkIdentifierFor(server);
        if (identifier) {
            g_hash_table_remove(gkDelivered, identifier);
        }
        return;
    }
    guint32 server = 0;
    const char *value = NULL;
    g_variant_get(parameters, "(u&s)", &server, &value);
    const char *identifier = gkIdentifierFor(server);
    if (!identifier || !gkNotificationHandle) {
        return;
    }
    GKGTKEvent event = {0};
    event.kind = GK_EVENT_NOTIFICATION;
    event.text = identifier;
    if (strcmp(signal, "NotificationReplied") == 0) {
        event.text2 = g_object_get_data(G_OBJECT(gkSessionBus()), "gk-reply-action");
        if (!event.text2) {
            event.text2 = "";
        }
        event.object = (void *)value;
    } else {
        event.text2 = strcmp(value, "default") == 0 ? "" : value;
        event.object = (void *)"";
    }
    gkGTKGoEvent(gkNotificationHandle, &event);
}

void gkGTKNotificationsAttach(uintptr_t handle) {
    gkNotificationHandle = handle;
    gkNotificationTables();
    GDBusConnection *bus = gkSessionBus();
    if (!bus) {
        return;
    }
    const char *signals[] = {"ActionInvoked", "NotificationClosed", "NotificationReplied"};
    for (size_t i = 0; i < G_N_ELEMENTS(signals); i++) {
        g_dbus_connection_signal_subscribe(bus, "org.freedesktop.Notifications", "org.freedesktop.Notifications",
                                           signals[i], "/org/freedesktop/Notifications", NULL,
                                           G_DBUS_SIGNAL_FLAGS_NONE, gkNotificationSignal, NULL, NULL);
    }
}

void gkGTKNotificationAuthorization(uintptr_t token) {
    /* The notification service has no permission model: an available
       service delivers notifications. */
    gkGTKGoNotificationResult(token, gkGTKNotificationsAvailable() ? 2 : 1, NULL, 0, NULL);
}

void gkGTKNotificationRequestAuthorization(uintptr_t token) {
    gkGTKNotificationAuthorization(token);
}

static int gkDeliver(GKPendingNotification *notification, char **failure) {
    GDBusConnection *bus = gkSessionBus();
    if (!bus) {
        *failure = strdup("there is no session bus");
        return 0;
    }
    GVariantBuilder actions;
    g_variant_builder_init(&actions, G_VARIANT_TYPE("as"));
    g_variant_builder_add(&actions, "s", "default");
    g_variant_builder_add(&actions, "s", "");
    const char *reply_action = NULL;
    for (int i = 0; i < notification->action_count; i++) {
        if ((notification->action_flags[i] & 2) && gkSupportsReply() && !reply_action) {
            reply_action = notification->action_ids[i];
            g_variant_builder_add(&actions, "s", "inline-reply");
            g_variant_builder_add(&actions, "s", notification->action_titles[i]);
            continue;
        }
        g_variant_builder_add(&actions, "s", notification->action_ids[i]);
        g_variant_builder_add(&actions, "s", notification->action_titles[i]);
    }
    if (reply_action) {
        g_object_set_data_full(G_OBJECT(bus), "gk-reply-action", g_strdup(reply_action), g_free);
    }
    GVariantBuilder hints;
    g_variant_builder_init(&hints, G_VARIANT_TYPE("a{sv}"));
    if (notification->category && *notification->category) {
        g_variant_builder_add(&hints, "{sv}", "category", g_variant_new_string(notification->category));
    }
    if (notification->sound) {
        g_variant_builder_add(&hints, "{sv}", "sound-name", g_variant_new_string("message-new-instant"));
    } else {
        g_variant_builder_add(&hints, "{sv}", "suppress-sound", g_variant_new_boolean(TRUE));
    }
    if (notification->attachment && *notification->attachment) {
        g_variant_builder_add(&hints, "{sv}", "image-path", g_variant_new_string(notification->attachment));
    }
    const char *prgname = g_get_prgname();
    if (prgname) {
        g_variant_builder_add(&hints, "{sv}", "desktop-entry", g_variant_new_string(prgname));
    }
    guint32 replaces = GPOINTER_TO_UINT(g_hash_table_lookup(gkDelivered, notification->identifier));
    const char *application = g_get_application_name();
    GError *error = NULL;
    GVariant *reply = g_dbus_connection_call_sync(
        bus, "org.freedesktop.Notifications", "/org/freedesktop/Notifications", "org.freedesktop.Notifications",
        "Notify",
        g_variant_new("(susssasa{sv}i)", application ? application : "GameKit", replaces, "", notification->title,
                      notification->body, &actions, &hints, -1),
        G_VARIANT_TYPE("(u)"), G_DBUS_CALL_FLAGS_NONE, 5000, NULL, &error);
    if (!reply) {
        *failure = strdup(error ? error->message : "the notification service did not answer");
        g_clear_error(&error);
        return 0;
    }
    guint32 server = 0;
    g_variant_get(reply, "(u)", &server);
    g_variant_unref(reply);
    g_hash_table_insert(gkDelivered, g_strdup(notification->identifier), GUINT_TO_POINTER(server));
    return 1;
}

static gboolean gkDeliverLater(gpointer data) {
    GKPendingNotification *notification = data;
    notification->timer = 0;
    char *failure = NULL;
    gkDeliver(notification, &failure);
    free(failure);
    g_hash_table_remove(gkPendingByID, notification->identifier);
    return G_SOURCE_REMOVE;
}

void gkGTKNotificationSend(uintptr_t token, const char *identifier, const char *title, const char *subtitle,
                           const char *body, const char *category, const char *thread, int sound,
                           const char *attachment, double delay, const char *action_ids, size_t action_ids_size,
                           const char *action_titles, size_t action_titles_size, const int *action_flags,
                           int action_count) {
    (void)thread;
    gkNotificationTables();
    GKPendingNotification *notification = g_new0(GKPendingNotification, 1);
    notification->identifier = g_strdup(identifier ? identifier : "");
    notification->title = g_strdup(title ? title : "");
    if (subtitle && *subtitle) {
        notification->body = g_strdup_printf("%s\n%s", subtitle, body ? body : "");
    } else {
        notification->body = g_strdup(body ? body : "");
    }
    notification->category = g_strdup(category ? category : "");
    notification->attachment = g_strdup(attachment ? attachment : "");
    notification->sound = sound != 0;
    notification->action_ids = gkSplit(action_ids, action_ids_size, NULL);
    notification->action_titles = gkSplit(action_titles, action_titles_size, NULL);
    notification->action_count = MIN(action_count, (int)MIN(g_strv_length(notification->action_ids),
                                                            g_strv_length(notification->action_titles)));
    notification->action_flags = g_new0(int, MAX(1, action_count));
    for (int i = 0; i < action_count; i++) {
        notification->action_flags[i] = action_flags[i];
    }
    g_hash_table_remove(gkPendingByID, notification->identifier);
    if (delay > 0) {
        notification->timer = g_timeout_add((guint)(delay * 1000), gkDeliverLater, notification);
        g_hash_table_insert(gkPendingByID, g_strdup(notification->identifier), notification);
        gkGTKGoNotificationResult(token, 0, NULL, 0, NULL);
        return;
    }
    char *failure = NULL;
    int delivered = gkDeliver(notification, &failure);
    gkPendingFree(notification);
    gkGTKGoNotificationResult(token, delivered ? 0 : -1, NULL, 0, failure);
}

static char *gkKeys(GHashTable *table, size_t *size) {
    GPtrArray *ids = g_ptr_array_new();
    GHashTableIter iter;
    gpointer key, value;
    g_hash_table_iter_init(&iter, table);
    while (g_hash_table_iter_next(&iter, &key, &value)) {
        g_ptr_array_add(ids, key);
    }
    char *result = ids->len ? gkJoin((char **)ids->pdata, (int)ids->len, size) : NULL;
    if (!ids->len) {
        *size = 0;
    }
    g_ptr_array_free(ids, TRUE);
    return result;
}

void gkGTKNotificationPending(uintptr_t token) {
    gkNotificationTables();
    size_t size = 0;
    char *ids = gkKeys(gkPendingByID, &size);
    gkGTKGoNotificationResult(token, 0, ids, size, NULL);
}

void gkGTKNotificationDelivered(uintptr_t token) {
    gkNotificationTables();
    size_t size = 0;
    char *ids = gkKeys(gkDelivered, &size);
    gkGTKGoNotificationResult(token, 0, ids, size, NULL);
}

void gkGTKNotificationWithdraw(const char *ids, size_t size) {
    gkNotificationTables();
    int count = 0;
    char **identifiers = gkSplit(ids, size, &count);
    for (int i = 0; i < count; i++) {
        g_hash_table_remove(gkPendingByID, identifiers[i]);
        gpointer server = g_hash_table_lookup(gkDelivered, identifiers[i]);
        if (server && gkSessionBus()) {
            GVariant *reply = g_dbus_connection_call_sync(
                gkSessionBus(), "org.freedesktop.Notifications", "/org/freedesktop/Notifications",
                "org.freedesktop.Notifications", "CloseNotification",
                g_variant_new("(u)", GPOINTER_TO_UINT(server)), NULL, G_DBUS_CALL_FLAGS_NONE, 2000, NULL, NULL);
            if (reply) {
                g_variant_unref(reply);
            }
        }
        g_hash_table_remove(gkDelivered, identifiers[i]);
    }
    g_strfreev(identifiers);
}

/* gkGTKSetBadge sets the count of the launcher entry of the application
   with the Unity launcher API, which docks such as Dash to Dock show. */
void gkGTKSetBadge(const char *badge) {
    g_free(gkBadgeText);
    gkBadgeText = g_strdup(badge ? badge : "");
    GDBusConnection *bus = gkSessionBus();
    if (!bus) {
        return;
    }
    char *end = NULL;
    gint64 count = g_ascii_strtoll(gkBadgeText, &end, 10);
    gboolean numeric = *gkBadgeText && end && *end == '\0';
    const char *prgname = g_get_prgname();
    char *uri = g_strdup_printf("application://%s.desktop", prgname ? prgname : "gamekit");
    GVariantBuilder properties;
    g_variant_builder_init(&properties, G_VARIANT_TYPE("a{sv}"));
    g_variant_builder_add(&properties, "{sv}", "count", g_variant_new_int64(numeric ? count : 0));
    g_variant_builder_add(&properties, "{sv}", "count-visible", g_variant_new_boolean(*gkBadgeText != '\0'));
    g_dbus_connection_emit_signal(bus, NULL, "/com/canonical/unity/launcherentry/gamekit",
                                  "com.canonical.Unity.LauncherEntry", "Update",
                                  g_variant_new("(sa{sv})", uri, &properties), NULL);
    g_dbus_connection_flush_sync(bus, NULL, NULL);
    g_free(uri);
}

char *gkGTKBadge(void) {
    return gkStrdup(gkBadgeText);
}
