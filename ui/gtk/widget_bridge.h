#ifndef GAMEKIT_GTK_WIDGET_BRIDGE_H
#define GAMEKIT_GTK_WIDGET_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

enum {
    GK_UI_EVENT_CLICK = 1,
    GK_UI_EVENT_TEXT_CHANGE = 2,
    GK_UI_EVENT_VALUE_CHANGE = 3,
    GK_UI_EVENT_SECTION_CHANGE = 5,
    GK_UI_EVENT_TOGGLE_CHANGE = 6,
    GK_UI_EVENT_SELECTION_CHANGE = 7,
    GK_UI_EVENT_TAB_CHANGE = 8,
    GK_UI_EVENT_SUBMIT = 9,
};

void *gkGTKUICreateLayout(uintptr_t window);
void *gkGTKUICreateLabel(uintptr_t window, const char *text);
void *gkGTKUICreateButton(uintptr_t window, const char *text, uintptr_t handle);
void *gkGTKUICreateTextField(uintptr_t window, uintptr_t handle);
void *gkGTKUICreateTextArea(uintptr_t window, const char *text, const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle);
void *gkGTKUICreateCheckBox(uintptr_t window, const char *text, int checked, uintptr_t handle);
void *gkGTKUICreateRadioGroup(uintptr_t window, uintptr_t handle);
void *gkGTKUICreateSelect(uintptr_t window, uintptr_t handle);
void gkGTKUISelectionAddItem(void *control, const char *text);
int gkGTKUISelectionIndex(void *control);
void gkGTKUISetSelectionIndex(void *control, int index);
void *gkGTKUICreateSlider(uintptr_t window, double minimum, double maximum, double value, double step, uintptr_t handle);
void *gkGTKUICreateProgressBar(uintptr_t window, double value);
void *gkGTKUICreateActivityIndicator(uintptr_t window, int running);
void *gkGTKUICreateSeparator(uintptr_t window, int direction);
void *gkGTKUICreateImage(uintptr_t window, int scaling);
void gkGTKUIImageSet(void *control, const unsigned char *pixels, int width, int height, int stride);
int gkGTKUIImageScaling(void *control);
void gkGTKUIImageSetScaling(void *control, int scaling);
void *gkGTKUICreateLink(uintptr_t window, const char *text, const char *url, uintptr_t handle);
char *gkGTKUILinkURL(void *control);
void gkGTKUISetLinkURL(void *control, const char *url);
void *gkGTKUICreateTabs(uintptr_t window, uintptr_t handle);
int gkGTKUITabsAdd(void *tabs, const char *title, void *content);
int gkGTKUITabsSelected(void *tabs);
void gkGTKUITabsSetSelected(void *tabs, int index);
void gkGTKUITabsContentSize(void *tabs, int *width, int *height);
void *gkGTKUICreateScrollView(uintptr_t window, int axes);
void gkGTKUIScrollViewportSize(void *scroll, int *width, int *height);
void gkGTKUIScrollSetContentSize(void *scroll, int width, int height);
void gkGTKUIScrollOffset(void *scroll, int *x, int *y);
void *gkGTKUICreateCanvas(uintptr_t window, uintptr_t handle);
void gkGTKUICanvasPointerPosition(void *canvas, double *x, double *y);
void *gkGTKUICreateSection(uintptr_t window, const char *title, int expanded, uintptr_t handle);
uintptr_t gkGTKUIControlHandle(void *control);
/* gkGTKUICanvasXDisplay returns the Xlib Display of a canvas X11 window. */
uintptr_t gkGTKUICanvasXDisplay(void *canvas);

void gkGTKUIDestroy(void *control);
void gkGTKUIClearAction(void *control);
void gkGTKUIAttachToWindow(uintptr_t window, void *control);
void gkGTKUISetParent(void *control, void *parent);
void gkGTKUIBringToFront(void *control);
void gkGTKUISetFrame(void *control, int x, int y, int width, int height);
void gkGTKUISetVisible(void *control, int visible);
void gkGTKUISetEnabled(void *control, int enabled);
void gkGTKUIPreferredSize(void *control, int *width, int *height);
void gkGTKUIFramebufferSize(void *control, int *width, int *height);

char *gkGTKUIText(void *control);
void gkGTKUISetText(void *control, const char *text);
void gkGTKUISetPlaceholder(void *control, const char *text);
char *gkGTKUIPlaceholder(void *control);
int gkGTKUIChecked(void *control);
void gkGTKUISetChecked(void *control, int checked);
int gkGTKUIReadOnly(void *control);
void gkGTKUISetReadOnly(void *control, int readOnly);
double gkGTKUISliderValue(void *control);
void gkGTKUISetSliderValue(void *control, double value);
double gkGTKUIProgressValue(void *control);
void gkGTKUISetProgressValue(void *control, double value);
int gkGTKUIActivityRunning(void *control);
void gkGTKUISetActivityRunning(void *control, int running);
int gkGTKUISectionExpanded(void *control);
void gkGTKUISectionSetExpanded(void *control, int expanded);

char *gkGTKUIOpenPanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories, int hidden, int packages, size_t *resultSize,
                    int *cancelled);
char *gkGTKUISavePanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int hidden, const char *defaultExtension, size_t *resultSize,
                    int *cancelled);

extern void gkGTKUIGoEvent(uintptr_t handle, int event);
extern void gkGTKUIGoCanvasPointerButton(uintptr_t handle, int button, int action, int mods);
extern void gkGTKUIGoCanvasScroll(uintptr_t handle, double x, double y);


#endif
