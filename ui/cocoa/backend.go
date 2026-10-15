package cocoa

import "github.com/bluescreen10/gamekit/ui"

// Backend creates AppKit windows. Its zero value is ready for use.
type Backend struct{}

// CreateWindow creates an AppKit application or utility window.
func (b *Backend) CreateWindow(options ui.WindowOptions) (ui.Window, error) {
	return createWindow(options)
}

var _ ui.Backend = (*Backend)(nil)
