//go:build windows && cgo

package win32_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/win32"
)

func TestRegistersWin32Backend(t *testing.T) {
	backend, err := ui.New(&ui.Config{Backend: win32.BackendName})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*win32.Backend); !ok {
		t.Fatalf("New() = %T, want *win32.Backend", backend)
	}
}
