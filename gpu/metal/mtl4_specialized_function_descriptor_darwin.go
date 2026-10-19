//go:build darwin && cgo

package metal

type mtl4SpecializedFunctionDescriptor id

func (d mtl4SpecializedFunctionDescriptor) release() {
	release(d)
}

func newMTL4SpecializedFunctionDescriptor() mtl4SpecializedFunctionDescriptor {
	return mtl4SpecializedFunctionDescriptor(newObject("MTL4SpecializedFunctionDescriptor"))
}

func (d mtl4SpecializedFunctionDescriptor) setFunctionDescriptor(f mtl4LibraryFunctionDescriptor) {
	specializedFunctionDescriptorSetFunctionDescriptor(d, f)
}

func (d mtl4SpecializedFunctionDescriptor) setConstantValues(v id) {
	specializedFunctionDescriptorSetConstantValues(d, v)
}
