//go:build darwin && cgo

package metal

type mtl4RenderPipelineColorAttachmentDescriptor id

func (d mtl4RenderPipelineColorAttachmentDescriptor) release() {
	release(d)
}

func newMTL4RenderPipelineColorAttachmentDescriptor() mtl4RenderPipelineColorAttachmentDescriptor {
	return mtl4RenderPipelineColorAttachmentDescriptor(newObject("MTL4RenderPipelineColorAttachmentDescriptor"))
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setPixelFormat(v mtlPixelFormat) {
	renderPipelineColorAttachmentDescriptorSetPixelFormat(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setWriteMask(v mtlColorWriteMask) {
	renderPipelineColorAttachmentDescriptorSetWriteMask(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setBlendingState(v mtl4BlendState) {
	renderPipelineColorAttachmentDescriptorSetBlendingState(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setSourceRGBBlendFactor(v mtlBlendFactor) {
	renderPipelineColorAttachmentDescriptorSetSourceRGBBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setDestinationRGBBlendFactor(v mtlBlendFactor) {
	renderPipelineColorAttachmentDescriptorSetDestinationRGBBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setRgbBlendOperation(v mtlBlendOperation) {
	renderPipelineColorAttachmentDescriptorSetRGBBlendOperation(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setSourceAlphaBlendFactor(v mtlBlendFactor) {
	renderPipelineColorAttachmentDescriptorSetSourceAlphaBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setDestinationAlphaBlendFactor(v mtlBlendFactor) {
	renderPipelineColorAttachmentDescriptorSetDestinationAlphaBlendFactor(d, v)
}

func (d mtl4RenderPipelineColorAttachmentDescriptor) setAlphaBlendOperation(v mtlBlendOperation) {
	renderPipelineColorAttachmentDescriptorSetAlphaBlendOperation(d, v)
}
