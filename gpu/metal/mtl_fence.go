//go:build darwin && cgo

package metal

type mtlFence id

func (f mtlFence) release() {
	release(f)
}
