package window

import (
	"errors"

	"github.com/bluescreen10/gamekit/ui"
)

// Backend is GameKit's built-in native UI backend. Its zero value is ready for
// use.
type Backend struct{}

// CreateWindow creates a native application or utility window.
func (b *Backend) CreateWindow(options ui.WindowOptions) (ui.Window, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	var owner *Window
	if options.Owner != nil {
		var ok bool
		owner, ok = options.Owner.(*Window)
		if !ok {
			return nil, errors.New("ui/window: owner belongs to another backend")
		}
	}
	nativeOptions := WindowOptions{
		Kind:                   options.Kind,
		Owner:                  owner,
		Position:               options.Position,
		Resizable:              options.Resizable,
		Hidden:                 options.Hidden,
		Borderless:             options.Borderless,
		Maximized:              options.Maximized,
		AlwaysOnTop:            options.AlwaysOnTop,
		TransparentFramebuffer: options.TransparentFramebuffer,
		Theme:                  options.Theme,
	}

	window, err := CreateWindow(
		options.Title,
		options.Size.Width,
		options.Size.Height,
		&nativeOptions,
	)
	if err != nil {
		return nil, err
	}
	return window, nil
}

var _ ui.Backend = (*Backend)(nil)
var _ ui.Window = (*Window)(nil)
