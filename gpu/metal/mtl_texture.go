//go:build darwin && cgo

package metal

type mtlTexture id

func (t mtlTexture) release() {
	release(t)
}

func (t mtlTexture) setLabel(label string) {
	setLabel(t, label)
}

func (t mtlTexture) gpuResourceID() uint64 {
	return textureGPUResourceID(t)
}

func (t mtlTexture) width() uint {
	return textureWidth(t)
}

func (t mtlTexture) height() uint {
	return textureHeight(t)
}

func (t mtlTexture) pixelFormat() mtlPixelFormat {
	return texturePixelFormat(t)
}

func (t mtlTexture) newTextureViewWithPixelFormat(format mtlPixelFormat, textureType mtlTextureType, firstLevel, levels, firstSlice, slices uint) mtlTexture {
	return textureNewTextureViewWithPixelFormat(t, format, textureType, firstLevel, levels, firstSlice, slices)
}
