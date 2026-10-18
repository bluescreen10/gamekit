//go:build darwin && cgo

package metal

import "unsafe"

type buffer id

func (b buffer) release() {
	release(b)
}

func (b buffer) setLabel(label string) {
	setLabel(b, label)
}

func (b buffer) contents() unsafe.Pointer {
	return bufferContents(b)
}

func (b buffer) gpuAddress() uint64 {
	return bufferGPUAddress(b)
}
