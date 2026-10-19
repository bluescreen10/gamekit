//go:build darwin && cgo

package metal

// mtl4RenderCommandEncoder is a Metal render encoder. Its methods dispatch through the
// native binding functions in objc_darwin.go.
type mtl4RenderCommandEncoder id

func (e mtl4RenderCommandEncoder) release() {
	release(e)
}

func (e mtl4RenderCommandEncoder) endEncoding() {
	commandEncoderEndEncoding(id(e))
}

func (e mtl4RenderCommandEncoder) updateFence(fence mtlFence, stages uint) {
	commandEncoderUpdateFence(id(e), fence, stages)
}

func (e mtl4RenderCommandEncoder) waitForFence(fence mtlFence, stages uint) {
	commandEncoderWaitForFence(id(e), fence, stages)
}

func (e mtl4RenderCommandEncoder) setArgumentTable(table mtl4ArgumentTable, stages uint) {
	renderCommandEncoderSetArgumentTable(e, table, stages)
}

func (e mtl4RenderCommandEncoder) setViewport(x, y, width, height, znear, zfar float64) {
	renderCommandEncoderSetViewport(e, x, y, width, height, znear, zfar)
}

func (e mtl4RenderCommandEncoder) setScissorRect(x, y, width, height uint) {
	renderCommandEncoderSetScissorRect(e, x, y, width, height)
}

func (e mtl4RenderCommandEncoder) setColorStoreAction(action, index uint) {
	renderCommandEncoderSetColorStoreAction(e, action, index)
}

func (e mtl4RenderCommandEncoder) setDepthStoreAction(action uint) {
	renderCommandEncoderSetDepthStoreAction(e, action)
}

func (e mtl4RenderCommandEncoder) setRenderPipelineState(pipeline mtlRenderPipelineState) {
	renderCommandEncoderSetRenderPipelineState(e, pipeline)
}

func (e mtl4RenderCommandEncoder) setDepthStencilState(state mtlDepthStencilState) {
	renderCommandEncoderSetDepthStencilState(e, state)
}

func (e mtl4RenderCommandEncoder) setCullMode(mode uint) {
	renderCommandEncoderSetCullMode(e, mode)
}

func (e mtl4RenderCommandEncoder) setFrontFacingWinding(winding uint) {
	renderCommandEncoderSetFrontFacingWinding(e, winding)
}

func (e mtl4RenderCommandEncoder) setDepthBias(bias, slope, clamp float32) {
	renderCommandEncoderSetDepthBias(e, bias, slope, clamp)
}

func (e mtl4RenderCommandEncoder) drawPrimitives(primitive, vertexStart, vertexCount, instanceCount, baseInstance uint) {
	renderCommandEncoderDrawPrimitives(e, primitive, vertexStart, vertexCount, instanceCount, baseInstance)
}

func (e mtl4RenderCommandEncoder) drawIndexedPrimitivesIndexCountIndexTypeIndexBufferIndexBufferLengthInstanceCountBaseVertexBaseInstance(primitive, indexCount, indexType uint, indexBuffer uint64,
	indexBufferLength, instanceCount uint, baseVertex int, baseInstance uint) {
	renderCommandEncoderDrawIndexedPrimitivesIndexCount(e, primitive, indexCount, indexType, indexBuffer, indexBufferLength,
		instanceCount, baseVertex, baseInstance)
}

func (e mtl4RenderCommandEncoder) drawIndexedPrimitivesIndexTypeIndexBufferIndexBufferLengthIndirectBuffer(primitive, indexType uint, indexBuffer uint64,
	indexBufferLength uint, indirectBuffer uint64) {
	renderCommandEncoderDrawIndexedPrimitivesIndirectBuffer(e, primitive, indexType, indexBuffer, indexBufferLength, indirectBuffer)
}

func (e mtl4RenderCommandEncoder) writeTimestampWithGranularity(granularity, stage uint, heap mtl4CounterHeap, index uint) {
	renderCommandEncoderWriteTimestampWithGranularity(e, granularity, stage, heap, index)
}
