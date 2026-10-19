package gpu_test

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	gputest "github.com/bluescreen10/gamekit/gpu/test"
)

func TestForeignTextureIsRejected(t *testing.T) {
	b := testBackend(t)
	other := gputest.New()
	if err := other.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Destroy)

	foreign := texture(t, other, gpu.TextureDescriptor{
		Width: 1, Height: 1, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled,
	})
	mustPanic(t, func() {
		b.DestroyTexture(foreign)
	})

	// Rejection must not leave backend locks held or affect local resources.
	colorTarget(t, b)
}

func TestNilResourcesAreOptional(t *testing.T) {
	b := testBackend(t)
	b.Free(nil)
	b.DestroyTexture(nil)
	b.DestroySampler(nil)
	b.DestroyPipeline(nil)
	b.DestroyTimestampPool(nil)
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
