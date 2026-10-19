//go:build darwin && cgo

package metal

// mtl4ComputeCommandEncoder is a Metal compute encoder. Its methods dispatch through
// the native binding functions in objc.go.
type mtl4ComputeCommandEncoder id

func (e mtl4ComputeCommandEncoder) release() {
	release(e)
}

func (e mtl4ComputeCommandEncoder) endEncoding() {
	commandEncoderEndEncoding(id(e))
}

func (e mtl4ComputeCommandEncoder) updateFence(fence mtlFence, stages mtlStages) {
	commandEncoderUpdateFence(id(e), fence, stages)
}

func (e mtl4ComputeCommandEncoder) waitForFence(fence mtlFence, stages mtlStages) {
	commandEncoderWaitForFence(id(e), fence, stages)
}

func (e mtl4ComputeCommandEncoder) setArgumentTable(table mtl4ArgumentTable) {
	computeCommandEncoderSetArgumentTable(e, table)
}

func (e mtl4ComputeCommandEncoder) setComputePipelineState(pipeline mtlComputePipelineState) {
	computeCommandEncoderSetComputePipelineState(e, pipeline)
}

func (e mtl4ComputeCommandEncoder) dispatchThreadgroups(groups, threads [3]uint) {
	computeCommandEncoderDispatchThreadgroups(e, groups, threads)
}

func (e mtl4ComputeCommandEncoder) dispatchThreadgroupsWithIndirectBuffer(address uint64, threads [3]uint) {
	computeCommandEncoderDispatchThreadgroupsWithIndirectBuffer(e, address, threads)
}

func (e mtl4ComputeCommandEncoder) copyFromBufferSourceOffsetToBufferDestinationOffsetSize(src mtlBuffer, srcOffset uint, dst mtlBuffer, dstOffset, size uint) {
	computeCommandEncoderCopyFromBufferToBuffer(e, src, srcOffset, dst, dstOffset, size)
}

func (e mtl4ComputeCommandEncoder) copyFromBufferSourceOffsetSourceBytesPerRowSourceBytesPerImageSourceSizeToTextureDestinationSliceDestinationLevelDestinationOrigin(src mtlBuffer, srcOffset, bytesPerRow, bytesPerImage uint, size [3]uint,
	dst mtlTexture, slice, level uint, origin [3]uint) {
	computeCommandEncoderCopyFromBufferToTexture(e, src, srcOffset, bytesPerRow, bytesPerImage, size, dst, slice, level, origin)
}

func (e mtl4ComputeCommandEncoder) copyFromTextureSourceSliceSourceLevelSourceOriginSourceSizeToBufferDestinationOffsetDestinationBytesPerRowDestinationBytesPerImage(src mtlTexture, slice, level uint, origin, size [3]uint,
	dst mtlBuffer, dstOffset, bytesPerRow, bytesPerImage uint) {
	computeCommandEncoderCopyFromTextureToBuffer(e, src, slice, level, origin, size, dst, dstOffset, bytesPerRow, bytesPerImage)
}

func (e mtl4ComputeCommandEncoder) copyFromTextureSourceSliceSourceLevelSourceOriginSourceSizeToTextureDestinationSliceDestinationLevelDestinationOrigin(src mtlTexture, srcSlice, srcLevel uint, srcOrigin, size [3]uint,
	dst mtlTexture, dstSlice, dstLevel uint, dstOrigin [3]uint) {
	computeCommandEncoderCopyFromTextureToTexture(e, src, srcSlice, srcLevel, srcOrigin, size, dst, dstSlice, dstLevel, dstOrigin)
}

func (e mtl4ComputeCommandEncoder) writeTimestampWithGranularity(granularity mtl4TimestampGranularity, heap mtl4CounterHeap, index uint) {
	computeCommandEncoderWriteTimestampWithGranularity(e, granularity, heap, index)
}
