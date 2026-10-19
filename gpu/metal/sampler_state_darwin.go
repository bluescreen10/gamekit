//go:build darwin && cgo

package metal

type samplerState id

func (s samplerState) release() {
	release(s)
}

func (s samplerState) gpuResourceID() uint64 {
	return samplerStateGPUResourceID(s)
}
