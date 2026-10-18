//go:build darwin && cgo

package metal

type specializedFunctionDescriptor id

func (d specializedFunctionDescriptor) release() {
	release(d)
}

func newSpecializedFunctionDescriptor() specializedFunctionDescriptor {
	return specializedFunctionDescriptor(newObject("MTL4SpecializedFunctionDescriptor"))
}

func (d specializedFunctionDescriptor) setFunctionDescriptor(f libraryFunctionDescriptor) {
	specializedFunctionDescriptorSetFunctionDescriptor(d, f)
}

func (d specializedFunctionDescriptor) setConstantValues(v id) {
	specializedFunctionDescriptorSetConstantValues(d, v)
}
