//go:build windows && cgo

package defaultui

import (
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/win32"
)

func init() {
	ui.SetDefaultBackend(win32.BackendName)
}
