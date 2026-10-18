//go:build darwin && cgo

package metal

// commandBuffer is a Metal command buffer. Its methods dispatch through the native
// binding functions in objc_darwin.go.
type commandBuffer id

func (b commandBuffer) beginCommandBufferWithAllocator(allocator id) {
	commandBufferBeginCommandBufferWithAllocator(b, allocator)
}

func (b commandBuffer) endCommandBuffer() {
	commandBufferEndCommandBuffer(b)
}

func (b commandBuffer) computeCommandEncoder() computeCommandEncoder {
	return commandBufferComputeCommandEncoder(b)
}

func (b commandBuffer) renderCommandEncoderWithDescriptor(descriptor id) renderCommandEncoder {
	return commandBufferRenderCommandEncoderWithDescriptor(b, descriptor)
}
