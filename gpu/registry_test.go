package gpu_test

import (
	"math"
	"os"
	"os/exec"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
)

// The registry tests only select backends. They never call their methods.
type registryBackend struct {
	gpu.Backend
}

func TestBackendRegistry(t *testing.T) {
	// Registration is permanent. A separate process protects the GPU tests from
	// these high-priority entries, including in shuffled and repeated runs.
	const marker = "GAMEKIT_TEST_GPU_REGISTRY"
	if os.Getenv(marker) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestBackendRegistry$")
		cmd.Env = append(os.Environ(), marker+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("registry subprocess: %v\n%s", err, output)
		}
		return
	}

	low, high, tie := &registryBackend{}, &registryBackend{}, &registryBackend{}
	gpu.RegisterBackend(low, "conformance-low", math.MaxInt-1)
	gpu.RegisterBackend(high, "conformance-high", math.MaxInt)
	gpu.RegisterBackend(tie, "conformance-tie", math.MaxInt)
	if !gpu.HasBackend() {
		t.Fatal("HasBackend is false after registration")
	}

	if got := gpu.Instance(nil); got != high {
		t.Fatalf("automatic selection = %p, want first highest-priority backend %p", got, high)
	}
	empty := ""
	if got := gpu.Instance(&empty); got != high {
		t.Fatalf("empty-name selection = %p, want %p", got, high)
	}
	name := "conformance-low"
	if got := gpu.Instance(&name); got != low {
		t.Fatalf("explicit lower-priority selection = %p, want %p", got, low)
	}
	if got, ok := gpu.Lookup("conformance-tie"); !ok || got != tie {
		t.Fatalf("Lookup = %p, %v; want %p, true", got, ok, tie)
	}
	if got, ok := gpu.Lookup("conformance-missing"); ok || got != nil {
		t.Fatalf("unknown Lookup = %v, %v; want nil, false", got, ok)
	}

	mustPanic(t, func() {
		missing := "conformance-missing"
		gpu.Instance(&missing)
	})
}
