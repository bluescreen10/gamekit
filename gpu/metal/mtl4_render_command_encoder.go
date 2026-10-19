//go:build darwin && cgo

package metal

// mtl4RenderCommandEncoder is a Metal render encoder. Its methods dispatch through the
// native binding functions in objc.go.
type mtl4RenderCommandEncoder id

func (e mtl4RenderCommandEncoder) release() {
	release(e)
}

func (e mtl4RenderCommandEncoder) endEncoding() {
	commandEncoderEndEncoding(id(e))
}

func (e mtl4RenderCommandEncoder) updateFence(fence mtlFence, stages mtlStages) {
	commandEncoderUpdateFence(id(e), fence, stages)
}

func (e mtl4RenderCommandEncoder) waitForFence(fence mtlFence, stages mtlStages) {
	commandEncoderWaitForFence(id(e), fence, stages)
}

func (e mtl4RenderCommandEncoder) setArgumentTable(table mtl4ArgumentTable, stages mtlRenderStages) {
	renderCommandEncoderSetArgumentTable(e, table, stages)
}

func (e mtl4RenderCommandEncoder) setViewport(x, y, width, height, znear, zfar float64) {
	renderCommandEncoderSetViewport(e, x, y, width, height, znear, zfar)
}

func (e mtl4RenderCommandEncoder) setScissorRect(x, y, width, height uint) {
	renderCommandEncoderSetScissorRect(e, x, y, width, height)
}

func (e mtl4RenderCommandEncoder) setColorStoreAction(action mtlStoreAction, index uint) {
	renderCommandEncoderSetColorStoreAction(e, action, index)
}

func (e mtl4RenderCommandEncoder) setDepthStoreAction(action mtlStoreAction) {
	renderCommandEncoderSetDepthStoreAction(e, action)
}

func (e mtl4RenderCommandEncoder) setRenderPipelineState(pipeline mtlRenderPipelineState) {
	renderCommandEncoderSetRenderPipelineState(e, pipeline)
}

func (e mtl4RenderCommandEncoder) setDepthStencilState(state mtlDepthStencilState) {
	renderCommandEncoderSetDepthStencilState(e, state)
}

func (e mtl4RenderCommandEncoder) setCullMode(mode mtlCullMode) {
	renderCommandEncoderSetCullMode(e, mode)
}

func (e mtl4RenderCommandEncoder) setFrontFacingWinding(winding mtlWinding) {
	renderCommandEncoderSetFrontFacingWinding(e, winding)
}

func (e mtl4RenderCommandEncoder) setDepthBias(bias, slope, clamp float32) {
	renderCommandEncoderSetDepthBias(e, bias, slope, clamp)
}

func (e mtl4RenderCommandEncoder) drawPrimitives(primitive mtlPrimitiveType, vertexStart, vertexCount, instanceCount, baseInstance uint) {
	renderCommandEncoderDrawPrimitives(e, primitive, vertexStart, vertexCount, instanceCount, baseInstance)
}

func (e mtl4RenderCommandEncoder) drawIndexedPrimitivesIndexCountIndexTypeIndexBufferIndexBufferLengthInstanceCountBaseVertexBaseInstance(primitive mtlPrimitiveType, indexCount uint, indexType mtlIndexType, indexBuffer uint64, indexBufferLength, instanceCount uint, baseVertex int, baseInstance uint) {
	renderCommandEncoderDrawIndexedPrimitivesIndexCount(e, primitive, indexCount, indexType, indexBuffer, indexBufferLength,
		instanceCount, baseVertex, baseInstance)
}

func (e mtl4RenderCommandEncoder) drawIndexedPrimitivesIndexTypeIndexBufferIndexBufferLengthIndirectBuffer(primitive mtlPrimitiveType, indexType mtlIndexType, indexBuffer uint64, indexBufferLength uint, indirectBuffer uint64) {
	renderCommandEncoderDrawIndexedPrimitivesIndirectBuffer(e, primitive, indexType, indexBuffer, indexBufferLength, indirectBuffer)
}

func (e mtl4RenderCommandEncoder) writeTimestampWithGranularity(granularity mtl4TimestampGranularity, stage mtlRenderStages, heap mtl4CounterHeap, index uint) {
	renderCommandEncoderWriteTimestampWithGranularity(e, granularity, stage, heap, index)
}
