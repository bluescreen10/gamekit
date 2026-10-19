//go:build darwin && cgo

package metal

// mtlDrawable wraps a CAMetalDrawable, which extends MTLDrawable with a texture.
type mtlDrawable id

func (d mtlDrawable) release() {
	release(d)
}

func (d mtlDrawable) retain() mtlDrawable {
	return mtlDrawable(retain(id(d)))
}

func (d mtlDrawable) texture() mtlTexture {
	return drawableTexture(d)
}

func (d mtlDrawable) present() {
	drawablePresent(d)
}
