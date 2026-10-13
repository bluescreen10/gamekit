<p align="center">
  <img src="logo/gamekit-logo.svg" alt="GameKit" width="560">
</p>

<p align="center">
  <a href="https://github.com/bluescreen10/gamekit/actions/workflows/test.yml"><img src="https://github.com/bluescreen10/gamekit/actions/workflows/test.yml/badge.svg" alt="Build status"></a>
  <a href="https://codecov.io/gh/bluescreen10/gamekit"><img src="https://codecov.io/gh/bluescreen10/gamekit/graph/badge.svg?branch=main" alt="Code coverage"></a>
</p>

GameKit is a low-level, cross-platform Go library for games and graphical tools.
It supplies native windows, input, UI controls, and a bindless GPU interface.

## Features

- Create native windows with Cocoa, Win32, or X11.
- Receive portable keyboard, pointer, text, and scroll input.
- Use native controls with Cocoa, Win32, or GTK 3.
- Send graphics work to Vulkan 1.3 or Metal 4.
- Use GPU addresses and a global bindless texture heap.

The GPU design follows Sebastian Aaltonen's
[No Graphics API](https://www.sebastianaaltonen.com/blog/no-graphics-api)
proposal.

## Requirements

- Install Go 1.27 or a later compatible version.
- Enable cgo and install a C compiler for native features.
- On Linux, install the X11 development files.
- Install the GTK 3 development files if you use the GTK backend.
- Use macOS 26 or later for the Metal backend.

The Vulkan backend also needs Vulkan headers, a loader, and a compatible
driver.

## Install

Add GameKit to your Go module:

```sh
go get github.com/bluescreen10/gamekit@latest
```

## Create a window

This example creates a native window and processes its events:

```go
package main

import (
	"runtime"
	"time"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

func main() {
	runtime.LockOSThread()

	backend := &window.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:     "GameKit",
		Size:      ui.Size{Width: 1280, Height: 720},
		Resizable: true,
	})
	if err != nil {
		panic(err)
	}
	defer win.Close()

	for !win.ShouldClose() {
		win.PollEvents()
		time.Sleep(time.Second / 120)
	}
}
```

Run the native UI example on macOS or Windows:

```sh
go run ./examples/ui
```

On Linux, run the example with the GTK build tag:

```sh
go run -tags gtk ./examples/ui
```

## Packages

| Package | Purpose |
| --- | --- |
| [`gpu`](https://pkg.go.dev/github.com/bluescreen10/gamekit/gpu) | Provides the bindless GPU interface. |
| [`gpu/vulkan`](https://pkg.go.dev/github.com/bluescreen10/gamekit/gpu/vulkan) | Implements the Vulkan backend. |
| [`gpu/metal`](https://pkg.go.dev/github.com/bluescreen10/gamekit/gpu/metal) | Implements the Metal backend. |
| [`ui`](https://pkg.go.dev/github.com/bluescreen10/gamekit/ui) | Defines the native UI protocol and layout types. |
| [`ui/window`](https://pkg.go.dev/github.com/bluescreen10/gamekit/ui/window) | Implements native windows and input. |
| [`ui/gtk`](https://pkg.go.dev/github.com/bluescreen10/gamekit/ui/gtk) | Implements GTK 3 controls on Linux. |
| [`keyboard`](https://pkg.go.dev/github.com/bluescreen10/gamekit/keyboard) | Defines portable keys and keyboard events. |
| [`pointer`](https://pkg.go.dev/github.com/bluescreen10/gamekit/pointer) | Defines portable pointer buttons and actions. |

See the [API reference](https://pkg.go.dev/github.com/bluescreen10/gamekit) for
all public types and functions.

## Test

Run all tests:

```sh
go test ./...
```
