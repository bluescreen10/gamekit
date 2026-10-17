//go:build windows && cgo

package win32_test

import (
	"math"
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui/win32"
)

// win32TestWindow is a hidden low-level window. TestMain creates it on the
// main thread.
var win32TestWindow *win32.Window
var win32TestWindowError error

func TestNativeWin32Window(t *testing.T) {
	if win32TestWindowError != nil {
		t.Fatal(win32TestWindowError)
	}
	if win32TestWindow.GetNativeHandle() == 0 {
		t.Fatal("native NSWindow handle is missing")
	}
	if width, height := win32TestWindow.GetSize(); width != 320 || height != 200 {
		t.Fatalf("window size = %dx%d, want 320x200", width, height)
	}
	x, y := win32TestWindow.GetPointerPos()
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatalf("cursor position = (%v, %v), want finite coordinates", x, y)
	}
	if action := win32TestWindow.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("initial escape action = %v, want KeyReleased", action)
	}
	win32TestWindow.SetShouldClose(true)
	if !win32TestWindow.ShouldClose() {
		t.Fatal("SetShouldClose(true) was ignored")
	}
}
