package gpu_test

import (
	"strings"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
)

// A compute pipeline must state its workgroup size: Metal has no other source for it.
func TestComputePipelineWithoutWorkgroupSizePanics(t *testing.T) {
	for _, size := range [][3]uint32{{0, 0, 0}, {64, 0, 1}} {
		b := testBackend(t)
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("CreateComputePipeline with WorkgroupSize %v did not panic", size)
				}
				if message, _ := r.(string); !strings.Contains(message, "WorkgroupSize") {
					t.Errorf("CreateComputePipeline with WorkgroupSize %v panicked with %v, want it to name WorkgroupSize", size, r)
				}
			}()
			b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: fillIndirect, WorkgroupSize: size, Label: "no-size"})
		}()
	}
}
