//go:build darwin && cgo

package metal

type mtlDepthStencilState id

func (s mtlDepthStencilState) release() {
	release(s)
}
