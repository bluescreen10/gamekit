//go:build darwin && cgo

package metal

type mtlComputePipelineState id

func (p mtlComputePipelineState) release() {
	release(p)
}

func (p mtlComputePipelineState) maxTotalThreadsPerThreadgroup() uint {
	return computePipelineStateMaxTotalThreadsPerThreadgroup(p)
}

func (p mtlComputePipelineState) reflection() id {
	return computePipelineStateReflection(p)
}
