//go:build darwin && cgo

package metal

type depthStencilState id

func (s depthStencilState) release() {
	release(s)
}
