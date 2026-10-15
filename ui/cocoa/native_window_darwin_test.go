//go:build darwin && cgo

package cocoa_test

import (
	"math"
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/ui/cocoa"
)

// cocoaTestWindow is a hidden low-level window. TestMain creates it on the
// main thread.
var cocoaTestWindow *cocoa.Window
var cocoaTestWindowError error

func TestNativeCocoaWindow(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	if cocoaTestWindow.GetNativeHandle() == 0 {
		t.Fatal("native NSWindow handle is missing")
	}
	if width, height := cocoaTestWindow.GetSize(); width != 320 || height != 200 {
		t.Fatalf("window size = %dx%d, want 320x200", width, height)
	}
	x, y := cocoaTestWindow.GetPointerPos()
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatalf("cursor position = (%v, %v), want finite coordinates", x, y)
	}
	if action := cocoaTestWindow.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("initial escape action = %v, want KeyReleased", action)
	}
	cocoaTestWindow.SetShouldClose(true)
	if !cocoaTestWindow.ShouldClose() {
		t.Fatal("SetShouldClose(true) was ignored")
	}
}
