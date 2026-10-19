//go:build darwin && cgo

package metal

type computePipelineState id

func (p computePipelineState) release() {
	release(p)
}

func (p computePipelineState) maxTotalThreadsPerThreadgroup() uint {
	return computePipelineStateMaxTotalThreadsPerThreadgroup(p)
}

func (p computePipelineState) reflection() id {
	return computePipelineStateReflection(p)
}
