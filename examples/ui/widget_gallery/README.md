# GameKit widget gallery

The gallery shows every component of the `ui` package, with controls that
change their states, so that a native backend can be examined by hand. It uses
Cocoa on macOS, Win32 on Windows, and GTK 4 on Linux, through `ui/default`.

Run the gallery:

```sh
go run ./examples/ui/widget_gallery
```

Linux requires the GTK 4 development package.

## Pages

The sidebar groups the pages into sections. Drag the divider to the right of
the sidebar to change its width.

| Section | Pages |
| --- | --- |
| Controls | Buttons; toggles and choices; text input; values and ranges; pickers |
| Content | Text display; images and media; feedback |
| Layout | Layouts; containers |
| Data | Lists and tables; navigation |
| Commands | Menus and toolbars; dialogs |
| Platform | Window; widget behavior; input and canvas; services; event log |

Each page is a column of cards. A card shows one component, or one aspect of
it, and has controls that change its state: enabled and visible, checked and
mixed, read-only, busy, severities, orientations, and the other options of the
component. The gallery builds a page the first time that you select it.

The status bar at the bottom of the window shows the last event or result. The
Event log page keeps the last 300 of them.

The gallery also installs a menu bar with File, View, and Help menus. The View
menu changes the theme and opens the event log.

## Unsupported components

A backend returns or panics with an error that wraps `ui.ErrNotSupported` for
a feature that its platform cannot provide. The gallery shows the error in
place of the card content, or in the status bar for a setter, and continues
with the other components.

## Command-line options

- `-page ID` shows the page with the ID first, for example `-page dialogs`.
  The IDs are `buttons`, `toggles`, `text-input`, `values`, `pickers`, `text`,
  `media`, `feedback`, `layouts`, `containers`, `collections`, `navigation`,
  `menus`, `dialogs`, `window`, `behavior`, `input`, `services`, and `events`.
- `-smoke` builds and shows every page, prints each event, and exits. The exit
  status is 1 when a component is not available on the backend.
