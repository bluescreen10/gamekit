//go:build linux && cgo

package defaultui

import (
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

func init() {
	ui.SetDefaultBackend(gtk.BackendName)
}
