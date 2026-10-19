//go:build darwin && cgo

package metal

type texture id

func (t texture) release() {
	release(t)
}

func (t texture) setLabel(label string) {
	setLabel(t, label)
}

func (t texture) gpuResourceID() uint64 {
	return textureGPUResourceID(t)
}

func (t texture) width() uint {
	return textureWidth(t)
}

func (t texture) height() uint {
	return textureHeight(t)
}

func (t texture) pixelFormat() uint {
	return texturePixelFormat(t)
}

func (t texture) newTextureViewWithPixelFormat(format, textureType, firstLevel, levels, firstSlice, slices uint) texture {
	return textureNewTextureViewWithPixelFormat(t, format, textureType, firstLevel, levels, firstSlice, slices)
}
