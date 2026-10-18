//go:build darwin && cgo

package metal

type argumentTable id

func (t argumentTable) release() {
	release(t)
}

func (t argumentTable) setAddress(address uint64, index uint) {
	argumentTableSetAddress(t, address, index)
}
