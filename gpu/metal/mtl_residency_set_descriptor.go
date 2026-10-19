//go:build darwin && cgo

package metal

type mtlResidencySetDescriptor id

func (d mtlResidencySetDescriptor) release() {
	release(d)
}

func (d mtlResidencySetDescriptor) setLabel(label string) {
	setLabel(d, label)
}

func newMTLResidencySetDescriptor() mtlResidencySetDescriptor {
	return mtlResidencySetDescriptor(newObject("MTLResidencySetDescriptor"))
}
