//go:build darwin && cgo

package metal

type sharedEvent id

func (e sharedEvent) release() {
	release(e)
}

func (e sharedEvent) waitUntilSignaledValue(value uint64, timeoutMS uint) bool {
	return sharedEventWaitUntilSignaledValue(e, value, timeoutMS)
}
