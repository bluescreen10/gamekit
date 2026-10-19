//go:build darwin && cgo

package metal

type mtl4RenderPipelineDescriptor id

func (d mtl4RenderPipelineDescriptor) release() {
	release(d)
}

func (d mtl4RenderPipelineDescriptor) setLabel(label string) {
	setLabel(d, label)
}

func newMTL4RenderPipelineDescriptor() mtl4RenderPipelineDescriptor {
	return mtl4RenderPipelineDescriptor(newObject("MTL4RenderPipelineDescriptor"))
}

func (d mtl4RenderPipelineDescriptor) setVertexFunctionDescriptor(f mtl4SpecializedFunctionDescriptor) {
	renderPipelineDescriptorSetVertexFunctionDescriptor(d, f)
}

func (d mtl4RenderPipelineDescriptor) setFragmentFunctionDescriptor(f mtl4SpecializedFunctionDescriptor) {
	renderPipelineDescriptorSetFragmentFunctionDescriptor(d, f)
}

func (d mtl4RenderPipelineDescriptor) setRasterSampleCount(v uint) {
	renderPipelineDescriptorSetRasterSampleCount(d, v)
}

func (d mtl4RenderPipelineDescriptor) setOptions(p id) {
	renderPipelineDescriptorSetOptions(d, p)
}

func (d mtl4RenderPipelineDescriptor) colorAttachments() id {
	return renderPipelineDescriptorColorAttachments(d)
}
