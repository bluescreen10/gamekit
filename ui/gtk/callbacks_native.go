//go:build linux && cgo

package gtk

import (
	"runtime/cgo"
	"unsafe"
)

// Native callbacks retain a handle in native storage, never a pointer into Go.
// Signals own it until their closure is destroyed; async calls own it until
// their completion callback runs, including cancellation and error paths.
func newNativeCallback(value any) unsafe.Pointer {
	data := gMalloc(gSize(unsafe.Sizeof(uintptr(0))))
	*(*uintptr)(data) = uintptr(cgo.NewHandle(value))
	return data
}

func nativeCallback(data unsafe.Pointer) any {
	return cgo.Handle(*(*uintptr)(data)).Value()
}

func freeNativeCallback(data unsafe.Pointer) {
	if value, ok := nativeCallback(data).(interface{ releaseNative() }); ok {
		value.releaseNative()
	}
	cgo.Handle(*(*uintptr)(data)).Delete()
	gFree(data)
}

// connectNativeSignal is for signals with one instance argument and no return
// value, such as clicked, changed, and closed.
func connectNativeSignal(instance unsafe.Pointer, signal string, callback func()) gUlong {
	return gSignalConnectData(instance, stringPointer(signal), gtkSignalCallback(),
		newNativeCallback(callback), gtkCallbackFree(), 0)
}

func invokeNativeSignal(data unsafe.Pointer) {
	nativeCallback(data).(func())()
}

func asyncNativeCallback(callback func(*gObject, *gAsyncResult)) unsafe.Pointer {
	return newNativeCallback(callback)
}

func invokeNativeAsync(source *gObject, result *gAsyncResult, data unsafe.Pointer) {
	defer freeNativeCallback(data)
	nativeCallback(data).(func(*gObject, *gAsyncResult))(source, result)
}

func runNativePending() {
	for gMainContextIteration(nil, 0) != 0 {
	}
}

func connectNativeCloseRequest(window *gtkWindow, callback func() bool) gUlong {
	return gSignalConnectData(unsafe.Pointer(window), stringPointer("close-request"), gtkCloseRequestCallback(),
		newNativeCallback(callback), gtkCallbackFree(), 0)
}

func invokeNativeCloseRequest(data unsafe.Pointer) gBoolean {
	if nativeCallback(data).(func() bool)() {
		return 1
	}
	return 0
}

func invokeNativeSource(data unsafe.Pointer) gBoolean {
	if nativeCallback(data).(func() bool)() {
		return 1
	}
	return 0
}

func invokeNativeDBusSignal(signal *cChar, parameters *gVariant, data unsafe.Pointer) {
	nativeCallback(data).(func(string, *gVariant))(goString(signal), parameters)
}

func connectNativeNotify(instance unsafe.Pointer, signal string, callback func()) gUlong {
	return gSignalConnectData(instance, stringPointer(signal), gtkNotifyCallback(), newNativeCallback(callback), gtkCallbackFree(), 0)
}

func invokeNativeTick(clock *gdkFrameClock, data unsafe.Pointer) gBoolean {
	return nativeBool(nativeCallback(data).(func(*gdkFrameClock) bool)(clock))
}

func invokeNativeKey(keyval, keycode uint32, state gdkModifierType, data unsafe.Pointer) gBoolean {
	return nativeBool(nativeCallback(data).(func(uint32, uint32, gdkModifierType) bool)(keyval, keycode, state))
}
