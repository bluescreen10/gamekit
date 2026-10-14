//go:build darwin && cgo

package window

import "github.com/bluescreen10/gamekit/ui"

// BackendName identifies the Cocoa backend in the ui registry.
const BackendName = "cocoa"

func init() {
	ui.RegisterBackend(BackendName, func() ui.Backend {
		return &Backend{}
	})
}
