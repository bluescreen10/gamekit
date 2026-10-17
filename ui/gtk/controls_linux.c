//go:build linux && cgo

#include "gtk_internal.h"

#include <math.h>
#include <stdio.h>

extern GtkWidget *gkAffixBox(GtkWidget *entry);
extern int gkFontFamilyInstalled(const char *family);

static GKView *gkControl(uintptr_t window, GtkWidget *outer, GtkWidget *control, int kind) {
    GKView *view = gkViewNew(outer, control, kind, window, 0);
    GKWin *win = gkWinFromHandle(window);
    if (win && win->root) {
        gkLayoutPut(win->root, outer);
    }
    return view;
}

/* gkReport sends a component event through the view's contract handle. */
static void gkReport(GKView *view, int kind, double x, double y, const char *text) {
    if (view && !view->suppress) {
        gkSend(view->handle, kind, 0, 0, x, y, text);
    }
}

/* ---- Toggle button ---- */

static void gkToggleChanged(GtkToggleButton *button, gpointer data) {
    (void)button;
    gkReport(data, GK_EVENT_VALUE, 0, 0, NULL);
}

void *gkGTKToggleButtonCreate(uintptr_t window, const char *text, void *icon, int checked, int toolbar) {
    GtkWidget *button = gtk_toggle_button_new_with_label(text ? text : "");
    if (toolbar) {
        gtk_widget_add_css_class(button, "flat");
    }
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(button), checked != 0);
    GKView *view = gkControl(window, button, NULL, GK_KIND_TOGGLE);
    if (icon) {
        gkGTKButtonConfigure(button, icon, 0, 0, NULL, 0);
    }
    g_signal_connect(button, "toggled", G_CALLBACK(gkToggleChanged), view);
    return button;
}

/* ---- Switch ---- */

typedef struct {
    char *on;
    char *off;
} GKSwitch;

static void gkSwitchFree(gpointer data) {
    GKSwitch *state = data;
    g_free(state->on);
    g_free(state->off);
    g_free(state);
}

static void gkSwitchUpdate(GKView *view) {
    GKSwitch *state = view->ext;
    const char *value = gtk_switch_get_active(GTK_SWITCH(view->control)) ? state->on : state->off;
    if (value && *value) {
        gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, value, -1);
    } else {
        gtk_accessible_reset_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT);
    }
}

static void gkSwitchChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    GKView *view = data;
    gkSwitchUpdate(view);
    gkReport(view, GK_EVENT_VALUE, 0, 0, NULL);
}

void *gkGTKSwitchCreate(uintptr_t window, const char *text, const char *on_label, const char *off_label, int checked,
                        int size) {
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    GtkWidget *label = gtk_label_new(text ? text : "");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_widget_set_hexpand(label, TRUE);
    gtk_widget_set_visible(label, text && *text);
    GtkWidget *toggle = gtk_switch_new();
    gtk_widget_set_valign(toggle, GTK_ALIGN_CENTER);
    gtk_switch_set_active(GTK_SWITCH(toggle), checked != 0);
    if (size == 1 || size == 2) {
        gtk_widget_add_css_class(toggle, "gk-small-switch");
        gkSetCSSRule("switch.gk-small-switch", "transform: scale(0.8);");
    }
    gtk_box_append(GTK_BOX(box), label);
    gtk_box_append(GTK_BOX(box), toggle);
    GKView *view = gkControl(window, box, toggle, GK_KIND_SWITCH);
    view->label = label;
    GKSwitch *state = g_new0(GKSwitch, 1);
    state->on = g_strdup(on_label ? on_label : "");
    state->off = g_strdup(off_label ? off_label : "");
    view->ext = state;
    view->ext_free = gkSwitchFree;
    gtk_accessible_update_property(GTK_ACCESSIBLE(toggle), GTK_ACCESSIBLE_PROPERTY_LABEL, text ? text : "", -1);
    gtk_accessible_update_relation(GTK_ACCESSIBLE(toggle), GTK_ACCESSIBLE_RELATION_LABELLED_BY, label, NULL, -1);
    gkSwitchUpdate(view);
    g_signal_connect(toggle, "notify::active", G_CALLBACK(gkSwitchChanged), view);
    return box;
}

int gkGTKSwitchChecked(void *pointer) {
    GKView *view = gkView(pointer);
    return view && gtk_switch_get_active(GTK_SWITCH(view->control));
}

void gkGTKSwitchSet(void *pointer, const char *text, int checked) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    gtk_label_set_text(GTK_LABEL(view->label), text ? text : "");
    gtk_widget_set_visible(view->label, text && *text);
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL, text ? text : "",
                                   -1);
    view->suppress++;
    gtk_switch_set_active(GTK_SWITCH(view->control), checked != 0);
    view->suppress--;
    gkSwitchUpdate(view);
}

/* ---- Password field ---- */

static void gkTextChanged(GtkEditable *editable, gpointer data) {
    (void)editable;
    gkReport(data, GK_EVENT_TEXT_CHANGE, 0, 0, NULL);
}

static void gkTextSubmitted(GtkWidget *widget, gpointer data) {
    (void)widget;
    gkReport(data, GK_EVENT_SUBMIT, 0, 0, NULL);
}

static void gkPasswordRevealed(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    GKView *view = data;
    if (view->suppress || !view->handle) {
        return;
    }
    gboolean revealed = FALSE;
    g_object_get(object, "show-peek-icon", &revealed, NULL);
    GtkWidget *text = gkDescendant(view->control, GTK_TYPE_TEXT);
    revealed = text && gtk_text_get_visibility(GTK_TEXT(text));
    gkSend(view->handle, GK_EVENT_EXPAND, revealed, 0, 0, 0, NULL);
}

void *gkGTKPasswordCreate(uintptr_t window, const char *placeholder, int reveal) {
    GtkWidget *entry = gtk_password_entry_new();
    gtk_password_entry_set_show_peek_icon(GTK_PASSWORD_ENTRY(entry), reveal != 0);
    g_object_set(entry, "placeholder-text", placeholder ? placeholder : "", "activates-default", TRUE, NULL);
    GKView *view = gkControl(window, entry, NULL, GK_KIND_PASSWORD);
    g_signal_connect(entry, "changed", G_CALLBACK(gkTextChanged), view);
    g_signal_connect(entry, "activate", G_CALLBACK(gkTextSubmitted), view);
    GtkWidget *text = gkDescendant(entry, GTK_TYPE_TEXT);
    if (text) {
        g_signal_connect(text, "notify::visibility", G_CALLBACK(gkPasswordRevealed), view);
    }
    return entry;
}

char *gkGTKPasswordText(void *pointer) {
    GKView *view = gkView(pointer);
    return gkStrdup(view ? gtk_editable_get_text(GTK_EDITABLE(view->control)) : "");
}

void gkGTKPasswordSet(void *pointer, const char *text, const char *placeholder, int revealed) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    view->suppress++;
    if (strcmp(gtk_editable_get_text(GTK_EDITABLE(view->control)), text ? text : "") != 0) {
        gtk_editable_set_text(GTK_EDITABLE(view->control), text ? text : "");
    }
    g_object_set(view->control, "placeholder-text", placeholder ? placeholder : "", NULL);
    GtkWidget *field = gkDescendant(view->control, GTK_TYPE_TEXT);
    if (field) {
        gtk_text_set_visibility(GTK_TEXT(field), revealed != 0);
    }
    view->suppress--;
}

/* ---- Search field ---- */

typedef struct {
    GtkWidget *popover;
    GtkWidget *list;
} GKSuggestions;

static void gkSuggestionChosen(GtkListBox *list, GtkListBoxRow *row, gpointer data) {
    (void)list;
    GKView *view = data;
    GKSuggestions *suggestions = view->ext;
    GtkWidget *label = gtk_list_box_row_get_child(row);
    view->suppress++;
    gtk_editable_set_text(GTK_EDITABLE(view->control), gtk_label_get_text(GTK_LABEL(label)));
    view->suppress--;
    gtk_popover_popdown(GTK_POPOVER(suggestions->popover));
    gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, NULL);
    gkReport(view, GK_EVENT_SUBMIT, 0, 0, NULL);
}

static void gkSearchChanged(GtkSearchEntry *entry, gpointer data) {
    (void)entry;
    GKView *view = data;
    GKSuggestions *suggestions = view->ext;
    if (!view->suppress && suggestions->list && gtk_widget_get_first_child(suggestions->list) &&
        gtk_widget_has_focus(gkDescendant(view->control, GTK_TYPE_TEXT))) {
        gtk_popover_popup(GTK_POPOVER(suggestions->popover));
    }
    gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, NULL);
}

static void gkSuggestionsFree(gpointer data) {
    GKSuggestions *suggestions = data;
    if (suggestions->popover) {
        gtk_widget_unparent(suggestions->popover);
    }
    g_free(suggestions);
}

void *gkGTKSearchCreate(uintptr_t window, const char *text, const char *placeholder) {
    GtkWidget *entry = gtk_search_entry_new();
    gtk_editable_set_text(GTK_EDITABLE(entry), text ? text : "");
    gtk_search_entry_set_placeholder_text(GTK_SEARCH_ENTRY(entry), placeholder ? placeholder : "");
    GKView *view = gkControl(window, entry, NULL, GK_KIND_SEARCH);
    GKSuggestions *suggestions = g_new0(GKSuggestions, 1);
    suggestions->popover = gtk_popover_new();
    gtk_popover_set_autohide(GTK_POPOVER(suggestions->popover), TRUE);
    gtk_popover_set_has_arrow(GTK_POPOVER(suggestions->popover), FALSE);
    gtk_widget_set_parent(suggestions->popover, entry);
    suggestions->list = gtk_list_box_new();
    gtk_popover_set_child(GTK_POPOVER(suggestions->popover), suggestions->list);
    g_signal_connect(suggestions->list, "row-activated", G_CALLBACK(gkSuggestionChosen), view);
    view->ext = suggestions;
    view->ext_free = gkSuggestionsFree;
    g_signal_connect(entry, "search-changed", G_CALLBACK(gkSearchChanged), view);
    g_signal_connect(entry, "activate", G_CALLBACK(gkTextSubmitted), view);
    return entry;
}

void gkGTKSearchSetSuggestions(void *pointer, const char *items, size_t size) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_SEARCH) {
        return;
    }
    GKSuggestions *suggestions = view->ext;
    gtk_list_box_remove_all(GTK_LIST_BOX(suggestions->list));
    int count = 0;
    char **values = gkSplit(items, size, &count);
    for (int i = 0; i < count; i++) {
        GtkWidget *label = gtk_label_new(values[i]);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_list_box_append(GTK_LIST_BOX(suggestions->list), label);
    }
    g_strfreev(values);
}

/* ---- Number field ---- */

static void gkNumberChanged(GtkSpinButton *spin, gpointer data) {
    gkReport(data, GK_EVENT_VALUE, gtk_spin_button_get_value(spin), 0, NULL);
}

void *gkGTKNumberCreate(uintptr_t window, double minimum, double maximum, double value, double step, int decimals) {
    GtkWidget *spin = gtk_spin_button_new_with_range(minimum, maximum, step > 0 ? step : 1);
    gtk_spin_button_set_digits(GTK_SPIN_BUTTON(spin), (guint)MAX(0, decimals));
    gtk_spin_button_set_numeric(GTK_SPIN_BUTTON(spin), TRUE);
    gtk_spin_button_set_value(GTK_SPIN_BUTTON(spin), value);
    GKView *view = gkControl(window, spin, NULL, GK_KIND_NUMBER);
    g_signal_connect(spin, "value-changed", G_CALLBACK(gkNumberChanged), view);
    return spin;
}

double gkGTKNumberValue(void *pointer) {
    GKView *view = gkView(pointer);
    return view ? gtk_spin_button_get_value(GTK_SPIN_BUTTON(view->control)) : 0;
}

void gkGTKNumberSetValue(void *pointer, double value) {
    GKView *view = gkView(pointer);
    if (view) {
        view->suppress++;
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(view->control), value);
        view->suppress--;
    }
}

/* ---- Editable combo ---- */

typedef struct {
    GtkWidget *entry;
    GtkWidget *button;
    GtkWidget *list;
} GKCombo;

static void gkComboChosen(GtkListBox *list, GtkListBoxRow *row, gpointer data) {
    (void)list;
    GKView *view = data;
    GKCombo *combo = view->ext;
    const char *text = gtk_label_get_text(GTK_LABEL(gtk_list_box_row_get_child(row)));
    char *value = g_strdup(text);
    view->suppress++;
    gtk_editable_set_text(GTK_EDITABLE(combo->entry), value);
    view->suppress--;
    gtk_menu_button_popdown(GTK_MENU_BUTTON(combo->button));
    gkReport(view, GK_EVENT_TEXT_CHANGE, 0, 0, value);
    g_free(value);
}

static void gkComboFocusLeave(GtkEventControllerFocus *controller, gpointer data) {
    (void)controller;
    gkReport(data, GK_EVENT_SUBMIT, 0, 0, NULL);
}

static void gkComboFill(GKCombo *combo, const char *items, size_t size) {
    gtk_list_box_remove_all(GTK_LIST_BOX(combo->list));
    int count = 0;
    char **values = gkSplit(items, size, &count);
    for (int i = 0; i < count; i++) {
        GtkWidget *label = gtk_label_new(values[i]);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_list_box_append(GTK_LIST_BOX(combo->list), label);
    }
    gtk_widget_set_sensitive(combo->button, count > 0);
    g_strfreev(values);
}

void *gkGTKComboCreate(uintptr_t window, const char *text, const char *items, size_t size) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "accessible-role",
                                  GTK_ACCESSIBLE_ROLE_COMBO_BOX, NULL);
    gtk_widget_add_css_class(box, "linked");
    GKCombo *combo = g_new0(GKCombo, 1);
    combo->entry = gtk_entry_new();
    gtk_widget_set_hexpand(combo->entry, TRUE);
    gtk_editable_set_text(GTK_EDITABLE(combo->entry), text ? text : "");
    combo->button = gtk_menu_button_new();
    GtkWidget *popover = gtk_popover_new();
    combo->list = gtk_list_box_new();
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_scrolled_window_set_propagate_natural_height(GTK_SCROLLED_WINDOW(scroll), TRUE);
    gtk_scrolled_window_set_max_content_height(GTK_SCROLLED_WINDOW(scroll), 300);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), combo->list);
    gtk_popover_set_child(GTK_POPOVER(popover), scroll);
    gtk_menu_button_set_popover(GTK_MENU_BUTTON(combo->button), popover);
    gtk_box_append(GTK_BOX(box), combo->entry);
    gtk_box_append(GTK_BOX(box), combo->button);
    GKView *view = gkControl(window, box, combo->entry, GK_KIND_COMBO);
    view->ext = combo;
    view->ext_free = g_free;
    gkComboFill(combo, items, size);
    g_signal_connect(combo->list, "row-activated", G_CALLBACK(gkComboChosen), view);
    g_signal_connect(combo->entry, "changed", G_CALLBACK(gkTextChanged), view);
    g_signal_connect(combo->entry, "activate", G_CALLBACK(gkTextSubmitted), view);
    GtkEventController *focus = gtk_event_controller_focus_new();
    g_signal_connect(focus, "leave", G_CALLBACK(gkComboFocusLeave), view);
    gtk_widget_add_controller(combo->entry, focus);
    return box;
}

void gkGTKComboSetItems(void *pointer, const char *items, size_t size) {
    GKView *view = gkView(pointer);
    if (view && view->kind == GK_KIND_COMBO) {
        gkComboFill(view->ext, items, size);
    }
}

/* ---- Segmented control ---- */

typedef struct {
    int mode;
    int clicked;
} GKSegments;

static void gkSegmentClicked(GtkButton *button, gpointer data) {
    GKView *view = data;
    GKSegments *segments = view->ext;
    for (guint i = 0; i < view->buttons->len; i++) {
        if (g_ptr_array_index(view->buttons, i) == button) {
            segments->clicked = (int)i;
        }
    }
    if (segments->mode == 0) {
        gkReport(view, GK_EVENT_SELECTION, 0, 0, NULL);
    }
}

static void gkSegmentToggled(GtkToggleButton *button, gpointer data) {
    GKView *view = data;
    GKSegments *segments = view->ext;
    if (segments->mode == 1 && !gtk_toggle_button_get_active(button)) {
        return;
    }
    gkReport(view, GK_EVENT_SELECTION, 0, 0, NULL);
}

void *gkGTKSegmentedCreate(uintptr_t window, const char *labels, size_t size, void **icons, const int *enabled,
                           int count, int mode) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "accessible-role",
                                  GTK_ACCESSIBLE_ROLE_RADIO_GROUP, NULL);
    gtk_widget_add_css_class(box, "linked");
    GKView *view = gkControl(window, box, NULL, GK_KIND_SEGMENTED);
    GKSegments *segments = g_new0(GKSegments, 1);
    segments->mode = mode;
    segments->clicked = -1;
    view->ext = segments;
    view->ext_free = g_free;
    view->buttons = g_ptr_array_new();
    int title_count = 0;
    char **titles = gkSplit(labels, size, &title_count);
    for (int i = 0; i < count; i++) {
        GtkWidget *button = mode == 0 ? gtk_button_new() : gtk_toggle_button_new();
        const char *title = i < title_count ? titles[i] : "";
        if (icons[i] && !*title) {
            gtk_button_set_child(GTK_BUTTON(button), gkIconImage(icons[i], 16));
            gtk_accessible_update_property(GTK_ACCESSIBLE(button), GTK_ACCESSIBLE_PROPERTY_LABEL, title, -1);
        } else if (icons[i]) {
            GtkWidget *content = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
            gtk_box_append(GTK_BOX(content), gkIconImage(icons[i], 16));
            gtk_box_append(GTK_BOX(content), gtk_label_new(title));
            gtk_button_set_child(GTK_BUTTON(button), content);
        } else {
            gtk_button_set_label(GTK_BUTTON(button), title);
        }
        if (mode == 1 && view->buttons->len > 0) {
            gtk_toggle_button_set_group(GTK_TOGGLE_BUTTON(button), g_ptr_array_index(view->buttons, 0));
        }
        gtk_widget_set_sensitive(button, enabled[i] != 0);
        gtk_widget_set_hexpand(button, TRUE);
        g_ptr_array_add(view->buttons, button);
        gtk_box_append(GTK_BOX(box), button);
        g_signal_connect(button, "clicked", G_CALLBACK(gkSegmentClicked), view);
        if (mode != 0) {
            g_signal_connect(button, "toggled", G_CALLBACK(gkSegmentToggled), view);
        }
    }
    g_strfreev(titles);
    return box;
}

int gkGTKSegmentSelected(void *pointer, int index) {
    GKView *view = gkView(pointer);
    if (!view || index < 0 || index >= (int)view->buttons->len) {
        return 0;
    }
    GtkWidget *button = g_ptr_array_index(view->buttons, index);
    return GTK_IS_TOGGLE_BUTTON(button) && gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(button));
}

int gkGTKSegmentClicked(void *pointer) {
    GKView *view = gkView(pointer);
    return view ? ((GKSegments *)view->ext)->clicked : -1;
}

void gkGTKSegmentSetSelected(void *pointer, int index, int selected) {
    GKView *view = gkView(pointer);
    if (!view || index < 0 || index >= (int)view->buttons->len) {
        return;
    }
    GtkWidget *button = g_ptr_array_index(view->buttons, index);
    if (GTK_IS_TOGGLE_BUTTON(button)) {
        view->suppress++;
        gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(button), selected != 0);
        view->suppress--;
    }
}

/* ---- Value controls ----

   A stepper, a dial, and a range slider keep their values in GKValue. */

typedef struct {
    double minimum, maximum, value, upper, step;
    int wraps;
    int dragging;
    int focused_knob;
    double drag_angle;
    GtkWidget *area;
} GKValue;

static void gkValueAccessible(GKView *view) {
    GKValue *state = view->ext;
    GtkAccessible *element = GTK_ACCESSIBLE(view->outer);
    if (view->kind == GK_KIND_RANGE) {
        char text[96];
        snprintf(text, sizeof(text), "%g to %g", state->value, state->upper);
        gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_VALUE_MIN, state->minimum,
                                       GTK_ACCESSIBLE_PROPERTY_VALUE_MAX, state->maximum,
                                       GTK_ACCESSIBLE_PROPERTY_VALUE_NOW, state->value,
                                       GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, text, -1);
        return;
    }
    gtk_accessible_update_property(element, GTK_ACCESSIBLE_PROPERTY_VALUE_MIN, state->minimum,
                                   GTK_ACCESSIBLE_PROPERTY_VALUE_MAX, state->maximum,
                                   GTK_ACCESSIBLE_PROPERTY_VALUE_NOW, state->value, -1);
}

static double gkSnap(GKValue *state, double value) {
    if (state->step > 0) {
        value = state->minimum + round((value - state->minimum) / state->step) * state->step;
    }
    return MAX(state->minimum, MIN(state->maximum, value));
}

static void gkValueChanged(GKView *view) {
    GKValue *state = view->ext;
    gkValueAccessible(view);
    if (state->area) {
        gtk_widget_queue_draw(state->area);
    }
    gkReport(view, GK_EVENT_VALUE, state->value, state->upper, NULL);
}

static void gkStepperStep(GKView *view, double direction) {
    GKValue *state = view->ext;
    double step = state->step > 0 ? state->step : 1;
    double value = state->value + direction * step;
    if (value > state->maximum) {
        value = state->wraps ? state->minimum : state->maximum;
    } else if (value < state->minimum) {
        value = state->wraps ? state->maximum : state->minimum;
    }
    if (value != state->value) {
        state->value = value;
        gkValueChanged(view);
    }
}

static void gkStepperUp(GtkButton *button, gpointer data) {
    (void)button;
    gkStepperStep(data, 1);
}

static void gkStepperDown(GtkButton *button, gpointer data) {
    (void)button;
    gkStepperStep(data, -1);
}

void *gkGTKStepperCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps) {
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "accessible-role",
                                  GTK_ACCESSIBLE_ROLE_SPIN_BUTTON, NULL);
    gtk_widget_add_css_class(box, "linked");
    GtkWidget *down = gtk_button_new_from_icon_name("list-remove-symbolic");
    GtkWidget *up = gtk_button_new_from_icon_name("list-add-symbolic");
    gtk_accessible_update_property(GTK_ACCESSIBLE(down), GTK_ACCESSIBLE_PROPERTY_LABEL, "Decrease", -1);
    gtk_accessible_update_property(GTK_ACCESSIBLE(up), GTK_ACCESSIBLE_PROPERTY_LABEL, "Increase", -1);
    gtk_box_append(GTK_BOX(box), down);
    gtk_box_append(GTK_BOX(box), up);
    GKView *view = gkControl(window, box, NULL, GK_KIND_STEPPER);
    GKValue *state = g_new0(GKValue, 1);
    state->minimum = minimum;
    state->maximum = maximum;
    state->value = MAX(minimum, MIN(maximum, value));
    state->step = step;
    state->wraps = wraps != 0;
    view->ext = state;
    view->ext_free = g_free;
    gkValueAccessible(view);
    g_signal_connect(up, "clicked", G_CALLBACK(gkStepperUp), view);
    g_signal_connect(down, "clicked", G_CALLBACK(gkStepperDown), view);
    return box;
}

/* A dial is at its minimum at the lower left and turns clockwise through
   300 degrees to the lower right, like a hardware knob. A dial that wraps
   is at its minimum at the top and turns through a full circle, like the
   circular slider of macOS. */
static double gkDialSweep(GKValue *state) {
    return state->wraps ? 2 * G_PI : 300 * G_PI / 180;
}

static double gkDialStart(GKValue *state) {
    return state->wraps ? -G_PI / 2 : 120 * G_PI / 180;
}

static double gkDialFraction(GKValue *state) {
    double span = state->maximum - state->minimum;
    return span > 0 ? (state->value - state->minimum) / span : 0;
}

static void gkDialDraw(GtkDrawingArea *area, cairo_t *cr, int width, int height, gpointer data) {
    GKView *view = data;
    GKValue *state = view->ext;
    GdkRGBA color;
    gtk_widget_get_color(GTK_WIDGET(area), &color);
    double radius = MIN(width, height) / 2.0 - 3;
    double cx = width / 2.0, cy = height / 2.0;
    double start = gkDialStart(state);
    cairo_set_line_width(cr, 2);
    cairo_set_source_rgba(cr, color.red, color.green, color.blue, 0.25);
    cairo_arc(cr, cx, cy, radius, 0, 2 * G_PI);
    cairo_stroke(cr);
    double angle = start + gkDialFraction(state) * gkDialSweep(state);
    cairo_set_source_rgba(cr, color.red, color.green, color.blue, gtk_widget_is_sensitive(view->outer) ? 1 : 0.4);
    cairo_arc(cr, cx, cy, radius, start, angle);
    cairo_stroke(cr);
    cairo_move_to(cr, cx, cy);
    cairo_line_to(cr, cx + cos(angle) * radius * 0.8, cy + sin(angle) * radius * 0.8);
    cairo_set_line_width(cr, 3);
    cairo_stroke(cr);
}

static void gkDialPoint(GKView *view, double x, double y) {
    GKValue *state = view->ext;
    int width = gtk_widget_get_width(state->area), height = gtk_widget_get_height(state->area);
    double angle = atan2(y - height / 2.0, x - width / 2.0);
    double sweep = gkDialSweep(state);
    double offset = fmod(angle - gkDialStart(state) + 4 * G_PI, 2 * G_PI);
    if (offset > sweep) {
        offset = offset - sweep < (2 * G_PI - sweep) / 2 ? sweep : 0;
    }
    double value = gkSnap(state, state->minimum + offset / sweep * (state->maximum - state->minimum));
    if (state->wraps && value >= state->maximum) {
        /* The top of a full circle is the minimum. */
        value = state->minimum;
    }
    if (value != state->value) {
        state->value = value;
        gkValueChanged(view);
    }
}

static void gkDialDragBegin(GtkGestureDrag *gesture, double x, double y, gpointer data) {
    GKView *view = data;
    GKValue *state = view->ext;
    state->drag_angle = 0;
    gtk_widget_grab_focus(state->area);
    g_object_set_data(G_OBJECT(gesture), "gk-start-x", NULL);
    double *origin = g_new(double, 2);
    origin[0] = x;
    origin[1] = y;
    g_object_set_data_full(G_OBJECT(gesture), "gk-origin", origin, g_free);
    gkDialPoint(view, x, y);
}

static void gkDialDragUpdate(GtkGestureDrag *gesture, double dx, double dy, gpointer data) {
    double *origin = g_object_get_data(G_OBJECT(gesture), "gk-origin");
    if (origin) {
        gkDialPoint(data, origin[0] + dx, origin[1] + dy);
    }
}

static gboolean gkValueKey(GtkEventControllerKey *controller, guint keyval, guint keycode, GdkModifierType state,
                           gpointer data) {
    (void)controller;
    (void)keycode;
    (void)state;
    GKView *view = data;
    GKValue *value = view->ext;
    double step = value->step > 0 ? value->step : (value->maximum - value->minimum) / 100;
    double delta = 0;
    switch (keyval) {
    case GDK_KEY_Left:
    case GDK_KEY_Down:
        delta = -step;
        break;
    case GDK_KEY_Right:
    case GDK_KEY_Up:
        delta = step;
        break;
    case GDK_KEY_Tab:
        if (view->kind == GK_KIND_RANGE && value->focused_knob == 0) {
            value->focused_knob = 1;
            gtk_widget_queue_draw(value->area);
            return TRUE;
        }
        return FALSE;
    default:
        return FALSE;
    }
    if (view->kind == GK_KIND_RANGE) {
        if (value->focused_knob == 0) {
            value->value = MAX(value->minimum, MIN(value->upper, value->value + delta));
        } else {
            value->upper = MIN(value->maximum, MAX(value->value, value->upper + delta));
        }
    } else {
        value->value = MAX(value->minimum, MIN(value->maximum, value->value + delta));
    }
    gkValueChanged(view);
    return TRUE;
}

static GKValue *gkValueWidget(GKView *view, double minimum, double maximum, double value, double step,
                              GtkDrawingAreaDrawFunc draw) {
    GKValue *state = g_new0(GKValue, 1);
    state->minimum = minimum;
    state->maximum = maximum;
    state->value = MAX(minimum, MIN(maximum, value));
    state->step = step;
    state->area = view->outer;
    view->ext = state;
    view->ext_free = g_free;
    gtk_drawing_area_set_draw_func(GTK_DRAWING_AREA(view->outer), draw, view, NULL);
    gtk_widget_set_focusable(view->outer, TRUE);
    GtkEventController *keys = gtk_event_controller_key_new();
    g_signal_connect(keys, "key-pressed", G_CALLBACK(gkValueKey), view);
    gtk_widget_add_controller(view->outer, keys);
    return state;
}

void *gkGTKDialCreate(uintptr_t window, double minimum, double maximum, double value, double step, int wraps) {
    GtkWidget *area = g_object_new(GTK_TYPE_DRAWING_AREA, "accessible-role", GTK_ACCESSIBLE_ROLE_SLIDER, NULL);
    gtk_drawing_area_set_content_width(GTK_DRAWING_AREA(area), 32);
    gtk_drawing_area_set_content_height(GTK_DRAWING_AREA(area), 32);
    GKView *view = gkControl(window, area, NULL, GK_KIND_DIAL);
    gkValueWidget(view, minimum, maximum, value, step, gkDialDraw);
    ((GKValue *)view->ext)->wraps = wraps != 0;
    GtkGesture *drag = gtk_gesture_drag_new();
    g_signal_connect(drag, "drag-begin", G_CALLBACK(gkDialDragBegin), view);
    g_signal_connect(drag, "drag-update", G_CALLBACK(gkDialDragUpdate), view);
    gtk_widget_add_controller(area, GTK_EVENT_CONTROLLER(drag));
    gkValueAccessible(view);
    return area;
}

double gkGTKControlValue(void *pointer) {
    GKView *view = gkView(pointer);
    return view && view->ext ? ((GKValue *)view->ext)->value : 0;
}

void gkGTKControlSetValue(void *pointer, double value) {
    GKView *view = gkView(pointer);
    if (!view || !view->ext) {
        return;
    }
    GKValue *state = view->ext;
    state->value = MAX(state->minimum, MIN(state->maximum, value));
    gkValueAccessible(view);
    if (state->area) {
        gtk_widget_queue_draw(state->area);
    }
}

/* ---- Range slider ---- */

static const double gkRangeInset = 9;

static double gkRangePosition(GKValue *state, int width, double value) {
    double span = state->maximum - state->minimum;
    double fraction = span > 0 ? (value - state->minimum) / span : 0;
    return gkRangeInset + fraction * MAX(0, width - gkRangeInset * 2);
}

static double gkRangeValue(GKValue *state, int width, double x) {
    double track = MAX(1, width - gkRangeInset * 2);
    double value = state->minimum + MAX(0, MIN(track, x - gkRangeInset)) / track * (state->maximum - state->minimum);
    return gkSnap(state, value);
}

static void gkRangeDraw(GtkDrawingArea *area, cairo_t *cr, int width, int height, gpointer data) {
    GKView *view = data;
    GKValue *state = view->ext;
    GdkRGBA color;
    gtk_widget_get_color(GTK_WIDGET(area), &color);
    double mid = height / 2.0;
    cairo_set_source_rgba(cr, color.red, color.green, color.blue, 0.2);
    cairo_rectangle(cr, gkRangeInset, mid - 2, MAX(0, width - gkRangeInset * 2), 4);
    cairo_fill(cr);
    double lower = gkRangePosition(state, width, state->value);
    double upper = gkRangePosition(state, width, state->upper);
    double alpha = gtk_widget_is_sensitive(view->outer) ? 1 : 0.4;
    cairo_set_source_rgba(cr, 0x35 / 255.0, 0x84 / 255.0, 0xe4 / 255.0, alpha);
    cairo_rectangle(cr, lower, mid - 2, upper - lower, 4);
    cairo_fill(cr);
    for (int knob = 0; knob < 2; knob++) {
        double x = knob == 0 ? lower : upper;
        cairo_arc(cr, x, mid, 8, 0, 2 * G_PI);
        cairo_set_source_rgba(cr, 1, 1, 1, 1);
        cairo_fill_preserve(cr);
        int focused = gtk_widget_has_focus(view->outer) && state->focused_knob == knob;
        cairo_set_source_rgba(cr, color.red, color.green, color.blue, focused ? 0.9 : 0.3);
        cairo_set_line_width(cr, focused ? 2 : 1);
        cairo_stroke(cr);
    }
}

static void gkRangeMove(GKView *view, double x) {
    GKValue *state = view->ext;
    double value = gkRangeValue(state, gtk_widget_get_width(view->outer), x);
    if (state->dragging == 0) {
        state->value = MIN(value, state->upper);
    } else {
        state->upper = MAX(value, state->value);
    }
    gkValueChanged(view);
}

static void gkRangeDragBegin(GtkGestureDrag *gesture, double x, double y, gpointer data) {
    (void)y;
    GKView *view = data;
    GKValue *state = view->ext;
    int width = gtk_widget_get_width(view->outer);
    double lower = gkRangePosition(state, width, state->value);
    double upper = gkRangePosition(state, width, state->upper);
    state->dragging = fabs(x - lower) <= fabs(x - upper) ? 0 : 1;
    if (lower == upper) {
        state->dragging = x < lower ? 0 : 1;
    }
    state->focused_knob = state->dragging;
    gtk_widget_grab_focus(view->outer);
    double *origin = g_new(double, 1);
    *origin = x;
    g_object_set_data_full(G_OBJECT(gesture), "gk-origin", origin, g_free);
    gkRangeMove(view, x);
}

static void gkRangeDragUpdate(GtkGestureDrag *gesture, double dx, double dy, gpointer data) {
    (void)dy;
    double *origin = g_object_get_data(G_OBJECT(gesture), "gk-origin");
    if (origin) {
        gkRangeMove(data, *origin + dx);
    }
}

void *gkGTKRangeSliderCreate(uintptr_t window, double minimum, double maximum, double lower, double upper,
                             double step) {
    GtkWidget *area = g_object_new(GTK_TYPE_DRAWING_AREA, "accessible-role", GTK_ACCESSIBLE_ROLE_SLIDER, NULL);
    gtk_drawing_area_set_content_width(GTK_DRAWING_AREA(area), 160);
    gtk_drawing_area_set_content_height(GTK_DRAWING_AREA(area), 22);
    GKView *view = gkControl(window, area, NULL, GK_KIND_RANGE);
    GKValue *state = gkValueWidget(view, minimum, maximum, lower, step, gkRangeDraw);
    state->upper = MAX(state->value, MIN(maximum, upper));
    GtkGesture *drag = gtk_gesture_drag_new();
    g_signal_connect(drag, "drag-begin", G_CALLBACK(gkRangeDragBegin), view);
    g_signal_connect(drag, "drag-update", G_CALLBACK(gkRangeDragUpdate), view);
    gtk_widget_add_controller(area, GTK_EVENT_CONTROLLER(drag));
    gkValueAccessible(view);
    return area;
}

void gkGTKRangeSliderValues(void *pointer, double *lower, double *upper) {
    GKView *view = gkView(pointer);
    GKValue *state = view ? view->ext : NULL;
    *lower = state ? state->value : 0;
    *upper = state ? state->upper : 0;
}

void gkGTKRangeSliderSet(void *pointer, double lower, double upper) {
    GKView *view = gkView(pointer);
    if (!view || !view->ext) {
        return;
    }
    GKValue *state = view->ext;
    state->value = lower;
    state->upper = upper;
    gkValueAccessible(view);
    gtk_widget_queue_draw(view->outer);
}

/* ---- Dates and times ----

   Dates are exchanged as seconds since 1970 in UTC, and pickers show them in
   UTC so a calendar date never shifts with the local time zone. */

enum {
    GK_ELEMENTS_TIME = 0x000c,
    GK_ELEMENTS_SECONDS = 0x0002,
    GK_ELEMENTS_DATE = 0x00e0,
};

typedef struct {
    int elements;
    double value;
    double minimum, maximum;
    int clock;
    GtkWidget *button;
    GtkWidget *calendar;
    GtkWidget *hour, *minute, *second;
    GtkWidget *meridiem;
} GKDate;

static GDateTime *gkUTC(double seconds) {
    GDateTime *value = g_date_time_new_from_unix_utc((gint64)floor(seconds));
    return value ? value : g_date_time_new_from_unix_utc(0);
}

static void gkDateLabel(GKView *view) {
    GKDate *date = view->ext;
    GDateTime *value = gkUTC(date->value);
    char *text = g_date_time_format(value, "%x");
    gtk_menu_button_set_label(GTK_MENU_BUTTON(date->button), text);
    gtk_accessible_update_property(GTK_ACCESSIBLE(date->button), GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, text, -1);
    g_free(text);
    g_date_time_unref(value);
}

static void gkDateCalendarSet(GKView *view) {
    GKDate *date = view->ext;
    GDateTime *value = gkUTC(date->value);
    view->suppress++;
    gtk_calendar_select_day(GTK_CALENDAR(date->calendar), value);
    view->suppress--;
    g_date_time_unref(value);
}

static void gkDateSelected(GtkCalendar *calendar, gpointer data) {
    GKView *view = data;
    if (view->suppress) {
        return;
    }
    GKDate *date = view->ext;
    GDateTime *selected = gtk_calendar_get_date(calendar);
    GDateTime *utc = g_date_time_new_utc(g_date_time_get_year(selected), g_date_time_get_month(selected),
                                         g_date_time_get_day_of_month(selected), 0, 0, 0);
    double value = (double)g_date_time_to_unix(utc);
    g_date_time_unref(utc);
    g_date_time_unref(selected);
    if (date->minimum != 0 && value < date->minimum) {
        value = date->minimum;
    }
    if (date->maximum != 0 && value > date->maximum) {
        value = date->maximum;
    }
    date->value = value;
    gkDateCalendarSet(view);
    gkDateLabel(view);
    gkReport(view, GK_EVENT_VALUE, value, 0, NULL);
}

static int gkTimeHour(GKDate *date) {
    int hour = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(date->hour));
    if (date->meridiem) {
        int pm = gtk_drop_down_get_selected(GTK_DROP_DOWN(date->meridiem)) == 1;
        hour = hour % 12 + (pm ? 12 : 0);
    }
    return hour;
}

static void gkTimeShow(GKView *view) {
    GKDate *date = view->ext;
    int seconds = (int)fmod(fmod(floor(date->value), 86400) + 86400, 86400);
    int hour = seconds / 3600;
    view->suppress++;
    if (date->meridiem) {
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(date->hour), hour % 12 == 0 ? 12 : hour % 12);
        gtk_drop_down_set_selected(GTK_DROP_DOWN(date->meridiem), hour >= 12);
    } else {
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(date->hour), hour);
    }
    gtk_spin_button_set_value(GTK_SPIN_BUTTON(date->minute), (seconds / 60) % 60);
    if (date->second) {
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(date->second), seconds % 60);
    }
    view->suppress--;
}

static void gkTimeChanged(GtkWidget *widget, gpointer data) {
    (void)widget;
    GKView *view = data;
    if (view->suppress) {
        return;
    }
    GKDate *date = view->ext;
    int seconds = gkTimeHour(date) * 3600 + gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(date->minute)) * 60;
    if (date->second) {
        seconds += gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(date->second));
    }
    date->value = seconds;
    gkReport(view, GK_EVENT_VALUE, seconds, 0, NULL);
}

static void gkTimeNotify(GObject *object, GParamSpec *spec, gpointer data) {
    (void)spec;
    gkTimeChanged(GTK_WIDGET(object), data);
}

static gboolean gkTimeOutput(GtkSpinButton *spin, gpointer data) {
    (void)data;
    char text[8];
    snprintf(text, sizeof(text), "%02d", gtk_spin_button_get_value_as_int(spin));
    gtk_editable_set_text(GTK_EDITABLE(spin), text);
    return TRUE;
}

static GtkWidget *gkTimeSpin(int minimum, int maximum, const char *label) {
    GtkWidget *spin = gtk_spin_button_new_with_range(minimum, maximum, 1);
    gtk_spin_button_set_wrap(GTK_SPIN_BUTTON(spin), TRUE);
    gtk_spin_button_set_numeric(GTK_SPIN_BUTTON(spin), TRUE);
    gtk_orientable_set_orientation(GTK_ORIENTABLE(spin), GTK_ORIENTATION_VERTICAL);
    gtk_accessible_update_property(GTK_ACCESSIBLE(spin), GTK_ACCESSIBLE_PROPERTY_LABEL, label, -1);
    g_signal_connect(spin, "output", G_CALLBACK(gkTimeOutput), NULL);
    return spin;
}

static int gkLocaleUses12Hour(void) {
    GDateTime *noon = g_date_time_new_utc(2000, 1, 1, 13, 0, 0);
    char *text = g_date_time_format(noon, "%X");
    int twelve = text && strstr(text, "13") == NULL;
    g_free(text);
    g_date_time_unref(noon);
    return twelve;
}

void *gkGTKDatePickerCreate(uintptr_t window, int style, int elements, double value, double minimum, double maximum,
                            int range, double interval, int clock) {
    (void)range;
    (void)interval;
    GKDate *date = g_new0(GKDate, 1);
    date->elements = elements;
    date->value = value;
    date->minimum = minimum;
    date->maximum = maximum;
    date->clock = clock;
    if (elements & GK_ELEMENTS_DATE) {
        GtkWidget *outer;
        date->calendar = gtk_calendar_new();
        if (style == 1) {
            outer = date->calendar;
        } else {
            date->button = gtk_menu_button_new();
            GtkWidget *popover = gtk_popover_new();
            gtk_popover_set_child(GTK_POPOVER(popover), date->calendar);
            gtk_menu_button_set_popover(GTK_MENU_BUTTON(date->button), popover);
            outer = date->button;
        }
        GKView *view = gkControl(window, outer, date->calendar, GK_KIND_DATE);
        view->ext = date;
        view->ext_free = g_free;
        gkDateCalendarSet(view);
        if (date->button) {
            gkDateLabel(view);
        }
        g_signal_connect(date->calendar, "day-selected", G_CALLBACK(gkDateSelected), view);
        return outer;
    }
    GtkWidget *box = g_object_new(GTK_TYPE_BOX, "orientation", GTK_ORIENTATION_HORIZONTAL, "spacing", 2,
                                  "accessible-role", GTK_ACCESSIBLE_ROLE_GROUP, NULL);
    int twelve = clock == 1 || (clock == 0 && gkLocaleUses12Hour());
    date->hour = twelve ? gkTimeSpin(1, 12, "Hour") : gkTimeSpin(0, 23, "Hour");
    date->minute = gkTimeSpin(0, 59, "Minute");
    gtk_box_append(GTK_BOX(box), date->hour);
    gtk_box_append(GTK_BOX(box), gtk_label_new(":"));
    gtk_box_append(GTK_BOX(box), date->minute);
    if (elements & GK_ELEMENTS_SECONDS) {
        date->second = gkTimeSpin(0, 59, "Second");
        gtk_box_append(GTK_BOX(box), gtk_label_new(":"));
        gtk_box_append(GTK_BOX(box), date->second);
    }
    if (twelve) {
        const char *names[] = {"AM", "PM", NULL};
        date->meridiem = gtk_drop_down_new_from_strings(names);
        gtk_widget_set_valign(date->meridiem, GTK_ALIGN_CENTER);
        gtk_box_append(GTK_BOX(box), date->meridiem);
    }
    GKView *view = gkControl(window, box, date->hour, GK_KIND_DATE);
    view->ext = date;
    view->ext_free = g_free;
    gkTimeShow(view);
    g_signal_connect(date->hour, "value-changed", G_CALLBACK(gkTimeChanged), view);
    g_signal_connect(date->minute, "value-changed", G_CALLBACK(gkTimeChanged), view);
    if (date->second) {
        g_signal_connect(date->second, "value-changed", G_CALLBACK(gkTimeChanged), view);
    }
    if (date->meridiem) {
        g_signal_connect(date->meridiem, "notify::selected", G_CALLBACK(gkTimeNotify), view);
    }
    return box;
}

void gkGTKDatePickerValue(void *pointer, double *value, double *interval) {
    GKView *view = gkView(pointer);
    GKDate *date = view && view->kind == GK_KIND_DATE ? view->ext : NULL;
    *value = date ? date->value : 0;
    *interval = 0;
}

void gkGTKDatePickerSet(void *pointer, double value, double interval) {
    (void)interval;
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_DATE) {
        return;
    }
    GKDate *date = view->ext;
    date->value = value;
    if (date->calendar) {
        gkDateCalendarSet(view);
        if (date->button) {
            gkDateLabel(view);
        }
    } else {
        gkTimeShow(view);
    }
}

/* ---- Calendar ----

   GtkCalendar selects one day. A range or a set of dates shows as marked
   days of the visible month, and the selected day of GtkCalendar only moves
   the visible month. */

typedef struct {
    int selection;
    GArray *dates;
    int anchor;
} GKCalendar;

static void gkCalendarFree(gpointer data) {
    GKCalendar *calendar = data;
    g_array_unref(calendar->dates);
    g_free(calendar);
}

static int gkDayNumber(double seconds) {
    return (int)floor(seconds / 86400);
}

static void gkCalendarMark(GKView *view) {
    GKCalendar *state = view->ext;
    GtkCalendar *calendar = GTK_CALENDAR(view->control);
    gtk_calendar_clear_marks(calendar);
    GDateTime *shown = gtk_calendar_get_date(calendar);
    int year = g_date_time_get_year(shown), month = g_date_time_get_month(shown);
    g_date_time_unref(shown);
    double first = -1, last = -1;
    if (state->selection == 2 && state->dates->len > 0) {
        first = g_array_index(state->dates, double, 0);
        last = state->dates->len > 1 ? g_array_index(state->dates, double, 1) : first;
    }
    for (int day = 1; day <= 31; day++) {
        GDateTime *value = g_date_time_new_utc(year, month, day, 0, 0, 0);
        if (!value || g_date_time_get_month(value) != month) {
            if (value) {
                g_date_time_unref(value);
            }
            break;
        }
        double seconds = (double)g_date_time_to_unix(value);
        g_date_time_unref(value);
        int marked = 0;
        if (state->selection == 2) {
            marked = first >= 0 && gkDayNumber(seconds) >= gkDayNumber(first) && gkDayNumber(seconds) <= gkDayNumber(last);
        } else if (state->selection == 1) {
            for (guint i = 0; i < state->dates->len; i++) {
                marked |= gkDayNumber(g_array_index(state->dates, double, i)) == gkDayNumber(seconds);
            }
        }
        if (marked) {
            gtk_calendar_mark_day(calendar, (guint)day);
        }
    }
}

static int gkCompareDates(gconstpointer a, gconstpointer b) {
    double x = *(const double *)a, y = *(const double *)b;
    return x < y ? -1 : x > y ? 1 : 0;
}

static void gkCalendarShowMonth(GKView *view, double seconds, int day) {
    GDateTime *value = gkUTC(seconds);
    GDateTime *local = g_date_time_new_local(g_date_time_get_year(value), g_date_time_get_month(value),
                                             day > 0 ? day : g_date_time_get_day_of_month(value), 12, 0, 0);
    view->suppress++;
    gtk_calendar_select_day(GTK_CALENDAR(view->control), local);
    view->suppress--;
    g_date_time_unref(local);
    g_date_time_unref(value);
    gkCalendarMark(view);
}

static void gkCalendarDaySelected(GtkCalendar *calendar, gpointer data) {
    GKView *view = data;
    if (view->suppress) {
        return;
    }
    GKCalendar *state = view->ext;
    GDateTime *selected = gtk_calendar_get_date(calendar);
    GDateTime *utc = g_date_time_new_utc(g_date_time_get_year(selected), g_date_time_get_month(selected),
                                         g_date_time_get_day_of_month(selected), 0, 0, 0);
    double seconds = (double)g_date_time_to_unix(utc);
    g_date_time_unref(utc);
    g_date_time_unref(selected);
    if (state->selection == 0) {
        g_array_set_size(state->dates, 0);
        g_array_append_val(state->dates, seconds);
    } else if (state->selection == 2) {
        if (state->anchor || state->dates->len == 0) {
            g_array_set_size(state->dates, 0);
            g_array_append_val(state->dates, seconds);
            state->anchor = 0;
        } else {
            g_array_append_val(state->dates, seconds);
            g_array_set_size(state->dates, 2);
            g_array_sort(state->dates, gkCompareDates);
            state->anchor = 1;
        }
    } else {
        int removed = 0;
        for (guint i = 0; i < state->dates->len; i++) {
            if (gkDayNumber(g_array_index(state->dates, double, i)) == gkDayNumber(seconds)) {
                g_array_remove_index(state->dates, i);
                removed = 1;
                break;
            }
        }
        if (!removed) {
            g_array_append_val(state->dates, seconds);
            g_array_sort(state->dates, gkCompareDates);
        }
    }
    gkCalendarMark(view);
    gkReport(view, GK_EVENT_VALUE, seconds, 0, NULL);
}

static void gkCalendarMonthChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkCalendarMark(data);
}

void *gkGTKCalendarCreate(uintptr_t window, int selection, const double *dates, int count, double visible) {
    GtkWidget *calendar = gtk_calendar_new();
    GKView *view = gkControl(window, calendar, NULL, GK_KIND_CALENDAR);
    GKCalendar *state = g_new0(GKCalendar, 1);
    state->selection = selection;
    state->dates = g_array_new(FALSE, FALSE, sizeof(double));
    state->anchor = 1;
    view->ext = state;
    view->ext_free = gkCalendarFree;
    gtk_calendar_set_show_day_names(GTK_CALENDAR(calendar), TRUE);
    gkGTKCalendarSetDates(calendar, dates, count);
    if (visible != 0) {
        gkGTKCalendarSetVisibleMonth(calendar, visible);
    }
    g_signal_connect(calendar, "day-selected", G_CALLBACK(gkCalendarDaySelected), view);
    g_signal_connect(calendar, "notify::month", G_CALLBACK(gkCalendarMonthChanged), view);
    g_signal_connect(calendar, "notify::year", G_CALLBACK(gkCalendarMonthChanged), view);
    return calendar;
}

int gkGTKCalendarDates(void *pointer, double *dates, int capacity) {
    GKView *view = gkView(pointer);
    GKCalendar *state = view && view->kind == GK_KIND_CALENDAR ? view->ext : NULL;
    if (!state) {
        return 0;
    }
    int count = (int)state->dates->len;
    for (int i = 0; i < count && i < capacity; i++) {
        dates[i] = g_array_index(state->dates, double, i);
    }
    return count;
}

void gkGTKCalendarSetDates(void *pointer, const double *dates, int count) {
    GKView *view = gkView(pointer);
    GKCalendar *state = view && view->kind == GK_KIND_CALENDAR ? view->ext : NULL;
    if (!state) {
        return;
    }
    g_array_set_size(state->dates, 0);
    for (int i = 0; i < count; i++) {
        g_array_append_val(state->dates, dates[i]);
    }
    g_array_sort(state->dates, gkCompareDates);
    state->anchor = 1;
    if (count > 0) {
        gkCalendarShowMonth(view, dates[0], 0);
    } else {
        gkCalendarMark(view);
    }
}

double gkGTKCalendarVisibleMonth(void *pointer) {
    GKView *view = gkView(pointer);
    if (!view) {
        return 0;
    }
    GDateTime *shown = gtk_calendar_get_date(GTK_CALENDAR(view->control));
    GDateTime *month = g_date_time_new_utc(g_date_time_get_year(shown), g_date_time_get_month(shown), 1, 0, 0, 0);
    double seconds = (double)g_date_time_to_unix(month);
    g_date_time_unref(month);
    g_date_time_unref(shown);
    return seconds;
}

void gkGTKCalendarSetVisibleMonth(void *pointer, double month) {
    GKView *view = gkView(pointer);
    GKCalendar *state = view && view->kind == GK_KIND_CALENDAR ? view->ext : NULL;
    if (!state) {
        return;
    }
    /* Keep the selected day when it is in the month, so GTK highlights it. */
    GDateTime *target = gkUTC(month);
    int day = 1;
    for (guint i = 0; i < state->dates->len; i++) {
        GDateTime *date = gkUTC(g_array_index(state->dates, double, i));
        if (g_date_time_get_year(date) == g_date_time_get_year(target) &&
            g_date_time_get_month(date) == g_date_time_get_month(target)) {
            day = g_date_time_get_day_of_month(date);
        }
        g_date_time_unref(date);
    }
    g_date_time_unref(target);
    gkCalendarShowMonth(view, month, day);
}

/* ---- Color ---- */

static void gkColorChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkReport(data, GK_EVENT_VALUE, 0, 0, NULL);
}

void *gkGTKColorWellCreate(uintptr_t window, double r, double g, double b, double a, int allows_alpha) {
    GtkColorDialog *dialog = gtk_color_dialog_new();
    gtk_color_dialog_set_with_alpha(dialog, allows_alpha != 0);
    GtkWidget *button = gtk_color_dialog_button_new(dialog);
    GdkRGBA color = {(float)r, (float)g, (float)b, (float)a};
    gtk_color_dialog_button_set_rgba(GTK_COLOR_DIALOG_BUTTON(button), &color);
    GKView *view = gkControl(window, button, NULL, GK_KIND_COLOR);
    g_signal_connect(button, "notify::rgba", G_CALLBACK(gkColorChanged), view);
    return button;
}

void gkGTKColorWellColor(void *pointer, double *rgba) {
    GKView *view = gkView(pointer);
    const GdkRGBA *color = view ? gtk_color_dialog_button_get_rgba(GTK_COLOR_DIALOG_BUTTON(view->control)) : NULL;
    rgba[0] = color ? color->red : 0;
    rgba[1] = color ? color->green : 0;
    rgba[2] = color ? color->blue : 0;
    rgba[3] = color ? color->alpha : 1;
}

void gkGTKColorWellSet(void *pointer, double r, double g, double b, double a) {
    GKView *view = gkView(pointer);
    if (!view) {
        return;
    }
    GdkRGBA color = {(float)r, (float)g, (float)b, (float)a};
    view->suppress++;
    gtk_color_dialog_button_set_rgba(GTK_COLOR_DIALOG_BUTTON(view->control), &color);
    view->suppress--;
}

/* ---- Font ---- */

static char *gkSystemFamily(void) {
    char *name = NULL;
    GtkSettings *settings = gtk_settings_get_default();
    if (settings) {
        g_object_get(settings, "gtk-font-name", &name, NULL);
    }
    PangoFontDescription *description = pango_font_description_from_string(name ? name : "Sans 11");
    char *family = g_strdup(pango_font_description_get_family(description));
    pango_font_description_free(description);
    g_free(name);
    return family ? family : g_strdup("Sans");
}

static PangoFontDescription *gkFontDescription(const char *family, double size, int weight, int italic) {
    PangoFontDescription *description = pango_font_description_new();
    char *system = NULL;
    if (!family || !*family || strcmp(family, "System") == 0) {
        system = gkSystemFamily();
        family = system;
    } else if (strcmp(family, "Monospace") == 0) {
        family = "Monospace";
    }
    pango_font_description_set_family(description, family);
    pango_font_description_set_size(description, (gint)((size > 0 ? size : 11) * PANGO_SCALE));
    pango_font_description_set_weight(description, weight > 0 ? (PangoWeight)weight : PANGO_WEIGHT_NORMAL);
    pango_font_description_set_style(description, italic ? PANGO_STYLE_ITALIC : PANGO_STYLE_NORMAL);
    g_free(system);
    return description;
}

static void gkFontChanged(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    GKView *view = data;
    if (!view->suppress) {
        g_object_set_data(G_OBJECT(view->outer), "gk-system", NULL);
    }
    gkReport(view, GK_EVENT_VALUE, 0, 0, NULL);
}

void *gkGTKFontPickerCreate(uintptr_t window, const char *family, double size, int weight, int italic) {
    GtkWidget *button = gtk_font_dialog_button_new(gtk_font_dialog_new());
    gtk_font_dialog_button_set_level(GTK_FONT_DIALOG_BUTTON(button), GTK_FONT_LEVEL_FONT);
    GKView *view = gkControl(window, button, NULL, GK_KIND_FONT);
    gkGTKFontPickerSet(button, family, size, weight, italic);
    g_signal_connect(button, "notify::font-desc", G_CALLBACK(gkFontChanged), view);
    return button;
}

void gkGTKFontPickerSet(void *pointer, const char *family, double size, int weight, int italic) {
    GKView *view = gkView(pointer);
    if (!view || !gkFontFamilyInstalled(family)) {
        return;
    }
    PangoFontDescription *description = gkFontDescription(family, size, weight, italic);
    view->suppress++;
    gtk_font_dialog_button_set_font_desc(GTK_FONT_DIALOG_BUTTON(view->control), description);
    view->suppress--;
    pango_font_description_free(description);
    int system = !family || !*family || strcmp(family, "System") == 0;
    g_object_set_data(G_OBJECT(view->outer), "gk-system", GINT_TO_POINTER(system));
}

char *gkGTKFontPickerFont(void *pointer, double *size, int *weight, int *italic) {
    GKView *view = gkView(pointer);
    PangoFontDescription *description =
        view ? gtk_font_dialog_button_get_font_desc(GTK_FONT_DIALOG_BUTTON(view->control)) : NULL;
    if (!description) {
        *size = 0;
        *weight = 400;
        *italic = 0;
        return gkStrdup("System");
    }
    *size = pango_font_description_get_size(description) / (double)PANGO_SCALE;
    *weight = (int)pango_font_description_get_weight(description);
    *italic = pango_font_description_get_style(description) != PANGO_STYLE_NORMAL;
    if (g_object_get_data(G_OBJECT(view->outer), "gk-system")) {
        return gkStrdup("System");
    }
    return gkStrdup(pango_font_description_get_family(description));
}

/* ---- Key recorder ---- */

typedef struct {
    char *display;
    int recording;
} GKRecorder;

static void gkRecorderFree(gpointer data) {
    GKRecorder *recorder = data;
    g_free(recorder->display);
    g_free(recorder);
}

static void gkRecorderUpdate(GKView *view) {
    GKRecorder *recorder = view->ext;
    const char *text = recorder->recording           ? "Type shortcut…"
                       : recorder->display && *recorder->display ? recorder->display
                                                                 : "Record Shortcut";
    gtk_button_set_label(GTK_BUTTON(view->control), text);
    gtk_accessible_update_property(GTK_ACCESSIBLE(view->control), GTK_ACCESSIBLE_PROPERTY_LABEL, "Shortcut",
                                   GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT, recorder->display ? recorder->display : "",
                                   -1);
}

static void gkRecorderClicked(GtkButton *button, gpointer data) {
    (void)button;
    GKView *view = data;
    GKRecorder *recorder = view->ext;
    recorder->recording = 1;
    gtk_widget_grab_focus(view->control);
    gkRecorderUpdate(view);
}

static gboolean gkRecorderKey(GtkEventControllerKey *controller, guint keyval, guint keycode, GdkModifierType state,
                              gpointer data) {
    (void)controller;
    GKView *view = data;
    GKRecorder *recorder = view->ext;
    if (!recorder->recording) {
        return FALSE;
    }
    if (keyval == GDK_KEY_Shift_L || keyval == GDK_KEY_Shift_R || keyval == GDK_KEY_Control_L ||
        keyval == GDK_KEY_Control_R || keyval == GDK_KEY_Alt_L || keyval == GDK_KEY_Alt_R ||
        keyval == GDK_KEY_Super_L || keyval == GDK_KEY_Super_R || keyval == GDK_KEY_Meta_L ||
        keyval == GDK_KEY_Meta_R) {
        return TRUE;
    }
    GdkModifierType modifiers = state & (GDK_CONTROL_MASK | GDK_ALT_MASK | GDK_SHIFT_MASK | GDK_SUPER_MASK);
    recorder->recording = 0;
    if (keyval == GDK_KEY_Escape && !(modifiers & (GDK_CONTROL_MASK | GDK_ALT_MASK))) {
        gkRecorderUpdate(view);
        return TRUE;
    }
    if ((keyval == GDK_KEY_BackSpace || keyval == GDK_KEY_Delete) && !modifiers) {
        gkRecorderUpdate(view);
        gkSend(view->handle, GK_EVENT_VALUE, 0, 0, 0, 0, "");
        return TRUE;
    }
    int bits = 0;
    if (modifiers & GDK_CONTROL_MASK) {
        bits |= 1;
    }
    if (modifiers & GDK_SUPER_MASK) {
        bits |= 2;
    }
    if (modifiers & GDK_ALT_MASK) {
        bits |= 4;
    }
    if (modifiers & GDK_SHIFT_MASK) {
        bits |= 8;
    }
    gunichar c = gdk_keyval_to_unicode(gdk_keyval_to_lower(keyval));
    char text[8] = {0};
    if (c >= 0x20) {
        g_unichar_to_utf8(g_unichar_toupper(c), text);
    }
    GKGTKEvent report = {0};
    report.kind = GK_EVENT_VALUE;
    report.a = bits;
    report.b = gkKeyFromKeycode(keycode);
    report.text = text;
    if (view->handle) {
        gkGTKGoEvent(view->handle, &report);
    }
    gkRecorderUpdate(view);
    return TRUE;
}

static void gkRecorderFocusLeave(GtkEventControllerFocus *controller, gpointer data) {
    (void)controller;
    GKView *view = data;
    ((GKRecorder *)view->ext)->recording = 0;
    gkRecorderUpdate(view);
}

void *gkGTKKeyRecorderCreate(uintptr_t window) {
    GtkWidget *button = gtk_button_new();
    GKView *view = gkControl(window, button, NULL, GK_KIND_KEY_RECORDER);
    view->ext = g_new0(GKRecorder, 1);
    view->ext_free = gkRecorderFree;
    gkRecorderUpdate(view);
    g_signal_connect(button, "clicked", G_CALLBACK(gkRecorderClicked), view);
    GtkEventController *keys = gtk_event_controller_key_new();
    gtk_event_controller_set_propagation_phase(keys, GTK_PHASE_CAPTURE);
    g_signal_connect(keys, "key-pressed", G_CALLBACK(gkRecorderKey), view);
    gtk_widget_add_controller(button, keys);
    GtkEventController *focus = gtk_event_controller_focus_new();
    g_signal_connect(focus, "leave", G_CALLBACK(gkRecorderFocusLeave), view);
    gtk_widget_add_controller(button, focus);
    return button;
}

void gkGTKKeyRecorderSet(void *pointer, const char *display) {
    GKView *view = gkView(pointer);
    if (!view || view->kind != GK_KIND_KEY_RECORDER) {
        return;
    }
    GKRecorder *recorder = view->ext;
    g_free(recorder->display);
    recorder->display = g_strdup(display ? display : "");
    gkRecorderUpdate(view);
}
