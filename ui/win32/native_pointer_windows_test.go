//go:build windows && cgo

package win32_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui/win32"
)

// TestWin32PointerInitialState: a freshly created window reports nothing held and no
// scroll accumulated, which is what a caller diffing against the first frame assumes.
func TestWin32PointerInitialState(t *testing.T) {
	if win32TestWindowError != nil {
		t.Fatal(win32TestWindowError)
	}
	for _, button := range []pointer.Button{pointer.ButtonLeft, pointer.ButtonRight, pointer.ButtonMiddle} {
		if action := win32TestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("button %v = %v on a new window, want Released", button, action)
		}
	}
	if x, y := win32TestWindow.GetScroll(); x != 0 || y != 0 {
		t.Errorf("scroll = (%v, %v) on a new window, want (0, 0)", x, y)
	}
	if mode := win32TestWindow.GetPointerMode(); mode != pointer.Normal {
		t.Errorf("cursor mode = %v on a new window, want Normal", mode)
	}
}

// TestWin32PointerButtonOutOfRange: an index past the tracked buttons must report
// Released rather than reading off the end of the array.
func TestWin32PointerButtonOutOfRange(t *testing.T) {
	if win32TestWindowError != nil {
		t.Fatal(win32TestWindowError)
	}
	for _, button := range []pointer.Button{-1, pointer.ButtonCount, pointer.ButtonCount + 100} {
		if action := win32TestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("out-of-range button %v = %v, want Released", button, action)
		}
	}
}

// TestWin32PointerModeRoundTrips checks the mode actually reaches the native layer and
// comes back.
//
// Disabled is deliberately not exercised: it calls
// CGAssociateMouseAndMouseCursorPosition(false), which detaches the pointer for the
// whole system, and a test that failed midway would leave the developer's mouse frozen.
// Hidden takes the same path through gkWin32WindowSetCursorMode without that risk.
func TestWin32PointerModeRoundTrips(t *testing.T) {
	if win32TestWindowError != nil {
		t.Fatal(win32TestWindowError)
	}
	defer win32TestWindow.EnablePointer()

	win32TestWindow.SetPointerMode(pointer.Hidden)
	if mode := win32TestWindow.GetPointerMode(); mode != pointer.Hidden {
		t.Fatalf("cursor mode = %v after SetPointerMode(Hidden)", mode)
	}
	win32TestWindow.EnablePointer()
	if mode := win32TestWindow.GetPointerMode(); mode != pointer.Normal {
		t.Fatalf("cursor mode = %v after EnablePointer", mode)
	}
}

// TestWin32CallbacksRegisterOnANativeWindow: callbacks must attach to and detach from a
// real native window, not just the zero value the portable tests use.
//
// It stops short of calling PollEvents, which would be the interesting half: the
// native layer runs on the thread that created the first window (see Window's doc comment), and Go
// runs every test function on another goroutine, so polling here would use the native layer from
// the wrong thread rather than test anything. Exercising a real event round trip needs a harness that
// drives the loop from TestMain.
func TestWin32CallbacksRegisterOnANativeWindow(t *testing.T) {
	if win32TestWindowError != nil {
		t.Fatal(win32TestWindowError)
	}
	defer win32TestWindow.SetKeyCallback(nil)
	defer win32TestWindow.SetCharCallback(nil)
	defer win32TestWindow.SetScrollCallback(nil)
	defer win32TestWindow.SetPointerButtonCallback(nil)

	win32TestWindow.SetKeyCallback(func(*win32.Window, keyboard.Key, int, keyboard.KeyAction, keyboard.ModifierKey) {
	})
	win32TestWindow.SetCharCallback(func(*win32.Window, rune) {
	})
	win32TestWindow.SetScrollCallback(func(*win32.Window, float64, float64) {
	})
	win32TestWindow.SetPointerButtonCallback(func(*win32.Window, pointer.Button, pointer.ButtonAction, keyboard.ModifierKey) {
	})

	if prev := win32TestWindow.SetScrollCallback(nil); prev == nil {
		t.Fatal("the scroll callback did not survive registration on a native window")
	}
}
