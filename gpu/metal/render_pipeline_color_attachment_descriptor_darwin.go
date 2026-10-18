//go:build darwin && cgo

package metal

type renderPipelineColorAttachmentDescriptor id

func (d renderPipelineColorAttachmentDescriptor) release() {
	release(d)
}

func newRenderPipelineColorAttachmentDescriptor() renderPipelineColorAttachmentDescriptor {
	return renderPipelineColorAttachmentDescriptor(newObject("MTL4RenderPipelineColorAttachmentDescriptor"))
}

func (d renderPipelineColorAttachmentDescriptor) setPixelFormat(v uint) {
	renderPipelineColorAttachmentDescriptorSetPixelFormat(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setWriteMask(v uint) {
	renderPipelineColorAttachmentDescriptorSetWriteMask(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setBlendingState(v uint) {
	renderPipelineColorAttachmentDescriptorSetBlendingState(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setSourceRGBBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetSourceRGBBlendFactor(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setDestinationRGBBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetDestinationRGBBlendFactor(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setRGBBlendOperation(v uint) {
	renderPipelineColorAttachmentDescriptorSetRGBBlendOperation(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setSourceAlphaBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetSourceAlphaBlendFactor(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setDestinationAlphaBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetDestinationAlphaBlendFactor(d, v)
}

func (d renderPipelineColorAttachmentDescriptor) setAlphaBlendOperation(v uint) {
	renderPipelineColorAttachmentDescriptorSetAlphaBlendOperation(d, v)
}
