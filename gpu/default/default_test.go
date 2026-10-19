//go:build cgo && (darwin || linux || windows)

package defaultgpu_test

import (
	"runtime"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	_ "github.com/bluescreen10/gamekit/gpu/default"
)

func TestRegistersPlatformDefault(t *testing.T) {
	name := "vulkan"
	if runtime.GOOS == "darwin" {
		name = "metal"
	}

	want, ok := gpu.Lookup(name)
	if !ok {
		t.Fatalf("platform backend %q is not registered", name)
	}

	// The macOS gamekit_vulkan opt-in preserves Vulkan's higher registry priority.
	if runtime.GOOS == "darwin" {
		if vulkan, ok := gpu.Lookup("vulkan"); ok {
			name, want = "vulkan", vulkan
		}
	}

	if got := gpu.Instance(nil); got != want {
		t.Fatalf("Instance(nil) = %T, want the registered %s backend %T", got, name, want)
	}

	empty := ""
	if got := gpu.Instance(&empty); got != want {
		t.Fatalf("Instance(empty name) = %T, want the registered %s backend %T", got, name, want)
	}
}
