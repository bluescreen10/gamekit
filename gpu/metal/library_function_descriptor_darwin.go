//go:build darwin && cgo

package metal

type libraryFunctionDescriptor id

func (d libraryFunctionDescriptor) release() {
	release(d)
}

func newLibraryFunctionDescriptor() libraryFunctionDescriptor {
	return libraryFunctionDescriptor(newObject("MTL4LibraryFunctionDescriptor"))
}

func (d libraryFunctionDescriptor) setLibrary(l library) {
	libraryFunctionDescriptorSetLibrary(d, l)
}

func (d libraryFunctionDescriptor) setName(name id) {
	libraryFunctionDescriptorSetName(d, name)
}
