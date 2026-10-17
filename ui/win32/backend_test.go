//go:build windows && cgo

package win32_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/win32"
)

func TestCreateWindowRejectsInvalidSize(t *testing.T) {
	backend := &win32.Backend{}

	created, err := backend.CreateWindow(ui.WindowOptions{
		Title: "invalid",
		Size:  ui.Size{Width: 0, Height: 480},
	})
	if err == nil || created != nil {
		t.Fatalf("CreateWindow() = (%v, %v), want (nil, error)", created, err)
	}
}
