//go:build darwin && cgo

package metal

import "errors"

type mtl4Compiler id

func (c mtl4Compiler) release() {
	release(c)
}

func (c mtl4Compiler) newComputePipelineState(descriptor mtl4ComputePipelineDescriptor, options id) (mtlComputePipelineState, error) {
	var nativeError id
	object := compilerNewComputePipelineStateWithDescriptor(c, descriptor, options, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "compute pipeline creation failed"))
	}

	return object, nil
}

func (c mtl4Compiler) newRenderPipelineState(descriptor mtl4RenderPipelineDescriptor, options id) (mtlRenderPipelineState, error) {
	var nativeError id
	object := compilerNewRenderPipelineStateWithDescriptor(c, descriptor, options, &nativeError)
	if object == 0 {
		return 0, errors.New(errorText(nativeError, "render pipeline creation failed"))
	}

	return object, nil
}
