//go:build darwin && cgo

package metal

type mtl4RenderPipelineColorAttachmentDescriptor id

func (d mtl4RenderPipelineColorAttachmentDescriptor) release() {
	release(d)
}

func newMTL4RenderPipelineColorAttachmentDescriptor() mtl4RenderPipelineColorAttachmentDescriptor {
	return mtl4RenderPipelineColorAttachmentDescriptor(newObject("MTL4RenderPipelineColorAttachmentDescriptor"))
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setPixelFormat(v uint) {
	renderPipelineColorAttachmentDescriptorSetPixelFormat(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setWriteMask(v uint) {
	renderPipelineColorAttachmentDescriptorSetWriteMask(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setBlendingState(v uint) {
	renderPipelineColorAttachmentDescriptorSetBlendingState(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setSourceRGBBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetSourceRGBBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setDestinationRGBBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetDestinationRGBBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setRgbBlendOperation(v uint) {
	renderPipelineColorAttachmentDescriptorSetRGBBlendOperation(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setSourceAlphaBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetSourceAlphaBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setDestinationAlphaBlendFactor(v uint) {
	renderPipelineColorAttachmentDescriptorSetDestinationAlphaBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setAlphaBlendOperation(v uint) {
	renderPipelineColorAttachmentDescriptorSetAlphaBlendOperation(d, v)
}
