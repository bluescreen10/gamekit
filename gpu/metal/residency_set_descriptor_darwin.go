//go:build darwin && cgo

package metal

type residencySetDescriptor id

func (d residencySetDescriptor) release() {
	release(d)
}

func newResidencySetDescriptor() residencySetDescriptor {
	return residencySetDescriptor(newObject("MTLResidencySetDescriptor"))
}
