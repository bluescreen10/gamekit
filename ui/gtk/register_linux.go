//go:build linux && cgo

package gtk

import "github.com/bluescreen10/gamekit/ui"

// BackendName identifies the GTK 4 backend in the ui registry.
const BackendName = "gtk"

func init() {
	ui.RegisterBackend(BackendName, func() ui.Backend {
		return &Backend{}
	})
}
