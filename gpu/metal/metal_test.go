//go:build darwin && cgo

package metal_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/metal"
	"github.com/bluescreen10/gamekit/utils"
)

func device(t *testing.T) *metal.Backend {
	t.Helper()
	b := metal.New()
	if err := b.Init(); err != nil {
		t.Skip(err)
	}
	t.Cleanup(b.Destroy)
	return b
}
func TestMemoryAndTextureCopies(t *testing.T) {
	b := device(t)
	src := b.Alloc(512, gpu.MemoryHost, "upload")
	dst := b.Alloc(512, gpu.MemoryHost, "readback")
	private := b.Alloc(512, gpu.MemoryDevice, "private")
	if src.Ptr() == nil || src.Addr() == 0 || private.Ptr() != nil || private.Addr() == 0 {
		t.Fatal("invalid mapped/private allocation")
	}
	data := unsafe.Slice((*byte)(src.Ptr()), 512)
	for i := range data {
		data[i] = byte(i*17 + 3)
	}
	c := b.Begin()
	c.CopyBuffer(private, src, 0, 0, 512)
	c.CopyBuffer(dst, private, 0, 0, 512)
	b.Wait(b.Submit(c))
	if !bytes.Equal(data, unsafe.Slice((*byte)(dst.Ptr()), 512)) {
		t.Fatal("private buffer copy mismatch")
	}
	// Odd-width, multi-row, nonzero mip/layer copies exercise tight packing.
	tex := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2DArray, Width: 14, Height: 10, Layers: 2, Mips: 2, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer})
	view := b.TextureView(tex, gpu.Texture2D, 1, 1, 1, 1)
	if view.Index() == 0 || view.Index() == tex.Index() {
		t.Fatal("view did not get its own bindless slot")
	}
	clear(unsafe.Slice((*byte)(dst.Ptr()), 512))
	c = b.Begin()
	c.CopyBufferToTexture(tex, 1, 1, src, 4)
	c.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
	c.CopyTextureToBuffer(dst, tex, 1, 1)
	b.Wait(b.Submit(c))
	if !bytes.Equal(data[4:4+7*5*4], unsafe.Slice((*byte)(dst.Ptr()), 7*5*4)) {
		t.Fatal("texture subresource copy mismatch")
	}
	// A view's base level is its first mip, so the view of mip 1 is 7x5, and reads back
	// into a buffer of exactly that size.
	exact := b.Alloc(7*5*4, gpu.MemoryHost, "view readback")
	c = b.Begin()
	c.CopyTextureToBuffer(exact, view, 0, 0)
	b.Wait(b.Submit(c))
	if !bytes.Equal(data[4:4+7*5*4], unsafe.Slice((*byte)(exact.Ptr()), 7*5*4)) {
		t.Fatal("mip view copy mismatch")
	}
	b.Free(exact)
	b.DestroyTexture(view)
	b.DestroyTexture(tex)
	b.Free(src)
	b.Free(dst)
	b.Free(private)
	b.Destroy()
	if err := b.Init(); err != nil {
		t.Fatal("reinitialize:", err)
	}
}
func TestDispatchIndirectAndEntries(t *testing.T) {
	b := device(t)
	source := []byte(`#include <metal_stdlib>
 using namespace metal;
 struct Root {device atomic_uint *value;}; struct Push {device Root *root;};
 kernel void increment(constant Push &p [[buffer(0)]]) {atomic_fetch_add_explicit(p.root->value,1u,memory_order_relaxed);}`)
	p := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: source, WorkgroupSize: [3]uint32{4, 2, 1}, Entry: "increment"})
	value := b.Alloc(4, gpu.MemoryHost, "counter")
	*(*uint32)(value.Ptr()) = 0
	root := b.Alloc(8, gpu.MemoryHost, "root")
	*(*uint64)(root.Ptr()) = value.Addr()
	args := b.Alloc(16, gpu.MemoryHost, "dispatch")
	copy(unsafe.Slice((*uint32)(args.Ptr()), 4), []uint32{99, 3, 2, 1})
	c := b.Begin()
	c.SetPipeline(p)
	rootAddr := root.Addr()
	c.DispatchIndirect(utils.ToBytes(&rootAddr), args, 4)
	c.Barrier(gpu.StageCompute, gpu.StageCompute, 0)
	c.Dispatch(utils.ToBytes(&rootAddr), 1, 1, 1)
	b.Wait(b.Submit(c))
	if got := *(*uint32)(value.Ptr()); got != 56 {
		t.Fatalf("dispatch executed %d threads, want 56", got)
	}
	b.DestroyPipeline(p)
	defer func() {
		if recover() == nil {
			t.Fatal("submitted command buffer was accepted twice")
		}
	}()
	b.Submit(c)
}
func TestTimestamps(t *testing.T) {
	b := device(t)
	p := b.CreateTimestampPool(3)
	if p == nil || !p.IsValid() {
		t.Skip("timestamp counters unavailable")
	}
	defer b.DestroyTimestampPool(p)
	src := b.Alloc(4096, gpu.MemoryHost, "src")
	dst := b.Alloc(4096, gpu.MemoryDevice, "dst")
	c := b.Begin()
	c.ResetTimestamps(p, 3)
	c.WriteTimestamp(p, 0, gpu.StageNone)
	c.CopyBuffer(dst, src, 0, 0, 4096)
	c.WriteTimestamp(p, 1, gpu.StageTransfer)
	b.Wait(b.Submit(c))
	times := b.ReadTimestamps(p, 3)
	if times[0] == 0 || times[1] < times[0] || times[2] != 0 {
		t.Fatalf("invalid timestamps: %v", times)
	}
}

// TestTimestampsAroundEachDispatchMeasureIt: timestamps written around each of two
// compute dispatches land around it, rather than where the encoder they share begins or
// ends — or timing one piece of compute work among others measures all or nothing.
func TestTimestampsAroundEachDispatchMeasureIt(t *testing.T) {
	b := device(t)
	pool := b.CreateTimestampPool(4)
	if pool == nil || !pool.IsValid() {
		t.Skip("timestamp counters unavailable")
	}
	defer b.DestroyTimestampPool(pool)
	source := []byte(`#include <metal_stdlib>
 using namespace metal;
 struct Push {device float *out;};
 kernel void spin(constant Push &p [[buffer(0)]], uint i [[thread_position_in_grid]]) {
  float x = float(i);
  for (int k = 0; k < 20000; k++) { x = fma(x, 1.000001, 0.5); }
  p.out[i] = x;
 }`)
	pipeline := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: source, WorkgroupSize: [3]uint32{64, 1, 1}, Entry: "spin"})
	defer b.DestroyPipeline(pipeline)
	const groups = 1024
	out := b.Alloc(groups*64*4, gpu.MemoryDevice, "out")
	outAddr := out.Addr()

	// Two pieces of work timed one after the other, as a profiler times consecutive
	// steps: each between a start and an end of its own, with nothing in between.
	c := b.Begin()
	c.ResetTimestamps(pool, 4)
	c.SetPipeline(pipeline)
	c.WriteTimestamp(pool, 0, gpu.StageCompute)
	c.Dispatch(utils.ToBytes(&outAddr), groups, 1, 1)
	c.WriteTimestamp(pool, 1, gpu.StageAll)
	c.WriteTimestamp(pool, 2, gpu.StageCompute)
	c.Dispatch(utils.ToBytes(&outAddr), groups, 1, 1)
	c.WriteTimestamp(pool, 3, gpu.StageAll)
	b.Wait(b.Submit(c))

	times := b.ReadTimestamps(pool, 4)
	if times[0] >= times[1] || times[2] >= times[3] {
		t.Fatalf("timestamps %v, want each dispatch's end after its start", times)
	}
}

func TestInvalidShader(t *testing.T) {
	b := device(t)

	defer func() {
		value := recover()
		if value == nil {
			t.Fatal("invalid shader accepted")
		}

		if message := fmt.Sprint(value); !strings.Contains(message, "error:") {
			t.Fatalf("invalid shader panic = %q, want a compiler diagnostic", message)
		}
	}()

	b.CreateComputePipeline(gpu.ComputePipelineDescriptor{
		Shader:        []byte("not valid MSL"),
		WorkgroupSize: [3]uint32{1, 1, 1},
	})
}

func TestRenderPassSplit(t *testing.T) {
	b := device(t)
	source := []byte(`#include <metal_stdlib>
 using namespace metal;
 vertex float4 vertex_entry(uint i [[vertex_id]]) {float2 p[3]={float2(-1,-1),float2(3,-1),float2(-1,3)};return float4(p[i],0,1);}
 fragment float4 fragment_entry(){return float4(1,0,0,1);}`)
	pipe := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{VertexShader: source, FragmentShader: source, VertexEntry: "vertex_entry", FragmentEntry: "fragment_entry", ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}})
	target := b.CreateTexture(gpu.TextureDescriptor{Width: 8, Height: 8, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureRenderTarget | gpu.TextureTransfer})
	read := b.Alloc(8*8*4, gpu.MemoryHost, "readback")
	pool := b.CreateTimestampPool(1)
	c := b.Begin()
	c.BeginRenderPass(gpu.RenderTargets{Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 1, 1}}}})
	c.SetPipeline(pipe)
	c.SetViewport(0, 0, 8, 8, 0, 1)
	c.SetScissor(0, 0, 4, 8)
	c.Draw(nil, 3, 1, 0, 0)
	c.Barrier(gpu.StageFragment, gpu.StageFragment, 0)
	c.WriteTimestamp(pool, 0, gpu.StageColorOutput)
	c.Draw(nil, 3, 1, 0, 0)
	c.EndRenderPass()
	c.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	c.CopyTextureToBuffer(read, target, 0, 0)
	b.Wait(b.Submit(c))
	pixels := unsafe.Slice((*byte)(read.Ptr()), 8*8*4)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			at := (y*8 + x) * 4
			want := []byte{0, 0, 255, 255}
			if x < 4 {
				want = []byte{255, 0, 0, 255}
			}
			if !bytes.Equal(pixels[at:at+4], want) {
				t.Fatalf("pixel %d,%d = %v, want %v", x, y, pixels[at:at+4], want)
			}
		}
	}
}
