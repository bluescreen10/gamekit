//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>
#include <stdio.h>

extern int gkGTKGoPrintPageCount(uintptr_t handle);
extern int gkGTKGoPrintPage(uintptr_t handle, int page, double width, double height);
extern int gkFontFamilyInstalled(const char *family);

/* ---- Message dialogs ----

   An alert is a modal window with an icon, a title, a message, optional
   details and text entry, and one button for each choice. */

typedef struct {
    GtkWidget *window;
    GtkWidget *entry;
    int cancel;
    int response;
    int done;
} GKAlert;

static GQuark gkAlertQuark(void) {
    static GQuark quark;
    if (!quark) {
        quark = g_quark_from_static_string("gk-alert");
    }
    return quark;
}

static void gkAlertFree(gpointer data) {
    GKAlert *alert = data;
    if (alert->window) {
        g_signal_handlers_disconnect_by_data(alert->window, alert);
        gtk_window_destroy(GTK_WINDOW(alert->window));
        g_object_unref(alert->window);
    }
    g_free(alert);
}

static GKAlert *gkAlert(void *pointer) {
    return pointer ? g_object_get_qdata(G_OBJECT(pointer), gkAlertQuark()) : NULL;
}

static void gkAlertChoose(GtkButton *button, gpointer data) {
    GKAlert *alert = data;
    alert->response = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(button), "gk-index"));
    alert->done = 1;
}

static gboolean gkAlertCloseRequest(GtkWindow *window, gpointer data) {
    (void)window;
    GKAlert *alert = data;
    alert->response = alert->cancel >= 0 ? alert->cancel : 0;
    alert->done = 1;
    return TRUE;
}

static void gkAlertActivate(GtkWidget *entry, gpointer data) {
    (void)entry;
    GKAlert *alert = data;
    alert->response = 0;
    alert->done = 1;
}

static const char *gkAlertIcon(int severity) {
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

void *gkGTKAlertCreate(int severity, const char *title, const char *message, const char *details,
                       const char *buttons, size_t buttons_size, int cancel_index, int destructive_mask, int prompt,
                       int secure, const char *text, const char *placeholder) {
    GKAlert *alert = g_new0(GKAlert, 1);
    alert->cancel = cancel_index;
    alert->response = -1;
    alert->window = g_object_new(GTK_TYPE_WINDOW, "accessible-role", GTK_ACCESSIBLE_ROLE_ALERT_DIALOG, NULL);
    g_object_ref_sink(alert->window);
    gtk_window_set_modal(GTK_WINDOW(alert->window), TRUE);
    gtk_window_set_resizable(GTK_WINDOW(alert->window), FALSE);
    gtk_window_set_title(GTK_WINDOW(alert->window), title ? title : "");
    g_signal_connect(alert->window, "close-request", G_CALLBACK(gkAlertCloseRequest), alert);
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 10);
    gtk_widget_set_margin_start(box, 24);
    gtk_widget_set_margin_end(box, 24);
    gtk_widget_set_margin_top(box, 20);
    gtk_widget_set_margin_bottom(box, 16);
    gtk_widget_set_size_request(box, 320, -1);
    GtkWidget *icon = gtk_image_new_from_icon_name(gkAlertIcon(severity));
    gtk_image_set_pixel_size(GTK_IMAGE(icon), 48);
    gtk_box_append(GTK_BOX(box), icon);
    GtkWidget *heading = gtk_label_new(title ? title : "");
    gtk_widget_add_css_class(heading, "title-3");
    gtk_label_set_wrap(GTK_LABEL(heading), TRUE);
    gtk_label_set_max_width_chars(GTK_LABEL(heading), 40);
    gtk_box_append(GTK_BOX(box), heading);
    if (message && *message) {
        GtkWidget *body = gtk_label_new(message);
        gtk_label_set_wrap(GTK_LABEL(body), TRUE);
        gtk_label_set_max_width_chars(GTK_LABEL(body), 50);
        gtk_label_set_justify(GTK_LABEL(body), GTK_JUSTIFY_CENTER);
        gtk_box_append(GTK_BOX(box), body);
        gtk_accessible_update_property(GTK_ACCESSIBLE(alert->window), GTK_ACCESSIBLE_PROPERTY_DESCRIPTION, message,
                                       -1);
    }
    if (details && *details) {
        GtkWidget *expander = gtk_expander_new("Details");
        GtkWidget *detail = gtk_label_new(details);
        gtk_label_set_wrap(GTK_LABEL(detail), TRUE);
        gtk_label_set_selectable(GTK_LABEL(detail), TRUE);
        gtk_label_set_xalign(GTK_LABEL(detail), 0);
        gtk_widget_add_css_class(detail, "dim-label");
        gtk_expander_set_child(GTK_EXPANDER(expander), detail);
        gtk_box_append(GTK_BOX(box), expander);
    }
    if (prompt) {
        alert->entry = secure ? gtk_password_entry_new() : gtk_entry_new();
        gtk_editable_set_text(GTK_EDITABLE(alert->entry), text ? text : "");
        g_object_set(alert->entry, "placeholder-text", placeholder ? placeholder : "", NULL);
        g_signal_connect(alert->entry, "activate", G_CALLBACK(gkAlertActivate), alert);
        gtk_box_append(GTK_BOX(box), alert->entry);
    }
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_halign(row, GTK_ALIGN_END);
    gtk_widget_set_margin_top(row, 6);
    int count = 0;
    char **titles = gkSplit(buttons, buttons_size, &count);
    GtkWidget *first = NULL;
    for (int i = count - 1; i >= 0; i--) {
        GtkWidget *button = gtk_button_new_with_label(titles[i]);
        g_object_set_data(G_OBJECT(button), "gk-index", GINT_TO_POINTER(i));
        if (destructive_mask & (1 << i)) {
            gtk_widget_add_css_class(button, "destructive-action");
        } else if (i == 0) {
            gtk_widget_add_css_class(button, "suggested-action");
        }
        g_signal_connect(button, "clicked", G_CALLBACK(gkAlertChoose), alert);
        gtk_box_append(GTK_BOX(row), button);
        if (i == 0) {
            first = button;
        }
    }
    g_strfreev(titles);
    gtk_box_append(GTK_BOX(box), row);
    gtk_window_set_child(GTK_WINDOW(alert->window), box);
    if (first) {
        gtk_window_set_default_widget(GTK_WINDOW(alert->window), first);
    }
    if (cancel_index >= 0) {
        GtkEventController *shortcuts = gtk_shortcut_controller_new();
        gtk_shortcut_controller_set_scope(GTK_SHORTCUT_CONTROLLER(shortcuts), GTK_SHORTCUT_SCOPE_GLOBAL);
        gtk_shortcut_controller_add_shortcut(GTK_SHORTCUT_CONTROLLER(shortcuts),
                                             gtk_shortcut_new(gtk_keyval_trigger_new(GDK_KEY_Escape, 0),
                                                              gtk_named_action_new("window.close")));
        gtk_widget_add_controller(alert->window, shortcuts);
    }
    GObject *object = g_object_new(G_TYPE_OBJECT, NULL);
    g_object_set_qdata_full(object, gkAlertQuark(), alert, gkAlertFree);
    return object;
}

void *gkGTKModalBegin(void *pointer, void *parent) {
    GKAlert *alert = gkAlert(pointer);
    GKWin *win = parent;
    if (!alert) {
        return NULL;
    }
    if (win && gtk_widget_get_visible(win->window)) {
        gtk_window_set_transient_for(GTK_WINDOW(alert->window), GTK_WINDOW(win->window));
    }
    alert->done = 0;
    alert->response = -1;
    gtk_window_present(GTK_WINDOW(alert->window));
    if (alert->entry) {
        gtk_widget_grab_focus(alert->entry);
    }
    return g_object_ref(pointer);
}

int gkGTKModalStep(void *modal) {
    GKAlert *alert = gkAlert(modal);
    if (!alert) {
        return 0;
    }
    gkRunPending();
    return alert->done ? alert->response : -1;
}

void gkGTKModalEnd(void *modal, int abort) {
    (void)abort;
    GKAlert *alert = gkAlert(modal);
    if (alert) {
        gtk_widget_set_visible(alert->window, FALSE);
    }
    if (modal) {
        g_object_unref(modal);
    }
}

char *gkGTKAlertText(void *pointer) {
    GKAlert *alert = gkAlert(pointer);
    return gkStrdup(alert && alert->entry ? gtk_editable_get_text(GTK_EDITABLE(alert->entry)) : "");
}

void gkGTKShowAbout(const char *name, const char *version, const char *copyright, const char *website,
                    const char *credits, void *icon) {
    GtkWidget *about = gtk_about_dialog_new();
    GtkAboutDialog *dialog = GTK_ABOUT_DIALOG(about);
    gtk_about_dialog_set_program_name(dialog, name && *name ? name : g_get_application_name());
    if (version && *version) {
        gtk_about_dialog_set_version(dialog, version);
    }
    if (copyright && *copyright) {
        gtk_about_dialog_set_copyright(dialog, copyright);
    }
    if (website && *website) {
        gtk_about_dialog_set_website(dialog, website);
    }
    if (credits && *credits) {
        gtk_about_dialog_set_comments(dialog, credits);
    }
    if (icon) {
        gtk_about_dialog_set_logo(dialog, GDK_PAINTABLE(icon));
    }
    gtk_window_set_hide_on_close(GTK_WINDOW(about), FALSE);
    gtk_window_present(GTK_WINDOW(about));
}

/* ---- Clipboard ----

   GdkClipboard reads asynchronously, so a read runs the main context until
   every format has arrived. */

static long gkClipboardCount;
static int gkClipboardWatched;

static void gkClipboardChanged(GdkClipboard *clipboard, gpointer data) {
    (void)clipboard;
    (void)data;
    gkClipboardCount++;
}

static GdkClipboard *gkClipboard(void) {
    GdkDisplay *display = gdk_display_get_default();
    if (!display) {
        return NULL;
    }
    GdkClipboard *clipboard = gdk_display_get_clipboard(display);
    if (!gkClipboardWatched) {
        gkClipboardWatched = 1;
        g_signal_connect(clipboard, "changed", G_CALLBACK(gkClipboardChanged), NULL);
    }
    return clipboard;
}

/* gkGTKClipboardChangeCount returns the number of clipboard changes so far.
   It does not run the main context: the window event processing that runs
   before each poll delivers the "changed" signals, and running the main
   context while the application builds widgets can leave them undrawn. */
long gkGTKClipboardChangeCount(void) {
    gkClipboard();
    return gkClipboardCount;
}

void gkGTKClipboardClear(void) {
    GdkClipboard *clipboard = gkClipboard();
    if (clipboard) {
        gdk_clipboard_set_content(clipboard, NULL);
    }
}

int gkGTKClipboardWrite(const char *text, int has_text, const char *files, size_t files_size,
                        const unsigned char *pixels, int width, int height, const char *types, size_t types_size,
                        const unsigned char *data, const size_t *sizes, int count) {
    GdkClipboard *clipboard = gkClipboard();
    if (!clipboard) {
        return 0;
    }
    GPtrArray *providers = g_ptr_array_new();
    if (has_text) {
        g_ptr_array_add(providers, gdk_content_provider_new_typed(G_TYPE_STRING, text ? text : ""));
    }
    int file_count = 0;
    char **paths = gkSplit(files, files_size, &file_count);
    if (file_count > 0) {
        GSList *list = NULL;
        for (int i = file_count - 1; i >= 0; i--) {
            list = g_slist_prepend(list, g_file_new_for_path(paths[i]));
        }
        GdkFileList *file_list = gdk_file_list_new_from_list(list);
        g_ptr_array_add(providers, gdk_content_provider_new_typed(GDK_TYPE_FILE_LIST, file_list));
        g_boxed_free(GDK_TYPE_FILE_LIST, file_list);
        g_slist_free_full(list, g_object_unref);
    }
    g_strfreev(paths);
    GdkTexture *texture = gkTextureFromRGBA(pixels, width, height, width * 4);
    if (texture) {
        g_ptr_array_add(providers, gdk_content_provider_new_typed(GDK_TYPE_TEXTURE, texture));
        g_object_unref(texture);
    }
    int type_count = 0;
    char **mimes = gkSplit(types, types_size, &type_count);
    size_t offset = 0;
    for (int i = 0; i < count && i < type_count; i++) {
        GBytes *bytes = g_bytes_new(data + offset, sizes[i]);
        g_ptr_array_add(providers, gdk_content_provider_new_for_bytes(mimes[i], bytes));
        g_bytes_unref(bytes);
        offset += sizes[i];
    }
    g_strfreev(mimes);
    GdkContentProvider *content =
        providers->len ? gdk_content_provider_new_union((GdkContentProvider **)providers->pdata, providers->len)
                       : NULL;
    g_ptr_array_free(providers, FALSE);
    gboolean set = gdk_clipboard_set_content(clipboard, content);
    if (content) {
        g_object_unref(content);
    }
    gkRunPending();
    return set;
}

typedef struct {
    int pending;
    char *text;
    int has_text;
    GString *files;
    GdkTexture *texture;
    GString *types;
    GByteArray *data;
    GArray *sizes;
} GKClipboardRead;

static void gkClipboardTextDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKClipboardRead *read = data;
    read->text = gdk_clipboard_read_text_finish(GDK_CLIPBOARD(source), result, NULL);
    read->has_text = read->text != NULL;
    read->pending--;
}

static void gkClipboardFilesDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKClipboardRead *read = data;
    const GValue *value = gdk_clipboard_read_value_finish(GDK_CLIPBOARD(source), result, NULL);
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
    read->pending--;
}

static void gkClipboardTextureDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKClipboardRead *read = data;
    read->texture = gdk_clipboard_read_texture_finish(GDK_CLIPBOARD(source), result, NULL);
    read->pending--;
}

typedef struct {
    GKClipboardRead *read;
    char *mime;
    GInputStream *input;
    GByteArray *bytes;
} GKClipboardStream;

/* gkClipboardStreamFinish stores the bytes of a custom format. The stream is
   read asynchronously, because GDK writes the data of the application's own
   clipboard content from the same main loop. */
static void gkClipboardStreamFinish(GKClipboardStream *stream) {
    GKClipboardRead *read = stream->read;
    if (stream->bytes) {
        g_string_append_len(read->types, stream->mime, (gssize)strlen(stream->mime) + 1);
        g_byte_array_append(read->data, stream->bytes->data, stream->bytes->len);
        size_t length = stream->bytes->len;
        g_array_append_val(read->sizes, length);
        g_byte_array_free(stream->bytes, TRUE);
    }
    if (stream->input) {
        g_object_unref(stream->input);
    }
    g_free(stream->mime);
    g_free(stream);
    read->pending--;
}

static void gkClipboardChunk(GObject *source, GAsyncResult *result, gpointer data) {
    GKClipboardStream *stream = data;
    GBytes *chunk = g_input_stream_read_bytes_finish(G_INPUT_STREAM(source), result, NULL);
    gsize size = chunk ? g_bytes_get_size(chunk) : 0;
    if (size > 0) {
        g_byte_array_append(stream->bytes, g_bytes_get_data(chunk, NULL), (guint)size);
        g_bytes_unref(chunk);
        g_input_stream_read_bytes_async(stream->input, 65536, G_PRIORITY_DEFAULT, NULL, gkClipboardChunk, stream);
        return;
    }
    if (chunk) {
        g_bytes_unref(chunk);
    }
    gkClipboardStreamFinish(stream);
}

static void gkClipboardStreamDone(GObject *source, GAsyncResult *result, gpointer data) {
    GKClipboardStream *stream = data;
    const char *mime = NULL;
    stream->input = gdk_clipboard_read_finish(GDK_CLIPBOARD(source), result, &mime, NULL);
    if (!stream->input) {
        gkClipboardStreamFinish(stream);
        return;
    }
    stream->bytes = g_byte_array_new();
    g_input_stream_read_bytes_async(stream->input, 65536, G_PRIORITY_DEFAULT, NULL, gkClipboardChunk, stream);
}

static int gkStandardMime(const char *mime) {
    return g_str_has_prefix(mime, "text/plain") || strcmp(mime, "UTF8_STRING") == 0 || strcmp(mime, "TEXT") == 0 ||
           strcmp(mime, "STRING") == 0 || strcmp(mime, "COMPOUND_TEXT") == 0 || strcmp(mime, "text/uri-list") == 0 ||
           g_str_has_prefix(mime, "image/") || strcmp(mime, "application/vnd.portal.filetransfer") == 0 ||
           strcmp(mime, "application/vnd.portal.files") == 0 || strcmp(mime, "x-special/gnome-copied-files") == 0;
}

void gkGTKClipboardRead(char **text, int *has_text, char **files, size_t *files_size, unsigned char **pixels,
                        int *width, int *height, char **types, size_t *types_size, unsigned char **data,
                        size_t **sizes, int *count) {
    *text = NULL;
    *has_text = 0;
    *files = NULL;
    *files_size = 0;
    *pixels = NULL;
    *width = *height = 0;
    *types = NULL;
    *types_size = 0;
    *data = NULL;
    *sizes = NULL;
    *count = 0;
    GdkClipboard *clipboard = gkClipboard();
    if (!clipboard) {
        return;
    }
    GKClipboardRead read = {0};
    read.files = g_string_new("");
    read.types = g_string_new("");
    read.data = g_byte_array_new();
    read.sizes = g_array_new(FALSE, FALSE, sizeof(size_t));
    GdkContentFormats *formats = gdk_clipboard_get_formats(clipboard);
    if (gdk_content_formats_contain_gtype(formats, G_TYPE_STRING) ||
        gdk_content_formats_contain_mime_type(formats, "text/plain;charset=utf-8") ||
        gdk_content_formats_contain_mime_type(formats, "text/plain")) {
        read.pending++;
        gdk_clipboard_read_text_async(clipboard, NULL, gkClipboardTextDone, &read);
    }
    if (gdk_content_formats_contain_gtype(formats, GDK_TYPE_FILE_LIST) ||
        gdk_content_formats_contain_mime_type(formats, "text/uri-list")) {
        read.pending++;
        gdk_clipboard_read_value_async(clipboard, GDK_TYPE_FILE_LIST, G_PRIORITY_DEFAULT, NULL, gkClipboardFilesDone,
                                       &read);
    }
    if (gdk_content_formats_contain_gtype(formats, GDK_TYPE_TEXTURE) ||
        gdk_content_formats_contain_mime_type(formats, "image/png")) {
        read.pending++;
        gdk_clipboard_read_texture_async(clipboard, NULL, gkClipboardTextureDone, &read);
    }
    gsize mime_count = 0;
    const char *const *mimes = gdk_content_formats_get_mime_types(formats, &mime_count);
    for (gsize i = 0; i < mime_count; i++) {
        if (gkStandardMime(mimes[i])) {
            continue;
        }
        GKClipboardStream *stream = g_new0(GKClipboardStream, 1);
        stream->read = &read;
        stream->mime = g_strdup(mimes[i]);
        const char *wanted[] = {mimes[i], NULL};
        read.pending++;
        gdk_clipboard_read_async(clipboard, wanted, G_PRIORITY_DEFAULT, NULL, gkClipboardStreamDone, stream);
    }
    gint64 deadline = g_get_monotonic_time() + 2 * G_USEC_PER_SEC;
    while (read.pending > 0 && g_get_monotonic_time() < deadline) {
        if (!g_main_context_iteration(NULL, FALSE)) {
            g_usleep(1000);
        }
    }
    if (read.has_text) {
        *text = strdup(read.text);
        *has_text = 1;
    }
    g_free(read.text);
    if (read.files->len) {
        *files = malloc(read.files->len);
        memcpy(*files, read.files->str, read.files->len);
        *files_size = read.files->len;
    }
    if (read.texture) {
        int w = gdk_texture_get_width(read.texture), h = gdk_texture_get_height(read.texture);
        *pixels = malloc((size_t)w * h * 4);
        GdkTextureDownloader *downloader = gdk_texture_downloader_new(read.texture);
        gdk_texture_downloader_set_format(downloader, GDK_MEMORY_R8G8B8A8);
        gdk_texture_downloader_download_into(downloader, *pixels, (gsize)w * 4);
        gdk_texture_downloader_free(downloader);
        *width = w;
        *height = h;
        g_object_unref(read.texture);
    }
    if (read.sizes->len) {
        *types = malloc(read.types->len);
        memcpy(*types, read.types->str, read.types->len);
        *types_size = read.types->len;
        *data = malloc(MAX(1, read.data->len));
        memcpy(*data, read.data->data, read.data->len);
        *sizes = malloc(read.sizes->len * sizeof(size_t));
        memcpy(*sizes, read.sizes->data, read.sizes->len * sizeof(size_t));
        *count = (int)read.sizes->len;
    }
    g_string_free(read.files, TRUE);
    g_string_free(read.types, TRUE);
    g_byte_array_free(read.data, TRUE);
    g_array_free(read.sizes, TRUE);
}

char *gkGTKClipboardFormats(size_t *size) {
    *size = 0;
    GdkClipboard *clipboard = gkClipboard();
    if (!clipboard) {
        return NULL;
    }
    GdkContentFormats *formats = gdk_clipboard_get_formats(clipboard);
    GPtrArray *names = g_ptr_array_new_with_free_func(g_free);
    gsize count = 0;
    const char *const *mimes = gdk_content_formats_get_mime_types(formats, &count);
    for (gsize i = 0; i < count; i++) {
        g_ptr_array_add(names, g_strdup(mimes[i]));
    }
    if (gdk_content_formats_contain_gtype(formats, G_TYPE_STRING) && !g_strv_contains(mimes, "text/plain")) {
        g_ptr_array_add(names, g_strdup("text/plain"));
    }
    if (gdk_content_formats_contain_gtype(formats, GDK_TYPE_FILE_LIST) && !g_strv_contains(mimes, "text/uri-list")) {
        g_ptr_array_add(names, g_strdup("text/uri-list"));
    }
    char *result = names->len ? gkJoin((char **)names->pdata, (int)names->len, size) : NULL;
    g_ptr_array_free(names, TRUE);
    return result;
}

/* ---- File manager and documents ---- */

int gkGTKRevealFile(const char *path, int select) {
    if (!path || !*path) {
        return 0;
    }
    GFile *file = g_file_new_for_path(path);
    GFile *target = select ? g_object_ref(file) : (g_file_query_file_type(file, G_FILE_QUERY_INFO_NONE, NULL) ==
                                                          G_FILE_TYPE_DIRECTORY
                                                      ? g_object_ref(file)
                                                      : g_file_get_parent(file));
    char *uri = g_file_get_uri(target ? target : file);
    GError *error = NULL;
    GDBusConnection *bus = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &error);
    int ok = 0;
    if (bus) {
        const char *uris[] = {uri, NULL};
        GVariant *reply = g_dbus_connection_call_sync(
            bus, "org.freedesktop.FileManager1", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1",
            select ? "ShowItems" : "ShowFolders", g_variant_new("(^ass)", uris, ""), NULL,
            G_DBUS_CALL_FLAGS_NONE, 5000, NULL, &error);
        if (reply) {
            ok = 1;
            g_variant_unref(reply);
        }
        g_object_unref(bus);
    }
    if (!ok) {
        g_clear_error(&error);
        ok = g_app_info_launch_default_for_uri(uri, NULL, NULL);
    }
    g_clear_error(&error);
    g_free(uri);
    if (target) {
        g_object_unref(target);
    }
    g_object_unref(file);
    return ok;
}

static char *gkFileURI(const char *path) {
    GFile *file = g_file_new_for_path(path);
    char *uri = g_file_get_uri(file);
    g_object_unref(file);
    return uri;
}

void gkGTKRecentAdd(const char *path) {
    if (!path || !*path) {
        return;
    }
    char *uri = gkFileURI(path);
    gtk_recent_manager_add_item(gtk_recent_manager_get_default(), uri);
    g_free(uri);
}

void gkGTKRecentRemove(const char *path) {
    if (!path || !*path) {
        return;
    }
    char *uri = gkFileURI(path);
    gtk_recent_manager_remove_item(gtk_recent_manager_get_default(), uri, NULL);
    g_free(uri);
}

static int gkRecentOurs(GtkRecentInfo *info) {
    const char *name = g_get_application_name();
    return name && gtk_recent_info_has_application(info, name);
}

char *gkGTKRecentList(size_t *size) {
    *size = 0;
    GList *items = gtk_recent_manager_get_items(gtk_recent_manager_get_default());
    GPtrArray *paths = g_ptr_array_new_with_free_func(g_free);
    for (GList *item = items; item; item = item->next) {
        GtkRecentInfo *info = item->data;
        if (gkRecentOurs(info)) {
            GFile *file = g_file_new_for_uri(gtk_recent_info_get_uri(info));
            char *path = g_file_get_path(file);
            if (path) {
                g_ptr_array_add(paths, path);
            }
            g_object_unref(file);
        }
        gtk_recent_info_unref(info);
    }
    g_list_free(items);
    char *result = paths->len ? gkJoin((char **)paths->pdata, (int)paths->len, size) : NULL;
    g_ptr_array_free(paths, TRUE);
    return result;
}

void gkGTKRecentClear(void) {
    GtkRecentManager *manager = gtk_recent_manager_get_default();
    GList *items = gtk_recent_manager_get_items(manager);
    for (GList *item = items; item; item = item->next) {
        GtkRecentInfo *info = item->data;
        if (gkRecentOurs(info)) {
            gtk_recent_manager_remove_item(manager, gtk_recent_info_get_uri(info), NULL);
        }
        gtk_recent_info_unref(info);
    }
    g_list_free(items);
}

char *gkGTKDefaultApplication(const char *extension) {
    const char *value = extension ? extension : "";
    char *name = g_strdup_printf("file%s%s", value[0] == '.' ? "" : ".", value);
    gboolean uncertain = FALSE;
    char *type = g_content_type_guess(name, NULL, 0, &uncertain);
    g_free(name);
    GAppInfo *info = type ? g_app_info_get_default_for_type(type, FALSE) : NULL;
    g_free(type);
    char *result = gkStrdup(info ? g_app_info_get_display_name(info) : "");
    if (info) {
        g_object_unref(info);
    }
    return result;
}

/* ---- Printing ---- */

static cairo_t *gkPrintContext;

static void gkPrintBegin(GtkPrintOperation *operation, GtkPrintContext *context, gpointer data) {
    (void)context;
    uintptr_t handle = (uintptr_t)data;
    gtk_print_operation_set_n_pages(operation, MAX(1, gkGTKGoPrintPageCount(handle)));
}

static void gkPrintDraw(GtkPrintOperation *operation, GtkPrintContext *context, int page, gpointer data) {
    uintptr_t handle = (uintptr_t)data;
    gkPrintContext = gtk_print_context_get_cairo_context(context);
    if (!gkGTKGoPrintPage(handle, page, gtk_print_context_get_width(context), gtk_print_context_get_height(context))) {
        gtk_print_operation_cancel(operation);
    }
    gkPrintContext = NULL;
}

int gkGTKPrint(uintptr_t handle, const char *title, int show_dialog, void *window) {
    GKWin *win = window;
    GtkPrintOperation *operation = gtk_print_operation_new();
    gtk_print_operation_set_job_name(operation, title && *title ? title : "GameKit");
    gtk_print_operation_set_unit(operation, GTK_UNIT_POINTS);
    gtk_print_operation_set_use_full_page(operation, FALSE);
    g_signal_connect(operation, "begin-print", G_CALLBACK(gkPrintBegin), (gpointer)handle);
    g_signal_connect(operation, "draw-page", G_CALLBACK(gkPrintDraw), (gpointer)handle);
    GError *error = NULL;
    GtkPrintOperationResult result = gtk_print_operation_run(
        operation, show_dialog ? GTK_PRINT_OPERATION_ACTION_PRINT_DIALOG : GTK_PRINT_OPERATION_ACTION_PRINT,
        win && gtk_widget_get_visible(win->window) ? GTK_WINDOW(win->window) : NULL, &error);
    int pages = -1;
    if (result == GTK_PRINT_OPERATION_RESULT_APPLY) {
        int count = 0;
        g_object_get(operation, "n-pages", &count, NULL);
        pages = count;
    }
    g_clear_error(&error);
    g_object_unref(operation);
    return pages;
}

void gkGTKPrintImage(const unsigned char *pixels, int width, int height, double x, double y, double w, double h) {
    if (!gkPrintContext || !pixels || width <= 0 || height <= 0) {
        return;
    }
    cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, width, height);
    unsigned char *data = cairo_image_surface_get_data(surface);
    int stride = cairo_image_surface_get_stride(surface);
    for (int row = 0; row < height; row++) {
        for (int column = 0; column < width; column++) {
            const unsigned char *p = pixels + ((size_t)row * width + column) * 4;
            guint32 alpha = p[3];
            guint32 red = p[0] * alpha / 255, green = p[1] * alpha / 255, blue = p[2] * alpha / 255;
            *(guint32 *)(data + row * stride + column * 4) = (alpha << 24) | (red << 16) | (green << 8) | blue;
        }
    }
    cairo_surface_mark_dirty(surface);
    cairo_save(gkPrintContext);
    cairo_translate(gkPrintContext, x, y);
    cairo_scale(gkPrintContext, w / width, h / height);
    cairo_set_source_surface(gkPrintContext, surface, 0, 0);
    cairo_paint(gkPrintContext);
    cairo_restore(gkPrintContext);
    cairo_surface_destroy(surface);
}

int gkGTKPrintText(const char *text, double x, double y, const char *family, double size, int weight, int italic) {
    if (!gkFontFamilyInstalled(family)) {
        return 0;
    }
    if (!gkPrintContext) {
        return 1;
    }
    PangoLayout *layout = pango_cairo_create_layout(gkPrintContext);
    PangoFontDescription *description = pango_font_description_new();
    pango_font_description_set_family(description, family && *family && strcmp(family, "System") != 0
                                                       ? family
                                                       : "Sans");
    pango_font_description_set_size(description, (gint)((size > 0 ? size : 12) * PANGO_SCALE));
    pango_font_description_set_weight(description, weight > 0 ? (PangoWeight)weight : PANGO_WEIGHT_NORMAL);
    pango_font_description_set_style(description, italic ? PANGO_STYLE_ITALIC : PANGO_STYLE_NORMAL);
    pango_layout_set_font_description(layout, description);
    pango_layout_set_text(layout, text ? text : "", -1);
    cairo_move_to(gkPrintContext, x, y);
    cairo_set_source_rgb(gkPrintContext, 0, 0, 0);
    pango_cairo_show_layout(gkPrintContext, layout);
    pango_font_description_free(description);
    g_object_unref(layout);
    return 1;
}

/* ---- Sharing ----

   Linux has no share sheet. The picker offers the platform services that
   take the items: the clipboard, the e-mail portal, and the applications
   that open a link or a file. */

typedef struct {
    GtkWidget *popover;
    char **texts;
    char **urls;
    char **files;
    char *service;
    int done;
} GKShare;

static void gkShareFinish(GKShare *share, const char *service) {
    if (share->done) {
        return;
    }
    share->done = 1;
    share->service = g_strdup(service ? service : "");
    gtk_popover_popdown(GTK_POPOVER(share->popover));
}

static void gkShareCopy(GtkButton *button, gpointer data) {
    (void)button;
    GKShare *share = data;
    GString *text = g_string_new("");
    for (int i = 0; share->texts[i]; i++) {
        g_string_append_printf(text, "%s%s", text->len ? "\n" : "", share->texts[i]);
    }
    for (int i = 0; share->urls[i]; i++) {
        g_string_append_printf(text, "%s%s", text->len ? "\n" : "", share->urls[i]);
    }
    size_t files_size = 0;
    int file_count = g_strv_length(share->files);
    char *files = file_count ? gkJoin(share->files, file_count, &files_size) : NULL;
    gkGTKClipboardWrite(text->str, text->len > 0, files, files_size, NULL, 0, 0, NULL, 0, NULL, NULL, 0);
    free(files);
    g_string_free(text, TRUE);
    gkShareFinish(share, "Copy");
}

static void gkShareEmail(GtkButton *button, gpointer data) {
    (void)button;
    GKShare *share = data;
    GString *body = g_string_new("");
    for (int i = 0; share->texts[i]; i++) {
        g_string_append_printf(body, "%s%s", body->len ? "\n" : "", share->texts[i]);
    }
    for (int i = 0; share->urls[i]; i++) {
        g_string_append_printf(body, "%s%s", body->len ? "\n" : "", share->urls[i]);
    }
    char *escaped = g_uri_escape_string(body->str, NULL, FALSE);
    char *mailto = g_strdup_printf("mailto:?body=%s", escaped);
    int ok = g_app_info_launch_default_for_uri(mailto, NULL, NULL);
    g_free(mailto);
    g_free(escaped);
    g_string_free(body, TRUE);
    gkShareFinish(share, ok ? "Mail" : "");
}

static void gkShareOpenDone(GObject *source, GAsyncResult *result, gpointer data) {
    (void)data;
    if (GTK_IS_FILE_LAUNCHER(source)) {
        gtk_file_launcher_launch_finish(GTK_FILE_LAUNCHER(source), result, NULL);
    } else {
        gtk_uri_launcher_launch_finish(GTK_URI_LAUNCHER(source), result, NULL);
    }
    g_object_unref(source);
}

static void gkShareOpen(GtkButton *button, gpointer data) {
    GKShare *share = data;
    GtkWindow *parent = GTK_WINDOW(gtk_widget_get_root(GTK_WIDGET(button)));
    if (share->files[0]) {
        GFile *file = g_file_new_for_path(share->files[0]);
        GtkFileLauncher *launcher = gtk_file_launcher_new(file);
        gtk_file_launcher_set_always_ask(launcher, TRUE);
        gtk_file_launcher_launch(launcher, parent, NULL, gkShareOpenDone, NULL);
        g_object_unref(file);
    } else if (share->urls[0]) {
        GtkUriLauncher *launcher = gtk_uri_launcher_new(share->urls[0]);
        gtk_uri_launcher_launch(launcher, parent, NULL, gkShareOpenDone, NULL);
    }
    gkShareFinish(share, "Open With");
}

static void gkShareClosed(GtkPopover *popover, gpointer data) {
    (void)popover;
    gkShareFinish(data, "");
}

static GtkWidget *gkShareButton(const char *icon, const char *label, GCallback callback, GKShare *share) {
    GtkWidget *button = gtk_button_new();
    gtk_widget_add_css_class(button, "flat");
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_box_append(GTK_BOX(row), gtk_image_new_from_icon_name(icon));
    gtk_box_append(GTK_BOX(row), gtk_label_new(label));
    gtk_button_set_child(GTK_BUTTON(button), row);
    g_signal_connect(button, "clicked", callback, share);
    return button;
}

void *gkGTKShareBegin(const char *texts, size_t texts_size, const char *urls, size_t urls_size, const char *files,
                      size_t files_size, void *view_pointer) {
    GKView *view = gkView(view_pointer);
    GKWin *win = view ? gkWinFromWidget(view->outer) : NULL;
    if (!view || !win || !gtk_widget_get_visible(win->window) || !gtk_widget_get_mapped(view->outer)) {
        return NULL;
    }
    GKShare *share = g_new0(GKShare, 1);
    share->texts = gkSplit(texts, texts_size, NULL);
    share->urls = gkSplit(urls, urls_size, NULL);
    share->files = gkSplit(files, files_size, NULL);
    share->popover = gtk_popover_new();
    g_object_ref_sink(share->popover);
    GtkWidget *list = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_box_append(GTK_BOX(list), gkShareButton("edit-copy-symbolic", "Copy", G_CALLBACK(gkShareCopy), share));
    if (share->texts[0] || share->urls[0]) {
        gtk_box_append(GTK_BOX(list), gkShareButton("mail-send-symbolic", "Mail", G_CALLBACK(gkShareEmail), share));
    }
    if (share->files[0] || share->urls[0]) {
        gtk_box_append(GTK_BOX(list),
                       gkShareButton("document-open-symbolic", "Open With", G_CALLBACK(gkShareOpen), share));
    }
    gtk_popover_set_child(GTK_POPOVER(share->popover), list);
    gtk_widget_set_parent(share->popover, view->outer);
    g_signal_connect(share->popover, "closed", G_CALLBACK(gkShareClosed), share);
    gtk_popover_popup(GTK_POPOVER(share->popover));
    return share;
}

int gkGTKShareStep(void *pointer, char **service) {
    GKShare *share = pointer;
    gkRunPending();
    if (!share->done) {
        return 0;
    }
    *service = strdup(share->service ? share->service : "");
    return 1;
}

void gkGTKShareEnd(void *pointer) {
    GKShare *share = pointer;
    if (!share) {
        return;
    }
    g_signal_handlers_disconnect_by_data(share->popover, share);
    gtk_popover_popdown(GTK_POPOVER(share->popover));
    gtk_widget_unparent(share->popover);
    g_object_unref(share->popover);
    g_strfreev(share->texts);
    g_strfreev(share->urls);
    g_strfreev(share->files);
    g_free(share->service);
    g_free(share);
}
