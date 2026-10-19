//go:build darwin && cgo

package metal

type library id

func (l library) release() {
	release(l)
}

func (l library) newFunctionWithName(name id) id {
	return libraryNewFunctionWithName(l, name)
}
