//go:build linux && cgo

package gtk

import (
	"errors"
	"sync"
	"unsafe"
)

var sessionBusMu sync.Mutex
var sessionBusConnection *gDBusConnection

func sessionBus() *gDBusConnection {
	sessionBusMu.Lock()
	defer sessionBusMu.Unlock()
	if sessionBusConnection == nil {
		sessionBusConnection = gBusGetSync(gBusTypeSession, nil, nil)
	}
	return sessionBusConnection
}

func callNativeBus(bus *gDBusConnection, destination, path, iface, method string, parameters *gVariant, replySignature string, timeout int) (*gVariant, error) {
	if bus == nil {
		if parameters != nil {
			gVariantUnref(parameters)
		}
		return nil, errors.New("ui/gtk: there is no session bus")
	}
	var replyType *gVariantType
	if replySignature != "" {
		replyType = gVariantTypeNew(stringPointer(replySignature))
		defer gVariantTypeFree(replyType)
	}
	var failure *gError
	reply := gDBusConnectionCallSync(bus, stringPointer(destination), stringPointer(path), stringPointer(iface), stringPointer(method),
		parameters, replyType, gDBusCallFlagsNone, cInt(timeout), nil, &failure)
	if failure != nil {
		defer gErrorFree(failure)
		return nil, errors.New("ui/gtk: " + goString(failure.message))
	}
	if reply == nil {
		return nil, errors.New("ui/gtk: the D-Bus service did not answer")
	}
	return reply, nil
}

// Variant constructors copy scalar values and adopt floating child variants.
func variantString(value string) *gVariant {
	return gVariantNewString(stringPointer(value))
}

func variantTuple(children ...*gVariant) *gVariant {
	return gVariantNewTuple(unsafe.SliceData(children), gSize(len(children)))
}

func variantArray(signature string, children []*gVariant) *gVariant {
	kind := gVariantTypeNew(stringPointer(signature))
	defer gVariantTypeFree(kind)
	return gVariantNewArray(kind, unsafe.SliceData(children), gSize(len(children)))
}

func variantStrings(values []string) *gVariant {
	children := make([]*gVariant, len(values))
	for i, value := range values {
		children[i] = variantString(value)
	}
	return variantArray("s", children)
}

func variantDictionary(values map[string]*gVariant) *gVariant {
	children := make([]*gVariant, 0, len(values))
	for key, value := range values {
		children = append(children, gVariantNewDictEntry(variantString(key), gVariantNewVariant(value)))
	}
	return variantArray("{sv}", children)
}

func variantStringValue(value *gVariant) string {
	return goString(gVariantGetString(value, nil))
}

func variantChildString(value *gVariant, index int) string {
	child := gVariantGetChildValue(value, gSize(index))
	defer gVariantUnref(child)
	return variantStringValue(child)
}

func variantChildUint32(value *gVariant, index int) uint32 {
	child := gVariantGetChildValue(value, gSize(index))
	defer gVariantUnref(child)
	return uint32(gVariantGetUint32(child))
}

type nativeBusObject struct {
	bus   *gDBusConnection
	id    cUint
	table *gDBusInterfaceVTable
}

type nativeBusMethods struct {
	method   func(string, *gVariant, *gDBusMethodInvocation)
	property func(string) *gVariant
}

type nativeBusWatch struct {
	appeared func()
	vanished func()
}

func registerNativeBusObject(bus *gDBusConnection, path, xml string, methods *nativeBusMethods) (*nativeBusObject, error) {
	var failure *gError
	info := gDBusNodeInfoNewForXml(stringPointer(xml), &failure)
	if failure != nil {
		defer gErrorFree(failure)
		return nil, errors.New("ui/gtk: " + goString(failure.message))
	}
	defer gDBusNodeInfoUnref(info)
	table := (*gDBusInterfaceVTable)(gMalloc(gSize(unsafe.Sizeof(gDBusInterfaceVTable{}))))
	*table = gDBusInterfaceVTable{method_call: gtkDBusMethodCallback(), get_property: gtkDBusPropertyCallback()}
	data := newNativeCallback(methods)
	id := gDBusConnectionRegisterObject(bus, stringPointer(path), *info.interfaces, table, data, gtkDestroyCallback(), &failure)
	if id == 0 {
		gFree(unsafe.Pointer(table))
		freeNativeCallback(data)
		if failure != nil {
			defer gErrorFree(failure)
			return nil, errors.New("ui/gtk: " + goString(failure.message))
		}
		return nil, errors.New("ui/gtk: D-Bus object registration failed")
	}
	return &nativeBusObject{bus: bus, id: id, table: table}, nil
}

func (object *nativeBusObject) close() {
	gDBusConnectionUnregisterObject(object.bus, object.id)
	gFree(unsafe.Pointer(object.table))
}

func invokeNativeBusMethod(method string, parameters *gVariant, invocation *gDBusMethodInvocation, data unsafe.Pointer) {
	nativeCallback(data).(*nativeBusMethods).method(method, parameters, invocation)
}

func invokeNativeBusProperty(name string, data unsafe.Pointer) *gVariant {
	return nativeCallback(data).(*nativeBusMethods).property(name)
}

func variantChildInt(value *gVariant, index int) int {
	child := gVariantGetChildValue(value, gSize(index))
	defer gVariantUnref(child)
	return int(gVariantGetInt32(child))
}

func variantBytes(data []byte) *gVariant {
	kind := gVariantTypeNew(stringPointer("y"))
	defer gVariantTypeFree(kind)
	return gVariantNewFixedArray(kind, unsafe.Pointer(unsafe.SliceData(data)), gSize(len(data)), 1)
}
