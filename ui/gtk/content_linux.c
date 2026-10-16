//go:build linux && cgo

#include "gtk_internal.h"

#include <dlfcn.h>
#include <math.h>
#include <stdio.h>

static GKView *gkContent(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind) {
    GKView *view = gkViewNew(outer, control, kind, window, 0);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, outer);
    }
    return view;
}

/* ---- Icon ---- */

void *gkGTKIconCreate(uintptr_t window) {
    GtkWidget *image = g_object_new(GTK_TYPE_IMAGE, "accessible-role", GTK_ACCESSIBLE_ROLE_IMG, NULL);
    gkContent(window, image, NULL, GK_KIND_ICON);
    return image;
}

void gkGTKIconSet(void *pointer, void *image, int width, int height, int tint, const char *name) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ICON) {
        return;
    }
    if (image) {
        gtk_image_set_from_paintable(GTK_IMAGE(view->outer), GDK_PAINTABLE(image));
    } else {
        gtk_image_clear(GTK_IMAGE(view->outer));
    }
    int size = MAX(width, height);
    gtk_image_set_pixel_size(GTK_IMAGE(view->outer), size > 0 ? size : 16);
    const char *klass = gkWidgetCSSClass(view);
    char selector[64], rule[96];
    snprintf(selector, sizeof(selector), "image.%s", klass);
    if (gkSemanticColor(tint)) {
        snprintf(rule, sizeof(rule), "color: %s;", gkSemanticColor(tint));
        gkSetCSSRule(selector, rule);
    } else {
        gkSetCSSRule(selector, NULL);
    }
    if (name && *name) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, name, -1);
        gtk_accessible_update_state(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_STATE_HIDDEN, FALSE, -1);
    } else {
        gtk_accessible_update_state(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_STATE_HIDDEN, TRUE, -1);
    }
}

/* ---- Documents ----

   Rich text and Markdown are a GtkTextView in a scrolled window. Styles are
   text tags; links are tags with a "gk-url" value. */

typedef struct {
    GtkWidget *text;
    int markdown;
} GKDocument;

static GtkTextBuffer *gkDocumentBuffer(GKView *view) {
    return gtk_text_view_get_buffer(GTK_TEXT_VIEW(view->control));
}

static void gkDocumentChanged(GtkTextBuffer *buffer, gpointer data) {
    (void)buffer;
    GKView *view = data;
    if (!view->suppress) {
        gkSend(view->handle, GK_EVENT_TEXT_CHANGE, 0, 0, 0, 0, NULL);
    }
}

static void gkDocumentLinkAt(GKView *view, double x, double y) {
    GtkTextView *text = GTK_TEXT_VIEW(view->control);
    int bx = 0, by = 0;
    gtk_text_view_window_to_buffer_coords(text, GTK_TEXT_WINDOW_WIDGET, (int)x, (int)y, &bx, &by);
    GtkTextIter iter;
    if (!gtk_text_view_get_iter_at_location(text, &iter, bx, by)) {
        return;
    }
    GSList *tags = gtk_text_iter_get_tags(&iter);
    for (GSList *item = tags; item; item = item->next) {
        const char *url = g_object_get_data(G_OBJECT(item->data), "gk-url");
        if (!url) {
            continue;
        }
        GKGTKEvent event = {0};
        event.kind = GK_EVENT_LINK;
        event.text = url;
        int handled = view->handle && gkGTKGoQuery(view->handle, &event);
        if (!handled) {
            g_app_info_launch_default_for_uri(url, NULL, NULL);
        }
        break;
    }
    g_slist_free(tags);
}

static void gkDocumentReleased(GtkGestureClick *gesture, int presses, double x, double y, gpointer data) {
    (void)presses;
    GKView *view = data;
    GtkTextBuffer *buffer = gkDocumentBuffer(view);
    GtkTextIter start, end;
    if (gtk_text_buffer_get_selection_bounds(buffer, &start, &end)) {
        return;
    }
    (void)gesture;
    gkDocumentLinkAt(view, x, y);
}

void *gkGTKDocumentCreate(uintptr_t window, int editable, int markdown) {
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_has_frame(GTK_SCROLLED_WINDOW(scroll), !markdown);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    GtkWidget *text = gtk_text_view_new();
    gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(text), GTK_WRAP_WORD_CHAR);
    gtk_text_view_set_editable(GTK_TEXT_VIEW(text), editable != 0);
    gtk_text_view_set_cursor_visible(GTK_TEXT_VIEW(text), editable != 0);
    gtk_text_view_set_left_margin(GTK_TEXT_VIEW(text), 6);
    gtk_text_view_set_right_margin(GTK_TEXT_VIEW(text), 6);
    gtk_text_view_set_top_margin(GTK_TEXT_VIEW(text), 6);
    gtk_text_view_set_bottom_margin(GTK_TEXT_VIEW(text), 6);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), text);
    g_object_set_data(G_OBJECT(scroll), "gk-scroll", scroll);
    GKView *view = gkContent(window, scroll, text, markdown ? GK_KIND_DOCUMENT : GK_KIND_TEXT_AREA);
    GKDocument *document = g_new0(GKDocument, 1);
    document->text = text;
    document->markdown = markdown != 0;
    view->ext = document;
    view->ext_free = g_free;
    g_signal_connect(gkDocumentBuffer(view), "changed", G_CALLBACK(gkDocumentChanged), view);
    GtkGesture *click = gtk_gesture_click_new();
    gtk_gesture_single_set_button(GTK_GESTURE_SINGLE(click), GDK_BUTTON_PRIMARY);
    g_signal_connect(click, "released", G_CALLBACK(gkDocumentReleased), view);
    gtk_widget_add_controller(text, GTK_EVENT_CONTROLLER(click));
    return scroll;
}

char *gkGTKDocumentText(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view) {
        return gkStrdup("");
    }
    GtkTextBuffer *buffer = gkDocumentBuffer(view);
    GtkTextIter start, end;
    gtk_text_buffer_get_bounds(buffer, &start, &end);
    char *text = gtk_text_buffer_get_text(buffer, &start, &end, FALSE);
    char *result = gkStrdup(text);
    g_free(text);
    return result;
}

void gkGTKDocumentSetText(void *pointer, const char *text) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    view->suppress++;
    gtk_text_buffer_set_text(gkDocumentBuffer(view), text ? text : "", -1);
    view->suppress--;
}

void gkGTKDocumentSetEditable(void *pointer, int editable) {
    GKView *view = gkView(pointer);
    if (view) {
        gtk_text_view_set_editable(GTK_TEXT_VIEW(view->control), editable != 0);
        gtk_text_view_set_cursor_visible(GTK_TEXT_VIEW(view->control), editable != 0);
    }
}

int gkGTKDocumentEditable(void *pointer) {
    GKView *view = gkView(pointer);
    return view && gtk_text_view_get_editable(GTK_TEXT_VIEW(view->control));
}

/* ---- Text styles ---- */

typedef struct {
    int bold;
    int italic;
    int underline;
    int strike;
    double size;
    int has_color;
    GdkRGBA color;
    char *family;
} GKStyle;

static GtkTextTag *gkStyleTag(GtkTextBuffer *buffer, const GKStyle *style) {
    char name[256];
    snprintf(name, sizeof(name), "gk:%d%d%d%d:%g:%d:%.3f,%.3f,%.3f:%s", style->bold, style->italic, style->underline,
             style->strike, style->size, style->has_color, style->color.red, style->color.green, style->color.blue,
             style->family ? style->family : "");
    GtkTextTagTable *table = gtk_text_buffer_get_tag_table(buffer);
    GtkTextTag *tag = gtk_text_tag_table_lookup(table, name);
    if (tag) {
        return tag;
    }
    tag = gtk_text_buffer_create_tag(buffer, name, NULL);
    if (style->bold) {
        g_object_set(tag, "weight", PANGO_WEIGHT_BOLD, NULL);
    }
    if (style->italic) {
        g_object_set(tag, "style", PANGO_STYLE_ITALIC, NULL);
    }
    if (style->underline) {
        g_object_set(tag, "underline", PANGO_UNDERLINE_SINGLE, NULL);
    }
    if (style->strike) {
        g_object_set(tag, "strikethrough", TRUE, NULL);
    }
    if (style->size > 0) {
        g_object_set(tag, "size-points", style->size, NULL);
    }
    if (style->has_color) {
        g_object_set(tag, "foreground-rgba", &style->color, NULL);
    }
    if (style->family && *style->family) {
        g_object_set(tag, "family", style->family, NULL);
    }
    g_object_set_data(G_OBJECT(tag), "gk-style", GINT_TO_POINTER(1));
    return tag;
}

static void gkStyleAt(GtkTextIter *iter, GKStyle *style) {
    memset(style, 0, sizeof(*style));
    GSList *tags = gtk_text_iter_get_tags(iter);
    for (GSList *item = tags; item; item = item->next) {
        GtkTextTag *tag = item->data;
        gboolean set = FALSE;
        int weight = 0;
        g_object_get(tag, "weight-set", &set, "weight", &weight, NULL);
        if (set && weight >= PANGO_WEIGHT_SEMIBOLD) {
            style->bold = 1;
        }
        PangoStyle slant = PANGO_STYLE_NORMAL;
        g_object_get(tag, "style-set", &set, "style", &slant, NULL);
        if (set && slant != PANGO_STYLE_NORMAL) {
            style->italic = 1;
        }
        PangoUnderline underline = PANGO_UNDERLINE_NONE;
        g_object_get(tag, "underline-set", &set, "underline", &underline, NULL);
        if (set && underline != PANGO_UNDERLINE_NONE) {
            style->underline = 1;
        }
        gboolean strike = FALSE;
        g_object_get(tag, "strikethrough-set", &set, "strikethrough", &strike, NULL);
        if (set && strike) {
            style->strike = 1;
        }
        double size = 0;
        g_object_get(tag, "size-set", &set, "size-points", &size, NULL);
        if (set && size > 0) {
            style->size = size;
        }
        GdkRGBA *color = NULL;
        g_object_get(tag, "foreground-set", &set, "foreground-rgba", &color, NULL);
        if (set && color) {
            style->has_color = 1;
            style->color = *color;
        }
        if (color) {
            gdk_rgba_free(color);
        }
        char *family = NULL;
        g_object_get(tag, "family-set", &set, "family", &family, NULL);
        if (set && family) {
            g_free(style->family);
            style->family = family;
        } else {
            g_free(family);
        }
    }
    g_slist_free(tags);
}

/* ---- RTF ----

   The writer and the reader cover the character formats of the text tags:
   bold, italic, underline, strikethrough, size, color, and font. */

static void gkRTFText(GString *out, const char *text, const char *end) {
    for (const char *p = text; p < end && *p; p = g_utf8_next_char(p)) {
        gunichar c = g_utf8_get_char(p);
        if (c == '\\' || c == '{' || c == '}') {
            g_string_append_c(out, '\\');
            g_string_append_c(out, (char)c);
        } else if (c == '\n') {
            g_string_append(out, "\\par\n");
        } else if (c == '\t') {
            g_string_append(out, "\\tab ");
        } else if (c < 0x80) {
            g_string_append_c(out, (char)c);
        } else if (c < 0x10000) {
            g_string_append_printf(out, "\\u%d?", (gint16)c);
        } else {
            gunichar2 *pair = g_ucs4_to_utf16(&c, 1, NULL, NULL, NULL);
            if (pair) {
                g_string_append_printf(out, "\\u%d?\\u%d?", (gint16)pair[0], (gint16)pair[1]);
                g_free(pair);
            }
        }
    }
}

static int gkRTFIndex(GPtrArray *values, const char *value) {
    for (guint i = 0; i < values->len; i++) {
        if (strcmp(g_ptr_array_index(values, i), value) == 0) {
            return (int)i;
        }
    }
    g_ptr_array_add(values, g_strdup(value));
    return (int)values->len - 1;
}

unsigned char *gkGTKDocumentRTF(void *pointer, size_t *size) {
    GKView *view = gkView(pointer);
    *size = 0;
    if (!view) {
        return NULL;
    }
    GtkTextBuffer *buffer = gkDocumentBuffer(view);
    GPtrArray *fonts = g_ptr_array_new_with_free_func(g_free);
    GPtrArray *colors = g_ptr_array_new_with_free_func(g_free);
    g_ptr_array_add(fonts, g_strdup("Sans"));
    GString *body = g_string_new("");
    GtkTextIter iter, end;
    gtk_text_buffer_get_start_iter(buffer, &iter);
    gtk_text_buffer_get_end_iter(buffer, &end);
    while (!gtk_text_iter_equal(&iter, &end)) {
        GtkTextIter next = iter;
        if (!gtk_text_iter_forward_to_tag_toggle(&next, NULL)) {
            next = end;
        }
        GKStyle style;
        gkStyleAt(&iter, &style);
        char *text = gtk_text_buffer_get_text(buffer, &iter, &next, FALSE);
        g_string_append_c(body, '{');
        if (style.bold) {
            g_string_append(body, "\\b");
        }
        if (style.italic) {
            g_string_append(body, "\\i");
        }
        if (style.underline) {
            g_string_append(body, "\\ul");
        }
        if (style.strike) {
            g_string_append(body, "\\strike");
        }
        if (style.size > 0) {
            g_string_append_printf(body, "\\fs%d", (int)round(style.size * 2));
        }
        if (style.has_color) {
            char color[64];
            snprintf(color, sizeof(color), "\\red%d\\green%d\\blue%d;", (int)round(style.color.red * 255),
                     (int)round(style.color.green * 255), (int)round(style.color.blue * 255));
            g_string_append_printf(body, "\\cf%d", gkRTFIndex(colors, color) + 1);
        }
        if (style.family) {
            g_string_append_printf(body, "\\f%d", gkRTFIndex(fonts, style.family));
        }
        g_string_append_c(body, ' ');
        gkRTFText(body, text, text + strlen(text));
        g_string_append_c(body, '}');
        g_free(text);
        g_free(style.family);
        iter = next;
    }
    GString *out = g_string_new("{\\rtf1\\ansi\\ansicpg1252\\deff0{\\fonttbl");
    for (guint i = 0; i < fonts->len; i++) {
        g_string_append_printf(out, "{\\f%u ", i);
        gkRTFText(out, g_ptr_array_index(fonts, i), (const char *)g_ptr_array_index(fonts, i) +
                                                        strlen(g_ptr_array_index(fonts, i)));
        g_string_append(out, ";}");
    }
    g_string_append(out, "}{\\colortbl;");
    for (guint i = 0; i < colors->len; i++) {
        g_string_append(out, g_ptr_array_index(colors, i));
    }
    g_string_append(out, "}\n");
    g_string_append_len(out, body->str, (gssize)body->len);
    g_string_append(out, "}\n");
    g_string_free(body, TRUE);
    g_ptr_array_free(fonts, TRUE);
    g_ptr_array_free(colors, TRUE);
    *size = out->len;
    unsigned char *result = malloc(out->len);
    memcpy(result, out->str, out->len);
    g_string_free(out, TRUE);
    return result;
}

typedef struct {
    GKStyle style;
    int skip;
    int destination;
    int unicode_skip;
} GKRTFState;

enum { GK_RTF_TEXT, GK_RTF_FONTS, GK_RTF_COLORS };

typedef struct {
    const unsigned char *data;
    size_t size;
    size_t at;
    GArray *stack;
    GPtrArray *fonts;
    GArray *colors;
    GString *font;
    GdkRGBA color;
    int font_index;
    GString *run;
    GtkTextBuffer *buffer;
    GKStyle run_style;
    int skip_chars;
} GKRTFReader;

static GKRTFState *gkRTFTop(GKRTFReader *reader) {
    return &g_array_index(reader->stack, GKRTFState, reader->stack->len - 1);
}

static void gkRTFFlush(GKRTFReader *reader) {
    if (!reader->run->len) {
        return;
    }
    GtkTextIter end;
    gtk_text_buffer_get_end_iter(reader->buffer, &end);
    GKStyle *style = &reader->run_style;
    if (style->bold || style->italic || style->underline || style->strike || style->size > 0 || style->has_color ||
        style->family) {
        GtkTextTag *tag = gkStyleTag(reader->buffer, style);
        gtk_text_buffer_insert_with_tags(reader->buffer, &end, reader->run->str, (int)reader->run->len, tag, NULL);
    } else {
        gtk_text_buffer_insert(reader->buffer, &end, reader->run->str, (int)reader->run->len);
    }
    g_string_truncate(reader->run, 0);
}

static int gkStyleEqual(const GKStyle *a, const GKStyle *b) {
    return a->bold == b->bold && a->italic == b->italic && a->underline == b->underline && a->strike == b->strike &&
           a->size == b->size && a->has_color == b->has_color &&
           (!a->has_color || gdk_rgba_equal(&a->color, &b->color)) && a->family == b->family;
}

static void gkRTFEmit(GKRTFReader *reader, const char *text, int length) {
    GKRTFState *state = gkRTFTop(reader);
    if (state->skip) {
        return;
    }
    if (state->destination == GK_RTF_FONTS) {
        g_string_append_len(reader->font, text, length);
        return;
    }
    if (state->destination != GK_RTF_TEXT) {
        return;
    }
    if (!gkStyleEqual(&state->style, &reader->run_style)) {
        gkRTFFlush(reader);
        reader->run_style = state->style;
    }
    g_string_append_len(reader->run, text, length);
}

static void gkRTFUnicode(GKRTFReader *reader, gunichar c) {
    char buffer[8];
    int length = g_unichar_to_utf8(c, buffer);
    gkRTFEmit(reader, buffer, length);
}

static void gkRTFWord(GKRTFReader *reader, const char *word, int has_value, int value) {
    GKRTFState *state = gkRTFTop(reader);
    if (strcmp(word, "fonttbl") == 0) {
        state->destination = GK_RTF_FONTS;
    } else if (strcmp(word, "colortbl") == 0) {
        state->destination = GK_RTF_COLORS;
        reader->color = (GdkRGBA){0, 0, 0, 1};
    } else if (strcmp(word, "stylesheet") == 0 || strcmp(word, "info") == 0 || strcmp(word, "pict") == 0 ||
               strcmp(word, "header") == 0 || strcmp(word, "footer") == 0 || strcmp(word, "object") == 0) {
        state->skip = 1;
    } else if (strcmp(word, "b") == 0) {
        state->style.bold = !has_value || value != 0;
    } else if (strcmp(word, "i") == 0) {
        state->style.italic = !has_value || value != 0;
    } else if (strcmp(word, "ul") == 0) {
        state->style.underline = !has_value || value != 0;
    } else if (strcmp(word, "ulnone") == 0) {
        state->style.underline = 0;
    } else if (strcmp(word, "strike") == 0) {
        state->style.strike = !has_value || value != 0;
    } else if (strcmp(word, "fs") == 0 && has_value) {
        state->style.size = value / 2.0;
    } else if (strcmp(word, "plain") == 0) {
        memset(&state->style, 0, sizeof(state->style));
    } else if (strcmp(word, "cf") == 0 && has_value) {
        if (value > 0 && value <= (int)reader->colors->len) {
            state->style.has_color = 1;
            state->style.color = g_array_index(reader->colors, GdkRGBA, value - 1);
        } else {
            state->style.has_color = 0;
        }
    } else if (strcmp(word, "f") == 0 && has_value) {
        if (state->destination == GK_RTF_FONTS) {
            reader->font_index = value;
        } else {
            const char *family = value >= 0 && value < (int)reader->fonts->len
                                     ? g_ptr_array_index(reader->fonts, value)
                                     : NULL;
            state->style.family = family && value > 0 ? (char *)family : NULL;
        }
    } else if (strcmp(word, "red") == 0 && state->destination == GK_RTF_COLORS) {
        reader->color.red = value / 255.0f;
    } else if (strcmp(word, "green") == 0 && state->destination == GK_RTF_COLORS) {
        reader->color.green = value / 255.0f;
    } else if (strcmp(word, "blue") == 0 && state->destination == GK_RTF_COLORS) {
        reader->color.blue = value / 255.0f;
    } else if (strcmp(word, "par") == 0 || strcmp(word, "line") == 0) {
        gkRTFEmit(reader, "\n", 1);
    } else if (strcmp(word, "tab") == 0) {
        gkRTFEmit(reader, "\t", 1);
    } else if (strcmp(word, "uc") == 0 && has_value) {
        state->unicode_skip = value;
    } else if (strcmp(word, "u") == 0 && has_value) {
        int code = value < 0 ? value + 65536 : value;
        gkRTFUnicode(reader, (gunichar)code);
        reader->skip_chars = state->unicode_skip;
    }
}

int gkGTKDocumentSetRTF(void *pointer, const unsigned char *rtf, size_t size) {
    GKView *view = gkView(pointer);
    if (!view || !rtf || size < 5 || memcmp(rtf, "{\\rtf", 5) != 0) {
        return 0;
    }
    GKRTFReader reader = {0};
    reader.data = rtf;
    reader.size = size;
    reader.stack = g_array_new(FALSE, TRUE, sizeof(GKRTFState));
    reader.fonts = g_ptr_array_new_with_free_func(g_free);
    reader.colors = g_array_new(FALSE, TRUE, sizeof(GdkRGBA));
    reader.font = g_string_new("");
    reader.run = g_string_new("");
    reader.buffer = gkDocumentBuffer(view);
    GKRTFState root = {0};
    root.unicode_skip = 1;
    g_array_append_val(reader.stack, root);
    view->suppress++;
    gtk_text_buffer_set_text(reader.buffer, "", -1);
    int depth = 0;
    while (reader.at < size) {
        unsigned char c = rtf[reader.at];
        if (c == '{') {
            GKRTFState copy = *gkRTFTop(&reader);
            g_array_append_val(reader.stack, copy);
            depth++;
            reader.at++;
            continue;
        }
        if (c == '}') {
            GKRTFState *state = gkRTFTop(&reader);
            if (state->destination == GK_RTF_FONTS && reader.font->len) {
                char *name = g_strdup(reader.font->str);
                char *semicolon = strchr(name, ';');
                if (semicolon) {
                    *semicolon = '\0';
                }
                g_strstrip(name);
                while ((int)reader.fonts->len <= reader.font_index) {
                    g_ptr_array_add(reader.fonts, g_strdup(""));
                }
                g_free(g_ptr_array_index(reader.fonts, reader.font_index));
                g_ptr_array_index(reader.fonts, reader.font_index) = name;
                g_string_truncate(reader.font, 0);
            }
            if (reader.stack->len > 1) {
                g_array_set_size(reader.stack, reader.stack->len - 1);
            }
            depth--;
            reader.at++;
            continue;
        }
        if (c == '\\') {
            reader.at++;
            if (reader.at >= size) {
                break;
            }
            unsigned char next = rtf[reader.at];
            if (g_ascii_isalpha(next)) {
                char word[32];
                int length = 0;
                while (reader.at < size && g_ascii_isalpha(rtf[reader.at]) && length < 31) {
                    word[length++] = (char)rtf[reader.at++];
                }
                word[length] = '\0';
                int has_value = 0, value = 0, negative = 0;
                if (reader.at < size && rtf[reader.at] == '-') {
                    negative = 1;
                    reader.at++;
                }
                while (reader.at < size && g_ascii_isdigit(rtf[reader.at])) {
                    has_value = 1;
                    value = value * 10 + (rtf[reader.at++] - '0');
                }
                if (negative) {
                    value = -value;
                }
                if (reader.at < size && rtf[reader.at] == ' ') {
                    reader.at++;
                }
                gkRTFWord(&reader, word, has_value, value);
                continue;
            }
            reader.at++;
            if (next == '*') {
                gkRTFTop(&reader)->skip = 1;
            } else if (next == '\'' && reader.at + 1 < size) {
                char hex[3] = {(char)rtf[reader.at], (char)rtf[reader.at + 1], 0};
                reader.at += 2;
                if (reader.skip_chars > 0) {
                    reader.skip_chars--;
                } else {
                    gkRTFUnicode(&reader, (gunichar)strtol(hex, NULL, 16));
                }
            } else if (next == '\\' || next == '{' || next == '}') {
                char text = (char)next;
                gkRTFEmit(&reader, &text, 1);
            } else if (next == '~') {
                gkRTFUnicode(&reader, 0xa0);
            } else if (next == '\n' || next == '\r') {
                gkRTFEmit(&reader, "\n", 1);
            }
            continue;
        }
        reader.at++;
        if (c == '\n' || c == '\r') {
            continue;
        }
        if (reader.skip_chars > 0) {
            reader.skip_chars--;
            continue;
        }
        GKRTFState *state = gkRTFTop(&reader);
        if (state->destination == GK_RTF_COLORS && c == ';') {
            g_array_append_val(reader.colors, reader.color);
            reader.color = (GdkRGBA){0, 0, 0, 1};
            continue;
        }
        char text = (char)c;
        gkRTFEmit(&reader, &text, 1);
    }
    gkRTFFlush(&reader);
    view->suppress--;
    g_array_free(reader.stack, TRUE);
    g_ptr_array_free(reader.fonts, TRUE);
    g_array_free(reader.colors, TRUE);
    g_string_free(reader.font, TRUE);
    g_string_free(reader.run, TRUE);
    return 1;
}

/* ---- Markdown ----

   The renderer covers CommonMark blocks (headings, paragraphs, block
   quotes, lists, code blocks, and thematic breaks) and inlines (emphasis,
   strong emphasis, code spans, and links). */

static GtkTextTag *gkMarkdownTag(GtkTextBuffer *buffer, const char *name) {
    GtkTextTagTable *table = gtk_text_buffer_get_tag_table(buffer);
    GtkTextTag *tag = gtk_text_tag_table_lookup(table, name);
    if (tag) {
        return tag;
    }
    if (strcmp(name, "h1") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "weight", PANGO_WEIGHT_BOLD, "scale", 1.8, "pixels-below-lines",
                                          6, NULL);
    }
    if (strcmp(name, "h2") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "weight", PANGO_WEIGHT_BOLD, "scale", 1.5, "pixels-below-lines",
                                          4, NULL);
    }
    if (strcmp(name, "h3") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "weight", PANGO_WEIGHT_BOLD, "scale", 1.25, NULL);
    }
    if (name[0] == 'h') {
        return gtk_text_buffer_create_tag(buffer, name, "weight", PANGO_WEIGHT_BOLD, NULL);
    }
    if (strcmp(name, "strong") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "weight", PANGO_WEIGHT_BOLD, NULL);
    }
    if (strcmp(name, "em") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "style", PANGO_STYLE_ITALIC, NULL);
    }
    if (strcmp(name, "code") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "family", "Monospace", NULL);
    }
    if (strcmp(name, "pre") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "family", "Monospace", "left-margin", 18, "wrap-mode",
                                          GTK_WRAP_NONE, NULL);
    }
    if (strcmp(name, "quote") == 0) {
        GdkRGBA gray = {0.5f, 0.5f, 0.5f, 1};
        return gtk_text_buffer_create_tag(buffer, name, "left-margin", 24, "style", PANGO_STYLE_ITALIC,
                                          "foreground-rgba", &gray, NULL);
    }
    if (strcmp(name, "item") == 0) {
        return gtk_text_buffer_create_tag(buffer, name, "left-margin", 24, NULL);
    }
    return gtk_text_buffer_create_tag(buffer, name, NULL);
}

static void gkInsertTagged(GtkTextBuffer *buffer, const char *text, int length, GtkTextTag *block, GtkTextTag *inline_tag,
                           GtkTextTag *link) {
    GtkTextIter end;
    gtk_text_buffer_get_end_iter(buffer, &end);
    gtk_text_buffer_insert_with_tags(buffer, &end, text, length, block ? block : inline_tag, block ? inline_tag : link,
                                     block ? link : NULL, NULL);
}

static void gkMarkdownInline(GtkTextBuffer *buffer, const char *text, GtkTextTag *block) {
    GString *plain = g_string_new("");
    const char *p = text;
    while (*p) {
        if (*p == '\\' && p[1] && strchr("\\`*_{}[]()#+-.!>", p[1])) {
            g_string_append_c(plain, p[1]);
            p += 2;
            continue;
        }
        if (*p == '`') {
            const char *close = strchr(p + 1, '`');
            if (close) {
                gkInsertTagged(buffer, plain->str, (int)plain->len, block, NULL, NULL);
                g_string_truncate(plain, 0);
                gkInsertTagged(buffer, p + 1, (int)(close - p - 1), block, gkMarkdownTag(buffer, "code"), NULL);
                p = close + 1;
                continue;
            }
        }
        if ((p[0] == '*' && p[1] == '*') || (p[0] == '_' && p[1] == '_')) {
            char marker[3] = {p[0], p[1], 0};
            const char *close = strstr(p + 2, marker);
            if (close && close > p + 2) {
                gkInsertTagged(buffer, plain->str, (int)plain->len, block, NULL, NULL);
                g_string_truncate(plain, 0);
                gkInsertTagged(buffer, p + 2, (int)(close - p - 2), block, gkMarkdownTag(buffer, "strong"), NULL);
                p = close + 2;
                continue;
            }
        }
        if ((*p == '*' || *p == '_') && p[1] && p[1] != ' ') {
            const char *close = strchr(p + 1, *p);
            if (close && close > p + 1) {
                gkInsertTagged(buffer, plain->str, (int)plain->len, block, NULL, NULL);
                g_string_truncate(plain, 0);
                gkInsertTagged(buffer, p + 1, (int)(close - p - 1), block, gkMarkdownTag(buffer, "em"), NULL);
                p = close + 1;
                continue;
            }
        }
        if (*p == '[') {
            const char *label_end = strstr(p, "](");
            const char *url_end = label_end ? strchr(label_end + 2, ')') : NULL;
            if (label_end && url_end) {
                gkInsertTagged(buffer, plain->str, (int)plain->len, block, NULL, NULL);
                g_string_truncate(plain, 0);
                char *url = g_strndup(label_end + 2, (gsize)(url_end - label_end - 2));
                GdkRGBA blue = {0.1f, 0.4f, 0.85f, 1};
                GtkTextTag *link =
                    gtk_text_buffer_create_tag(buffer, NULL, "underline", PANGO_UNDERLINE_SINGLE, "foreground-rgba",
                                               &blue, NULL);
                g_object_set_data_full(G_OBJECT(link), "gk-url", url, g_free);
                gkInsertTagged(buffer, p + 1, (int)(label_end - p - 1), block, link, NULL);
                p = url_end + 1;
                continue;
            }
        }
        g_string_append_c(plain, *p);
        p++;
    }
    gkInsertTagged(buffer, plain->str, (int)plain->len, block, NULL, NULL);
    g_string_free(plain, TRUE);
}

static void gkMarkdownNewline(GtkTextBuffer *buffer) {
    GtkTextIter end;
    gtk_text_buffer_get_end_iter(buffer, &end);
    gtk_text_buffer_insert(buffer, &end, "\n", 1);
}

int gkGTKMarkdownSet(void *pointer, const char *source) {
    GKView *view = gkView(pointer);
    if (!view || !g_utf8_validate(source ? source : "", -1, NULL)) {
        return 0;
    }
    GtkTextBuffer *buffer = gkDocumentBuffer(view);
    view->suppress++;
    gtk_text_buffer_set_text(buffer, "", -1);
    char **lines = g_strsplit(source ? source : "", "\n", -1);
    GString *paragraph = g_string_new("");
    int in_code = 0;
    int first = 1;
    for (int i = 0; lines[i]; i++) {
        char *line = lines[i];
        g_strchomp(line);
        if (g_str_has_prefix(line, "```") || g_str_has_prefix(line, "~~~")) {
            if (paragraph->len) {
                gkMarkdownInline(buffer, paragraph->str, NULL);
                gkMarkdownNewline(buffer);
                g_string_truncate(paragraph, 0);
            }
            in_code = !in_code;
            continue;
        }
        if (in_code) {
            gkInsertTagged(buffer, line, -1, gkMarkdownTag(buffer, "pre"), NULL, NULL);
            gkMarkdownNewline(buffer);
            continue;
        }
        char *trimmed = line;
        while (*trimmed == ' ') {
            trimmed++;
        }
        int blank = *trimmed == '\0';
        int heading = 0;
        while (trimmed[heading] == '#' && heading < 7) {
            heading++;
        }
        int is_heading = heading > 0 && heading <= 6 && (trimmed[heading] == ' ' || trimmed[heading] == '\0');
        int is_quote = *trimmed == '>';
        int is_bullet = (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ';
        int digits = 0;
        while (g_ascii_isdigit(trimmed[digits])) {
            digits++;
        }
        int is_number = digits > 0 && (trimmed[digits] == '.' || trimmed[digits] == ')') && trimmed[digits + 1] == ' ';
        int is_rule = (strcmp(trimmed, "---") == 0 || strcmp(trimmed, "***") == 0 || strcmp(trimmed, "___") == 0);
        if (blank || is_heading || is_quote || is_bullet || is_number || is_rule) {
            if (paragraph->len) {
                gkMarkdownInline(buffer, paragraph->str, NULL);
                gkMarkdownNewline(buffer);
                g_string_truncate(paragraph, 0);
                first = 0;
            }
        }
        if (blank) {
            if (!first) {
                gkMarkdownNewline(buffer);
            }
            continue;
        }
        if (is_heading) {
            char name[4];
            snprintf(name, sizeof(name), "h%d", heading);
            const char *text = trimmed + heading;
            while (*text == ' ') {
                text++;
            }
            gkMarkdownInline(buffer, text, gkMarkdownTag(buffer, name));
            gkMarkdownNewline(buffer);
        } else if (is_quote) {
            const char *text = trimmed + 1;
            while (*text == ' ') {
                text++;
            }
            gkMarkdownInline(buffer, text, gkMarkdownTag(buffer, "quote"));
            gkMarkdownNewline(buffer);
        } else if (is_bullet || is_number) {
            GtkTextTag *item = gkMarkdownTag(buffer, "item");
            char *marker = is_bullet ? g_strdup("• ") : g_strndup(trimmed, (gsize)digits + 2);
            gkInsertTagged(buffer, marker, -1, item, NULL, NULL);
            g_free(marker);
            gkMarkdownInline(buffer, trimmed + (is_bullet ? 2 : digits + 2), item);
            gkMarkdownNewline(buffer);
        } else if (is_rule) {
            gkInsertTagged(buffer, "────────", -1, NULL, NULL, NULL);
            gkMarkdownNewline(buffer);
        } else {
            if (paragraph->len) {
                g_string_append_c(paragraph, ' ');
            }
            g_string_append(paragraph, trimmed);
        }
        first = 0;
    }
    if (paragraph->len) {
        gkMarkdownInline(buffer, paragraph->str, NULL);
    }
    g_string_free(paragraph, TRUE);
    g_strfreev(lines);
    view->suppress--;
    return 1;
}

/* ---- SVG ---- */

void *gkGTKVectorCreate(uintptr_t window) {
    GtkWidget *picture = g_object_new(GTK_TYPE_PICTURE, "accessible-role", GTK_ACCESSIBLE_ROLE_IMG, NULL);
    gtk_picture_set_content_fit(GTK_PICTURE(picture), GTK_CONTENT_FIT_CONTAIN);
    gkContent(window, picture, NULL, GK_KIND_VECTOR);
    return picture;
}

int gkGTKVectorSet(void *pointer, const unsigned char *data, size_t size, const char *label, double *width,
                   double *height) {
    GKView *view = gkView(pointer);
    if (!view || !data || !size) {
        return 0;
    }
    GBytes *bytes = g_bytes_new(data, size);
    GdkPaintable *paintable = NULL;
    GError *error = NULL;
#if GTK_CHECK_VERSION(4, 22, 0)
    /* GtkSvg renders the document as vectors at every size. */
    GtkSvg *svg = gtk_svg_new_from_bytes(bytes);
    if (svg && gdk_paintable_get_intrinsic_width(GDK_PAINTABLE(svg)) > 0) {
        paintable = GDK_PAINTABLE(svg);
    } else if (svg) {
        g_object_unref(svg);
    }
#endif
    if (!paintable) {
        GdkTexture *texture = gdk_texture_new_from_bytes(bytes, &error);
        paintable = texture ? GDK_PAINTABLE(texture) : NULL;
    }
    g_bytes_unref(bytes);
    if (error) {
        g_error_free(error);
    }
    if (!paintable) {
        return 0;
    }
    gtk_picture_set_paintable(GTK_PICTURE(view->outer), paintable);
    *width = gdk_paintable_get_intrinsic_width(paintable);
    *height = gdk_paintable_get_intrinsic_height(paintable);
    g_object_unref(paintable);
    if (label && *label) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
    }
    return 1;
}

/* ---- Animated image ---- */

typedef struct {
    GPtrArray *frames;
    double *durations;
    int count;
    int repeats;
    int index;
    int playing;
    gint64 shown;
    guint tick;
} GKAnimation;

static void gkAnimationFree(gpointer data) {
    GKAnimation *animation = data;
    g_ptr_array_free(animation->frames, TRUE);
    g_free(animation->durations);
    g_free(animation);
}

static gboolean gkAnimationTick(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
    GKView *view = data;
    GKAnimation *animation = view->ext;
    if (!animation->playing || animation->count < 2) {
        animation->tick = 0;
        return G_SOURCE_REMOVE;
    }
    gint64 now = gdk_frame_clock_get_frame_time(clock);
    if (!animation->shown) {
        animation->shown = now;
    }
    double duration = animation->durations[animation->index] > 0 ? animation->durations[animation->index] : 0.1;
    if (now - animation->shown < (gint64)(duration * G_USEC_PER_SEC)) {
        return G_SOURCE_CONTINUE;
    }
    animation->shown = now;
    if (animation->index + 1 >= animation->count && !animation->repeats) {
        animation->playing = 0;
        animation->tick = 0;
        return G_SOURCE_REMOVE;
    }
    animation->index = (animation->index + 1) % animation->count;
    gtk_picture_set_paintable(GTK_PICTURE(widget), g_ptr_array_index(animation->frames, animation->index));
    return G_SOURCE_CONTINUE;
}

void *gkGTKAnimationCreate(uintptr_t window, void **images, const double *durations, int count, int repeats) {
    GtkWidget *picture = g_object_new(GTK_TYPE_PICTURE, "accessible-role", GTK_ACCESSIBLE_ROLE_IMG, NULL);
    gtk_picture_set_content_fit(GTK_PICTURE(picture), GTK_CONTENT_FIT_CONTAIN);
    GKView *view = gkContent(window, picture, NULL, GK_KIND_ANIMATION);
    GKAnimation *animation = g_new0(GKAnimation, 1);
    animation->frames = g_ptr_array_new_with_free_func(g_object_unref);
    animation->durations = g_new0(double, MAX(1, count));
    animation->count = count;
    animation->repeats = repeats != 0;
    for (int i = 0; i < count; i++) {
        g_ptr_array_add(animation->frames, images[i] ? g_object_ref(images[i]) : NULL);
        animation->durations[i] = durations[i];
    }
    view->ext = animation;
    view->ext_free = gkAnimationFree;
    if (count > 0 && images[0]) {
        gtk_picture_set_paintable(GTK_PICTURE(picture), GDK_PAINTABLE(images[0]));
    }
    return picture;
}

void gkGTKAnimationPlay(void *pointer, int play) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_ANIMATION) {
        return;
    }
    GKAnimation *animation = view->ext;
    animation->playing = play != 0 && animation->count > 1;
    if (animation->playing && !animation->tick) {
        if (!animation->repeats && animation->index + 1 >= animation->count) {
            animation->index = 0;
            gtk_picture_set_paintable(GTK_PICTURE(view->outer), g_ptr_array_index(animation->frames, 0));
        }
        animation->shown = 0;
        animation->tick = gtk_widget_add_tick_callback(view->outer, gkAnimationTick, view, NULL);
    }
}

int gkGTKAnimationPlaying(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_ANIMATION && ((GKAnimation *)view->ext)->playing;
}

/* ---- Video ---- */

void *gkGTKVideoCreate(uintptr_t window, const char *source, int controls, int autoplay, int muted) {
    GFile *file = NULL;
    if (source && strstr(source, "://")) {
        file = g_file_new_for_uri(source);
    } else {
        file = g_file_new_for_path(source ? source : "");
    }
    GtkMediaStream *stream = gtk_media_file_new_for_file(file);
    g_object_unref(file);
    gtk_media_stream_set_muted(stream, muted != 0);
    GtkWidget *outer;
    if (controls) {
        outer = gtk_video_new();
        gtk_video_set_media_stream(GTK_VIDEO(outer), stream);
    } else {
        outer = gtk_picture_new_for_paintable(GDK_PAINTABLE(stream));
        gtk_picture_set_content_fit(GTK_PICTURE(outer), GTK_CONTENT_FIT_CONTAIN);
    }
    GKView *view = gkContent(window, outer, NULL, GK_KIND_VIDEO);
    view->ext = stream;
    view->ext_free = g_object_unref;
    if (autoplay) {
        gtk_media_stream_play(stream);
    }
    return outer;
}

static GtkMediaStream *gkStream(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_VIDEO ? view->ext : NULL;
}

void gkGTKVideoPlay(void *pointer, int play) {
    GtkMediaStream *stream = gkStream(pointer);
    if (!stream) {
        return;
    }
    if (play) {
        gtk_media_stream_play(stream);
    } else {
        gtk_media_stream_pause(stream);
    }
}

int gkGTKVideoPlaying(void *pointer) {
    GtkMediaStream *stream = gkStream(pointer);
    return stream && gtk_media_stream_get_playing(stream);
}

double gkGTKVideoPosition(void *pointer) {
    GtkMediaStream *stream = gkStream(pointer);
    return stream ? gtk_media_stream_get_timestamp(stream) / 1e6 : 0;
}

double gkGTKVideoDuration(void *pointer) {
    GtkMediaStream *stream = gkStream(pointer);
    return stream ? gtk_media_stream_get_duration(stream) / 1e6 : 0;
}

void gkGTKVideoSeek(void *pointer, double seconds) {
    GtkMediaStream *stream = gkStream(pointer);
    if (stream && gtk_media_stream_is_seekable(stream)) {
        gtk_media_stream_seek(stream, (gint64)(MAX(0, seconds) * 1e6));
    }
}

void gkGTKVideoRelease(void *pointer) {
    GtkMediaStream *stream = gkStream(pointer);
    if (stream) {
        gtk_media_stream_pause(stream);
    }
}

/* ---- Web view ----

   WebKitGTK 6 is loaded when the first web view is created, so that it is
   not a build dependency. */

typedef GtkWidget *(*GKWebViewNew)(void);
typedef void (*GKWebViewLoadURI)(GtkWidget *view, const char *uri);
typedef void (*GKWebViewLoadHTML)(GtkWidget *view, const char *content, const char *base_uri);
typedef const char *(*GKWebViewGetURI)(GtkWidget *view);
typedef GObject *(*GKWebViewGetSettings)(GtkWidget *view);
typedef void (*GKSettingsSetJavaScript)(GObject *settings, gboolean enabled);
typedef void (*GKWebViewStop)(GtkWidget *view);

static struct {
    int loaded;
    void *library;
    GKWebViewNew view_new;
    GKWebViewLoadURI load_uri;
    GKWebViewLoadHTML load_html;
    GKWebViewGetURI get_uri;
    GKWebViewGetSettings get_settings;
    GKSettingsSetJavaScript set_javascript;
    GKWebViewStop stop;
} gkWebKit;

static int gkLoadWebKit(void) {
    if (gkWebKit.loaded) {
        return gkWebKit.library != NULL;
    }
    gkWebKit.loaded = 1;
    gkWebKit.library = dlopen("libwebkitgtk-6.0.so.4", RTLD_NOW | RTLD_GLOBAL);
    if (!gkWebKit.library) {
        return 0;
    }
    gkWebKit.view_new = (GKWebViewNew)dlsym(gkWebKit.library, "webkit_web_view_new");
    gkWebKit.load_uri = (GKWebViewLoadURI)dlsym(gkWebKit.library, "webkit_web_view_load_uri");
    gkWebKit.load_html = (GKWebViewLoadHTML)dlsym(gkWebKit.library, "webkit_web_view_load_html");
    gkWebKit.get_uri = (GKWebViewGetURI)dlsym(gkWebKit.library, "webkit_web_view_get_uri");
    gkWebKit.get_settings = (GKWebViewGetSettings)dlsym(gkWebKit.library, "webkit_web_view_get_settings");
    gkWebKit.set_javascript = (GKSettingsSetJavaScript)dlsym(gkWebKit.library, "webkit_settings_set_enable_javascript");
    gkWebKit.stop = (GKWebViewStop)dlsym(gkWebKit.library, "webkit_web_view_stop_loading");
    if (!gkWebKit.view_new || !gkWebKit.load_uri || !gkWebKit.load_html || !gkWebKit.get_uri) {
        dlclose(gkWebKit.library);
        gkWebKit.library = NULL;
        return 0;
    }
    return 1;
}

void *gkGTKWebViewCreate(uintptr_t window, int scripts) {
    if (!gkLoadWebKit()) {
        return NULL;
    }
    GtkWidget *web = gkWebKit.view_new();
    if (!web) {
        return NULL;
    }
    if (gkWebKit.get_settings && gkWebKit.set_javascript) {
        gkWebKit.set_javascript(gkWebKit.get_settings(web), scripts != 0);
    }
    gkContent(window, web, NULL, GK_KIND_WEB);
    return web;
}

int gkGTKWebViewLoad(void *pointer, const char *url) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_WEB || !url) {
        return 0;
    }
    char *scheme = g_uri_parse_scheme(url);
    if (!scheme) {
        return 0;
    }
    g_free(scheme);
    gkWebKit.load_uri(view->outer, url);
    return 1;
}

void gkGTKWebViewLoadHTML(void *pointer, const char *html) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_WEB) {
        gkWebKit.load_html(view->outer, html ? html : "", NULL);
    }
}

char *gkGTKWebViewURL(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_WEB) {
        return gkStrdup("");
    }
    return gkStrdup(gkWebKit.get_uri(view->outer));
}

void gkGTKWebViewRelease(void *pointer) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_WEB && gkWebKit.stop) {
        gkWebKit.stop(view->outer);
    }
}
