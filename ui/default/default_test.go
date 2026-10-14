//go:build cgo && (darwin || linux || windows)

package defaultui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

func TestRegistersPlatformDefault(t *testing.T) {
	backend, err := ui.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if backend == nil {
		t.Fatal("New(nil) returned a nil backend")
	}
}
