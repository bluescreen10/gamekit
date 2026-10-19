//go:build darwin && cgo

package metal

type fence id

func (f fence) release() {
	release(f)
}
