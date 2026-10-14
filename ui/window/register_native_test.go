//go:build cgo && (darwin || linux || windows)

package window_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

func TestRegistersNativeBackend(t *testing.T) {
	backend, err := ui.New(&ui.Config{Backend: window.BackendName})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*window.Backend); !ok {
		t.Fatalf("New() = %T, want *window.Backend", backend)
	}
}
