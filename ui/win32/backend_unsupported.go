//go:build !windows

package win32

import "github.com/bluescreen10/gamekit/ui"

func createWindow(ui.WindowOptions) (ui.Window, error) {
	return nil, ui.ErrNotSupported
}
