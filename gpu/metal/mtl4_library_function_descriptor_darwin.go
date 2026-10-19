//go:build darwin && cgo

package metal

type mtl4LibraryFunctionDescriptor id

func (d mtl4LibraryFunctionDescriptor) release() {
	release(d)
}

func newMTL4LibraryFunctionDescriptor() mtl4LibraryFunctionDescriptor {
	return mtl4LibraryFunctionDescriptor(newObject("MTL4LibraryFunctionDescriptor"))
}

func (d mtl4LibraryFunctionDescriptor) setLibrary(l mtlLibrary) {
	libraryFunctionDescriptorSetLibrary(d, l)
}

func (d mtl4LibraryFunctionDescriptor) setName(name id) {
	libraryFunctionDescriptorSetName(d, name)
}
