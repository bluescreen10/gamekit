//go:build darwin && cgo

package metal

type mtl4CompilerDescriptor id

func (d mtl4CompilerDescriptor) release() {
	release(d)
}

func newMTL4CompilerDescriptor() mtl4CompilerDescriptor {
	return mtl4CompilerDescriptor(newObject("MTL4CompilerDescriptor"))
}
