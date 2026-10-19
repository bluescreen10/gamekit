//go:build darwin && cgo

package metal

type renderPipelineState id

func (p renderPipelineState) release() {
	release(p)
}

func (p renderPipelineState) reflection() id {
	return renderPipelineStateReflection(p)
}
