//go:build darwin && cgo

package metal

type sampler id

func (s sampler) release() {
	release(s)
}

func (s sampler) gpuResourceID() uint64 {
	return samplerStateGPUResourceID(s)
}
