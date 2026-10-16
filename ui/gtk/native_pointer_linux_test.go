//go:build linux && cgo

package gtk_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

// TestGTKPointerInitialState: a freshly created window reports nothing held and no
// scroll accumulated, which is what a caller diffing against the first frame assumes.
func TestGTKPointerInitialState(t *testing.T) {
	if gtkTestWindowError != nil {
		t.Fatal(gtkTestWindowError)
	}
	for _, button := range []pointer.Button{pointer.ButtonLeft, pointer.ButtonRight, pointer.ButtonMiddle} {
		if action := gtkTestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("button %v = %v on a new window, want Released", button, action)
		}
	}
	if x, y := gtkTestWindow.GetScroll(); x != 0 || y != 0 {
		t.Errorf("scroll = (%v, %v) on a new window, want (0, 0)", x, y)
	}
	if mode := gtkTestWindow.GetPointerMode(); mode != pointer.Normal {
		t.Errorf("cursor mode = %v on a new window, want Normal", mode)
	}
}

// TestGTKPointerButtonOutOfRange: an index past the tracked buttons must report
// Released rather than reading off the end of the array.
func TestGTKPointerButtonOutOfRange(t *testing.T) {
	if gtkTestWindowError != nil {
		t.Fatal(gtkTestWindowError)
	}
	for _, button := range []pointer.Button{-1, pointer.ButtonCount, pointer.ButtonCount + 100} {
		if action := gtkTestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("out-of-range button %v = %v, want Released", button, action)
		}
	}
}

// TestGTKPointerModeRoundTrips checks the mode actually reaches the native layer and
// comes back.
//
// Disabled is deliberately not exercised: it calls
// CGAssociateMouseAndMouseCursorPosition(false), which detaches the pointer for the
// whole system, and a test that failed midway would leave the developer's mouse frozen.
// Hidden takes the same path through gkGTKWindowSetCursorMode without that risk.
func TestGTKPointerModeRoundTrips(t *testing.T) {
	if gtkTestWindowError != nil {
		t.Fatal(gtkTestWindowError)
	}
	defer gtkTestWindow.EnablePointer()

	gtkTestWindow.SetPointerMode(pointer.Hidden)
	if mode := gtkTestWindow.GetPointerMode(); mode != pointer.Hidden {
		t.Fatalf("cursor mode = %v after SetPointerMode(Hidden)", mode)
	}
	gtkTestWindow.EnablePointer()
	if mode := gtkTestWindow.GetPointerMode(); mode != pointer.Normal {
		t.Fatalf("cursor mode = %v after EnablePointer", mode)
	}
}

// TestGTKCallbacksRegisterOnANativeWindow: callbacks must attach to and detach from a
// real native window, not just the zero value the portable tests use.
//
// It stops short of calling PollEvents, which would be the interesting half: GTK
// runs on the thread that created the first window (see Window's doc comment), and Go
// runs every test function on another goroutine, so polling here would use GTK from
// the wrong thread rather than test anything. Exercising a real event round trip needs a harness that
// drives the loop from TestMain.
func TestGTKCallbacksRegisterOnANativeWindow(t *testing.T) {
	if gtkTestWindowError != nil {
		t.Fatal(gtkTestWindowError)
	}
	defer gtkTestWindow.SetKeyCallback(nil)
	defer gtkTestWindow.SetCharCallback(nil)
	defer gtkTestWindow.SetScrollCallback(nil)
	defer gtkTestWindow.SetPointerButtonCallback(nil)

	gtkTestWindow.SetKeyCallback(func(*gtk.Window, keyboard.Key, int, keyboard.KeyAction, keyboard.ModifierKey) {
	})
	gtkTestWindow.SetCharCallback(func(*gtk.Window, rune) {
	})
	gtkTestWindow.SetScrollCallback(func(*gtk.Window, float64, float64) {
	})
	gtkTestWindow.SetPointerButtonCallback(func(*gtk.Window, pointer.Button, pointer.ButtonAction, keyboard.ModifierKey) {
	})

	if prev := gtkTestWindow.SetScrollCallback(nil); prev == nil {
		t.Fatal("the scroll callback did not survive registration on a native window")
	}
}
