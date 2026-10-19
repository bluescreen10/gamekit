package gpu_test

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

func TestReleasedBufferIsInvalid(t *testing.T) {
	b := testBackend(t)
	buf := b.Alloc(4, gpu.MemoryHost, "released buffer")
	if buf == nil || !buf.IsValid() {
		t.Fatal("Alloc returned no valid buffer")
	}

	b.Free(buf)
	if buf.IsValid() {
		t.Fatal("Free left the buffer valid")
	}

	b.Free(buf)
}

func TestDestroyedResourcesAreInvalid(t *testing.T) {
	b := testBackend(t)
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 1, Height: 1, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled,
	})
	view := b.TextureView(tex, gpu.Texture2D, 0, 1, 0, 1)
	s := sampler(t, b, gpu.SamplerDescriptor{})
	compute := computePipeline(t, b, gpu.ComputePipelineDescriptor{
		Shader: payloadComp, WorkgroupSize: [3]uint32{1, 1, 1},
	})
	graphics := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: triangleFrag, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})

	b.DestroyTexture(view)
	if view.IsValid() || !tex.IsValid() {
		t.Fatal("DestroyTexture(view) must invalidate only the view")
	}
	b.DestroyTexture(view)

	b.DestroyTexture(tex)
	b.DestroySampler(s)
	b.DestroyPipeline(compute)
	b.DestroyPipeline(graphics)
	if tex.IsValid() || s.IsValid() || compute.IsValid() || graphics.IsValid() {
		t.Fatal("resource destruction left a resource valid")
	}

	if pool := b.CreateTimestampPool(1); pool != nil {
		b.DestroyTimestampPool(pool)
		if pool.IsValid() {
			t.Fatal("DestroyTimestampPool left the pool valid")
		}
		b.DestroyTimestampPool(pool)
	}
}

func TestWaitIdleRetiresFence(t *testing.T) {
	b := testBackend(t)
	fence := b.Submit(b.Begin())
	if fence == nil || !fence.IsValid() {
		t.Fatal("Submit returned no valid fence")
	}

	b.WaitIdle()
	if fence.IsValid() {
		t.Fatal("WaitIdle left the completion fence valid")
	}
	b.Wait(fence)
}

func TestForeignTextureIsRejected(t *testing.T) {
	b := testBackend(t)
	mustPanic(t, func() {
		b.DestroyTexture(foreignTexture{})
	})

	// Rejection must not leave backend locks held or affect local resources.
	colorTarget(t, b)
}

type foreignTexture struct{}

func (foreignTexture) IsValid() bool {
	return true
}

func (foreignTexture) Index() uint32 {
	return 0
}

func TestNilResourcesAreOptional(t *testing.T) {
	b := testBackend(t)
	b.Free(nil)
	b.DestroyTexture(nil)
	b.DestroySampler(nil)
	b.DestroyPipeline(nil)
	b.DestroyTimestampPool(nil)
	b.DestroySwapchain(nil)
	b.Wait(nil)

	if pool := b.CreateTimestampPool(0); pool != nil {
		b.DestroyTimestampPool(pool)
		t.Fatal("CreateTimestampPool(0) must return a nil interface")
	}
	if got := b.ReadTimestamps(nil, 4); got != nil {
		t.Fatalf("ReadTimestamps(nil) = %v, want nil", got)
	}

	src := allocation(t, b, 4, gpu.MemoryHost)
	dst := allocation(t, b, 4, gpu.MemoryHost)
	want := []byte{21, 42, 63, 84}
	src.Write(want, 0)

	cmd := b.Begin()
	cmd.ResetTimestamps(nil, 4)
	cmd.WriteTimestamp(nil, 0, gpu.StageAll)
	cmd.CopyBuffer(dst, src, 0, 0, 4)
	fence := b.Submit(cmd)
	if fence == nil || !fence.IsValid() {
		b.WaitIdle()
		t.Fatal("Submit returned no completion fence")
	}
	b.Wait(fence)

	if got := unsafe.Slice((*byte)(dst.Ptr()), 4); !bytes.Equal(got, want) {
		t.Fatalf("copy after nil timestamp operations = %v, want %v", got, want)
	}
}
