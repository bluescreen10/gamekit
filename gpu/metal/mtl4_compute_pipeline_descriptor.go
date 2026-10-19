//go:build darwin && cgo

package metal

type mtl4ComputePipelineDescriptor id

func (d mtl4ComputePipelineDescriptor) release() {
	release(d)
}

func (d mtl4ComputePipelineDescriptor) setLabel(label string) {
	setLabel(d, label)
}

func newMTL4ComputePipelineDescriptor() mtl4ComputePipelineDescriptor {
	return mtl4ComputePipelineDescriptor(newObject("MTL4ComputePipelineDescriptor"))
}

func (d mtl4ComputePipelineDescriptor) setComputeFunctionDescriptor(f mtl4SpecializedFunctionDescriptor) {
	computePipelineDescriptorSetComputeFunctionDescriptor(d, f)
}

func (d mtl4ComputePipelineDescriptor) setOptions(p id) {
	computePipelineDescriptorSetOptions(d, p)
}
