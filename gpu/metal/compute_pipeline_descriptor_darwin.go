//go:build darwin && cgo

package metal

type computePipelineDescriptor id

func (d computePipelineDescriptor) release() {
	release(d)
}

func newComputePipelineDescriptor() computePipelineDescriptor {
	return computePipelineDescriptor(newObject("MTL4ComputePipelineDescriptor"))
}

func (d computePipelineDescriptor) setComputeFunctionDescriptor(f specializedFunctionDescriptor) {
	computePipelineDescriptorSetComputeFunctionDescriptor(d, f)
}

func (d computePipelineDescriptor) setOptions(p id) {
	computePipelineDescriptorSetOptions(d, p)
}
