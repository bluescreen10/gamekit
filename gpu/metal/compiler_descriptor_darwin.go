//go:build darwin && cgo

package metal

type compilerDescriptor id

func (d compilerDescriptor) release() {
	release(d)
}

func newCompilerDescriptor() compilerDescriptor {
	return compilerDescriptor(newObject("MTL4CompilerDescriptor"))
}
