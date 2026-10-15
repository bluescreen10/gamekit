//go:build !darwin || !cgo

package cocoa

import "github.com/bluescreen10/gamekit/ui"

func createWindow(ui.WindowOptions) (ui.Window, error) {
	return nil, ui.ErrNotSupported
}
