package gpu_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

func TestDrawInstanceRange(t *testing.T) {
	b := testBackend(t)
	for _, draw := range []string{"direct", "indexed", "indirect"} {
		t.Run(draw, func(t *testing.T) {
			target := colorTarget(t, b)
			pipeline := graphicsPipeline(t, b, gpu.PipelineDescriptor{
				VertexShader: instancedVert, FragmentShader: meshFrag,
				ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
			})
			vb := allocation(t, b, 6*24, gpu.MemoryHost)
			copy(unsafe.Slice((*vertex3D)(vb.Ptr()), 6), []vertex3D{
				{-1, -1, 0, 0, 1, 0}, {1, -1, 0, 0, 1, 0}, {1, 1, 0, 0, 1, 0},
				{-1, -1, 0, 0, 1, 0}, {1, 1, 0, 0, 1, 0}, {-1, 1, 0, 0, 1, 0},
			})
			indices := allocation(t, b, 6*4, gpu.MemoryHost)
			copy(unsafe.Slice((*uint32)(indices.Ptr()), 6), []uint32{0, 1, 2, 3, 4, 5})
			models := allocation(t, b, 3*64, gpu.MemoryHost)
			transforms := unsafe.Slice((*mat4)(models.Ptr()), 3)
			transforms[0] = translate(3, 0, 0) // Instance zero must not draw.
			transforms[1] = translate(-0.5, 0, 0)
			transforms[1][0] = 0.5
			transforms[2] = translate(0.5, 0, 0)
			transforms[2][0] = 0.5
			drawables := allocation(t, b, 3*20, gpu.MemoryHost)
			copy(unsafe.Slice((*gpuDrawable)(drawables.Ptr()), 3), []gpuDrawable{
				{transformID: 0}, {transformID: 1}, {transformID: 2},
			})
			visible := allocation(t, b, 3*4, gpu.MemoryHost)
			copy(unsafe.Slice((*uint32)(visible.Ptr()), 3), []uint32{0, 1, 2})
			root := allocation(t, b, 96, gpu.MemoryHost)
			*(*drawRoot)(root.Ptr()) = drawRoot{
				viewProj: translate(0, 0, 0), verts: vb.Addr(), models: models.Addr(),
				drawables: drawables.Addr(), visible: visible.Addr(),
			}
			args := allocation(t, b, 20, gpu.MemoryHost)
			*(*drawIndexedIndirect)(args.Ptr()) = drawIndexedIndirect{indexCount: 6, instanceCount: 2, firstInstance: 1}

			cmd := b.Begin()
			cmd.BeginRenderPass(gpu.RenderTargets{
				Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep}},
			})
			cmd.SetPipeline(pipeline)
			cmd.SetViewport(0, 0, renderSize, renderSize, 0, 1)
			cmd.SetScissor(0, 0, renderSize, renderSize)
			addr := root.Addr()
			switch draw {
			case "direct":
				cmd.Draw(utils.ToBytes(&addr), 6, 2, 0, 1)
			case "indexed":
				cmd.DrawIndexed(utils.ToBytes(&addr), indices, gpu.IndexUint32, 6, 2, 0, 0, 1)
			case "indirect":
				cmd.DrawIndexedIndirect(utils.ToBytes(&addr), indices, gpu.IndexUint32, args, 0, 1, 20)
			}
			cmd.EndRenderPass()

			wantPixels(t, readColor(t, b, cmd, target), [4]byte{0, 255, 0, 255}, 0)
		})
	}
}
