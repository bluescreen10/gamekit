//go:build linux && cgo

package gtk

func (w *Window) nativeGTKWindow() *gtkWindow { return (*gtkWindow)(w.nativePointer()) }
