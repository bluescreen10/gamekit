//go:build darwin && cgo

package metal

type renderPipelineDescriptor id

func (d renderPipelineDescriptor) release() {
	release(d)
}

func newRenderPipelineDescriptor() renderPipelineDescriptor {
	return renderPipelineDescriptor(newObject("MTL4RenderPipelineDescriptor"))
}

func (d renderPipelineDescriptor) setVertexFunctionDescriptor(f specializedFunctionDescriptor) {
	renderPipelineDescriptorSetVertexFunctionDescriptor(d, f)
}

func (d renderPipelineDescriptor) setFragmentFunctionDescriptor(f specializedFunctionDescriptor) {
	renderPipelineDescriptorSetFragmentFunctionDescriptor(d, f)
}

func (d renderPipelineDescriptor) setRasterSampleCount(v uint) {
	renderPipelineDescriptorSetRasterSampleCount(d, v)
}

func (d renderPipelineDescriptor) setOptions(p id) {
	renderPipelineDescriptorSetOptions(d, p)
}

func (d renderPipelineDescriptor) colorAttachments() id {
	return renderPipelineDescriptorColorAttachments(d)
}
