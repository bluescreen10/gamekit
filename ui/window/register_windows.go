//go:build windows && cgo

package window

import "github.com/bluescreen10/gamekit/ui"

// BackendName identifies the Win32 backend in the ui registry.
const BackendName = "win32"

func init() {
	ui.RegisterBackend(BackendName, func() ui.Backend {
		return &Backend{}
	})
}
