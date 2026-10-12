# GameKit widget tester

This example displays the native widgets implemented by the `ui` package. It
uses Cocoa on macOS, Win32 on Windows, and GTK 3 on Linux.

The tabs cover editable controls, selection controls, images, a native scroll
view, and a GPU-presentable canvas. The scrolling page can be moved with the
mouse wheel, native scrollbars, or the Top/Middle/Bottom buttons.

The inspector is a `ui.Window` with `Kind: ui.WindowUtility` and an owner. It is
backed by `NSPanel`, a Win32 tool window, or a transient GTK window, so its title
bar, shadow, and movement are provided by the platform. Its native close button
hides the window; applications can clear its close request and show it again
with `SetShouldClose(false)` and `SetVisible(true)`. The tester's Show inspector
button demonstrates that lifecycle.

Run it on macOS or Windows:

```sh
go run ./examples/ui
```

Run it on Linux after installing the GTK 3 development package:

```sh
go run -tags gtk ./examples/ui
```

The Canvas tab exposes a native surface that a GPU backend can use for 2D or 3D
presentation. Its input labels show the separate window-wide and canvas-local
button and scroll streams.
