//go:build darwin && cgo

package metal

type mtlSharedEvent id

func (e mtlSharedEvent) release() {
	release(e)
}

func (e mtlSharedEvent) waitUntilSignaledValue(value uint64, timeoutMS uint) bool {
	return sharedEventWaitUntilSignaledValue(e, value, timeoutMS)
}
