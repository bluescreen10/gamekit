package gpu_test

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// TestCopyTextureToTexture uploads distinct texels into both mips of a 4x4 texture,
// copies each mip into the same mip of a second texture, and reads it back: every texel
// arrives, in its place.
func TestCopyTextureToTexture(t *testing.T) {
	b := testBackend(t)

	desc := gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: 4, Height: 4, Mips: 2,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer}
	src := b.CreateTexture(desc)
	defer b.DestroyTexture(src)
	dst := b.CreateTexture(desc)
	defer b.DestroyTexture(dst)

	// mip 0 is 4x4 and mip 1 2x2; every byte of each differs from every other.
	mipBytes := [2]int{4 * 4 * 4, 2 * 2 * 4}
	staging := b.Alloc(uint64(mipBytes[0]+mipBytes[1]), gpu.MemoryHost, "staging")
	defer b.Free(staging)
	texels := unsafe.Slice((*byte)(staging.Ptr()), mipBytes[0]+mipBytes[1])
	for i := range texels {
		texels[i] = byte(i + 1)
	}
	readbacks := [2]gpu.Buffer{
		b.Alloc(uint64(mipBytes[0]), gpu.MemoryHost, "readback-0"),
		b.Alloc(uint64(mipBytes[1]), gpu.MemoryHost, "readback-1"),
	}
	for _, readback := range readbacks {
		defer b.Free(readback)
	}

	for mip := range uint32(2) {
		t.Run(map[uint32]string{0: "mip 0", 1: "mip 1"}[mip], func(t *testing.T) {
			cmd := b.Begin()
			cmd.CopyBufferToTexture(src, 0, 0, staging, 0)
			cmd.CopyBufferToTexture(src, 1, 0, staging, uint64(mipBytes[0]))
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			cmd.CopyTextureToTexture(dst, src, mip, 0)
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			cmd.CopyTextureToBuffer(readbacks[mip], dst, mip, 0)
			b.Wait(b.Submit(cmd))

			start := 0
			if mip == 1 {
				start = mipBytes[0]
			}
			want := texels[start : start+mipBytes[mip]]
			got := unsafe.Slice((*byte)(readbacks[mip].Ptr()), mipBytes[mip])
			if !bytes.Equal(got, want) {
				t.Errorf("CopyTextureToTexture mip %d: read back %v, want %v", mip, got, want)
			}
		})
	}
}
