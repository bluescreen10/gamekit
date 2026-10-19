//go:build darwin && cgo

package metal

type caMetalLayer id

// newCAMetalLayer returns an autoreleased layer from CAMetalLayer.layer.
func newCAMetalLayer() caMetalLayer {
	return metalLayerLayer()
}

func (l caMetalLayer) release() {
	release(l)
}

func (l caMetalLayer) retain() caMetalLayer {
	return caMetalLayer(retain(id(l)))
}

func (l caMetalLayer) setDevice(device mtlDevice) {
	metalLayerSetDevice(l, device)
}

func (l caMetalLayer) setPixelFormat(format mtlPixelFormat) {
	metalLayerSetPixelFormat(l, format)
}

func (l caMetalLayer) setFramebufferOnly(enabled bool) {
	metalLayerSetFramebufferOnly(l, enabled)
}

func (l caMetalLayer) setDisplaySyncEnabled(enabled bool) {
	metalLayerSetDisplaySyncEnabled(l, enabled)
}

func (l caMetalLayer) setContentsScale(scale float64) {
	metalLayerSetContentsScale(l, scale)
}

func (l caMetalLayer) setDrawableSize(width, height float64) {
	metalLayerSetDrawableSize(l, width, height)
}

func (l caMetalLayer) drawableSize() (width, height float64) {
	return metalLayerDrawableSize(l)
}

func (l caMetalLayer) nextDrawable() mtlDrawable {
	return metalLayerNextDrawable(l)
}
