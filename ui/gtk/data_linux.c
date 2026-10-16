//go:build linux && cgo

#include "gtk_internal.h"

#include <stdio.h>

/* ---- Row and node objects ---- */

#define GK_TYPE_ROW (gk_row_get_type())
G_DECLARE_FINAL_TYPE(GKRow, gk_row, GK, ROW, GObject)

struct _GKRow {
    GObject parent_instance;
    char *id;
    char **cells;
    int count;
    char *detail;
    GdkPaintable *icon;
};

G_DEFINE_FINAL_TYPE(GKRow, gk_row, G_TYPE_OBJECT)

static void gk_row_finalize(GObject *object) {
    GKRow *row = GK_ROW(object);
    g_free(row->id);
    g_strfreev(row->cells);
    g_free(row->detail);
    if (row->icon) {
        g_object_unref(row->icon);
    }
    G_OBJECT_CLASS(gk_row_parent_class)->finalize(object);
}

static void gk_row_class_init(GKRowClass *klass) {
    G_OBJECT_CLASS(klass)->finalize = gk_row_finalize;
}

static void gk_row_init(GKRow *row) {
    (void)row;
}

#define GK_TYPE_NODE (gk_node_get_type())
G_DECLARE_FINAL_TYPE(GKNode, gk_node, GK, NODE, GObject)

struct _GKNode {
    GObject parent_instance;
    char *id;
    char *text;
    char *badge;
    GdkPaintable *icon;
    int has_children;
    int group;
    GListStore *children;
    int requested;
};

G_DEFINE_FINAL_TYPE(GKNode, gk_node, G_TYPE_OBJECT)

static void gk_node_finalize(GObject *object) {
    GKNode *node = GK_NODE(object);
    g_free(node->id);
    g_free(node->text);
    g_free(node->badge);
    if (node->icon) {
        g_object_unref(node->icon);
    }
    g_object_unref(node->children);
    G_OBJECT_CLASS(gk_node_parent_class)->finalize(object);
}

static void gk_node_class_init(GKNodeClass *klass) {
    G_OBJECT_CLASS(klass)->finalize = gk_node_finalize;
}

static void gk_node_init(GKNode *node) {
    node->children = g_list_store_new(GK_TYPE_NODE);
}

static GKView *gkData(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind) {
    GKView *view = gkViewNew(outer, control, kind, window, 0);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, outer);
    }
    return view;
}

static void gkDataReport(GKView *view, int kind, int a, const char *text) {
    if (view && !view->suppress) {
        gkSend(view->handle, kind, a, 0, 0, 0, text);
    }
}

static char *gkPackIDs(GPtrArray *ids, size_t *size) {
    if (!ids->len) {
        *size = 0;
        return NULL;
    }
    return gkJoin((char **)ids->pdata, (int)ids->len, size);
}

static GtkSelectionModel *gkSelectionFor(GListModel *model, int selection) {
    switch (selection) {
    case 0:
        return GTK_SELECTION_MODEL(gtk_no_selection_new(model));
    case 2:
        return GTK_SELECTION_MODEL(gtk_multi_selection_new(model));
    default: {
        GtkSingleSelection *single = gtk_single_selection_new(model);
        gtk_single_selection_set_autoselect(single, FALSE);
        gtk_single_selection_set_can_unselect(single, TRUE);
        gtk_single_selection_set_selected(single, GTK_INVALID_LIST_POSITION);
        return GTK_SELECTION_MODEL(single);
    }
    }
}

static void gkSelectPositions(GKView *view, GtkSelectionModel *model, GtkBitset *positions) {
    view->suppress++;
    if (GTK_IS_SINGLE_SELECTION(model)) {
        guint position = gtk_bitset_is_empty(positions) ? GTK_INVALID_LIST_POSITION : gtk_bitset_get_minimum(positions);
        gtk_single_selection_set_selected(GTK_SINGLE_SELECTION(model), position);
    } else if (GTK_IS_MULTI_SELECTION(model)) {
        GtkBitset *mask = gtk_bitset_new_range(0, g_list_model_get_n_items(G_LIST_MODEL(model)));
        gtk_selection_model_set_selection(model, positions, mask);
        gtk_bitset_unref(mask);
    }
    view->suppress--;
}

/* ---- Tables and lists ---- */

typedef struct {
    GListStore *rows;
    GtkSelectionModel *selection;
    GtkWidget *list;
    char **columns;
    int column_count;
    int is_list;
} GKTable;

static void gkTableFree(gpointer data) {
    GKTable *table = data;
    g_object_unref(table->rows);
    g_object_unref(table->selection);
    g_strfreev(table->columns);
    g_free(table);
}

static void gkTableSelectionChanged(GtkSelectionModel *model, guint position, guint count, gpointer data) {
    (void)model;
    (void)position;
    (void)count;
    gkDataReport(data, GK_EVENT_SELECTION, 0, NULL);
}

static void gkTableActivated(GtkWidget *widget, guint position, gpointer data) {
    (void)widget;
    GKView *view = data;
    GKTable *table = view->ext;
    GKRow *row = g_list_model_get_item(G_LIST_MODEL(table->rows), position);
    if (row) {
        gkSend(view->handle, GK_EVENT_ACTIVATE, 0, 0, 0, 0, row->id);
        g_object_unref(row);
    }
}

static void gkListSetup(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    GKView *view = data;
    GKTable *table = view->ext;
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    GtkWidget *image = gtk_image_new();
    gtk_image_set_pixel_size(GTK_IMAGE(image), 16);
    GtkWidget *text = gtk_label_new("");
    gtk_label_set_xalign(GTK_LABEL(text), 0);
    gtk_label_set_ellipsize(GTK_LABEL(text), PANGO_ELLIPSIZE_END);
    GtkWidget *detail = gtk_label_new("");
    gtk_widget_add_css_class(detail, "dim-label");
    gtk_label_set_xalign(GTK_LABEL(detail), 0);
    gtk_widget_set_hexpand(detail, TRUE);
    gtk_box_append(GTK_BOX(box), image);
    gtk_box_append(GTK_BOX(box), text);
    gtk_box_append(GTK_BOX(box), detail);
    gtk_widget_set_visible(image, table->is_list);
    gtk_widget_set_visible(detail, table->is_list);
    gtk_list_item_set_child(item, box);
}

static void gkListBind(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    GtkWidget *box = gtk_list_item_get_child(item);
    GKRow *row = gtk_list_item_get_item(item);
    int column = GPOINTER_TO_INT(data);
    GtkWidget *image = gtk_widget_get_first_child(box);
    GtkWidget *text = gtk_widget_get_next_sibling(image);
    GtkWidget *detail = gtk_widget_get_next_sibling(text);
    gtk_label_set_text(GTK_LABEL(text), row && column < row->count ? row->cells[column] : "");
    gtk_label_set_text(GTK_LABEL(detail), row && row->detail ? row->detail : "");
    if (row && row->icon) {
        gtk_image_set_from_paintable(GTK_IMAGE(image), row->icon);
    } else {
        gtk_image_clear(GTK_IMAGE(image));
    }
    gtk_widget_set_visible(detail, row && row->detail && *row->detail);
}

static void gkCellBind(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    gkListBind(factory, item, data);
}

static int gkEqualSort(gconstpointer a, gconstpointer b, gpointer data) {
    (void)a;
    (void)b;
    (void)data;
    return GTK_ORDERING_EQUAL;
}

static void gkSorterChanged(GtkSorter *sorter, GtkSorterChange change, gpointer data) {
    (void)change;
    GKView *view = data;
    GtkColumnViewColumn *column = gtk_column_view_sorter_get_primary_sort_column(GTK_COLUMN_VIEW_SORTER(sorter));
    if (!column) {
        return;
    }
    GtkSortType order = gtk_column_view_sorter_get_primary_sort_order(GTK_COLUMN_VIEW_SORTER(sorter));
    const char *identifier = gtk_column_view_column_get_id(column);
    gkDataReport(view, GK_EVENT_SORT, order == GTK_SORT_ASCENDING, identifier);
}

void *gkGTKTableCreate(uintptr_t window, int list, const char *column_ids, size_t column_ids_size,
                       const char *titles, size_t titles_size, const int *widths, const int *sortable,
                       int selection) {
    GKTable *table = g_new0(GKTable, 1);
    table->is_list = list != 0;
    table->rows = g_list_store_new(GK_TYPE_ROW);
    table->columns = gkSplit(column_ids, column_ids_size, &table->column_count);
    g_object_ref(table->rows);
    table->selection = gkSelectionFor(G_LIST_MODEL(table->rows), selection);
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_has_frame(GTK_SCROLLED_WINDOW(scroll), TRUE);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_AUTOMATIC, GTK_POLICY_AUTOMATIC);
    GKView *view = gkData(window, scroll, NULL, GK_KIND_TABLE);
    view->ext = table;
    view->ext_free = gkTableFree;
    if (list) {
        GtkListItemFactory *factory = gtk_signal_list_item_factory_new();
        g_signal_connect(factory, "setup", G_CALLBACK(gkListSetup), view);
        g_signal_connect(factory, "bind", G_CALLBACK(gkListBind), GINT_TO_POINTER(0));
        table->list = gtk_list_view_new(g_object_ref(table->selection), factory);
        gtk_list_view_set_single_click_activate(GTK_LIST_VIEW(table->list), FALSE);
        g_signal_connect(table->list, "activate", G_CALLBACK(gkTableActivated), view);
    } else {
        table->list = gtk_column_view_new(g_object_ref(table->selection));
        gtk_column_view_set_show_row_separators(GTK_COLUMN_VIEW(table->list), FALSE);
        gtk_column_view_set_reorderable(GTK_COLUMN_VIEW(table->list), FALSE);
        int title_count = 0;
        char **names = gkSplit(titles, titles_size, &title_count);
        for (int i = 0; i < table->column_count; i++) {
            GtkListItemFactory *factory = gtk_signal_list_item_factory_new();
            g_signal_connect(factory, "setup", G_CALLBACK(gkListSetup), view);
            g_signal_connect(factory, "bind", G_CALLBACK(gkCellBind), GINT_TO_POINTER(i));
            GtkColumnViewColumn *column =
                gtk_column_view_column_new(i < title_count ? names[i] : "", factory);
            gtk_column_view_column_set_id(column, table->columns[i]);
            gtk_column_view_column_set_resizable(column, TRUE);
            if (widths[i] > 0) {
                gtk_column_view_column_set_fixed_width(column, widths[i]);
            } else {
                gtk_column_view_column_set_expand(column, TRUE);
            }
            if (sortable[i]) {
                GtkSorter *sorter = GTK_SORTER(gtk_custom_sorter_new(gkEqualSort, NULL, NULL));
                gtk_column_view_column_set_sorter(column, sorter);
                g_object_unref(sorter);
            }
            gtk_column_view_append_column(GTK_COLUMN_VIEW(table->list), column);
            g_object_unref(column);
        }
        g_strfreev(names);
        g_signal_connect(gtk_column_view_get_sorter(GTK_COLUMN_VIEW(table->list)), "changed",
                         G_CALLBACK(gkSorterChanged), view);
        g_signal_connect(table->list, "activate", G_CALLBACK(gkTableActivated), view);
    }
    view->control = table->list;
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), table->list);
    g_signal_connect(table->selection, "selection-changed", G_CALLBACK(gkTableSelectionChanged), view);
    return scroll;
}

static GKTable *gkTable(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_TABLE ? view->ext : NULL;
}

void gkGTKTableSetRows(void *pointer, const char *ids, size_t ids_size, const char *cells, size_t cells_size,
                       const char *details, size_t details_size, void **icons, int columns) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTable(pointer);
    if (!table) {
        return;
    }
    int row_count = 0, cell_count = 0, detail_count = 0;
    char **row_ids = gkSplit(ids, ids_size, &row_count);
    char **values = gkSplit(cells, cells_size, &cell_count);
    char **row_details = gkSplit(details, details_size, &detail_count);
    GPtrArray *rows = g_ptr_array_new_with_free_func(g_object_unref);
    for (int i = 0; i < row_count; i++) {
        GKRow *row = g_object_new(GK_TYPE_ROW, NULL);
        row->id = g_strdup(row_ids[i]);
        row->count = MAX(1, columns);
        row->cells = g_new0(char *, row->count + 1);
        for (int column = 0; column < row->count; column++) {
            int index = i * row->count + column;
            row->cells[column] = g_strdup(index < cell_count ? values[index] : "");
        }
        row->detail = g_strdup(i < detail_count ? row_details[i] : "");
        row->icon = icons && icons[i] ? g_object_ref(GDK_PAINTABLE(icons[i])) : NULL;
        g_ptr_array_add(rows, row);
    }
    view->suppress++;
    g_list_store_splice(table->rows, 0, g_list_model_get_n_items(G_LIST_MODEL(table->rows)), rows->pdata, rows->len);
    view->suppress--;
    g_ptr_array_free(rows, TRUE);
    g_strfreev(row_ids);
    g_strfreev(values);
    g_strfreev(row_details);
}

char *gkGTKTableSelection(void *pointer, size_t *size) {
    GKTable *table = gkTable(pointer);
    *size = 0;
    if (!table) {
        return NULL;
    }
    GtkBitset *selected = gtk_selection_model_get_selection(table->selection);
    GPtrArray *ids = g_ptr_array_new_with_free_func(g_free);
    GtkBitsetIter iter;
    guint position;
    for (gboolean valid = gtk_bitset_iter_init_first(&iter, selected, &position); valid;
         valid = gtk_bitset_iter_next(&iter, &position)) {
        GKRow *row = g_list_model_get_item(G_LIST_MODEL(table->rows), position);
        if (row) {
            g_ptr_array_add(ids, g_strdup(row->id));
            g_object_unref(row);
        }
    }
    gtk_bitset_unref(selected);
    char *result = gkPackIDs(ids, size);
    g_ptr_array_free(ids, TRUE);
    return result;
}

void gkGTKTableSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKTable *table = gkTable(pointer);
    if (!table) {
        return;
    }
    int count = 0;
    char **wanted = gkSplit(ids, size, &count);
    GtkBitset *positions = gtk_bitset_new_empty();
    guint total = g_list_model_get_n_items(G_LIST_MODEL(table->rows));
    for (guint i = 0; i < total; i++) {
        GKRow *row = g_list_model_get_item(G_LIST_MODEL(table->rows), i);
        if (row && g_strv_contains((const char *const *)wanted, row->id)) {
            gtk_bitset_add(positions, i);
        }
        if (row) {
            g_object_unref(row);
        }
    }
    gkSelectPositions(view, table->selection, positions);
    gtk_bitset_unref(positions);
    g_strfreev(wanted);
}

int gkGTKTableScrollTo(void *pointer, const char *identifier) {
    GKTable *table = gkTable(pointer);
    if (!table) {
        return 0;
    }
    guint total = g_list_model_get_n_items(G_LIST_MODEL(table->rows));
    for (guint i = 0; i < total; i++) {
        GKRow *row = g_list_model_get_item(G_LIST_MODEL(table->rows), i);
        int match = row && strcmp(row->id, identifier ? identifier : "") == 0;
        if (row) {
            g_object_unref(row);
        }
        if (match) {
            if (GTK_IS_LIST_VIEW(table->list)) {
                gtk_list_view_scroll_to(GTK_LIST_VIEW(table->list), i, GTK_LIST_SCROLL_NONE, NULL);
            } else {
                gtk_column_view_scroll_to(GTK_COLUMN_VIEW(table->list), i, NULL, GTK_LIST_SCROLL_NONE, NULL);
            }
            return 1;
        }
    }
    return 0;
}

/* ---- Outlines: trees and navigation lists ---- */

typedef struct {
    GListStore *roots;
    GtkTreeListModel *tree;
    GtkSelectionModel *selection;
    GtkWidget *list;
    int navigation;
    GKView *view;
} GKOutline;

static void gkOutlineFree(gpointer data) {
    GKOutline *outline = data;
    g_object_unref(outline->selection);
    g_object_unref(outline->tree);
    g_object_unref(outline->roots);
    g_free(outline);
}

static GListModel *gkOutlineChildren(gpointer item, gpointer data) {
    GKOutline *outline = data;
    GKNode *node = item;
    if (!node->has_children && g_list_model_get_n_items(G_LIST_MODEL(node->children)) == 0) {
        return NULL;
    }
    if (node->has_children && g_list_model_get_n_items(G_LIST_MODEL(node->children)) == 0 && !node->requested) {
        node->requested = 1;
        gkDataReport(outline->view, GK_EVENT_EXPAND, 1, node->id);
    }
    return G_LIST_MODEL(g_object_ref(node->children));
}

static void gkOutlineSetup(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    (void)data;
    GtkWidget *expander = gtk_tree_expander_new();
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    GtkWidget *image = gtk_image_new();
    gtk_image_set_pixel_size(GTK_IMAGE(image), 16);
    GtkWidget *text = gtk_label_new("");
    gtk_label_set_xalign(GTK_LABEL(text), 0);
    gtk_label_set_ellipsize(GTK_LABEL(text), PANGO_ELLIPSIZE_END);
    gtk_widget_set_hexpand(text, TRUE);
    GtkWidget *badge = gtk_label_new("");
    gtk_widget_add_css_class(badge, "dim-label");
    gtk_box_append(GTK_BOX(box), image);
    gtk_box_append(GTK_BOX(box), text);
    gtk_box_append(GTK_BOX(box), badge);
    gtk_tree_expander_set_child(GTK_TREE_EXPANDER(expander), box);
    gtk_list_item_set_child(item, expander);
}

static void gkOutlineBind(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    GKOutline *outline = data;
    GtkTreeListRow *row = gtk_list_item_get_item(item);
    GKNode *node = gtk_tree_list_row_get_item(row);
    GtkWidget *expander = gtk_list_item_get_child(item);
    gtk_tree_expander_set_list_row(GTK_TREE_EXPANDER(expander), row);
    GtkWidget *box = gtk_tree_expander_get_child(GTK_TREE_EXPANDER(expander));
    GtkWidget *image = gtk_widget_get_first_child(box);
    GtkWidget *text = gtk_widget_get_next_sibling(image);
    GtkWidget *badge = gtk_widget_get_next_sibling(text);
    gtk_label_set_text(GTK_LABEL(text), node->text ? node->text : "");
    gtk_label_set_text(GTK_LABEL(badge), node->badge ? node->badge : "");
    gtk_widget_set_visible(badge, node->badge && *node->badge);
    if (node->icon && !node->group) {
        gtk_image_set_from_paintable(GTK_IMAGE(image), node->icon);
        gtk_widget_set_visible(image, TRUE);
    } else {
        gtk_image_clear(GTK_IMAGE(image));
        gtk_widget_set_visible(image, FALSE);
    }
    if (node->group) {
        gtk_widget_add_css_class(text, "heading");
        gtk_widget_add_css_class(text, "dim-label");
    } else {
        gtk_widget_remove_css_class(text, "heading");
        gtk_widget_remove_css_class(text, "dim-label");
    }
    gtk_list_item_set_selectable(item, !node->group);
    gtk_tree_expander_set_hide_expander(GTK_TREE_EXPANDER(expander), outline->navigation && node->group);
    g_object_unref(node);
}

void *gkGTKOutlineCreate(uintptr_t window, int navigation, int multiple) {
    GKOutline *outline = g_new0(GKOutline, 1);
    outline->navigation = navigation != 0;
    outline->roots = g_list_store_new(GK_TYPE_NODE);
    outline->tree = gtk_tree_list_model_new(G_LIST_MODEL(g_object_ref(outline->roots)), FALSE, navigation != 0,
                                            gkOutlineChildren, outline, NULL);
    g_object_ref(outline->tree);
    outline->selection = gkSelectionFor(G_LIST_MODEL(outline->tree), multiple ? 2 : 1);
    GtkListItemFactory *factory = gtk_signal_list_item_factory_new();
    g_signal_connect(factory, "setup", G_CALLBACK(gkOutlineSetup), outline);
    g_signal_connect(factory, "bind", G_CALLBACK(gkOutlineBind), outline);
    outline->list = gtk_list_view_new(g_object_ref(outline->selection), factory);
    if (navigation) {
        gtk_widget_add_css_class(outline->list, "navigation-sidebar");
    }
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_has_frame(GTK_SCROLLED_WINDOW(scroll), !navigation);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), outline->list);
    GKView *view = gkData(window, scroll, outline->list, GK_KIND_OUTLINE);
    outline->view = view;
    view->ext = outline;
    view->ext_free = gkOutlineFree;
    g_signal_connect(outline->selection, "selection-changed", G_CALLBACK(gkTableSelectionChanged), view);
    return scroll;
}

static GKOutline *gkOutline(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_OUTLINE ? view->ext : NULL;
}

static GKNode *gkFindNode(GListModel *nodes, const char *identifier) {
    guint count = g_list_model_get_n_items(nodes);
    for (guint i = 0; i < count; i++) {
        GKNode *node = g_list_model_get_item(nodes, i);
        if (strcmp(node->id, identifier) == 0) {
            return node;
        }
        GKNode *found = gkFindNode(G_LIST_MODEL(node->children), identifier);
        g_object_unref(node);
        if (found) {
            return found;
        }
    }
    return NULL;
}

/* gkNodePath returns the IDs from a root to identifier, or NULL. */
static gboolean gkNodePath(GListModel *nodes, const char *identifier, GPtrArray *path) {
    guint count = g_list_model_get_n_items(nodes);
    for (guint i = 0; i < count; i++) {
        GKNode *node = g_list_model_get_item(nodes, i);
        g_ptr_array_add(path, node);
        if (strcmp(node->id, identifier) == 0 || gkNodePath(G_LIST_MODEL(node->children), identifier, path)) {
            return TRUE;
        }
        g_ptr_array_remove_index(path, path->len - 1);
    }
    return FALSE;
}

static GtkTreeListRow *gkOutlineRow(GKOutline *outline, GKNode *target, guint *position) {
    guint count = g_list_model_get_n_items(G_LIST_MODEL(outline->tree));
    for (guint i = 0; i < count; i++) {
        GtkTreeListRow *row = gtk_tree_list_model_get_row(outline->tree, i);
        if (!row) {
            continue;
        }
        GKNode *node = gtk_tree_list_row_get_item(row);
        int match = node == target;
        g_object_unref(node);
        if (match) {
            if (position) {
                *position = i;
            }
            return row;
        }
        g_object_unref(row);
    }
    return NULL;
}

void gkGTKOutlineSetNodes(void *pointer, const char *parent, const int *depths, const int *flags, const char *ids,
                          size_t ids_size, const char *texts, size_t texts_size, const char *badges,
                          size_t badges_size, void **icons, int count) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutline(pointer);
    if (!outline) {
        return;
    }
    int id_count = 0, text_count = 0, badge_count = 0;
    char **node_ids = gkSplit(ids, ids_size, &id_count);
    char **node_texts = gkSplit(texts, texts_size, &text_count);
    char **node_badges = gkSplit(badges, badges_size, &badge_count);
    GListStore *roots = g_list_store_new(GK_TYPE_NODE);
    GPtrArray *stack = g_ptr_array_new();
    for (int i = 0; i < count; i++) {
        GKNode *node = g_object_new(GK_TYPE_NODE, NULL);
        node->id = g_strdup(i < id_count ? node_ids[i] : "");
        node->text = g_strdup(i < text_count ? node_texts[i] : "");
        node->badge = g_strdup(i < badge_count ? node_badges[i] : "");
        node->icon = icons && icons[i] ? g_object_ref(GDK_PAINTABLE(icons[i])) : NULL;
        node->has_children = (flags[i] & 1) != 0;
        node->group = (flags[i] & 2) != 0;
        while ((int)stack->len > depths[i]) {
            g_ptr_array_remove_index(stack, stack->len - 1);
        }
        if (stack->len == 0) {
            g_list_store_append(roots, node);
        } else {
            g_list_store_append(((GKNode *)g_ptr_array_index(stack, stack->len - 1))->children, node);
        }
        g_ptr_array_add(stack, node);
        g_object_unref(node);
    }
    g_ptr_array_free(stack, TRUE);
    view->suppress++;
    GListStore *target = outline->roots;
    GKNode *owner = parent ? gkFindNode(G_LIST_MODEL(outline->roots), parent) : NULL;
    if (parent) {
        target = owner ? owner->children : NULL;
    }
    if (target) {
        guint total = g_list_model_get_n_items(G_LIST_MODEL(roots));
        GPtrArray *items = g_ptr_array_new_with_free_func(g_object_unref);
        for (guint i = 0; i < total; i++) {
            g_ptr_array_add(items, g_list_model_get_item(G_LIST_MODEL(roots), i));
        }
        if (owner) {
            owner->has_children = total > 0;
            owner->requested = 1;
        }
        g_list_store_splice(target, 0, g_list_model_get_n_items(G_LIST_MODEL(target)), items->pdata, items->len);
        g_ptr_array_free(items, TRUE);
    }
    view->suppress--;
    if (owner) {
        g_object_unref(owner);
    }
    g_object_unref(roots);
    g_strfreev(node_ids);
    g_strfreev(node_texts);
    g_strfreev(node_badges);
}

char *gkGTKOutlineSelection(void *pointer, size_t *size) {
    GKOutline *outline = gkOutline(pointer);
    *size = 0;
    if (!outline) {
        return NULL;
    }
    GtkBitset *selected = gtk_selection_model_get_selection(outline->selection);
    GPtrArray *ids = g_ptr_array_new_with_free_func(g_free);
    GtkBitsetIter iter;
    guint position;
    for (gboolean valid = gtk_bitset_iter_init_first(&iter, selected, &position); valid;
         valid = gtk_bitset_iter_next(&iter, &position)) {
        GtkTreeListRow *row = gtk_tree_list_model_get_row(outline->tree, position);
        if (row) {
            GKNode *node = gtk_tree_list_row_get_item(row);
            g_ptr_array_add(ids, g_strdup(node->id));
            g_object_unref(node);
            g_object_unref(row);
        }
    }
    gtk_bitset_unref(selected);
    char *result = gkPackIDs(ids, size);
    g_ptr_array_free(ids, TRUE);
    return result;
}

static GtkTreeListRow *gkOutlineReveal(GKOutline *outline, const char *identifier, guint *position) {
    GPtrArray *path = g_ptr_array_new_with_free_func(g_object_unref);
    GtkTreeListRow *result = NULL;
    if (gkNodePath(G_LIST_MODEL(outline->roots), identifier, path)) {
        for (guint i = 0; i + 1 < path->len; i++) {
            GtkTreeListRow *row = gkOutlineRow(outline, g_ptr_array_index(path, i), NULL);
            if (row) {
                gtk_tree_list_row_set_expanded(row, TRUE);
                g_object_unref(row);
            }
        }
        result = gkOutlineRow(outline, g_ptr_array_index(path, path->len - 1), position);
    }
    g_ptr_array_free(path, TRUE);
    return result;
}

void gkGTKOutlineSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutline(pointer);
    if (!outline) {
        return;
    }
    int count = 0;
    char **wanted = gkSplit(ids, size, &count);
    GtkBitset *positions = gtk_bitset_new_empty();
    view->suppress++;
    for (int i = 0; i < count; i++) {
        guint position = 0;
        GtkTreeListRow *row = gkOutlineReveal(outline, wanted[i], &position);
        if (row) {
            gtk_bitset_add(positions, position);
            g_object_unref(row);
        }
    }
    view->suppress--;
    gkSelectPositions(view, outline->selection, positions);
    gtk_bitset_unref(positions);
    g_strfreev(wanted);
}

int gkGTKOutlineExpanded(void *pointer, const char *identifier) {
    GKOutline *outline = gkOutline(pointer);
    if (!outline) {
        return 0;
    }
    GKNode *node = gkFindNode(G_LIST_MODEL(outline->roots), identifier ? identifier : "");
    if (!node) {
        return 0;
    }
    GtkTreeListRow *row = gkOutlineRow(outline, node, NULL);
    g_object_unref(node);
    int expanded = row && gtk_tree_list_row_get_expanded(row);
    if (row) {
        g_object_unref(row);
    }
    return expanded;
}

int gkGTKOutlineSetExpanded(void *pointer, const char *identifier, int expanded) {
    GKView *view = gkView(pointer);
    GKOutline *outline = gkOutline(pointer);
    if (!outline) {
        return 0;
    }
    GKNode *node = gkFindNode(G_LIST_MODEL(outline->roots), identifier ? identifier : "");
    if (!node) {
        return 0;
    }
    g_object_unref(node);
    GtkTreeListRow *row = gkOutlineReveal(outline, identifier, NULL);
    if (row) {
        gtk_tree_list_row_set_expanded(row, expanded != 0);
        g_object_unref(row);
    }
    (void)view;
    return 1;
}

/* ---- Collection view ---- */

typedef struct {
    GListStore *items;
    GtkSelectionModel *selection;
    int width, height;
} GKCollection;

static void gkCollectionFree(gpointer data) {
    GKCollection *collection = data;
    g_object_unref(collection->items);
    g_object_unref(collection->selection);
    g_free(collection);
}

static void gkCollectionSetup(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    GKCollection *collection = data;
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    gtk_widget_set_size_request(box, collection->width, collection->height);
    GtkWidget *picture = gtk_picture_new();
    gtk_picture_set_content_fit(GTK_PICTURE(picture), GTK_CONTENT_FIT_CONTAIN);
    gtk_widget_set_vexpand(picture, TRUE);
    GtkWidget *text = gtk_label_new("");
    gtk_label_set_ellipsize(GTK_LABEL(text), PANGO_ELLIPSIZE_END);
    gtk_box_append(GTK_BOX(box), picture);
    gtk_box_append(GTK_BOX(box), text);
    gtk_list_item_set_child(item, box);
}

static void gkCollectionBind(GtkSignalListItemFactory *factory, GtkListItem *item, gpointer data) {
    (void)factory;
    (void)data;
    GKRow *row = gtk_list_item_get_item(item);
    GtkWidget *box = gtk_list_item_get_child(item);
    GtkWidget *picture = gtk_widget_get_first_child(box);
    GtkWidget *text = gtk_widget_get_next_sibling(picture);
    gtk_label_set_text(GTK_LABEL(text), row && row->count > 0 ? row->cells[0] : "");
    gtk_picture_set_paintable(GTK_PICTURE(picture), row ? row->icon : NULL);
}

void *gkGTKCollectionCreate(uintptr_t window, int width, int height, int selection) {
    GKCollection *collection = g_new0(GKCollection, 1);
    collection->width = width > 0 ? width : 96;
    collection->height = height > 0 ? height : 96;
    collection->items = g_list_store_new(GK_TYPE_ROW);
    g_object_ref(collection->items);
    collection->selection = gkSelectionFor(G_LIST_MODEL(collection->items), selection);
    GtkListItemFactory *factory = gtk_signal_list_item_factory_new();
    g_signal_connect(factory, "setup", G_CALLBACK(gkCollectionSetup), collection);
    g_signal_connect(factory, "bind", G_CALLBACK(gkCollectionBind), collection);
    GtkWidget *grid = gtk_grid_view_new(g_object_ref(collection->selection), factory);
    gtk_grid_view_set_max_columns(GTK_GRID_VIEW(grid), 64);
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), grid);
    GKView *view = gkData(window, scroll, grid, GK_KIND_COLLECTION);
    view->ext = collection;
    view->ext_free = gkCollectionFree;
    g_signal_connect(collection->selection, "selection-changed", G_CALLBACK(gkTableSelectionChanged), view);
    return scroll;
}

static GKCollection *gkCollection(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->kind == GK_KIND_COLLECTION ? view->ext : NULL;
}

void gkGTKCollectionSetItems(void *pointer, const char *ids, size_t ids_size, const char *texts, size_t texts_size,
                             void **images, int count) {
    GKView *view = gkView(pointer);
    GKCollection *collection = gkCollection(pointer);
    if (!collection) {
        return;
    }
    int id_count = 0, text_count = 0;
    char **item_ids = gkSplit(ids, ids_size, &id_count);
    char **item_texts = gkSplit(texts, texts_size, &text_count);
    GPtrArray *rows = g_ptr_array_new_with_free_func(g_object_unref);
    for (int i = 0; i < count; i++) {
        GKRow *row = g_object_new(GK_TYPE_ROW, NULL);
        row->id = g_strdup(i < id_count ? item_ids[i] : "");
        row->count = 1;
        row->cells = g_new0(char *, 2);
        row->cells[0] = g_strdup(i < text_count ? item_texts[i] : "");
        row->icon = images && images[i] ? g_object_ref(GDK_PAINTABLE(images[i])) : NULL;
        g_ptr_array_add(rows, row);
    }
    view->suppress++;
    g_list_store_splice(collection->items, 0, g_list_model_get_n_items(G_LIST_MODEL(collection->items)), rows->pdata,
                        rows->len);
    view->suppress--;
    g_ptr_array_free(rows, TRUE);
    g_strfreev(item_ids);
    g_strfreev(item_texts);
}

char *gkGTKCollectionSelection(void *pointer, size_t *size) {
    GKCollection *collection = gkCollection(pointer);
    *size = 0;
    if (!collection) {
        return NULL;
    }
    GtkBitset *selected = gtk_selection_model_get_selection(collection->selection);
    GPtrArray *ids = g_ptr_array_new_with_free_func(g_free);
    GtkBitsetIter iter;
    guint position;
    for (gboolean valid = gtk_bitset_iter_init_first(&iter, selected, &position); valid;
         valid = gtk_bitset_iter_next(&iter, &position)) {
        GKRow *row = g_list_model_get_item(G_LIST_MODEL(collection->items), position);
        if (row) {
            g_ptr_array_add(ids, g_strdup(row->id));
            g_object_unref(row);
        }
    }
    gtk_bitset_unref(selected);
    char *result = gkPackIDs(ids, size);
    g_ptr_array_free(ids, TRUE);
    return result;
}

void gkGTKCollectionSelect(void *pointer, const char *ids, size_t size) {
    GKView *view = gkView(pointer);
    GKCollection *collection = gkCollection(pointer);
    if (!collection) {
        return;
    }
    int count = 0;
    char **wanted = gkSplit(ids, size, &count);
    GtkBitset *positions = gtk_bitset_new_empty();
    guint total = g_list_model_get_n_items(G_LIST_MODEL(collection->items));
    for (guint i = 0; i < total; i++) {
        GKRow *row = g_list_model_get_item(G_LIST_MODEL(collection->items), i);
        if (row && g_strv_contains((const char *const *)wanted, row->id)) {
            gtk_bitset_add(positions, i);
        }
        if (row) {
            g_object_unref(row);
        }
    }
    gkSelectPositions(view, collection->selection, positions);
    gtk_bitset_unref(positions);
    g_strfreev(wanted);
}

/* ---- Breadcrumbs ---- */

static void gkCrumbClicked(GtkButton *button, gpointer data) {
    int index = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(button), "gk-index"));
    gkDataReport(data, GK_EVENT_ACTIVATE, index, NULL);
}

void *gkGTKBreadcrumbsCreate(uintptr_t window) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 2,
                                  "accessible-role", GTK_ACCESSIBLE_ROLE_NAVIGATION, NULL);
    gtk_accessible_update_property(GTK_ACCESSIBLE(box), GTK_ACCESSIBLE_PROPERTY_LABEL, "Breadcrumbs", -1);
    gkData(window, box, NULL, GK_KIND_BREADCRUMBS);
    return box;
}

void gkGTKBreadcrumbsSetItems(void *pointer, const char *titles, size_t size) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_BREADCRUMBS) {
        return;
    }
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(view->outer))) {
        gtk_box_remove(GTK_BOX(view->outer), child);
    }
    int count = 0;
    char **items = gkSplit(titles, size, &count);
    for (int i = 0; i < count; i++) {
        if (i > 0) {
            GtkWidget *separator = gtk_image_new_from_icon_name("go-next-symbolic");
            gtk_widget_add_css_class(separator, "dim-label");
            gtk_box_append(GTK_BOX(view->outer), separator);
        }
        GtkWidget *button = gtk_button_new_with_label(items[i]);
        gtk_widget_add_css_class(button, "flat");
        if (i == count - 1) {
            gtk_accessible_update_state(GTK_ACCESSIBLE(button), GTK_ACCESSIBLE_STATE_SELECTED, TRUE, -1);
        }
        g_object_set_data(G_OBJECT(button), "gk-index", GINT_TO_POINTER(i));
        g_signal_connect(button, "clicked", G_CALLBACK(gkCrumbClicked), view);
        gtk_box_append(GTK_BOX(view->outer), button);
    }
    g_strfreev(items);
}

/* ---- Pagination ---- */

typedef struct {
    GtkWidget *previous;
    GtkWidget *next;
    GtkWidget *label;
    GtkWidget *pages;
    int count;
    int selected;
    int compact;
} GKPagination;

static int gkPaginationStart(GKPagination *pagination) {
    int visible = MIN(pagination->count, 7);
    int start = pagination->selected - visible / 2;
    return MAX(0, MIN(start, pagination->count - visible));
}

static void gkPageClicked(GtkToggleButton *button, gpointer data);

static void gkPaginationUpdate(GKView *view) {
    GKPagination *pagination = view->ext;
    view->suppress++;
    gtk_widget_set_sensitive(pagination->previous, pagination->selected > 0);
    gtk_widget_set_sensitive(pagination->next, pagination->selected + 1 < pagination->count);
    char text[64];
    snprintf(text, sizeof(text), "Page %d of %d", pagination->selected + 1, pagination->count);
    gtk_label_set_text(GTK_LABEL(pagination->label), text);
    gtk_widget_set_visible(pagination->label, pagination->compact);
    gtk_widget_set_visible(pagination->pages, !pagination->compact);
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(pagination->pages))) {
        gtk_box_remove(GTK_BOX(pagination->pages), child);
    }
    if (!pagination->compact) {
        int visible = MIN(pagination->count, 7);
        int start = gkPaginationStart(pagination);
        GtkWidget *first = NULL;
        for (int i = 0; i < visible; i++) {
            char number[16];
            snprintf(number, sizeof(number), "%d", start + i + 1);
            GtkWidget *button = gtk_toggle_button_new_with_label(number);
            if (first) {
                gtk_toggle_button_set_group(GTK_TOGGLE_BUTTON(button), GTK_TOGGLE_BUTTON(first));
            } else {
                first = button;
            }
            gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(button), start + i == pagination->selected);
            g_object_set_data(G_OBJECT(button), "gk-page", GINT_TO_POINTER(start + i));
            g_signal_connect(button, "toggled", G_CALLBACK(gkPageClicked), view);
            gtk_box_append(GTK_BOX(pagination->pages), button);
        }
    }
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, text, -1);
    view->suppress--;
}

static void gkPaginationChoose(GKView *view, int page) {
    GKPagination *pagination = view->ext;
    page = MAX(0, MIN(pagination->count - 1, page));
    if (page == pagination->selected) {
        return;
    }
    pagination->selected = page;
    gkPaginationUpdate(view);
    gkDataReport(view, GK_EVENT_SELECTION, page, NULL);
}

static gboolean gkPaginationIdle(gpointer data) {
    GKView *view = data;
    int page = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(view->outer), "gk-pending"));
    g_object_set_data(G_OBJECT(view->outer), "gk-pending", NULL);
    gkPaginationChoose(view, page - 1);
    g_object_unref(view->outer);
    return G_SOURCE_REMOVE;
}

static void gkPageClicked(GtkToggleButton *button, gpointer data) {
    GKView *view = data;
    if (view->suppress || !gtk_toggle_button_get_active(button)) {
        return;
    }
    /* The buttons are rebuilt after a choice, so choose outside the
       handler. */
    int page = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(button), "gk-page"));
    g_object_set_data(G_OBJECT(view->outer), "gk-pending", GINT_TO_POINTER(page + 1));
    g_idle_add(gkPaginationIdle, view);
    g_object_ref(view->outer);
}

static void gkPagePrevious(GtkButton *button, gpointer data) {
    (void)button;
    GKView *view = data;
    gkPaginationChoose(view, ((GKPagination *)view->ext)->selected - 1);
}

static void gkPageNext(GtkButton *button, gpointer data) {
    (void)button;
    GKView *view = data;
    gkPaginationChoose(view, ((GKPagination *)view->ext)->selected + 1);
}

void *gkGTKPaginationCreate(uintptr_t window, int count, int selected, int compact) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 6,
                                  "accessible-role", GTK_ACCESSIBLE_ROLE_GROUP, NULL);
    gtk_accessible_update_property(GTK_ACCESSIBLE(box), GTK_ACCESSIBLE_PROPERTY_LABEL, "Pagination", -1);
    GKPagination *pagination = g_new0(GKPagination, 1);
    pagination->previous = gtk_button_new_from_icon_name("go-previous-symbolic");
    pagination->next = gtk_button_new_from_icon_name("go-next-symbolic");
    gtk_accessible_update_property(GTK_ACCESSIBLE(pagination->previous), GTK_ACCESSIBLE_PROPERTY_LABEL,
                                   "Previous page", -1);
    gtk_accessible_update_property(GTK_ACCESSIBLE(pagination->next), GTK_ACCESSIBLE_PROPERTY_LABEL, "Next page", -1);
    pagination->label = gtk_label_new("");
    pagination->pages = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
    gtk_widget_add_css_class(pagination->pages, "linked");
    GtkWidget *middle = gtk_center_box_new();
    gtk_widget_set_hexpand(middle, TRUE);
    GtkWidget *center = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
    gtk_box_append(GTK_BOX(center), pagination->label);
    gtk_box_append(GTK_BOX(center), pagination->pages);
    gtk_center_box_set_center_widget(GTK_CENTER_BOX(middle), center);
    gtk_box_append(GTK_BOX(box), pagination->previous);
    gtk_box_append(GTK_BOX(box), middle);
    gtk_box_append(GTK_BOX(box), pagination->next);
    pagination->count = MAX(1, count);
    pagination->selected = MAX(0, MIN(selected, pagination->count - 1));
    pagination->compact = compact != 0;
    GKView *view = gkData(window, box, NULL, GK_KIND_PAGINATION);
    view->ext = pagination;
    view->ext_free = g_free;
    g_signal_connect(pagination->previous, "clicked", G_CALLBACK(gkPagePrevious), view);
    g_signal_connect(pagination->next, "clicked", G_CALLBACK(gkPageNext), view);
    gkPaginationUpdate(view);
    return box;
}

void gkGTKPaginationSet(void *pointer, int count, int selected) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_PAGINATION) {
        return;
    }
    GKPagination *pagination = view->ext;
    pagination->count = MAX(1, count);
    pagination->selected = MAX(0, MIN(selected, pagination->count - 1));
    gkPaginationUpdate(view);
}
