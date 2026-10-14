//go:build linux && cgo

package window

import "github.com/bluescreen10/gamekit/ui"

// BackendName identifies the X11 window backend in the ui registry.
const BackendName = "x11"

func init() {
	ui.RegisterBackend(BackendName, func() ui.Backend {
		return &Backend{}
	})
}
