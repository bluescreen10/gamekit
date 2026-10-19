//go:build darwin && cgo

package metal

import "unsafe"

type mtlBuffer id

func (b mtlBuffer) release() {
	release(b)
}

func (b mtlBuffer) setLabel(label string) {
	setLabel(b, label)
}

func (b mtlBuffer) contents() unsafe.Pointer {
	return bufferContents(b)
}

func (b mtlBuffer) gpuAddress() uint64 {
	return bufferGPUAddress(b)
}
