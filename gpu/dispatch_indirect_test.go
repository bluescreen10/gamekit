package gpu_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// TestDispatchIndirectRunsWholeWorkgroups writes a 64x64 storage image with workgroup
// counts read from a buffer. Each group must run the pipeline's whole WorkgroupSize:
// with fewer invocations per group, most pixels stay unwritten.
func TestDispatchIndirectRunsWholeWorkgroups(t *testing.T) {
	b := testBackend(t)

	const imageSize = 64
	image := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: imageSize, Height: imageSize,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureStorage | gpu.TextureTransfer, Label: "storage"})
	defer b.DestroyTexture(image)
	comp := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{
		Shader: storageWrite, WorkgroupSize: [3]uint32{8, 8, 1}, Label: "storage-write",
	})
	defer b.DestroyPipeline(comp)

	// The workgroup counts sit at a non-zero offset, as in a buffer that holds more.
	const argsOffset = 16
	args := b.Alloc(argsOffset+12, gpu.MemoryHost, "dispatch args")
	defer b.Free(args)
	*(*[3]uint32)(unsafe.Add(args.Ptr, argsOffset)) = [3]uint32{imageSize / 8, imageSize / 8, 1}
	readback := b.Alloc(imageSize*imageSize*4, gpu.MemoryHost, "readback")
	defer b.Free(readback)

	cmd := b.Begin()
	cmd.SetPipeline(comp)
	data := storageWriteData{image: image.Index, size: imageSize}
	cmd.DispatchIndirect(utils.ToBytes(&data), args, argsOffset)
	cmd.Barrier(gpu.StageCompute, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, image, 0, 0)
	b.Wait(b.Submit(cmd))

	px := unsafe.Slice((*byte)(readback.Ptr), imageSize*imageSize*4)
	for _, probe := range []struct {
		x, y int
		want [3]byte
		name string
	}{
		{0, 0, [3]byte{255, 0, 0}, "top-left red"},
		{imageSize - 1, 0, [3]byte{0, 255, 0}, "top-right green"},
		{0, imageSize - 1, [3]byte{0, 0, 255}, "bottom-left blue"},
		{imageSize - 1, imageSize - 1, [3]byte{255, 255, 255}, "bottom-right white"},
	} {
		i := (probe.y*imageSize + probe.x) * 4
		if got := [3]byte{px[i], px[i+1], px[i+2]}; got != probe.want {
			t.Errorf("%s corner = %v, want %v", probe.name, got, probe.want)
		}
	}
}

// A DispatchIndirect whose counts are not three whole uint32s inside the buffer, at a
// multiple of 4, panics and records nothing: the command buffer still submits.
func TestDispatchIndirectOutsideItsBufferPanics(t *testing.T) {
	b := testBackend(t)
	comp := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{
		Shader: storageWrite, WorkgroupSize: [3]uint32{8, 8, 1}, Label: "storage-write",
	})
	defer b.DestroyPipeline(comp)
	args := b.Alloc(64, gpu.MemoryHost, "dispatch args")
	defer b.Free(args)

	for _, offset := range []uint64{2, 64 - 8} {
		cmd := b.Begin()
		cmd.SetPipeline(comp)
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("DispatchIndirect at offset %d of a 64-byte buffer did not panic", offset)
				}
			}()
			cmd.DispatchIndirect(nil, args, offset)
		}()
		b.Wait(b.Submit(cmd))
	}
}
