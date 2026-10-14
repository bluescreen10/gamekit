//go:build darwin && cgo

package defaultui

import (
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

func init() {
	ui.SetDefaultBackend(window.BackendName)
}
