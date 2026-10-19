//go:build darwin && cgo

package metal

// mtl4CommandBuffer is a Metal command buffer. Its methods dispatch through the native
// binding functions in objc.go.
type mtl4CommandBuffer id

func (b mtl4CommandBuffer) release() {
	release(b)
}

func (b mtl4CommandBuffer) beginCommandBufferWithAllocator(allocator mtl4CommandAllocator) {
	commandBufferBeginCommandBufferWithAllocator(b, allocator)
}

func (b mtl4CommandBuffer) endCommandBuffer() {
	commandBufferEndCommandBuffer(b)
}

func (b mtl4CommandBuffer) computeCommandEncoder() mtl4ComputeCommandEncoder {
	return commandBufferComputeCommandEncoder(b)
}

func (b mtl4CommandBuffer) renderCommandEncoder(descriptor mtl4RenderPassDescriptor) mtl4RenderCommandEncoder {
	return commandBufferRenderCommandEncoderWithDescriptor(b, descriptor)
}
