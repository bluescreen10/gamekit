//go:build cgo && (!darwin || gamekit_vulkan)

package vulkan_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/vulkan"
)

func TestCallerDestroysResourcesBeforeShutdown(t *testing.T) {
	b := vulkan.New()
	if err := b.Init(); err != nil {
		t.Skipf("backend unavailable: %v", err)
	}
	t.Cleanup(b.Destroy)

	buf := b.Alloc(4, gpu.MemoryHost, "shutdown buffer")
	defer b.Free(buf)
	tex := b.CreateTexture(gpu.TextureDescriptor{Width: 1, Height: 1, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled})
	defer b.DestroyTexture(tex)
	view := b.TextureView(tex, gpu.Texture2D, 0, 1, 0, 1)
	defer b.DestroyTexture(view)
	sampler := b.CreateSampler(gpu.SamplerDescriptor{})
	defer b.DestroySampler(sampler)
	pool := b.CreateTimestampPool(1)
	defer b.DestroyTimestampPool(pool)
	fence := b.Submit(b.Begin())

	b.Wait(fence)
	b.DestroyTimestampPool(pool)
	b.DestroySampler(sampler)
	b.DestroyTexture(view)
	b.DestroyTexture(tex)
	b.Free(buf)
	b.Destroy()
	if buf.IsValid() || tex.IsValid() || view.IsValid() || sampler.IsValid() || fence.IsValid() {
		t.Fatal("explicit destruction left a resource valid")
	}
	if pool != nil && pool.IsValid() {
		t.Fatal("DestroyTimestampPool left the pool valid")
	}

	b.Destroy()
	if err := b.Init(); err != nil {
		t.Fatalf("reinitialize: %v", err)
	}

	// Old objects must not release resources from the new device.
	fresh := b.Alloc(4, gpu.MemoryHost, "new buffer")
	defer b.Free(fresh)
	b.Free(buf)
	b.DestroyTexture(view)
	b.DestroyTexture(tex)
	b.DestroySampler(sampler)
	b.DestroyTimestampPool(pool)
	b.Wait(fence)
	fresh.Write([]byte{1, 2, 3, 4}, 0)
	if !fresh.IsValid() {
		t.Fatal("old resource destruction invalidated the new buffer")
	}
}
