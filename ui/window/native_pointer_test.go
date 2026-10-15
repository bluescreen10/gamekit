package window_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui/window"
)

// A zero Window stands in for one whose native handle is gone — after Destroy, or on a
// platform without native window support. Every accessor has to stay safe there rather
// than dereference a nil handle.

func TestZeroWindowPointerButton(t *testing.T) {
	win := &window.Window{}
	if action := win.GetPointerButton(pointer.ButtonLeft); action != pointer.Released {
		t.Fatalf("zero window left button = %v, want Released", action)
	}
}

func TestZeroWindowScroll(t *testing.T) {
	win := &window.Window{}
	if x, y := win.GetScroll(); x != 0 || y != 0 {
		t.Fatalf("zero window scroll = (%v, %v), want (0, 0)", x, y)
	}
}

func TestZeroWindowPointerMode(t *testing.T) {
	win := &window.Window{}
	if mode := win.GetPointerMode(); mode != pointer.Normal {
		t.Fatalf("zero window cursor mode = %v, want Normal", mode)
	}
	// Must not panic.
	win.DisablePointer()
	win.EnablePointer()
	win.SetPointerMode(pointer.Hidden)
}

// TestNilWindowCallbacks: registering on a nil *Window is a no-op rather than a panic,
// matching the rest of the Window API.
func TestNilWindowCallbacks(t *testing.T) {
	var win *window.Window
	if cb := win.SetKeyCallback(nil); cb != nil {
		t.Error("SetKeyCallback on a nil window returned a callback")
	}
	if cb := win.SetCharCallback(nil); cb != nil {
		t.Error("SetCharCallback on a nil window returned a callback")
	}
	if cb := win.SetScrollCallback(nil); cb != nil {
		t.Error("SetScrollCallback on a nil window returned a callback")
	}
	if cb := win.SetPointerButtonCallback(nil); cb != nil {
		t.Error("SetPointerButtonCallback on a nil window returned a callback")
	}
}

// TestCallbacksReturnThePrevious pins the GLFW contract: registering returns whatever
// was registered before, so a caller can chain or restore rather than clobber another
// consumer's handler.
func TestCallbacksReturnThePrevious(t *testing.T) {
	win := &window.Window{}

	first := window.ScrollCallback(func(*window.Window, float64, float64) {
	})
	if prev := win.SetScrollCallback(first); prev != nil {
		t.Error("the first registration reported a previous callback")
	}
	second := window.ScrollCallback(func(*window.Window, float64, float64) {
	})
	if prev := win.SetScrollCallback(second); prev == nil {
		t.Fatal("replacing a callback did not return the one it replaced")
	}
	if prev := win.SetScrollCallback(nil); prev == nil {
		t.Fatal("clearing a callback did not return the one it replaced")
	}
	if prev := win.SetScrollCallback(nil); prev != nil {
		t.Fatal("a cleared callback was still reported afterwards")
	}
}
