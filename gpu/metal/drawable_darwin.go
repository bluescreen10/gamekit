//go:build darwin && cgo

package metal

type drawable id

func (d drawable) release() {
	release(d)
}

func (d drawable) retain() drawable {
	return drawable(retain(id(d)))
}

func (d drawable) texture() texture {
	return drawableTexture(d)
}

func (d drawable) present() {
	drawablePresent(d)
}
