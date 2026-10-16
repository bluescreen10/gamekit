//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>

static GKView *gkContainer(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind) {
    GKView *view = gkViewNew(outer, control, kind, window, 0);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, outer);
    }
    return view;
}

void gkGTKAddChild(void *container, void *child) {
    GKView *parent = gkView(container);
    GKView *view = gkView(child);
    if (!parent || !view) {
        return;
    }
    GtkWidget *target = gkViewContainer(parent);
    if (target && GK_IS_LAYOUT(target)) {
        gkLayoutPut(target, view->outer);
    }
}

void *gkGTKClipViewCreate(uintptr_t window) {
    GtkWidget *layout = gkLayoutNew();
    GKView *view = gkContainer(window, layout, NULL, GK_KIND_CLIP);
    view->body = layout;
    return layout;
}

/* ---- Group box ---- */

typedef struct {
    GtkWidget *title;
    GtkWidget *subtitle;
    GtkWidget *content;
} GKGroupBox;

static void gkGroupBoxArrange(GtkWidget *layout, int width, int height, gpointer data) {
    (void)layout;
    (void)height;
    GKView *view = data;
    GKGroupBox *box = view->ext;
    if (gtk_widget_get_visible(box->subtitle)) {
        int natural = 0;
        gtk_widget_measure(box->subtitle, GTK_ORIENTATION_VERTICAL, MAX(0, width - 8), NULL, &natural, NULL, NULL);
        gkLayoutMove(box->subtitle, 4, 2, MAX(0, width - 8), natural);
    }
}

void *gkGTKGroupBoxCreate(uintptr_t window, const char *title, const char *subtitle) {
    GtkWidget *frame = g_object_new(GTK_TYPE_FRAME, "accessible-role", GTK_ACCESSIBLE_ROLE_GROUP, NULL);
    GKGroupBox *box = g_new0(GKGroupBox, 1);
    box->title = gtk_label_new(title ? title : "");
    gtk_widget_add_css_class(box->title, "heading");
    gtk_frame_set_label_widget(GTK_FRAME(frame), box->title);
    GtkWidget *body = gkLayoutNew();
    box->subtitle = gtk_label_new(subtitle ? subtitle : "");
    gtk_label_set_wrap(GTK_LABEL(box->subtitle), TRUE);
    gtk_label_set_xalign(GTK_LABEL(box->subtitle), 0);
    gtk_widget_add_css_class(box->subtitle, "dim-label");
    gtk_widget_add_css_class(box->subtitle, "caption");
    gtk_widget_set_parent(box->subtitle, body);
    gtk_widget_set_visible(box->subtitle, subtitle && *subtitle);
    gtk_frame_set_child(GTK_FRAME(frame), body);
    GKView *view = gkContainer(window, frame, NULL, GK_KIND_GROUP_BOX);
    view->body = body;
    view->ext = box;
    view->ext_free = g_free;
    gkLayoutOnAllocate(body, gkGroupBoxArrange, view);
    gtk_accessible_update_property(GTK_ACCESSIBLE(frame), GTK_ACCESSIBLE_PROPERTY_LABEL, title ? title : "", -1);
    return frame;
}

void gkGTKGroupBoxSet(void *pointer, const char *title, const char *subtitle) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_GROUP_BOX) {
        return;
    }
    GKGroupBox *box = view->ext;
    gtk_label_set_text(GTK_LABEL(box->title), title ? title : "");
    gtk_label_set_text(GTK_LABEL(box->subtitle), subtitle ? subtitle : "");
    gtk_widget_set_visible(box->subtitle, subtitle && *subtitle);
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->outer), GTK_ACCESSIBLE_PROPERTY_LABEL, title ? title : "",
                                   -1);
    gtk_widget_queue_allocate(view->body);
}

void gkGTKGroupBoxChrome(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 2, h = 2;
    if (view && view->kind == GK_KIND_GROUP_BOX) {
        GKGroupBox *box = view->ext;
        int title = 0;
        gtk_widget_measure(box->title, GTK_ORIENTATION_VERTICAL, -1, NULL, &title, NULL, NULL);
        int frame = 0;
        gtk_widget_measure(view->outer, GTK_ORIENTATION_VERTICAL, -1, &frame, NULL, NULL, NULL);
        h = MAX(frame, title + 2);
    }
    *width = w;
    *height = h;
}

/* gkGTKGroupBoxSubtitleHeight returns the space that the subtitle takes
   above the content when the content area is width pixels wide. */
int gkGTKGroupBoxSubtitleHeight(void *pointer, int width) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_GROUP_BOX) {
        return 0;
    }
    GKGroupBox *box = view->ext;
    if (!gtk_widget_get_visible(box->subtitle)) {
        return 0;
    }
    int natural = 0;
    gtk_widget_measure(box->subtitle, GTK_ORIENTATION_VERTICAL, MAX(0, width - 8), NULL, &natural, NULL, NULL);
    return natural + 6;
}

void gkGTKGroupBoxBody(void *pointer, int *width, int *height, int *top) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0, t = 0;
    if (view && view->kind == GK_KIND_GROUP_BOX) {
        GKGroupBox *box = view->ext;
        int chrome_width, chrome_height;
        gkGTKGroupBoxChrome(pointer, &chrome_width, &chrome_height);
        w = MAX(0, view->width - chrome_width);
        h = MAX(0, view->height - chrome_height);
        if (gtk_widget_get_visible(box->subtitle)) {
            int natural = 0;
            gtk_widget_measure(box->subtitle, GTK_ORIENTATION_VERTICAL, MAX(0, w - 8), NULL, &natural, NULL, NULL);
            t = natural + 6;
        }
    }
    *width = w;
    *height = h;
    *top = t;
}

/* ---- Split view ----

   GtkPaned holds two panes, so a split view of n panes nests n - 1 of them.
   The split view keeps the absolute divider positions so that the Go layout
   code reads the pane sizes before GTK allocates. */

typedef struct {
    int vertical;
    int count;
    GtkWidget **panes;
    GtkWidget **paneds;
    double *positions;
    double *minimums;
    int *collapsible;
    int updating;
} GKSplit;

static void gkSplitFree(gpointer data) {
    GKSplit *split = data;
    g_free(split->panes);
    g_free(split->paneds);
    g_free(split->positions);
    g_free(split->minimums);
    g_free(split->collapsible);
    g_free(split);
}

static int gkSplitLength(GKView *view) {
    GKSplit *split = view->ext;
    return split->vertical ? view->width : view->height;
}

static int gkSplitCross(GKView *view) {
    GKSplit *split = view->ext;
    return split->vertical ? view->height : view->width;
}

static void gkSplitDefaults(GKView *view) {
    GKSplit *split = view->ext;
    int length = gkSplitLength(view);
    for (int i = 0; i < split->count - 1; i++) {
        if (split->positions[i] < 0) {
            split->positions[i] = (length - (split->count - 1)) * (i + 1.0) / split->count + i;
        }
    }
}

static void gkSplitApply(GKView *view) {
    GKSplit *split = view->ext;
    split->updating++;
    double origin = 0;
    for (int i = 0; i < split->count - 1; i++) {
        gtk_paned_set_position(GTK_PANED(split->paneds[i]), (int)round(MAX(0, split->positions[i] - origin)));
        origin = split->positions[i] + 1;
    }
    split->updating--;
}

static void gkSplitPositionChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    GKView *view = data;
    GKSplit *split = view->ext;
    if (split->updating || !gtk_widget_get_mapped(view->outer)) {
        return;
    }
    double origin = 0;
    for (int i = 0; i < split->count - 1; i++) {
        double position = origin + gtk_paned_get_position(GTK_PANED(split->paneds[i]));
        split->positions[i] = position;
        origin = position + 1;
    }
    gkSend(view->handle, GK_EVENT_RESIZE, 0, 0, 0, 0, NULL);
}

void *gkGTKSplitViewCreate(uintptr_t window, int vertical, int panes) {
    GKSplit *split = g_new0(GKSplit, 1);
    split->vertical = vertical != 0;
    split->count = MAX(1, panes);
    split->panes = g_new0(GtkWidget *, split->count);
    split->paneds = g_new0(GtkWidget *, split->count);
    split->positions = g_new0(double, split->count);
    split->minimums = g_new0(double, split->count);
    split->collapsible = g_new0(int, split->count);
    GtkOrientation orientation = vertical ? GTK_ORIENTATION_HORIZONTAL : GTK_ORIENTATION_VERTICAL;
    for (int i = 0; i < split->count; i++) {
        split->panes[i] = gkLayoutNew();
        split->positions[i] = -1;
    }
    GtkWidget *outer = NULL;
    if (split->count == 1) {
        outer = gkLayoutNew();
        gtk_widget_set_parent(split->panes[0], outer);
        gkLayoutMove(split->panes[0], 0, 0, 1, 1);
    } else {
        GtkWidget *tail = split->panes[split->count - 1];
        for (int i = split->count - 2; i >= 0; i--) {
            GtkWidget *paned = gtk_paned_new(orientation);
            gtk_paned_set_start_child(GTK_PANED(paned), split->panes[i]);
            gtk_paned_set_end_child(GTK_PANED(paned), tail);
            gtk_paned_set_resize_start_child(GTK_PANED(paned), FALSE);
            gtk_paned_set_shrink_start_child(GTK_PANED(paned), FALSE);
            gtk_paned_set_shrink_end_child(GTK_PANED(paned), FALSE);
            split->paneds[i] = paned;
            tail = paned;
        }
        outer = tail;
    }
    GKView *view = gkContainer(window, outer, NULL, GK_KIND_SPLIT);
    view->ext = split;
    view->ext_free = gkSplitFree;
    for (int i = 0; i < split->count - 1; i++) {
        g_signal_connect(split->paneds[i], "notify::position", G_CALLBACK(gkSplitPositionChanged), view);
    }
    return outer;
}

void *gkGTKSplitViewPane(void *pointer, int index) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SPLIT) {
        return NULL;
    }
    GKSplit *split = view->ext;
    if (index < 0 || index >= split->count) {
        return NULL;
    }
    GtkWidget *pane = split->panes[index];
    if (!gkView(pane)) {
        GKView *record = gkViewNew(pane, NULL, GK_KIND_CLIP, (uintptr_t)view->window, 0);
        record->body = pane;
        /* The split view owns the pane; the record must not hold another
           reference. */
        g_object_unref(pane);
    }
    return pane;
}

void gkGTKSplitViewSetPane(void *pointer, int index, double minimum, int collapsible) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SPLIT) {
        return;
    }
    GKSplit *split = view->ext;
    if (index < 0 || index >= split->count) {
        return;
    }
    split->minimums[index] = MAX(0, minimum);
    split->collapsible[index] = collapsible != 0;
    GtkWidget *pane = split->panes[index];
    gkLayoutSetMinimum(pane, split->vertical ? (int)minimum : 0, split->vertical ? 0 : (int)minimum);
    if (index < split->count - 1) {
        gtk_paned_set_shrink_start_child(GTK_PANED(split->paneds[index]), collapsible != 0);
    } else if (split->count > 1) {
        gtk_paned_set_shrink_end_child(GTK_PANED(split->paneds[split->count - 2]), collapsible != 0);
    }
}

void gkGTKSplitViewPaneFrame(void *pointer, int index, int *x, int *y, int *width, int *height) {
    GKView *view = gkView(pointer);
    *x = *y = *width = *height = 0;
    if (!view || view->kind != GK_KIND_SPLIT) {
        return;
    }
    GKSplit *split = view->ext;
    if (index < 0 || index >= split->count) {
        return;
    }
    gkSplitDefaults(view);
    double start = index == 0 ? 0 : split->positions[index - 1] + 1;
    double end = index == split->count - 1 ? gkSplitLength(view) : split->positions[index];
    int length = (int)floor(MAX(0, end - start));
    int cross = gkSplitCross(view);
    if (split->vertical) {
        *x = (int)start;
        *width = length;
        *height = cross;
    } else {
        *y = (int)start;
        *width = cross;
        *height = length;
    }
}

double gkGTKSplitViewDivider(void *pointer, int index) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SPLIT) {
        return 0;
    }
    GKSplit *split = view->ext;
    if (index < 0 || index >= split->count - 1) {
        return 0;
    }
    gkSplitDefaults(view);
    return split->positions[index];
}

void gkGTKSplitViewSetDivider(void *pointer, int index, double position) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SPLIT) {
        return;
    }
    GKSplit *split = view->ext;
    if (index < 0 || index >= split->count - 1) {
        return;
    }
    gkSplitDefaults(view);
    double lower = (index == 0 ? 0 : split->positions[index - 1] + 1) + split->minimums[index];
    double upper = (index + 1 < split->count - 1 ? split->positions[index + 1] : gkSplitLength(view)) - 1 -
                   split->minimums[index + 1];
    split->positions[index] = MAX(lower, MIN(upper, position));
    gkSplitApply(view);
}

/* ---- Popover ---- */

typedef struct {
    GtkWidget *popover;
    GtkWidget *body;
    uintptr_t handle;
} GKPopover;

static void gkPopoverClosed(GtkPopover *widget, gpointer data) {
    (void)widget;
    GKPopover *popover = data;
    gkSend(popover->handle, GK_EVENT_CLOSE, 0, 0, 0, 0, NULL);
}

void *gkGTKPopoverCreate(uintptr_t handle, int transient) {
    GKPopover *popover = g_new0(GKPopover, 1);
    popover->handle = handle;
    popover->popover = gtk_popover_new();
    g_object_ref_sink(popover->popover);
    gtk_popover_set_autohide(GTK_POPOVER(popover->popover), transient != 0);
    popover->body = gkLayoutNew();
    GKView *view = gkViewNew(popover->body, NULL, GK_KIND_CLIP, 0, 0);
    view->body = popover->body;
    gtk_popover_set_child(GTK_POPOVER(popover->popover), popover->body);
    /* The popover holds the body now; the Go layer retains it. */
    g_object_unref(popover->body);
    g_signal_connect(popover->popover, "closed", G_CALLBACK(gkPopoverClosed), popover);
    return popover;
}

void *gkGTKPopoverBody(void *pointer) {
    GKPopover *popover = pointer;
    return popover ? popover->body : NULL;
}

int gkGTKPopoverShow(void *pointer, void *anchor_pointer, int edge, int width, int height) {
    GKPopover *popover = pointer;
    GKView *anchor = gkView(anchor_pointer);
    GKWin *win = anchor ? gkWinFromWidget(anchor->outer) : NULL;
    if (!popover || !anchor || !win || !gtk_widget_get_visible(win->window) || !gtk_widget_get_mapped(anchor->outer)) {
        return 0;
    }
    GKView *body = gkView(popover->body);
    if (body) {
        body->window = win;
    }
    if (gtk_widget_get_parent(popover->popover) != anchor->outer) {
        if (gtk_widget_get_parent(popover->popover)) {
            gtk_widget_unparent(popover->popover);
        }
        gtk_widget_set_parent(popover->popover, anchor->outer);
    }
    static const GtkPositionType positions[] = {GTK_POS_TOP, GTK_POS_BOTTOM, GTK_POS_LEFT, GTK_POS_RIGHT};
    gtk_popover_set_position(GTK_POPOVER(popover->popover), positions[edge >= 0 && edge < 4 ? edge : 1]);
    gtk_widget_set_size_request(popover->body, MAX(1, width), MAX(1, height));
    gtk_popover_popup(GTK_POPOVER(popover->popover));
    return 1;
}

void gkGTKPopoverHide(void *pointer) {
    GKPopover *popover = pointer;
    if (popover) {
        gtk_popover_popdown(GTK_POPOVER(popover->popover));
    }
}

int gkGTKPopoverShown(void *pointer) {
    GKPopover *popover = pointer;
    return popover && gtk_widget_get_visible(popover->popover);
}

void gkGTKPopoverRelease(void *pointer) {
    GKPopover *popover = pointer;
    if (!popover) {
        return;
    }
    popover->handle = 0;
    g_signal_handlers_disconnect_by_data(popover->popover, popover);
    gtk_popover_popdown(GTK_POPOVER(popover->popover));
    if (gtk_widget_get_parent(popover->popover)) {
        gtk_widget_unparent(popover->popover);
    }
    g_object_unref(popover->popover);
    g_free(popover);
}

/* ---- Title bar accessory ----

   Accessories share a bar at the top of the window, above the content and
   below the title bar that the window manager draws. */

typedef struct {
    GKWin *win;
    GtkWidget *bar;
    GtkWidget *view;
} GKAccessory;

static GtkWidget *gkAccessoryBar(GKWin *win) {
    GtkWidget *bar = g_object_get_data(G_OBJECT(win->window), "gk-accessories");
    if (bar) {
        return bar;
    }
    bar = gtk_center_box_new();
    gtk_widget_add_css_class(bar, "toolbar");
    GtkWidget *start = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    GtkWidget *center = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    GtkWidget *end = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    gtk_center_box_set_start_widget(GTK_CENTER_BOX(bar), start);
    gtk_center_box_set_center_widget(GTK_CENTER_BOX(bar), center);
    gtk_center_box_set_end_widget(GTK_CENTER_BOX(bar), end);
    gkWinAddChrome(win, bar, 1);
    gtk_widget_set_visible(bar, FALSE);
    g_object_set_data(G_OBJECT(win->window), "gk-accessories", bar);
    return bar;
}

static void gkAccessoryBarUpdate(GtkWidget *bar) {
    int any = 0;
    GtkWidget *parts[] = {gtk_center_box_get_start_widget(GTK_CENTER_BOX(bar)),
                          gtk_center_box_get_center_widget(GTK_CENTER_BOX(bar)),
                          gtk_center_box_get_end_widget(GTK_CENTER_BOX(bar))};
    for (int i = 0; i < 3; i++) {
        any |= gtk_widget_get_first_child(parts[i]) != NULL;
    }
    gtk_widget_set_visible(bar, any);
}

int gkGTKTitleAccessoryAdd(void *window, void *view_pointer, int alignment, int width, int height,
                           void **controller) {
    GKWin *win = window;
    GKView *view = gkView(view_pointer);
    if (!win || !view || win->borderless) {
        return 0;
    }
    GtkWidget *bar = gkAccessoryBar(win);
    GtkWidget *part = alignment == 1   ? gtk_center_box_get_start_widget(GTK_CENTER_BOX(bar))
                      : alignment == 3 ? gtk_center_box_get_end_widget(GTK_CENTER_BOX(bar))
                                       : gtk_center_box_get_center_widget(GTK_CENTER_BOX(bar));
    gkRemoveFromParent(view->outer);
    gtk_widget_set_size_request(view->outer, MAX(1, width), MAX(1, height));
    gtk_widget_set_child_visible(view->outer, TRUE);
    gtk_box_append(GTK_BOX(part), view->outer);
    gkAccessoryBarUpdate(bar);
    GKAccessory *accessory = g_new0(GKAccessory, 1);
    accessory->win = win;
    accessory->bar = bar;
    accessory->view = view->outer;
    g_object_ref(view->outer);
    *controller = accessory;
    return 1;
}

void gkGTKTitleAccessoryRemove(void *pointer) {
    GKAccessory *accessory = pointer;
    if (!accessory) {
        return;
    }
    if (gtk_widget_get_parent(accessory->view)) {
        gkRemoveFromParent(accessory->view);
    }
    if (!accessory->win->destroyed) {
        gkAccessoryBarUpdate(accessory->bar);
    }
    g_object_unref(accessory->view);
    g_free(accessory);
}

void gkGTKViewSize(void *pointer, int *width, int *height) {
    GKView *view = gkView(pointer);
    int w = 0, h = 0;
    if (view) {
        w = gtk_widget_get_width(view->outer);
        h = gtk_widget_get_height(view->outer);
        if (w <= 0 || h <= 0) {
            w = view->width;
            h = view->height;
        }
    }
    *width = w;
    *height = h;
}
