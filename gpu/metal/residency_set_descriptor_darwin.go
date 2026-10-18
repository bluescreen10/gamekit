//go:build darwin && cgo

package metal

type residencySetDescriptor id

func (d residencySetDescriptor) release() {
	release(d)
}

func (d residencySetDescriptor) setLabel(label string) {
	setLabel(d, label)
}

func newResidencySetDescriptor() residencySetDescriptor {
	return residencySetDescriptor(newObject("MTLResidencySetDescriptor"))
}
