//go:build darwin && cgo

package cocoa_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/cocoa"
)

func TestRegistersBackend(t *testing.T) {
	backend, err := ui.New(&ui.Config{Backend: cocoa.BackendName})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*cocoa.Backend); !ok {
		t.Fatalf("New() = %T, want *cocoa.Backend", backend)
	}
}
