//go:build darwin && cgo

package defaultui

import (
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/cocoa"
)

func init() {
	ui.SetDefaultBackend(cocoa.BackendName)
}
