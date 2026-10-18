//go:build darwin && cgo

package metal_test

import (
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/metal"
)

// BenchmarkRecord measures the CPU cost of recording a typical pass: one render pass
// with 100 draws and 100 indexed draws, then a barrier. Submission and the GPU work are
// left out of the timing.
func BenchmarkRecord(b *testing.B) {
	be := metal.New()
	if err := be.Init(); err != nil {
		b.Skip(err)
	}
	defer be.Destroy()

	vert, err := os.ReadFile("../testdata/triangle.vert.metalbin")
	if err != nil {
		b.Fatal(err)
	}
	frag, err := os.ReadFile("../testdata/triangle.frag.metalbin")
	if err != nil {
		b.Fatal(err)
	}
	const size = 64
	target := be.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: size, Height: size, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget, Label: "target",
	})
	defer be.DestroyTexture(target)
	pipe := be.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: vert, FragmentShader: frag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		Label: "triangle",
	})
	defer be.DestroyPipeline(pipe)
	// The shaders never run on these: every draw has zero instances.
	index := be.Alloc(64, gpu.MemoryDevice, "index")
	defer be.Free(index)
	data := make([]byte, 32)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cmd := be.Begin()
		cmd.BeginRenderPass(gpu.RenderTargets{
			Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
		})
		cmd.SetPipeline(pipe)
		cmd.SetViewport(0, 0, size, size, 0, 1)
		cmd.SetScissor(0, 0, size, size)
		for range 100 {
			cmd.Draw(data, 3, 0, 0, 0)
			cmd.DrawIndexed(data, index, gpu.IndexUint32, 3, 0, 0, 0, 0)
		}
		cmd.EndRenderPass()
		cmd.Barrier(gpu.StageColorOutput, gpu.StageFragment, 0)
		b.StopTimer()
		be.Wait(be.Submit(cmd))
		b.StartTimer()
	}
}
