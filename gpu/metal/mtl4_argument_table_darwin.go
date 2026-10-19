//go:build darwin && cgo

package metal

type mtl4ArgumentTable id

func (t mtl4ArgumentTable) release() {
	release(t)
}

func (t mtl4ArgumentTable) setAddress(address uint64, index uint) {
	argumentTableSetAddress(t, address, index)
}
