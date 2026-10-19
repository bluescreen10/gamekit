//go:build darwin && cgo

package metal

// computeCommandEncoder is a Metal compute encoder. Its methods dispatch through
// the native binding functions in objc_darwin.go.
type computeCommandEncoder id

func (e computeCommandEncoder) release() {
	release(e)
}

func (e computeCommandEncoder) endEncoding() {
	commandEncoderEndEncoding(id(e))
}

func (e computeCommandEncoder) updateFence(fence fence, stages uint) {
	commandEncoderUpdateFence(id(e), fence, stages)
}

func (e computeCommandEncoder) waitForFence(fence fence, stages uint) {
	commandEncoderWaitForFence(id(e), fence, stages)
}

func (e computeCommandEncoder) setArgumentTable(table argumentTable) {
	computeCommandEncoderSetArgumentTable(e, table)
}

func (e computeCommandEncoder) setComputePipelineState(pipeline computePipelineState) {
	computeCommandEncoderSetComputePipelineState(e, pipeline)
}

func (e computeCommandEncoder) dispatchThreadgroups(groups, threads [3]uint) {
	computeCommandEncoderDispatchThreadgroups(e, groups, threads)
}

func (e computeCommandEncoder) dispatchThreadgroupsWithIndirectBuffer(address uint64, threads [3]uint) {
	computeCommandEncoderDispatchThreadgroupsWithIndirectBuffer(e, address, threads)
}

func (e computeCommandEncoder) copyFromBufferToBuffer(src buffer, srcOffset uint, dst buffer, dstOffset, size uint) {
	computeCommandEncoderCopyFromBufferToBuffer(e, src, srcOffset, dst, dstOffset, size)
}

func (e computeCommandEncoder) copyFromBufferToTexture(src buffer, srcOffset, bytesPerRow, bytesPerImage uint, size [3]uint,
	dst texture, slice, level uint, origin [3]uint) {
	computeCommandEncoderCopyFromBufferToTexture(e, src, srcOffset, bytesPerRow, bytesPerImage, size, dst, slice, level, origin)
}

func (e computeCommandEncoder) copyFromTextureToBuffer(src texture, slice, level uint, origin, size [3]uint,
	dst buffer, dstOffset, bytesPerRow, bytesPerImage uint) {
	computeCommandEncoderCopyFromTextureToBuffer(e, src, slice, level, origin, size, dst, dstOffset, bytesPerRow, bytesPerImage)
}

func (e computeCommandEncoder) copyFromTextureToTexture(src texture, srcSlice, srcLevel uint, srcOrigin, size [3]uint,
	dst texture, dstSlice, dstLevel uint, dstOrigin [3]uint) {
	computeCommandEncoderCopyFromTextureToTexture(e, src, srcSlice, srcLevel, srcOrigin, size, dst, dstSlice, dstLevel, dstOrigin)
}

func (e computeCommandEncoder) writeTimestampWithGranularity(granularity uint, heap counterHeap, index uint) {
	computeCommandEncoderWriteTimestampWithGranularity(e, granularity, heap, index)
}
