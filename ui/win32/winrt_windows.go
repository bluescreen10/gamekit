package win32

import (
	"unsafe"
)

// The backend reaches the Windows Runtime through its ABI, without a
// projection; win32.go declares the methods of the interfaces it calls.

// activationFactory returns the activation factory of a runtime class with an
// interface, or 0.
func activationFactory(class string, iid *guid) comObject {
	name := win32WindowsCreateString(class)
	defer win32WindowsDeleteString(name)
	var factory comObject
	win32RoGetActivationFactory(name, iid, &factory)
	return factory
}

// takeString returns the text of an HSTRING that a method returned and deletes
// it.
func takeString(text hstring) string {
	defer win32WindowsDeleteString(text)
	return win32WindowsGetStringRawBuffer(text)
}

// awaitOperation waits for an asynchronous operation, processing messages,
// and returns its result. It releases the operation.
func awaitOperation(operation comObject, timeout float64) comObject {
	if operation == 0 {
		return 0
	}
	defer win32UnknownRelease(operation)
	info := query(operation, iidAsyncInfo)
	if info == 0 {
		return 0
	}
	status := int32(asyncStatusStarted)
	pump(func() bool {
		win32AsyncInfoGetStatus(info, &status)
		return status != asyncStatusStarted
	}, timeout)
	win32UnknownRelease(info)
	var result comObject
	if status == asyncStatusCompleted {
		win32AsyncOperationGetResults(operation, &result)
	}
	return result
}

// eventHandlerTable is the method table of a TypedEventHandler, whose Invoke
// receives the sender and the arguments of an event.
var eventHandlerTable []uintptr

// newEventHandler returns a TypedEventHandler of the Windows Runtime with the
// interface iid. It is agile, so its events can arrive on any thread.
func newEventHandler(iid *guid, work func(sender, args comObject)) *comServer {
	if eventHandlerTable == nil {
		eventHandlerTable = methodTable(callback("TypedEventHandler.Invoke", func(this, sender, args uintptr) uintptr {
			if work, ok := serverObject[func(comObject, comObject)](this); ok {
				work(comObject(sender), comObject(args))
			}
			return sOK
		}))
	}
	return newCOMServer(eventHandlerTable, []*guid{iid, iidAgileObject}, work)
}

// inspectableMethods returns the methods of IInspectable for a server, which
// names its first interface as its only one.
func inspectableMethods() []uintptr {
	return []uintptr{
		callback("IInspectable.GetIids", func(this, count, ids uintptr) uintptr {
			server := serverAt(this)
			if server == nil || len(server.interfaces) == 0 {
				return uintptr(eNoInterface)
			}
			memory := win32CoTaskMemAlloc(unsafe.Sizeof(guid{}))
			if memory == 0 {
				return uintptr(eOutOfMemory)
			}
			*(*guid)(memoryAt(memory)) = *server.interfaces[0]
			*(*uint32)(memoryAt(count)) = 1
			*(*uintptr)(memoryAt(ids)) = memory
			return sOK
		}),
		callback("IInspectable.GetRuntimeClassName", func(this, name uintptr) uintptr {
			*(*hstring)(memoryAt(name)) = 0
			return sOK
		}),
		callback("IInspectable.GetTrustLevel", func(this, level uintptr) uintptr {
			*(*int32)(memoryAt(level)) = baseTrust
			return sOK
		}),
	}
}
