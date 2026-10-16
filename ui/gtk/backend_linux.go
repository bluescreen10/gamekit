//go:build linux && cgo

package gtk

import (
	"errors"

	"github.com/bluescreen10/gamekit/ui"
)

func createWindow(options ui.WindowOptions) (ui.Window, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	var owner *Window
	if options.Owner != nil {
		var ok bool
		owner, ok = options.Owner.(*Window)
		if !ok {
			return nil, errors.New("ui/gtk: owner belongs to another backend")
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
		Fullscreen:             options.Fullscreen,
		MinimumSize:            options.MinimumSize,
		MaximumSize:            options.MaximumSize,
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

var _ ui.Window = (*Window)(nil)
