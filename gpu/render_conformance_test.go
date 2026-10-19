package gpu_test

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

const renderSize = 16

func graphicsPipeline(t *testing.T, b gpu.Backend, desc gpu.PipelineDescriptor) gpu.Pipeline {
	t.Helper()
	desc.Label = t.Name()
	p := b.CreateGraphicsPipeline(desc)
	t.Cleanup(func() {
		b.DestroyPipeline(p)
	})

	if p == nil || !p.IsValid() {
		t.Fatal("CreateGraphicsPipeline returned an invalid pipeline")
	}

	return p
}

func colorTarget(t *testing.T, b gpu.Backend) gpu.Texture {
	t.Helper()
	return texture(t, b, gpu.TextureDescriptor{
		Width: renderSize, Height: renderSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer,
	})
}

func readColor(t *testing.T, b gpu.Backend, cmd gpu.CommandBuffer, target gpu.Texture) [][4]byte {
	t.Helper()
	readback := allocation(t, b, renderSize*renderSize*4, gpu.MemoryHost)
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, target, 0, 0)
	b.Wait(b.Submit(cmd))

	return append([][4]byte(nil), unsafe.Slice((*[4]byte)(readback.Ptr()), renderSize*renderSize)...)
}

func TestRenderPassClearAndLoad(t *testing.T) {
	b := testBackend(t)
	attachments := []struct {
		clear    [4]float32
		want     [4]byte
		readback gpu.Buffer
	}{
		{clear: [4]float32{1, 0, 0, 1}, want: [4]byte{255, 0, 0, 255}},
		{clear: [4]float32{0, 1, 0, 0}, want: [4]byte{0, 255, 0, 0}},
		{clear: [4]float32{0, 0, 1, 1}, want: [4]byte{0, 0, 255, 255}},
		{clear: [4]float32{1, 1, 1, 0}, want: [4]byte{255, 255, 255, 0}},
	}
	var targets []gpu.ColorAttachment
	for i := range attachments {
		targets = append(targets, gpu.ColorAttachment{Texture: colorTarget(t, b), Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: attachments[i].clear})
		attachments[i].readback = allocation(t, b, renderSize*renderSize*4, gpu.MemoryHost)
	}

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{Color: targets})
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageColorOutput, 0)
	for i := range targets {
		targets[i].Load = gpu.LoadKeep
		targets[i].Clear = [4]float32{1, 0, 1, 1} // LoadKeep must ignore Clear.
	}
	cmd.BeginRenderPass(gpu.RenderTargets{Color: targets})
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	for i, target := range targets {
		cmd.CopyTextureToBuffer(attachments[i].readback, target.Texture, 0, 0)
	}
	b.Wait(b.Submit(cmd))

	for i, attachment := range attachments {
		t.Run(fmt.Sprintf("attachment=%d", i), func(t *testing.T) {
			wantPixels(t, unsafe.Slice((*[4]byte)(attachment.readback.Ptr()), renderSize*renderSize), attachment.want, 0)
		})
	}
}

func TestDynamicViewportAndScissor(t *testing.T) {
	b := testBackend(t)
	target := colorTarget(t, b)
	p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
	})
	cmd.SetPipeline(p)
	cmd.SetViewport(2, 1, 10, 12, 0, 1)
	cmd.SetScissor(5, 4, 8, 7)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	pixels := readColor(t, b, cmd, target)

	for i, pixel := range pixels {
		x, y := i%renderSize, i/renderSize
		want := [4]byte{0, 0, 0, 255}
		if x >= 5 && x < 12 && y >= 4 && y < 11 {
			want = [4]byte{0, 0, 255, 255}
		}
		if pixel != want {
			t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, pixel, want)
		}
	}
}

func TestRenderStateChangesBetweenDraws(t *testing.T) {
	b := testBackend(t)
	target := colorTarget(t, b)
	red := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}, Constants: map[string]float64{"RED": 1, "BLUE": 0},
	})
	blue := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
	})
	cmd.SetPipeline(red)
	cmd.SetViewport(1, 2, 4, 10, 0, 1)
	cmd.SetScissor(0, 0, renderSize, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.SetPipeline(blue)
	cmd.SetViewport(9, 4, 6, 4, 0, 1)
	cmd.SetScissor(10, 0, 3, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	pixels := readColor(t, b, cmd, target)

	for i, pixel := range pixels {
		x, y := i%renderSize, i/renderSize
		want := [4]byte{0, 0, 0, 255}
		if x >= 1 && x < 5 && y >= 2 && y < 12 {
			want = [4]byte{255, 0, 0, 255}
		}
		if x >= 10 && x < 13 && y >= 4 && y < 8 {
			want = [4]byte{0, 0, 255, 255}
		}
		if pixel != want {
			t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, pixel, want)
		}
	}
}

func TestDrawCopiesPerCallData(t *testing.T) {
	b := testBackend(t)
	target := colorTarget(t, b)
	p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: tintFrag, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})
	vb := allocation(t, b, 60, gpu.MemoryHost)
	copy(unsafe.Slice((*vertex)(vb.Ptr()), 3), []vertex{{-1, -1, 1, 1, 1}, {3, -1, 1, 1, 1}, {-1, 3, 1, 1, 1}})
	red := allocation(t, b, 32, gpu.MemoryHost)
	green := allocation(t, b, 32, gpu.MemoryHost)
	*(*rootData)(red.Ptr()) = rootData{tint: [4]float32{1, 0, 0, 1}, verts: vb.Addr()}
	*(*rootData)(green.Ptr()) = rootData{tint: [4]float32{0, 1, 0, 1}, verts: vb.Addr()}

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
	})
	cmd.SetPipeline(p)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
	cmd.SetScissor(0, 0, renderSize/2, renderSize)
	addr := red.Addr()
	data := utils.ToBytes(&addr)
	cmd.Draw(data, 3, 1, 0, 0)
	addr = green.Addr()
	cmd.SetScissor(renderSize/2, 0, renderSize/2, renderSize)
	cmd.Draw(data, 3, 1, 0, 0)
	clear(data) // The caller can reuse the data before Submit.
	cmd.EndRenderPass()
	pixels := readColor(t, b, cmd, target)

	for i, pixel := range pixels {
		want := [4]byte{255, 0, 0, 255}
		if i%renderSize >= renderSize/2 {
			want = [4]byte{0, 255, 0, 255}
		}
		if pixel != want {
			t.Fatalf("pixel %d = %v, want the draw's own data %v", i, pixel, want)
		}
	}
}

func TestDrawOffsets(t *testing.T) {
	b := testBackend(t)
	for _, name := range []string{"first-vertex", "indexed16", "indexed32", "negative-base16", "negative-base32", "indirect16", "indirect32"} {
		t.Run(name, func(t *testing.T) {
			target := colorTarget(t, b)
			p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
				VertexShader: triangleVert, FragmentShader: tintFrag, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
			})
			vb := allocation(t, b, 12*20, gpu.MemoryHost)
			vertices := unsafe.Slice((*vertex)(vb.Ptr()), 12)
			clear(vertices)
			copy(vertices[3:], []vertex{
				{-1, -1, 1, 1, 1}, {1, -1, 1, 1, 1}, {1, 1, 1, 1, 1},
				{-1, -1, 1, 1, 1}, {1, 1, 1, 1, 1}, {-1, 1, 1, 1, 1},
			})
			root := allocation(t, b, 32, gpu.MemoryHost)
			*(*rootData)(root.Ptr()) = rootData{tint: [4]float32{0, 0, 1, 1}, verts: vb.Addr()}
			it := gpu.IndexUint32
			if name == "indexed16" || name == "negative-base16" || name == "indirect16" {
				it = gpu.IndexUint16
			}
			baseVertex := int32(3)
			indexBase := uint32(0)
			if name == "negative-base16" || name == "negative-base32" {
				baseVertex, indexBase = -3, 6
			}
			indices := allocation(t, b, uint64(8*it.Size()), gpu.MemoryHost)
			for i := range 8 {
				value := uint32(0)
				if i >= 2 {
					value = uint32(i-2) + indexBase
				}
				if it == gpu.IndexUint16 {
					*(*uint16)(unsafe.Add(indices.Ptr(), i*2)) = uint16(value)
				} else {
					*(*uint32)(unsafe.Add(indices.Ptr(), i*4)) = value
				}
			}
			args := allocation(t, b, 68, gpu.MemoryHost)
			clear(unsafe.Slice((*byte)(args.Ptr()), args.Size()))
			*(*drawIndexedIndirect)(unsafe.Add(args.Ptr(), 16)) = drawIndexedIndirect{indexCount: 3, instanceCount: 1, firstIndex: 2, vertexOffset: 3}
			*(*drawIndexedIndirect)(unsafe.Add(args.Ptr(), 48)) = drawIndexedIndirect{indexCount: 3, instanceCount: 1, firstIndex: 5, vertexOffset: 3}

			cmd := b.Begin()
			cmd.BeginRenderPass(gpu.RenderTargets{
				Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
			})
			cmd.SetPipeline(p)
			cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
			cmd.SetScissor(0, 0, renderSize, renderSize)
			addr := root.Addr()
			switch name {
			case "first-vertex":
				cmd.Draw(utils.ToBytes(&addr), 6, 1, 3, 0)
			case "indirect16", "indirect32":
				cmd.DrawIndexedIndirect(utils.ToBytes(&addr), indices, it, args, 16, 2, 32)
			default:
				cmd.DrawIndexed(utils.ToBytes(&addr), indices, it, 6, 1, 2, baseVertex, 0)
			}
			cmd.EndRenderPass()

			wantPixels(t, readColor(t, b, cmd, target), [4]byte{0, 0, 255, 255}, 0)
		})
	}
}

func TestDepthCompare(t *testing.T) {
	b := testBackend(t)
	for _, compare := range []struct {
		name string
		op   gpu.CompareOp
		pass [3]bool // incoming depth below, equal to, or above 0.5
	}{
		{"never", gpu.CompareNever, [3]bool{false, false, false}},
		{"less", gpu.CompareLess, [3]bool{true, false, false}},
		{"equal", gpu.CompareEqual, [3]bool{false, true, false}},
		{"less-equal", gpu.CompareLessEqual, [3]bool{true, true, false}},
		{"greater", gpu.CompareGreater, [3]bool{false, false, true}},
		{"not-equal", gpu.CompareNotEqual, [3]bool{true, false, true}},
		{"greater-equal", gpu.CompareGreaterEqual, [3]bool{false, true, true}},
		{"always", gpu.CompareAlways, [3]bool{true, true, true}},
	} {
		t.Run(compare.name, func(t *testing.T) {
			p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
				VertexShader: constantsVert, FragmentShader: constantsFrag,
				ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}, DepthFormat: gpu.FormatDepth32F,
				DepthTest: true, DepthCompare: compare.op,
			})
			for i, incoming := range []float32{0.25, 0.5, 0.75} {
				t.Run(fmt.Sprintf("depth=%.2f", incoming), func(t *testing.T) {
					target := colorTarget(t, b)
					depth := texture(t, b, gpu.TextureDescriptor{
						Width: renderSize, Height: renderSize, Format: gpu.FormatDepth32F, Usage: gpu.TextureDepth,
					})
					cmd := b.Begin()
					cmd.BeginRenderPass(gpu.RenderTargets{
						Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
						Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 0.5},
					})
					cmd.SetPipeline(p)
					cmd.SetViewport(0, 0, renderSize, renderSize, incoming, incoming)
					cmd.SetScissor(0, 0, renderSize, renderSize)
					cmd.Draw(nil, 6, 1, 0, 0)
					cmd.EndRenderPass()

					want := [4]byte{0, 0, 0, 255}
					if compare.pass[i] {
						want = [4]byte{0, 0, 255, 255}
					}
					wantPixels(t, readColor(t, b, cmd, target), want, 0)
				})
			}
		})
	}
}

func TestDepthOnlyAndReadOnlyPasses(t *testing.T) {
	b := testBackend(t)
	depth := texture(t, b, gpu.TextureDescriptor{
		Width: renderSize, Height: renderSize, Format: gpu.FormatDepth32F, Usage: gpu.TextureDepth | gpu.TextureTransfer,
	})
	target := colorTarget(t, b)
	write := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, DepthFormat: gpu.FormatDepth32F,
		DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareAlways,
	})
	read := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}, DepthFormat: gpu.FormatDepth32F,
		DepthTest: true, DepthCompare: gpu.CompareGreater,
	})
	readback := allocation(t, b, renderSize*renderSize*4, gpu.MemoryHost)

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 1},
	})
	cmd.SetPipeline(write)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0.25, 1)
	cmd.SetScissor(0, 0, renderSize, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageDepth, gpu.StageDepth, 0)
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
		Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadKeep, Store: gpu.StoreKeep, ReadOnly: true},
	})
	cmd.SetPipeline(read)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0.5, 1)
	cmd.SetScissor(0, 0, renderSize, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageDepth, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, depth, 0, 0)
	wantPixels(t, readColor(t, b, cmd, target), [4]byte{0, 0, 255, 255}, 0)

	for i, value := range unsafe.Slice((*float32)(readback.Ptr()), renderSize*renderSize) {
		if value != 0.25 {
			t.Fatalf("depth pixel %d = %v, want the depth-only pass's 0.25", i, value)
		}
	}
}

func TestDepthBias(t *testing.T) {
	b := testBackend(t)
	for _, bias := range []float32{0, -1, 1} {
		t.Run(fmt.Sprintf("bias=%g", bias), func(t *testing.T) {
			target := colorTarget(t, b)
			depth := texture(t, b, gpu.TextureDescriptor{
				Width: renderSize, Height: renderSize, Format: gpu.FormatDepth32F, Usage: gpu.TextureDepth,
			})
			p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
				VertexShader: constantsVert, FragmentShader: constantsFrag,
				ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}, DepthFormat: gpu.FormatDepth32F,
				DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareLess,
			})

			cmd := b.Begin()
			cmd.BeginRenderPass(gpu.RenderTargets{
				Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
				Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 0.5},
			})
			cmd.SetPipeline(p)
			cmd.SetViewport(0, 0, renderSize, renderSize, 0.5, 1)
			cmd.SetScissor(0, 0, renderSize, renderSize)
			cmd.SetDepthBias(bias, 0, 0)
			cmd.Draw(nil, 6, 1, 0, 0)
			cmd.EndRenderPass()

			want := [4]byte{0, 0, 0, 255}
			if bias < 0 {
				want = [4]byte{0, 0, 255, 255}
			}
			wantPixels(t, readColor(t, b, cmd, target), want, 0)
		})
	}
}

func TestTriangleStrip(t *testing.T) {
	b := testBackend(t)
	target := colorTarget(t, b)
	p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: tintFrag,
		Topology: gpu.TopologyTriangleStrip, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})
	vb := allocation(t, b, 80, gpu.MemoryHost)
	copy(unsafe.Slice((*vertex)(vb.Ptr()), 4), []vertex{
		{-1, -1, 1, 1, 1}, {1, -1, 1, 1, 1}, {-1, 1, 1, 1, 1}, {1, 1, 1, 1, 1},
	})
	root := allocation(t, b, 32, gpu.MemoryHost)
	*(*rootData)(root.Ptr()) = rootData{tint: [4]float32{1, 0, 0, 1}, verts: vb.Addr()}

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
	})
	cmd.SetPipeline(p)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
	cmd.SetScissor(0, 0, renderSize, renderSize)
	addr := root.Addr()
	cmd.Draw(utils.ToBytes(&addr), 4, 1, 0, 0)
	cmd.EndRenderPass()

	wantPixels(t, readColor(t, b, cmd, target), [4]byte{255, 0, 0, 255}, 0)
}

func TestFaceCulling(t *testing.T) {
	b := testBackend(t)
	for _, tc := range []struct {
		name                        string
		cull                        gpu.CullMode
		counterClockwise, clockwise bool
	}{
		{"none", gpu.CullNone, true, true},
		// The quad's triangles are clockwise in framebuffer coordinates: Y grows down.
		{"back", gpu.CullBack, false, true},
		{"front", gpu.CullFront, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, clockwise := range []bool{false, true} {
				t.Run(fmt.Sprintf("clockwise=%v", clockwise), func(t *testing.T) {
					target := colorTarget(t, b)
					p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
						VertexShader: constantsVert, FragmentShader: constantsFrag,
						ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm}, CullMode: tc.cull, FrontFaceCW: clockwise,
					})
					cmd := b.Begin()
					cmd.BeginRenderPass(gpu.RenderTargets{
						Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
					})
					cmd.SetPipeline(p)
					cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
					cmd.SetScissor(0, 0, renderSize, renderSize)
					cmd.Draw(nil, 6, 1, 0, 0)
					cmd.EndRenderPass()

					visible := tc.counterClockwise
					if clockwise {
						visible = tc.clockwise
					}
					want := [4]byte{0, 0, 0, 255}
					if visible {
						want = [4]byte{0, 0, 255, 255}
					}
					wantPixels(t, readColor(t, b, cmd, target), want, 0)
				})
			}
		})
	}
}

func TestRenderBarrierPreservesAttachmentsAndState(t *testing.T) {
	b := testBackend(t)
	target := colorTarget(t, b)
	p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
	})

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{1, 0, 0, 1}}},
	})
	cmd.SetPipeline(p)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
	cmd.SetScissor(0, 0, renderSize/2, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.Barrier(gpu.StageColorOutput, gpu.StageFragment, 0)
	cmd.SetScissor(renderSize/2, renderSize/2, renderSize/2, renderSize/2)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	pixels := readColor(t, b, cmd, target)

	for i, pixel := range pixels {
		want := [4]byte{0, 0, 255, 255}
		if i%renderSize >= renderSize/2 && i/renderSize < renderSize/2 {
			want = [4]byte{255, 0, 0, 255}
		}
		if pixel != want {
			t.Fatalf("pixel %d after an in-pass barrier = %v, want %v", i, pixel, want)
		}
	}
}

func TestMultipleRenderTargets(t *testing.T) {
	b := testBackend(t)
	first := colorTarget(t, b)
	second := colorTarget(t, b)
	p := graphicsPipeline(t, b, gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: attachmentsFrag,
		ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm, gpu.FormatRGBA8Unorm},
		Blend: []gpu.BlendState{
			{WriteMask: 1 | 4},
			{
				Enable:  true,
				ColorOp: gpu.BlendFactorOp{Src: gpu.BlendSrcAlpha, Dst: gpu.BlendOneMinusSrcAlpha, Op: gpu.BlendAdd},
				AlphaOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendOne, Op: gpu.BlendAdd},
			},
		},
	})
	readback := allocation(t, b, renderSize*renderSize*4, gpu.MemoryHost)

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{
			{Texture: first, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{1, 1, 1, 1}},
			{Texture: second, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{1, 0, 0, 1}},
		},
	})
	cmd.SetPipeline(p)
	cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
	cmd.SetScissor(0, 0, renderSize, renderSize)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, second, 0, 0)
	wantPixels(t, readColor(t, b, cmd, first), [4]byte{128, 255, 0, 255}, 1)
	wantPixels(t, unsafe.Slice((*[4]byte)(readback.Ptr()), renderSize*renderSize), [4]byte{128, 64, 128, 255}, 1)
}
