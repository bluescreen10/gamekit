//go:build darwin && cgo

package metal

type mtl4Compiler id

func (c mtl4Compiler) release() {
	release(c)
}

func (c mtl4Compiler) newComputePipelineStateWithDescriptor(descriptor mtl4ComputePipelineDescriptor, options id, err *id) mtlComputePipelineState {
	return compilerNewComputePipelineStateWithDescriptor(c, descriptor, options, err)
}

func (c mtl4Compiler) newRenderPipelineStateWithDescriptor(descriptor mtl4RenderPipelineDescriptor, options id, err *id) mtlRenderPipelineState {
	return compilerNewRenderPipelineStateWithDescriptor(c, descriptor, options, err)
}
