//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan_test

import (
	"os"
	"strings"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/vulkan"
)

// Vulkan reads the workgroup size from the shader, so a pipeline that states another
// would dispatch differently here than on Metal, which takes the stated one.
func TestComputePipelineWorkgroupSizeMustMatchShader(t *testing.T) {
	b := vulkan.New()
	if err := b.Init(); err != nil {
		t.Skipf("vulkan device unavailable: %v", err)
	}
	defer b.Destroy()
	shader, err := os.ReadFile("../testdata/instanced.comp.spv") // local_size_x = 64
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("CreateComputePipeline with WorkgroupSize [32 1 1] for a [64 1 1] shader did not panic")
		}
		if message, _ := r.(string); !strings.Contains(message, "WorkgroupSize") || !strings.Contains(message, "[64 1 1]") {
			t.Errorf("CreateComputePipeline panicked with %v, want it to name WorkgroupSize and the shader's [64 1 1]", r)
		}
	}()
	b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: shader, WorkgroupSize: [3]uint32{32, 1, 1}, Label: "mismatch"})
}
