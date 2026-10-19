package gpu_test

import (
	"math"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

func sampler(t *testing.T, b gpu.Backend, desc gpu.SamplerDescriptor) gpu.Sampler {
	t.Helper()
	desc.Label = t.Name()
	s := b.CreateSampler(desc)
	t.Cleanup(func() {
		b.DestroySampler(s)
	})

	if s == nil || !s.IsValid() {
		t.Fatal("CreateSampler returned an invalid sampler")
	}

	return s
}

// samplePixels samples the texture across a 4x4 target. uv names the UV rectangle.
func samplePixels(t *testing.T, b gpu.Backend, tex gpu.Texture, s gpu.Sampler, uv [4]float32) [][4]byte {
	t.Helper()
	return samplePixelsWithShader(t, b, tex, s, uv, texturedFrag)
}

func samplePixelsWithShader(t *testing.T, b gpu.Backend, tex gpu.Texture, s gpu.Sampler, uv [4]float32, fragment []byte) [][4]byte {
	t.Helper()
	const size = 4
	color := texture(t, b, gpu.TextureDescriptor{
		Width: size, Height: size, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer,
	})
	vertices := []texVertex{
		{-1, -1, uv[0], uv[1]}, {1, -1, uv[2], uv[1]}, {1, 1, uv[2], uv[3]},
		{-1, -1, uv[0], uv[1]}, {1, 1, uv[2], uv[3]}, {-1, 1, uv[0], uv[3]},
	}
	vb := allocation(t, b, uint64(len(vertices))*16, gpu.MemoryHost)
	copy(unsafe.Slice((*texVertex)(vb.Ptr()), len(vertices)), vertices)
	root := allocation(t, b, 16, gpu.MemoryHost)
	*(*texRoot)(root.Ptr()) = texRoot{verts: vb.Addr(), tex: tex.Index(), samp: s.Index()}
	readback := allocation(t, b, size*size*4, gpu.MemoryHost)
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: texturedVert, FragmentShader: fragment,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})
	t.Cleanup(func() {
		b.DestroyPipeline(pipeline)
	})

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, size, size, 0, 1)
	cmd.SetScissor(0, 0, size, size)
	addr := root.Addr()
	cmd.Draw(utils.ToBytes(&addr), 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, color, 0, 0)
	b.Wait(b.Submit(cmd))

	return append([][4]byte(nil), unsafe.Slice((*[4]byte)(readback.Ptr()), size*size)...)
}

func wantPixels(t *testing.T, pixels [][4]byte, want [4]byte, tolerance int) {
	t.Helper()
	for i, pixel := range pixels {
		for channel := range pixel {
			if delta := int(pixel[channel]) - int(want[channel]); delta < -tolerance || delta > tolerance {
				t.Fatalf("pixel %d = %v, want %v (tolerance %d)", i, pixel, want, tolerance)
			}
		}
	}
}

func TestSamplerAddressModes(t *testing.T) {
	b := testBackend(t)
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 2, Height: 2, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	colors := [4][4]byte{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	upload := allocation(t, b, 16, gpu.MemoryHost)
	copy(unsafe.Slice((*[4]byte)(upload.Ptr()), 4), colors[:])
	cmd := b.Begin()
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
	b.Wait(b.Submit(cmd))

	for _, tc := range []struct {
		name    string
		address gpu.AddressMode
		uv      [2]float32
		texel   int
	}{
		{"clamp-negative-u", gpu.AddressClamp, [2]float32{-0.25, 0.25}, 0},
		{"repeat-negative-u", gpu.AddressRepeat, [2]float32{-0.25, 0.25}, 1},
		{"mirror-negative-u", gpu.AddressMirror, [2]float32{-0.25, 0.25}, 0},
		{"clamp-positive-u", gpu.AddressClamp, [2]float32{1.25, 0.25}, 1},
		{"repeat-positive-u", gpu.AddressRepeat, [2]float32{1.25, 0.25}, 0},
		{"mirror-positive-u", gpu.AddressMirror, [2]float32{1.25, 0.25}, 1},
		{"clamp-negative-v", gpu.AddressClamp, [2]float32{0.25, -0.25}, 0},
		{"repeat-negative-v", gpu.AddressRepeat, [2]float32{0.25, -0.25}, 2},
		{"mirror-negative-v", gpu.AddressMirror, [2]float32{0.25, -0.25}, 0},
		{"clamp-positive-v", gpu.AddressClamp, [2]float32{0.25, 1.25}, 2},
		{"repeat-positive-v", gpu.AddressRepeat, [2]float32{0.25, 1.25}, 0},
		{"mirror-positive-v", gpu.AddressMirror, [2]float32{0.25, 1.25}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sampler(t, b, gpu.SamplerDescriptor{AddressU: tc.address, AddressV: tc.address})
			uv := [4]float32{tc.uv[0], tc.uv[1], tc.uv[0], tc.uv[1]}
			wantPixels(t, samplePixels(t, b, tex, s, uv), colors[tc.texel], 0)
		})
	}
}

func TestSamplerFiltering(t *testing.T) {
	b := testBackend(t)
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 2, Height: 2, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	upload := allocation(t, b, 16, gpu.MemoryHost)
	upload.Write([]byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 255, 255, 255, 255}, 0)
	cmd := b.Begin()
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
	b.Wait(b.Submit(cmd))

	for _, tc := range []struct {
		name string
		desc gpu.SamplerDescriptor
		uv   [4]float32
		want [4]byte
	}{
		{"nearest", gpu.SamplerDescriptor{}, [4]float32{0.25, 0.25, 0.25, 0.25}, [4]byte{255, 0, 0, 255}},
		{"magnification-linear", gpu.SamplerDescriptor{MagLinear: true}, [4]float32{0.5, 0.5, 0.5, 0.5}, [4]byte{128, 128, 128, 255}},
		{"minification-linear", gpu.SamplerDescriptor{MinLinear: true, AddressU: gpu.AddressRepeat, AddressV: gpu.AddressRepeat}, [4]float32{-1.5, -1.5, 14.5, 14.5}, [4]byte{128, 128, 128, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sampler(t, b, tc.desc)
			wantPixels(t, samplePixels(t, b, tex, s, tc.uv), tc.want, 1)
		})
	}
}

func TestSamplerMipFiltering(t *testing.T) {
	b := testBackend(t)
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 8, Height: 8, Mips: 4, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	colors := [4][4]byte{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	var uploads []gpu.Buffer
	for mip, color := range colors {
		count := (8 >> mip) * (8 >> mip)
		upload := allocation(t, b, uint64(count*4), gpu.MemoryHost)
		for i := range count {
			*(*[4]byte)(unsafe.Add(upload.Ptr(), i*4)) = color
		}
		uploads = append(uploads, upload)
	}

	cmd := b.Begin()
	for mip, upload := range uploads {
		cmd.CopyBufferToTexture(tex, uint32(mip), 0, upload, 0)
	}
	cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
	b.Wait(b.Submit(cmd))

	for _, tc := range []struct {
		name   string
		linear bool
		span   float32
		want   [4]byte
	}{
		{"base-level", false, 0, colors[0]},
		{"nearest-mip", false, 1, colors[1]},
		{"last-level", false, 16, colors[3]},
		// Eight texels across four pixels gives LOD 1. Scale by sqrt(2) for LOD 1.5.
		{"linear-between-mips", true, float32(math.Sqrt2), [4]byte{0, 128, 128, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sampler(t, b, gpu.SamplerDescriptor{MipLinear: tc.linear})
			wantPixels(t, samplePixels(t, b, tex, s, [4]float32{0, 0, tc.span, tc.span}), tc.want, 2)
		})
	}
}

func TestSampledTextureViews(t *testing.T) {
	b := testBackend(t)
	parent := texture(t, b, gpu.TextureDescriptor{
		Kind: gpu.Texture2DArray, Width: 4, Height: 4, Layers: 2, Mips: 2,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	s := sampler(t, b, gpu.SamplerDescriptor{})
	colors := [4][4]byte{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	var uploads []gpu.Buffer
	for i, color := range colors {
		count := (4 >> (i % 2)) * (4 >> (i % 2))
		upload := allocation(t, b, uint64(count*4), gpu.MemoryHost)
		for texel := range count {
			*(*[4]byte)(unsafe.Add(upload.Ptr(), texel*4)) = color
		}
		uploads = append(uploads, upload)
	}

	cmd := b.Begin()
	for i, upload := range uploads {
		cmd.CopyBufferToTexture(parent, uint32(i%2), uint32(i/2), upload, 0)
	}
	cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
	b.Wait(b.Submit(cmd))

	var views []gpu.Texture
	for i := range colors {
		view := b.TextureView(parent, gpu.Texture2D, uint32(i%2), 1, uint32(i/2), 1)
		if view == nil || !view.IsValid() {
			t.Fatal("TextureView returned an invalid texture")
		}
		for _, other := range views {
			if view.Index() == other.Index() {
				t.Fatal("distinct live views share a bindless index")
			}
		}
		views = append(views, view)
	}
	t.Cleanup(func() {
		for _, view := range views {
			b.DestroyTexture(view)
		}
	})

	for i, view := range views {
		wantPixels(t, samplePixels(t, b, view, s, [4]float32{0, 0, 1, 1}), colors[i], 0)
	}

	// Destroy one view. The parent image and the other views must remain usable.
	b.DestroyTexture(views[0])
	views = views[1:]
	replacement := b.TextureView(parent, gpu.Texture2D, 0, 1, 0, 1)
	views = append(views, replacement)
	wantPixels(t, samplePixels(t, b, replacement, s, [4]float32{0, 0, 1, 1}), colors[0], 0)
	wantPixels(t, samplePixels(t, b, views[0], s, [4]float32{0, 0, 1, 1}), colors[1], 0)
}

func TestSRGBTextureSampling(t *testing.T) {
	b := testBackend(t)
	tex := texture(t, b, gpu.TextureDescriptor{
		Width: 1, Height: 1, Format: gpu.FormatRGBA8Srgb,
		Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	upload := allocation(t, b, 4, gpu.MemoryHost)
	upload.Write([]byte{128, 128, 128, 128}, 0)
	s := sampler(t, b, gpu.SamplerDescriptor{})
	cmd := b.Begin()
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
	b.Wait(b.Submit(cmd))

	// RGB decodes from sRGB to linear. Alpha remains linear.
	wantPixels(t, samplePixels(t, b, tex, s, [4]float32{0, 0, 1, 1}), [4]byte{55, 55, 55, 128}, 1)
}

func TestComparisonSamplers(t *testing.T) {
	b := testBackend(t)
	for _, tc := range []struct {
		name string
		op   gpu.CompareOp
		pass [3]bool // The reference is 0.5. The texture contains 0.25, 0.5, or 0.75.
	}{
		{"less", gpu.CompareLess, [3]bool{false, false, true}},
		{"less-equal", gpu.CompareLessEqual, [3]bool{false, true, true}},
		{"equal", gpu.CompareEqual, [3]bool{false, true, false}},
		{"greater", gpu.CompareGreater, [3]bool{true, false, false}},
		{"greater-equal", gpu.CompareGreaterEqual, [3]bool{true, true, false}},
		{"not-equal", gpu.CompareNotEqual, [3]bool{true, false, true}},
		{"always", gpu.CompareAlways, [3]bool{true, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sampler(t, b, gpu.SamplerDescriptor{Compare: tc.op})
			for i, depth := range []float32{0.25, 0.5, 0.75} {
				tex := texture(t, b, gpu.TextureDescriptor{
					Width: 4, Height: 4, Format: gpu.FormatDepth32F, Usage: gpu.TextureDepth | gpu.TextureSampled,
				})
				cmd := b.Begin()
				cmd.BeginRenderPass(gpu.RenderTargets{
					Depth: &gpu.DepthAttachment{Texture: tex, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: depth},
				})
				cmd.EndRenderPass()
				cmd.Barrier(gpu.StageDepth, gpu.StageFragment, 0)
				b.Wait(b.Submit(cmd))

				want := [4]byte{0, 0, 0, 255}
				if tc.pass[i] {
					want = [4]byte{255, 255, 255, 255}
				}
				t.Logf("reference 0.5 compared with texture depth %g", depth)
				wantPixels(t, samplePixelsWithShader(t, b, tex, s, [4]float32{0, 0, 1, 1}, shadowFrag), want, 0)
			}
		})
	}
}
