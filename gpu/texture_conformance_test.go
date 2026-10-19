package gpu_test

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

func texture(t *testing.T, b gpu.Backend, desc gpu.TextureDescriptor) gpu.Texture {
	t.Helper()
	desc.Label = t.Name()
	tex := b.CreateTexture(desc)
	t.Cleanup(func() {
		b.DestroyTexture(tex)
	})

	if !tex.IsValid() {
		t.Fatal("CreateTexture returned an invalid texture")
	}

	return tex
}

func TestSupportedFormats(t *testing.T) {
	b := testBackend(t)
	formats := b.SupportedFormats()
	if len(formats) == 0 {
		t.Fatal("SupportedFormats returned no formats")
	}

	seen := make(map[gpu.Format]bool)
	for _, format := range formats {
		if format == gpu.FormatUndefined || format > gpu.FormatASTC12x12Srgb {
			t.Errorf("SupportedFormats contains invalid format %d", format)
		}
		if seen[format] {
			t.Errorf("SupportedFormats contains duplicate format %d", format)
		}
		seen[format] = true
	}

	for _, format := range []gpu.Format{gpu.FormatRGBA8Unorm, gpu.FormatDepth32F} {
		if !seen[format] {
			t.Errorf("SupportedFormats does not contain the conformance fixture's format %d", format)
		}
	}
}

func TestTextureFormatCopies(t *testing.T) {
	b := testBackend(t)
	for _, tc := range []struct {
		name   string
		format gpu.Format
		bytes  int
	}{
		{"R8", gpu.FormatR8Unorm, 1},
		{"RG8", gpu.FormatRG8Unorm, 2},
		{"RGBA8", gpu.FormatRGBA8Unorm, 4},
		{"BGRA8", gpu.FormatBGRA8Unorm, 4},
		{"RGBA8-sRGB", gpu.FormatRGBA8Srgb, 4},
		{"BGRA8-sRGB", gpu.FormatBGRA8Srgb, 4},
		{"RGBA16F", gpu.FormatRGBA16F, 8},
		{"RGBA32F", gpu.FormatRGBA32F, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !slices.Contains(b.SupportedFormats(), tc.format) {
				t.Skip("format not supported by this device")
			}

			// Odd row widths detect hidden row-pitch assumptions. Use finite float
			// encodings too: this test checks raw copies, not numeric conversions.
			const width, height = 7, 3
			size := width * height * tc.bytes
			upload := allocation(t, b, uint64(size+16), gpu.MemoryHost)
			readback := allocation(t, b, uint64(size+16), gpu.MemoryHost)
			want := make([]byte, size)
			for i := range want {
				want[i] = byte(i%61 + 1)
			}
			upload.Write(want, 16)
			readback.Write(bytes.Repeat([]byte{0xa5}, size+16), 0)
			tex := texture(t, b, gpu.TextureDescriptor{
				Width: width, Height: height, Format: tc.format,
				Usage: gpu.TextureSampled | gpu.TextureTransfer,
			})

			cmd := b.Begin()
			cmd.CopyBufferToTexture(tex, 0, 0, upload, 16)
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			cmd.CopyTextureToBuffer(readback, tex, 0, 0)
			b.Wait(b.Submit(cmd))

			got := unsafe.Slice((*byte)(readback.Ptr), readback.Size)
			want = append(want, bytes.Repeat([]byte{0xa5}, 16)...)
			if !bytes.Equal(got, want) {
				t.Fatalf("texture copy = %v, want %v (including unchanged guard)", got, want)
			}
		})
	}
}

func TestTextureSubresources(t *testing.T) {
	b := testBackend(t)
	for _, tc := range []struct {
		name   string
		kind   gpu.TextureKind
		width  uint32
		height uint32
		depth  uint32
		layers uint32
	}{
		{"2D", gpu.Texture2D, 14, 10, 1, 1},
		{"array", gpu.Texture2DArray, 14, 10, 1, 3},
		{"cube", gpu.TextureCube, 8, 8, 1, 6},
		{"cube-array", gpu.TextureCubeArray, 8, 8, 1, 12},
		{"3D", gpu.Texture3D, 10, 6, 4, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desc := gpu.TextureDescriptor{
				Kind: tc.kind, Width: tc.width, Height: tc.height, Depth: tc.depth,
				Layers: tc.layers, Mips: 4, Format: gpu.FormatRGBA8Unorm,
				Usage: gpu.TextureSampled | gpu.TextureTransfer,
			}
			src := texture(t, b, desc)
			dst := texture(t, b, desc)

			type subresource struct {
				mip, layer       uint32
				upload, readback gpu.Buffer
				want             []byte
			}
			var resources []subresource
			for layer := range tc.layers {
				for mip := range desc.Mips {
					size := max(tc.width>>mip, 1) * max(tc.height>>mip, 1) * max(tc.depth>>mip, 1) * 4
					want := make([]byte, size)
					for i := range want {
						want[i] = byte(i*17 + int(layer)*29 + int(mip)*47)
					}
					upload := allocation(t, b, uint64(size)+16, gpu.MemoryHost)
					upload.Write(want, 16)
					readback := allocation(t, b, uint64(size), gpu.MemoryHost)
					resources = append(resources, subresource{mip, layer, upload, readback, want})
				}
			}

			cmd := b.Begin()
			for _, r := range resources {
				cmd.CopyBufferToTexture(src, r.mip, r.layer, r.upload, 16)
			}
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			for _, r := range resources {
				cmd.CopyTextureToTexture(dst, src, r.mip, r.layer)
			}
			cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
			for _, r := range resources {
				cmd.CopyTextureToBuffer(r.readback, dst, r.mip, r.layer)
			}
			b.Wait(b.Submit(cmd))

			for _, r := range resources {
				t.Run(fmt.Sprintf("layer=%d/mip=%d", r.layer, r.mip), func(t *testing.T) {
					got := unsafe.Slice((*byte)(r.readback.Ptr), r.readback.Size)
					if !bytes.Equal(got, r.want) {
						t.Fatalf("subresource = %v, want %v", got, r.want)
					}
				})
			}
		})
	}
}

func TestTextureCreatedDuringRecording(t *testing.T) {
	b := testBackend(t)
	upload := allocation(t, b, 16, gpu.MemoryHost)
	want := []byte{1, 2, 3, 4, 11, 12, 13, 14, 21, 22, 23, 24, 31, 32, 33, 34}
	upload.Write(want, 0)
	readback := allocation(t, b, 16, gpu.MemoryHost)

	cmd := b.Begin()
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 2, Height: 2, Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureTransfer,
	})
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, tex, 0, 0)
	b.Wait(b.Submit(cmd))

	if got := unsafe.Slice((*byte)(readback.Ptr), 16); !bytes.Equal(got, want) {
		t.Fatalf("texture created after Begin = %v, want %v", got, want)
	}
}
