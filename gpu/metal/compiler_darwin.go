//go:build darwin && cgo

package metal

type compiler id

func (c compiler) release() {
	release(c)
}

func (c compiler) newComputePipelineStateWithDescriptor(descriptor computePipelineDescriptor, options id, err *id) id {
	return compilerNewComputePipelineStateWithDescriptor(c, descriptor, options, err)
}

func (c compiler) newRenderPipelineStateWithDescriptor(descriptor renderPipelineDescriptor, options id, err *id) id {
	return compilerNewRenderPipelineStateWithDescriptor(c, descriptor, options, err)
}
