//go:build darwin && cgo

package metal

// renderCommandEncoder is a Metal render encoder. Its methods dispatch through the
// native binding functions in objc_darwin.go.
type renderCommandEncoder id

func (e renderCommandEncoder) release() {
	release(e)
}

func (e renderCommandEncoder) endEncoding() {
	commandEncoderEndEncoding(id(e))
}

func (e renderCommandEncoder) updateFence(fence id, stages uint) {
	commandEncoderUpdateFence(id(e), fence, stages)
}

func (e renderCommandEncoder) waitForFence(fence id, stages uint) {
	commandEncoderWaitForFence(id(e), fence, stages)
}

func (e renderCommandEncoder) setArgumentTable(table id, stages uint) {
	renderCommandEncoderSetArgumentTable(e, table, stages)
}

func (e renderCommandEncoder) setViewport(x, y, width, height, znear, zfar float64) {
	renderCommandEncoderSetViewport(e, x, y, width, height, znear, zfar)
}

func (e renderCommandEncoder) setScissorRect(x, y, width, height uint) {
	renderCommandEncoderSetScissorRect(e, x, y, width, height)
}

func (e renderCommandEncoder) setColorStoreAction(action, index uint) {
	renderCommandEncoderSetColorStoreAction(e, action, index)
}

func (e renderCommandEncoder) setDepthStoreAction(action uint) {
	renderCommandEncoderSetDepthStoreAction(e, action)
}

func (e renderCommandEncoder) setRenderPipelineState(pipeline id) {
	renderCommandEncoderSetRenderPipelineState(e, pipeline)
}

func (e renderCommandEncoder) setDepthStencilState(state id) {
	renderCommandEncoderSetDepthStencilState(e, state)
}

func (e renderCommandEncoder) setCullMode(mode uint) {
	renderCommandEncoderSetCullMode(e, mode)
}

func (e renderCommandEncoder) setFrontFacingWinding(winding uint) {
	renderCommandEncoderSetFrontFacingWinding(e, winding)
}

func (e renderCommandEncoder) setDepthBias(bias, slope, clamp float32) {
	renderCommandEncoderSetDepthBias(e, bias, slope, clamp)
}

func (e renderCommandEncoder) drawPrimitives(primitive, vertexStart, vertexCount, instanceCount, baseInstance uint) {
	renderCommandEncoderDrawPrimitives(e, primitive, vertexStart, vertexCount, instanceCount, baseInstance)
}

func (e renderCommandEncoder) drawIndexedPrimitivesIndexCount(primitive, indexCount, indexType uint, indexBuffer uint64,
	indexBufferLength, instanceCount uint, baseVertex int, baseInstance uint) {
	renderCommandEncoderDrawIndexedPrimitivesIndexCount(e, primitive, indexCount, indexType, indexBuffer, indexBufferLength,
		instanceCount, baseVertex, baseInstance)
}

func (e renderCommandEncoder) drawIndexedPrimitivesIndirectBuffer(primitive, indexType uint, indexBuffer uint64,
	indexBufferLength uint, indirectBuffer uint64) {
	renderCommandEncoderDrawIndexedPrimitivesIndirectBuffer(e, primitive, indexType, indexBuffer, indexBufferLength, indirectBuffer)
}

func (e renderCommandEncoder) writeTimestampWithGranularity(granularity, stage uint, heap id, index uint) {
	renderCommandEncoderWriteTimestampWithGranularity(e, granularity, stage, heap, index)
}
