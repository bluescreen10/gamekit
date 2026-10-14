//go:build linux && cgo

package gtk_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

func TestRegistersGTKBackend(t *testing.T) {
	backend, err := ui.New(&ui.Config{Backend: gtk.BackendName})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*gtk.Backend); !ok {
		t.Fatalf("New() = %T, want *gtk.Backend", backend)
	}
}
