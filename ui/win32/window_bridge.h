#ifndef GAMEKIT_WIN32_WINDOW_BRIDGE_H
#define GAMEKIT_WIN32_WINDOW_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

enum {
    GK_WINDOW_RESIZABLE = 1u << 0,
    GK_WINDOW_HIDDEN = 1u << 1,
    GK_WINDOW_BORDERLESS = 1u << 2,
    GK_WINDOW_MAXIMIZED = 1u << 3,
    GK_WINDOW_ALWAYS_ON_TOP = 1u << 4,
    GK_WINDOW_TRANSPARENT = 1u << 5,
    GK_WINDOW_UTILITY = 1u << 6,
};

enum {
    GK_KEY_RELEASED = 0,
    GK_KEY_PRESSED = 1,
    GK_KEY_REPEAT = 2,
    GK_KEY_COUNT = 349,
};

/* Pointing-device buttons and their state. Numbering matches GLFW. */
enum {
    GK_POINTER_RELEASED = 0,
    GK_POINTER_PRESSED = 1,
    GK_POINTER_BUTTON_LEFT = 0,
    GK_POINTER_BUTTON_RIGHT = 1,
    GK_POINTER_BUTTON_MIDDLE = 2,
    GK_POINTER_BUTTON_COUNT = 8,
};

/* Modifier bits reported alongside key and button events. Matches GLFW. */
enum {
    GK_MOD_SHIFT = 1u << 0,
    GK_MOD_CONTROL = 1u << 1,
    GK_MOD_ALT = 1u << 2,
    GK_MOD_SUPER = 1u << 3,
    GK_MOD_CAPS_LOCK = 1u << 4,
    GK_MOD_NUM_LOCK = 1u << 5,
};

/* Cursor behaviour over a window. */
enum {
    GK_CURSOR_NORMAL = 0,
    GK_CURSOR_HIDDEN = 1,
    GK_CURSOR_DISABLED = 2,
};

void *gkWin32WindowCreate(const char *title, int width, int height, uint32_t flags, void *owner, int x, int y, char *error,
                     size_t errorSize);
void gkWin32WindowDestroy(void *window);
void gkWin32WindowPoll(void *window);
int gkWin32WindowShouldClose(void *window);
void gkWin32WindowSetShouldClose(void *window, int close);
void gkWin32WindowSize(void *window, int *width, int *height);
void gkWin32WindowFramebufferSize(void *window, int *width, int *height);
void gkWin32WindowSetSize(void *window, int width, int height);
void gkWin32WindowPosition(void *window, int *x, int *y);
void gkWin32WindowSetPosition(void *window, int x, int y);
int gkWin32WindowVisible(void *window);
void gkWin32WindowSetVisible(void *window, int visible);
void gkWin32WindowBringToFront(void *window);
void gkWin32WindowSetTitle(void *window, const char *title);
void gkWin32WindowSetTheme(void *window, int theme);
uintptr_t gkWin32WindowNativeHandle(void *window);
uintptr_t gkWin32WindowNativeDisplayHandle(void *window);
void *gkWin32WindowNativePointer(void *window);
void *gkWin32WindowNativeDisplay(void *window);
void gkWin32WindowCursorPosition(void *window, double *x, double *y);
/* Pointing device. */
void gkWin32WindowGetScroll(void *window, double *x, double *y);
void gkWin32WindowSetCursorMode(void *window, int mode);
int gkWin32WindowGetCursorMode(void *window);

/*
 * Associate a window with its Go counterpart. The value is a runtime/cgo Handle, which
 * is how a Go value is referred to from C — C may not hold a Go pointer directly, and a
 * handle is an opaque integer the Go side can resolve back. Zero means "not set", which
 * cgo.Handle never produces.
 */
void gkWin32WindowSetHandle(void *window, uintptr_t handle);

/*
 * Event dispatch.
 *
 * Rather than storing C function pointers per window, the platform layers call these
 * unconditionally and Go decides whether a callback is registered. That keeps the
 * platform code free of registration bookkeeping, and the cost — one cgo transition
 * per input event — is irrelevant at the handful of events a frame produces.
 *
 * Implemented in Go; see callbacks.go.
 */
extern void gkWin32GoKeyEvent(uintptr_t handle, int key, int scancode, int action, int mods);
extern void gkWin32GoCharEvent(uintptr_t handle, unsigned int codepoint);
extern void gkWin32GoScrollEvent(uintptr_t handle, double x, double y);
extern void gkWin32GoPointerButtonEvent(uintptr_t handle, int button, int action, int mods);
extern void gkWin32GoWindowMove(uintptr_t handle, int x, int y);

#endif
