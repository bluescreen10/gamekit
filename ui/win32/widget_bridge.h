#ifndef GAMEKIT_WIN32_WIDGET_BRIDGE_H
#define GAMEKIT_WIN32_WIDGET_BRIDGE_H

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

void *gkWin32UICreateLayout(uintptr_t window);
void *gkWin32UICreateLabel(uintptr_t window, const char *text);
void *gkWin32UICreateButton(uintptr_t window, const char *text, uintptr_t handle);
void *gkWin32UICreateTextField(uintptr_t window, uintptr_t handle);
void *gkWin32UICreateTextArea(uintptr_t window, const char *text, const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle);
void *gkWin32UICreateCheckBox(uintptr_t window, const char *text, int checked, uintptr_t handle);
void *gkWin32UICreateRadioGroup(uintptr_t window, uintptr_t handle);
void *gkWin32UICreateSelect(uintptr_t window, uintptr_t handle);
void gkWin32UISelectionAddItem(void *control, const char *text);
int gkWin32UISelectionIndex(void *control);
void gkWin32UISetSelectionIndex(void *control, int index);
void *gkWin32UICreateSlider(uintptr_t window, double minimum, double maximum, double value, double step, uintptr_t handle);
void *gkWin32UICreateProgressBar(uintptr_t window, double value);
void *gkWin32UICreateActivityIndicator(uintptr_t window, int running);
void *gkWin32UICreateSeparator(uintptr_t window, int direction);
void *gkWin32UICreateImage(uintptr_t window, int scaling);
void gkWin32UIImageSet(void *control, const unsigned char *pixels, int width, int height, int stride);
int gkWin32UIImageScaling(void *control);
void gkWin32UIImageSetScaling(void *control, int scaling);
void *gkWin32UICreateLink(uintptr_t window, const char *text, const char *url, uintptr_t handle);
char *gkWin32UILinkURL(void *control);
void gkWin32UISetLinkURL(void *control, const char *url);
void *gkWin32UICreateTabs(uintptr_t window, uintptr_t handle);
int gkWin32UITabsAdd(void *tabs, const char *title, void *content);
int gkWin32UITabsSelected(void *tabs);
void gkWin32UITabsSetSelected(void *tabs, int index);
void gkWin32UITabsContentSize(void *tabs, int *width, int *height);
void *gkWin32UICreateScrollView(uintptr_t window, int axes);
void gkWin32UIScrollViewportSize(void *scroll, int *width, int *height);
void gkWin32UIScrollSetContentSize(void *scroll, int width, int height);
void gkWin32UIScrollOffset(void *scroll, int *x, int *y);
void *gkWin32UICreateCanvas(uintptr_t window, uintptr_t handle);
void gkWin32UICanvasPointerPosition(void *canvas, double *x, double *y);
void *gkWin32UICreateSection(uintptr_t window, const char *title, int expanded, uintptr_t handle);
uintptr_t gkWin32UIControlHandle(void *control);

void gkWin32UIDestroy(void *control);
void gkWin32UIClearAction(void *control);
void gkWin32UIAttachToWindow(uintptr_t window, void *control);
void gkWin32UISetParent(void *control, void *parent);
void gkWin32UIBringToFront(void *control);
void gkWin32UISetFrame(void *control, int x, int y, int width, int height);
void gkWin32UISetVisible(void *control, int visible);
void gkWin32UISetEnabled(void *control, int enabled);
void gkWin32UIPreferredSize(void *control, int *width, int *height);
void gkWin32UIFramebufferSize(void *control, int *width, int *height);

char *gkWin32UIText(void *control);
void gkWin32UISetText(void *control, const char *text);
void gkWin32UISetPlaceholder(void *control, const char *text);
char *gkWin32UIPlaceholder(void *control);
int gkWin32UIChecked(void *control);
void gkWin32UISetChecked(void *control, int checked);
int gkWin32UIReadOnly(void *control);
void gkWin32UISetReadOnly(void *control, int readOnly);
double gkWin32UISliderValue(void *control);
void gkWin32UISetSliderValue(void *control, double value);
double gkWin32UIProgressValue(void *control);
void gkWin32UISetProgressValue(void *control, double value);
int gkWin32UIActivityRunning(void *control);
void gkWin32UISetActivityRunning(void *control, int running);
int gkWin32UISectionExpanded(void *control);
void gkWin32UISectionSetExpanded(void *control, int expanded);

char *gkWin32UIOpenPanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories, int hidden, int packages, size_t *resultSize,
                    int *cancelled);
char *gkWin32UISavePanel(uintptr_t window, const char *title, const char *directory, const char *filename,
                    const char *extensions, int hidden, const char *defaultExtension, size_t *resultSize,
                    int *cancelled);

extern void gkWin32UIGoEvent(uintptr_t handle, int event);
extern void gkWin32UIGoCanvasPointerButton(uintptr_t handle, int button, int action, int mods);
extern void gkWin32UIGoCanvasScroll(uintptr_t handle, double x, double y);


#endif
