//go:build windows && cgo

package win32_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui/win32"
)

func TestNativeCreateWindowRejectsInvalidSize(t *testing.T) {
	for _, size := range [][2]int{{0, 100}, {100, 0}, {-1, 100}, {100, -1}} {
		window, err := win32.CreateWindow("invalid", size[0], size[1], nil)
		if err == nil || window != nil {
			t.Fatalf("CreateWindow(%d, %d) = (%v, %v), want (nil, error)", size[0], size[1], window, err)
		}
	}
}

func TestZeroWindowCursorPosition(t *testing.T) {
	window := &win32.Window{}
	if x, y := window.GetPointerPos(); x != 0 || y != 0 {
		t.Fatalf("zero window cursor position = (%v, %v), want (0, 0)", x, y)
	}
}

func TestZeroWindowKeyState(t *testing.T) {
	window := &win32.Window{}
	if action := window.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("zero window escape action = %v, want KeyReleased", action)
	}
}
