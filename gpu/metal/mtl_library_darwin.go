//go:build darwin && cgo

package metal

type mtlLibrary id

func (l mtlLibrary) release() {
	release(l)
}

func (l mtlLibrary) newFunctionWithName(name id) id {
	return libraryNewFunctionWithName(l, name)
}
