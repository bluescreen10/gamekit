//go:build darwin && cgo

package metal

type mtlSamplerState id

func (s mtlSamplerState) release() {
	release(s)
}

func (s mtlSamplerState) gpuResourceID() uint64 {
	return samplerStateGPUResourceID(s)
}
