//go:build linux && cgo

package gtk_test

import (
	"math"
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

// gtkTestWindow is a hidden low-level window. TestMain creates it on the
// main thread.
var gtkTestWindow *gtk.Window
var gtkTestWindowError error

func TestNativeGTKWindow(t *testing.T) {
	if gtkTestWindowError != nil {
		t.Fatal(gtkTestWindowError)
	}
	if gtkTestWindow.GetNativeHandle() == 0 {
		t.Fatal("native NSWindow handle is missing")
	}
	if width, height := gtkTestWindow.GetSize(); width != 320 || height != 200 {
		t.Fatalf("window size = %dx%d, want 320x200", width, height)
	}
	x, y := gtkTestWindow.GetPointerPos()
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatalf("cursor position = (%v, %v), want finite coordinates", x, y)
	}
	if action := gtkTestWindow.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("initial escape action = %v, want KeyReleased", action)
	}
	gtkTestWindow.SetShouldClose(true)
	if !gtkTestWindow.ShouldClose() {
		t.Fatal("SetShouldClose(true) was ignored")
	}
}
