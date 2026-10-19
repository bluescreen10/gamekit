//go:build darwin && cgo

package metal

type mtlRenderPipelineState id

func (p mtlRenderPipelineState) release() {
	release(p)
}

func (p mtlRenderPipelineState) reflection() id {
	return renderPipelineStateReflection(p)
}
