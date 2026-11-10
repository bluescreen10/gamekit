//go:build windows

package win32

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// The package calls Windows without cgo. This file is the package's only
// contact with Windows: it loads the libraries of Windows, calls their
// functions and the methods of their COM objects, and implements the COM
// objects that Windows calls back. The rest of the package calls the functions
// declared here, which follow one naming convention, so that a Windows name
// always gives its Go name:
//
//   - A function is win32 and its Windows name, without the W suffix of the
//     wide-character functions: CreateWindowExW is win32CreateWindowEx, and
//     ImageList_Create is win32ImageListCreate.
//   - A COM method is win32, its interface without the I, and the method:
//     ID2D1RenderTarget's FillRectangle is win32D2D1RenderTargetFillRectangle,
//     IDWriteTextLayout's GetMetrics is win32DWriteTextLayoutGetMetrics, and
//     IUnknown's Release is win32UnknownRelease. A method is declared for the
//     interface that introduces it, and called on the interfaces that inherit
//     it.
//   - A constant or a struct is its Windows name in camel case at its word
//     boundaries, with the first word lowered and the W suffix dropped:
//     WM_LBUTTONDOWN is wmLButtonDown, and PAINTSTRUCT is paintStruct. A
//     struct whose name would be the name of a function or a constant takes
//     the suffix Info: TRACKMOUSEEVENT is trackMouseEventInfo.
//   - Parameters and results are Go types: a handle is a named uintptr, a
//     BOOL a bool, a string a Go string, and a COM interface a comObject.
//
// Go structs that Windows reads hold Go pointers only in pointer-typed fields,
// so the garbage collector keeps what they point at alive during a call.

// library is a DLL of Windows, loaded when a function of it is first called.
type library struct {
	name string
	// load loads the library. It is nil for a library of System32.
	load func() uintptr

	once   sync.Once
	handle uintptr
}

// proc is a function of a library, found when it is first called.
type proc struct {
	library *library
	name    string
	ordinal uint16

	once    sync.Once
	address uintptr
}

func newLibrary(name string) *library {
	return &library{name: name}
}

func (l *library) proc(name string) *proc {
	return &proc{library: l, name: name}
}

// ordinalProc is a function that its library exports only by ordinal.
func (l *library) ordinalProc(ordinal uint16) *proc {
	return &proc{library: l, name: fmt.Sprintf("#%d", ordinal), ordinal: ordinal}
}

var (
	kernel32Library        = syscall.NewLazyDLL("kernel32.dll")
	procLoadLibraryEx      = kernel32Library.NewProc("LoadLibraryExW")
	procLoadLibrary        = kernel32Library.NewProc("LoadLibraryW")
	procGetProcAddress     = kernel32Library.NewProc("GetProcAddress")
	loadLibrarySearchSys32 = uintptr(0x00000800)
)

func (l *library) loaded() uintptr {
	l.once.Do(func() {
		if l.load != nil {
			l.handle = l.load()
			return
		}
		name, _ := syscall.UTF16PtrFromString(l.name)
		l.handle, _, _ = procLoadLibraryEx.Call(uintptr(unsafe.Pointer(name)), 0, loadLibrarySearchSys32)
	})
	return l.handle
}

// loadLibraryDefault loads a library with the default search of Windows, for a
// library that must load in the activation context of the caller.
func loadLibraryDefault(name string) uintptr {
	wide, _ := syscall.UTF16PtrFromString(name)
	handle, _, _ := procLoadLibrary.Call(uintptr(unsafe.Pointer(wide)))
	return handle
}

// find returns the address of the function, or 0 if Windows does not have it.
func (p *proc) find() uintptr {
	p.once.Do(func() {
		module := p.library.loaded()
		if module == 0 {
			return
		}
		if p.ordinal != 0 {
			p.address, _, _ = procGetProcAddress.Call(module, uintptr(p.ordinal))
			return
		}
		name, _ := syscall.BytePtrFromString(p.name)
		p.address, _, _ = procGetProcAddress.Call(module, uintptr(unsafe.Pointer(name)))
	})
	return p.address
}

func (p *proc) addr() uintptr {
	address := p.find()
	if address == 0 {
		panic(fmt.Sprintf("ui/win32: %s does not export %s", p.library.name, p.name))
	}
	return address
}

// call calls the function with integer and pointer arguments and returns its
// result. A pointer must be converted to uintptr in the argument list of the
// call, which makes what it points at escape to the heap: a callback during the
// call can grow the stack, which would move it.
//
//go:uintptrescapes
func (p *proc) call(args ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(p.addr(), args...)
	return result
}

// callError is like call and also returns the last error of the thread.
//
//go:uintptrescapes
func (p *proc) callError(args ...uintptr) (uintptr, error) {
	result, _, err := syscall.SyscallN(p.addr(), args...)
	return result, err
}

// call64 calls a function that returns a 64-bit integer, which 32-bit Windows
// returns in two registers.
//
//go:uintptrescapes
func (p *proc) call64(args ...uintptr) uint64 {
	low, high, _ := syscall.SyscallN(p.addr(), args...)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return uint64(low)
	}
	return uint64(high)<<32 | uint64(uint32(low))
}

// callAddress calls a function by its address, as call does.
//
//go:uintptrescapes
func callAddress(address uintptr, args ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(address, args...)
	return result
}

// memoryAt reinterprets an address of Windows memory as a pointer. It copies the
// bits rather than converting, which keeps vet from flagging a uintptr turned
// pointer; the memory is never Go memory.
func memoryAt(address uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&address))
}

// pairArgs returns the argument slots of an 8-byte struct of two 32-bit values
// passed by value, such as a POINT or a D2D1_SIZE_U: one slot on 64-bit
// Windows, and two on 32-bit Windows.
func pairArgs(low, high uint32) []uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return []uintptr{uintptr(uint64(high)<<32 | uint64(low))}
	}
	return []uintptr{uintptr(low), uintptr(high)}
}

// pairOf returns the struct that a callback received by value in one slot on
// 64-bit Windows; see pairArgs.
func pairOf(value uintptr) (low, high uint32) {
	return uint32(value), uint32(uint64(value) >> 32)
}

// guidArgs returns the argument slots of a GUID passed by value: a pointer to
// it on amd64, its 16 bytes in two registers on arm64, and four slots on
// 386. g must not move or be freed, so it is a package variable.
func guidArgs(g *guid) []uintptr {
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(g)), unsafe.Sizeof(*g))
	word := func(i int) uint32 {
		return uint32(bytes[i]) | uint32(bytes[i+1])<<8 | uint32(bytes[i+2])<<16 | uint32(bytes[i+3])<<24
	}
	switch {
	case arm64ABI:
		return []uintptr{
			uintptr(uint64(word(0)) | uint64(word(4))<<32),
			uintptr(uint64(word(8)) | uint64(word(12))<<32),
		}
	case unsafe.Sizeof(uintptr(0)) == 8:
		return []uintptr{heapAddress(uintptr(unsafe.Pointer(g)))}
	default:
		return []uintptr{uintptr(word(0)), uintptr(word(4)), uintptr(word(8)), uintptr(word(12))}
	}
}

// signExtend returns a 32-bit value that Windows defines as a negative LONG,
// such as HKEY_CURRENT_USER, as a pointer-sized handle.
func signExtend(value uint32) uintptr {
	return uintptr(int32(value))
}

// boolArg converts a Go bool to a BOOL argument.
func boolArg(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}

// ---- COM ----

// comObject is a pointer to a COM interface. Its first word points at the
// table of the interface's methods.
type comObject uintptr

// comMethod returns the address of the method at index in the table of an
// interface.
func comMethod(object comObject, index int) uintptr {
	table := *(*uintptr)(memoryAt(uintptr(object)))
	return *(*uintptr)(memoryAt(table + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

// comCall calls the method at index with integer and pointer arguments, as
// proc.call does.
//
//go:uintptrescapes
func comCall(object comObject, index int, args ...uintptr) uintptr {
	var all [16]uintptr
	all[0] = uintptr(object)
	n := copy(all[1:], args)
	result, _, _ := syscall.SyscallN(comMethod(object, index), all[:n+1]...)
	return result
}

// hresult is the result of a COM method or function.
type hresult uint32

func (h hresult) failed() bool {
	return h&0x80000000 != 0
}

func (h hresult) succeeded() bool {
	return !h.failed()
}

func (h hresult) Error() string {
	return fmt.Sprintf("HRESULT 0x%08x", uint32(h))
}

// heapAddress returns address. The object at address moves to the heap,
// where it does not move, so the address stays correct after the call. The
// caller keeps the object alive until Windows stops using it.
//
//go:uintptrescapes
//go:noinline
func heapAddress(address uintptr) uintptr {
	return address
}

// comResult calls a method that returns an HRESULT.
//
//go:uintptrescapes
func comResult(object comObject, index int, args ...uintptr) hresult {
	var all [16]uintptr
	all[0] = uintptr(object)
	n := copy(all[1:], args)
	result, _, _ := syscall.SyscallN(comMethod(object, index), all[:n+1]...)
	return hresult(uint32(result))
}

// IUnknown methods.

func win32UnknownQueryInterface(object comObject, iid *guid, result *comObject) hresult {
	return comResult(object, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(result)))
}

func win32UnknownAddRef(object comObject) {
	if object != 0 {
		comCall(object, 1)
	}
}

func win32UnknownRelease(object comObject) {
	if object != 0 {
		comCall(object, 2)
	}
}

// query returns the interface iid of an object, or 0.
func query(object comObject, iid *guid) comObject {
	if object == 0 {
		return 0
	}
	var result comObject
	if win32UnknownQueryInterface(object, iid, &result).failed() {
		return 0
	}
	return result
}

// guid is a GUID.
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// newGUID returns a GUID from its parts, as it is written:
// newGUID(0x2cd906a8, 0x12e2, 0x11dc, 0x9fed, 0x001143a055f9).
func newGUID(a uint32, b, c, d uint16, e uint64) *guid {
	return &guid{
		Data1: a,
		Data2: b,
		Data3: c,
		Data4: [8]byte{byte(d >> 8), byte(d), byte(e >> 40), byte(e >> 32), byte(e >> 24), byte(e >> 16), byte(e >> 8), byte(e)},
	}
}

// ---- COM objects implemented in Go ----
// COM objects that the backend implements in Go.
//
// Windows holds a COM object by the address of its first word, which points at
// the table of its methods. comServer is that object: its first field is the
// table, an array of callbacks that the interface's methods share by name. The
// object lives in Go memory, which the garbage collector does not move; the
// servers map keeps it alive while Windows holds a reference, and resolves the
// address that a method receives back to the object.

type comServer struct {
	vtable *uintptr
	refs   atomic.Int32
	// interfaces are the IIDs that QueryInterface answers, besides IUnknown.
	interfaces []*guid
	// object is the Go value that implements the methods.
	object any
	// release runs when the last reference goes.
	release func()
}

var (
	serversMu sync.Mutex
	servers   = map[uintptr]*comServer{}
)

// newCOMServer creates a COM object with one reference, which belongs to the
// caller. table is the interface's method table, which must start with the
// methods of IUnknown from unknownMethods.
func newCOMServer(table []uintptr, interfaces []*guid, object any) *comServer {
	server := &comServer{vtable: &table[0], interfaces: interfaces, object: object}
	server.refs.Store(1)
	serversMu.Lock()
	servers[server.address()] = server
	serversMu.Unlock()
	return server
}

func (s *comServer) address() uintptr {
	return uintptr(unsafe.Pointer(s))
}

// com returns the object as a COM interface pointer.
func (s *comServer) com() comObject {
	return comObject(s.address())
}

// serverAt returns the server at an address that a method received, or nil.
func serverAt(this uintptr) *comServer {
	serversMu.Lock()
	defer serversMu.Unlock()
	return servers[this]
}

// serverObject returns the Go value of the server at an address.
func serverObject[T any](this uintptr) (T, bool) {
	var zero T
	server := serverAt(this)
	if server == nil {
		return zero, false
	}
	object, ok := server.object.(T)
	return object, ok
}

// unknownMethods returns the methods of IUnknown, which every table starts
// with.
func unknownMethods() []uintptr {
	return []uintptr{
		callback("IUnknown.QueryInterface", serverQueryInterface),
		callback("IUnknown.AddRef", serverAddRef),
		callback("IUnknown.Release", serverRelease),
	}
}

func serverQueryInterface(this, iid, result uintptr) uintptr {
	server := serverAt(this)
	out := (*uintptr)(memoryAt(result))
	if server == nil || out == nil {
		return uintptr(eNoInterface)
	}
	requested := *(*guid)(memoryAt(iid))
	if requested == *iidUnknown {
		*out = this
		server.refs.Add(1)
		return sOK
	}
	for _, supported := range server.interfaces {
		if requested == *supported {
			*out = this
			server.refs.Add(1)
			return sOK
		}
	}
	*out = 0
	return uintptr(eNoInterface)
}

func serverAddRef(this uintptr) uintptr {
	server := serverAt(this)
	if server == nil {
		return 0
	}
	return uintptr(server.refs.Add(1))
}

func serverRelease(this uintptr) uintptr {
	server := serverAt(this)
	if server == nil {
		return 0
	}
	count := server.refs.Add(-1)
	if count == 0 {
		serversMu.Lock()
		delete(servers, this)
		serversMu.Unlock()
		if server.release != nil {
			server.release()
		}
	}
	return uintptr(count)
}

// methodTable returns a method table of IUnknown and methods, which are named
// for the callbacks that implement them.
func methodTable(methods ...uintptr) []uintptr {
	return append(unknownMethods(), methods...)
}

// hresultValue converts an HRESULT to the uintptr that a method returns.
func hresultValue(result hresult) uintptr {
	return uintptr(uint32(result))
}

// ---- Floating-point arguments ----

// arm64ABI reports the calling convention of Windows on arm64, which passes
// floating-point arguments in their own registers, which syscall cannot fill,
// and small structs in registers.
const arm64ABI = runtime.GOARCH == "arm64"

// floatCall collects the arguments of a call that passes floating-point
// values or small structs of them, which the ABIs of Windows place
// differently. On amd64 and 386 a float takes the next argument slot as an
// integer would (amd64 copies the first four slots to the XMM registers too),
// while on arm64 floats go in their own registers. syscall cannot place them
// on arm64, so there the call goes through a thunk; see floatThunk.
type floatCall struct {
	ints       [16]uintptr
	intCount   int
	floats     [8]uint64
	floatCount int
	// pointers keeps the Go memory that pointer arguments point at alive and
	// on the heap until the call returns.
	pointers [8]unsafe.Pointer
}

// arg adds an integer argument or a handle.
func (c *floatCall) arg(value uintptr) {
	c.ints[c.intCount] = value
	c.intCount++
}

// pointerArg adds a pointer argument.
func (c *floatCall) pointerArg(p unsafe.Pointer) {
	for i := range c.pointers {
		if c.pointers[i] == nil {
			c.pointers[i] = p
			break
		}
	}
	c.arg(uintptr(p))
}

// float adds a FLOAT argument.
func (c *floatCall) float(value float32) {
	if arm64ABI {
		c.floats[c.floatCount] = uint64(math.Float32bits(value))
		c.floatCount++
		return
	}
	c.arg(uintptr(math.Float32bits(value)))
}

// double adds a DOUBLE argument.
func (c *floatCall) double(value float64) {
	bits := math.Float64bits(value)
	switch {
	case arm64ABI:
		c.floats[c.floatCount] = bits
		c.floatCount++
	case unsafe.Sizeof(uintptr(0)) == 8:
		c.arg(uintptr(bits))
	default:
		c.arg(uintptr(uint32(bits)))
		c.arg(uintptr(uint32(bits >> 32)))
	}
}

// pair adds an 8-byte struct of two 32-bit integers passed by value; see
// pairArgs.
func (c *floatCall) pair(low, high uint32) {
	for _, slot := range pairArgs(low, high) {
		c.arg(slot)
	}
}

// point adds a D2D1_POINT_2F argument, which Windows passes by value.
func (c *floatCall) point(x, y float32) {
	switch {
	case arm64ABI:
		// The struct is a homogeneous floating-point aggregate.
		c.float(x)
		c.float(y)
	case unsafe.Sizeof(uintptr(0)) == 8:
		// The 8-byte struct goes in one slot.
		c.arg(uintptr(uint64(math.Float32bits(y))<<32 | uint64(math.Float32bits(x))))
	default:
		c.arg(uintptr(math.Float32bits(x)))
		c.arg(uintptr(math.Float32bits(y)))
	}
}

// call calls the function at address with the collected arguments.
func (c *floatCall) call(address uintptr) uintptr {
	if arm64ABI {
		if c.intCount > 8 {
			panic("ui/win32: a call with floating-point arguments takes more than 8 integer arguments")
		}
		frame := floatFrame{function: address}
		copy(frame.ints[:], c.ints[:c.intCount])
		copy(frame.floats[:], c.floats[:c.floatCount])
		result := callFloatFrame(&frame)
		runtime.KeepAlive(c)
		return result
	}
	result, _, _ := syscall.SyscallN(address, c.ints[:c.intCount]...)
	runtime.KeepAlive(c)
	return result
}

// floatFrame is what floatThunk reads: the function, its integer and
// floating-point register arguments, and its integer result. Its layout is
// part of the thunk.
type floatFrame struct {
	function    uintptr
	ints        [8]uintptr
	floats      [8]uint64
	result      uintptr
	floatResult uint64
}

//go:uintptrescapes
func callFloatFrame(frame *floatFrame) uintptr {
	syscall.SyscallN(floatThunkAddress(), uintptr(unsafe.Pointer(frame)))
	return frame.result
}

// comFloatCall starts a call of a COM method with floating-point arguments.
func comFloatCall(object comObject) *floatCall {
	call := &floatCall{}
	call.arg(uintptr(object))
	return call
}

// ---- Callbacks ----

// callbacks holds the functions that Windows calls. syscall can create only
// a limited number of them, so each is created once.
var (
	callbacksMu sync.Mutex
	callbacks   = map[string]uintptr{}
)

// callback returns the address that Windows calls for a Go function, creating
// it on the first call with a name. The function takes and returns
// uintptr-sized values.
func callback(name string, function any) uintptr {
	callbacksMu.Lock()
	defer callbacksMu.Unlock()
	if address, ok := callbacks[name]; ok {
		return address
	}
	address := syscall.NewCallback(function)
	callbacks[name] = address
	return address
}

// ---- Strings ----

// wide returns s as a NUL-terminated UTF-16 string. A NUL in s ends the string.
func wide(s string) *uint16 {
	return &wideSlice(s)[0]
}

// wideSlice returns s as NUL-terminated UTF-16.
func wideSlice(s string) []uint16 {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			s = s[:i]
			break
		}
	}
	result, _ := syscall.UTF16FromString(s)
	return result
}

// wideOrNil returns s as wide does, or nil for an empty string.
func wideOrNil(s string) *uint16 {
	if s == "" {
		return nil
	}
	return wide(s)
}

// goString converts UTF-16 to a string, up to the first NUL.
func goString(text []uint16) string {
	return syscall.UTF16ToString(text)
}

// goStringAt converts a NUL-terminated UTF-16 string in Windows memory.
func goStringAt(address uintptr) string {
	if address == 0 {
		return ""
	}
	n := 0
	for *(*uint16)(memoryAt(address + uintptr(n)*2)) != 0 {
		n++
	}
	return goStringAtLength(address, n)
}

// goStringAtLength converts n UTF-16 units in Windows memory.
func goStringAtLength(address uintptr, n int) string {
	if address == 0 || n <= 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(memoryAt(address)), n))
}

// goBytesAt copies n bytes of Windows memory.
func goBytesAt(address uintptr, n int) []byte {
	if address == 0 || n <= 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(memoryAt(address)), n)...)
}

// ---- Types ----

type (
	hwnd       uintptr
	hdc        uintptr
	hmenu      uintptr
	hfont      uintptr
	hbrush     uintptr
	hbitmap    uintptr
	hicon      uintptr
	hcursor    uintptr
	hinstance  uintptr
	himagelist uintptr
	hglobal    uintptr
	hgdiobj    uintptr
	htheme     uintptr
	hrgn       uintptr
)

type rect struct {
	left, top, right, bottom int32
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    hwnd
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type paintStruct struct {
	hdc         hdc
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     hinstance
	hIcon         hicon
	hCursor       hcursor
	hbrBackground hbrush
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       hicon
}

// trackMouseEventInfo is TRACKMOUSEEVENT. A struct whose name would be the
// name of a function or a constant takes the suffix Info.
type trackMouseEventInfo struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   hwnd
	dwHoverTime uint32
}

type logFont struct {
	lfHeight         int32
	lfWidth          int32
	lfEscapement     int32
	lfOrientation    int32
	lfWeight         int32
	lfItalic         byte
	lfUnderline      byte
	lfStrikeOut      byte
	lfCharSet        byte
	lfOutPrecision   byte
	lfClipPrecision  byte
	lfQuality        byte
	lfPitchAndFamily byte
	lfFaceName       [32]uint16
}

type nonClientMetrics struct {
	cbSize             uint32
	iBorderWidth       int32
	iScrollWidth       int32
	iScrollHeight      int32
	iCaptionWidth      int32
	iCaptionHeight     int32
	lfCaptionFont      logFont
	iSmCaptionWidth    int32
	iSmCaptionHeight   int32
	lfSmCaptionFont    logFont
	iMenuWidth         int32
	iMenuHeight        int32
	lfMenuFont         logFont
	lfStatusFont       logFont
	lfMessageFont      logFont
	iPaddedBorderWidth int32
}

type minMaxInfo struct {
	ptReserved     point
	ptMaxSize      point
	ptMaxPosition  point
	ptMinTrackSize point
	ptMaxTrackSize point
}

type windowPlacement struct {
	length           uint32
	flags            uint32
	showCmd          uint32
	ptMinPosition    point
	ptMaxPosition    point
	rcNormalPosition rect
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

type monitorInfoEx struct {
	monitorInfo
	szDevice [32]uint16
}

type devMode struct {
	dmDeviceName       [32]uint16
	dmSpecVersion      uint16
	dmDriverVersion    uint16
	dmSize             uint16
	dmDriverExtra      uint16
	dmFields           uint32
	dmPosition         [16]byte
	dmColor            int16
	dmDuplex           int16
	dmYResolution      int16
	dmTTOption         int16
	dmCollate          int16
	dmFormName         [32]uint16
	dmLogPixels        uint16
	dmBitsPerPel       uint32
	dmPelsWidth        uint32
	dmPelsHeight       uint32
	dmDisplayFlags     uint32
	dmDisplayFrequency uint32
	dmICMMethod        uint32
	dmICMIntent        uint32
	dmMediaType        uint32
	dmDitherType       uint32
	dmReserved1        uint32
	dmReserved2        uint32
	dmPanningWidth     uint32
	dmPanningHeight    uint32
}

type displayDevice struct {
	cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

type highContrast struct {
	cbSize            uint32
	dwFlags           uint32
	lpszDefaultScheme *uint16
}

type menuBarInfo struct {
	cbSize   uint32
	rcBar    rect
	hMenu    hmenu
	hwndMenu hwnd
	// flags holds the bit fields fBarFocused and fFocused.
	flags int32
}

type menuItemInfo struct {
	cbSize        uint32
	fMask         uint32
	fType         uint32
	fState        uint32
	wID           uint32
	hSubMenu      hmenu
	hbmpChecked   hbitmap
	hbmpUnchecked hbitmap
	dwItemData    uintptr
	dwTypeData    *uint16
	cch           uint32
	hbmpItem      hbitmap
}

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	itemID     uint32
	itemAction uint32
	itemState  uint32
	hwndItem   hwnd
	hDC        hdc
	rcItem     rect
	itemData   uintptr
}

type nmhdr struct {
	hwndFrom hwnd
	idFrom   uintptr
	code     uint32
}

type nmCustomDrawInfo struct {
	hdr         nmhdr
	dwDrawStage uint32
	hdc         hdc
	rc          rect
	dwItemSpec  uintptr
	uItemState  uint32
	lItemlParam uintptr
}

type scrollInfo struct {
	cbSize    uint32
	fMask     uint32
	nMin      int32
	nMax      int32
	nPage     uint32
	nPos      int32
	nTrackPos int32
}

type comboBoxInfo struct {
	cbSize      uint32
	rcItem      rect
	rcButton    rect
	stateButton uint32
	hwndCombo   hwnd
	hwndItem    hwnd
	hwndList    hwnd
}

// toolInfo is TTTOOLINFOW.
type toolInfo struct {
	cbSize     uint32
	uFlags     uint32
	hwnd       hwnd
	uId        uintptr
	rect       rect
	hinst      hinstance
	lpszText   *uint16
	lParam     uintptr
	lpReserved uintptr
}

type buttonImageList struct {
	himl   himagelist
	margin rect
	uAlign uint32
}

type lItem struct {
	mask      uint32
	iLink     int32
	state     uint32
	stateMask uint32
	szID      [48]uint16
	szUrl     [2083]uint16
}

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         hwnd
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     hinstance
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     uintptr
}

type actCtx struct {
	cbSize                 uint32
	dwFlags                uint32
	lpSource               *uint16
	wProcessorArchitecture uint16
	wLangId                uint16
	lpAssemblyDirectory    *uint16
	// lpResourceName is an integer resource identifier.
	lpResourceName    uintptr
	lpApplicationName *uint16
	hModule           uintptr
}

// initCommonControlsExInfo is INITCOMMONCONTROLSEX.
type initCommonControlsExInfo struct {
	dwSize uint32
	dwICC  uint32
}

type formatEtc struct {
	cfFormat uint16
	ptd      uintptr
	dwAspect uint32
	lindex   int32
	tymed    uint32
}

type stgMedium struct {
	tymed uint32
	// handle is the union of the medium: an HGLOBAL for TYMED_HGLOBAL.
	handle         uintptr
	pUnkForRelease uintptr
}

type statStg struct {
	pwcsName          uintptr
	typ               uint32
	cbSize            uint64
	mtime             uint64
	ctime             uint64
	atime             uint64
	grfMode           uint32
	grfLocksSupported uint32
	clsid             guid
	grfStateBits      uint32
	reserved          uint32
}

type dropFiles struct {
	pFiles uint32
	pt     point
	fNC    int32
	fWide  int32
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type comdlgFilterSpec struct {
	pszName *uint16
	pszSpec *uint16
}

type d2d1ColorF struct {
	r, g, b, a float32
}

type d2d1RectF struct {
	left, top, right, bottom float32
}

type d2d1Point2F struct {
	x, y float32
}

type d2d1SizeF struct {
	width, height float32
}

type d2d1RoundedRect struct {
	rect    d2d1RectF
	radiusX float32
	radiusY float32
}

type d2d1Ellipse struct {
	point   d2d1Point2F
	radiusX float32
	radiusY float32
}

type d2d1ArcSegment struct {
	point          d2d1Point2F
	size           d2d1SizeF
	rotationAngle  float32
	sweepDirection uint32
	arcSize        uint32
}

type d2d1StrokeStyleProperties struct {
	startCap   uint32
	endCap     uint32
	dashCap    uint32
	lineJoin   uint32
	miterLimit float32
	dashStyle  uint32
	dashOffset float32
}

type d2d1PixelFormat struct {
	format    uint32
	alphaMode uint32
}

type d2d1BitmapProperties struct {
	pixelFormat d2d1PixelFormat
	dpiX        float32
	dpiY        float32
}

type d2d1RenderTargetProperties struct {
	typ         uint32
	pixelFormat d2d1PixelFormat
	dpiX        float32
	dpiY        float32
	usage       uint32
	minLevel    uint32
}

type dwriteTrimming struct {
	granularity    uint32
	delimiter      uint32
	delimiterCount uint32
}

type dwriteOverhangMetrics struct {
	left, top, right, bottom float32
}

type dwriteTextMetrics struct {
	left                             float32
	top                              float32
	width                            float32
	widthIncludingTrailingWhitespace float32
	height                           float32
	layoutWidth                      float32
	layoutHeight                     float32
	maxBidiReorderingDepth           uint32
	lineCount                        uint32
}

// ---- Constants ----

const (
	maxPath             = 260
	localeNameMaxLength = 85
	errorSuccess        = 0

	sOK                        = 0
	eNotImpl                   = 0x80004001
	eNoInterface               = 0x80004002
	eUnexpected                = 0x8000ffff
	rpcEChangedMode            = 0x80010106
	d2dErrRecreateTarget       = 0x8899000c
	dvEFormatEtc               = 0x80040064
	oleEAdviseNotSupported     = 0x80040003
	dragdropSDrop              = 0x00040100
	dragdropSCancel            = 0x00040101
	dragdropSUseDefaultCursors = 0x00040102

	wmNull                  = 0x0000
	wmDestroy               = 0x0002
	wmSize                  = 0x0005
	wmActivate              = 0x0006
	wmSetFocus              = 0x0007
	wmKillFocus             = 0x0008
	wmPaint                 = 0x000f
	wmClose                 = 0x0010
	wmQueryEndSession       = 0x0011
	wmQuit                  = 0x0012
	wmEraseBkgnd            = 0x0014
	wmSysColorChange        = 0x0015
	wmShowWindow            = 0x0018
	wmSettingChange         = 0x001a
	wmCancelMode            = 0x001f
	wmSetCursor             = 0x0020
	wmGetMinMaxInfo         = 0x0024
	wmDrawItem              = 0x002b
	wmSetFont               = 0x0030
	wmWindowPosChanged      = 0x0047
	wmNotify                = 0x004e
	wmContextMenu           = 0x007b
	wmNCCreate              = 0x0081
	wmNCDestroy             = 0x0082
	wmNCHitTest             = 0x0084
	wmNCPaint               = 0x0085
	wmNCActivate            = 0x0086
	wmGetDlgCode            = 0x0087
	wmKeyFirst              = 0x0100
	wmKeyDown               = 0x0100
	wmKeyUp                 = 0x0101
	wmChar                  = 0x0102
	wmSysKeyDown            = 0x0104
	wmSysKeyUp              = 0x0105
	wmSysChar               = 0x0106
	wmKeyLast               = 0x0109
	wmCommand               = 0x0111
	wmTimer                 = 0x0113
	wmHScroll               = 0x0114
	wmVScroll               = 0x0115
	wmQueryUIState          = 0x0129
	wmCtlColorEdit          = 0x0133
	wmCtlColorListBox       = 0x0134
	wmCtlColorBtn           = 0x0135
	wmCtlColorStatic        = 0x0138
	wmMouseFirst            = 0x0200
	wmMouseMove             = 0x0200
	wmLButtonDown           = 0x0201
	wmLButtonUp             = 0x0202
	wmLButtonDblClk         = 0x0203
	wmRButtonDown           = 0x0204
	wmRButtonUp             = 0x0205
	wmRButtonDblClk         = 0x0206
	wmMButtonDown           = 0x0207
	wmMButtonUp             = 0x0208
	wmMButtonDblClk         = 0x0209
	wmMouseWheel            = 0x020a
	wmXButtonDown           = 0x020b
	wmXButtonUp             = 0x020c
	wmXButtonDblClk         = 0x020d
	wmMouseHWheel           = 0x020e
	wmMouseLast             = 0x020e
	wmCaptureChanged        = 0x0215
	wmDropFiles             = 0x0233
	wmMouseLeave            = 0x02a3
	wmDpiChanged            = 0x02e0
	wmDpiChangedAfterParent = 0x02e3
	wmPrintClient           = 0x0318
	wmThemeChanged          = 0x031a
	wmUser                  = 0x0400
	wmApp                   = 0x8000

	wsOverlapped      = 0x00000000
	wsPopup           = 0x80000000
	wsChild           = 0x40000000
	wsVisible         = 0x10000000
	wsClipSiblings    = 0x04000000
	wsClipChildren    = 0x02000000
	wsCaption         = 0x00c00000
	wsBorder          = 0x00800000
	wsVScroll         = 0x00200000
	wsHScroll         = 0x00100000
	wsSysMenu         = 0x00080000
	wsThickFrame      = 0x00040000
	wsGroup           = 0x00020000
	wsTabStop         = 0x00010000
	wsMinimizeBox     = 0x00020000
	wsMaximizeBox     = 0x00010000
	wsExTopmost       = 0x00000008
	wsExAcceptFiles   = 0x00000010
	wsExTransparent   = 0x00000020
	wsExToolWindow    = 0x00000080
	wsExClientEdge    = 0x00000200
	wsExControlParent = 0x00010000
	wsExLayered       = 0x00080000

	csVRedraw    = 0x0001
	csHRedraw    = 0x0002
	csDblClks    = 0x0008
	csDropShadow = 0x00020000

	swHide           = 0
	swShowNormal     = 1
	swShowMaximized  = 3
	swMaximize       = 3
	swShowNoActivate = 4
	swShow           = 5
	swMinimize       = 6
	swShowNA         = 8
	swRestore        = 9

	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoZOrder      = 0x0004
	swpNoRedraw      = 0x0008
	swpNoActivate    = 0x0010
	swpFrameChanged  = 0x0020
	swpShowWindow    = 0x0040
	swpHideWindow    = 0x0080
	swpNoOwnerZOrder = 0x0200

	gwlStyle   = -16
	gwlExStyle = -20

	gwHwndNext = 2
	gwChild    = 5
	gaRoot     = 2

	rdwInvalidate  = 0x0001
	rdwErase       = 0x0004
	rdwAllChildren = 0x0080
	rdwFrame       = 0x0400

	pmRemove           = 0x0001
	qsAllInput         = 0x1cff
	mwmoInputAvailable = 0x0004

	sizeMinimized = 1
	waInactive    = 0
	htClient      = 1
	xbutton1      = 1
	wheelDelta    = 120

	dlgcWantArrows  = 0x0001
	dlgcWantTab     = 0x0002
	dlgcWantAllKeys = 0x0004
	dlgcWantChars   = 0x0080

	tmeLeave = 0x00000002

	mkLButton = 0x0001
	mkRButton = 0x0002
	mkShift   = 0x0004
	mkControl = 0x0008

	smCXDrag = 68
	smCYDrag = 69

	cwUseDefault = -0x80000000

	idcArrow       = 32512
	idcIBeam       = 32513
	idcWait        = 32514
	idcCross       = 32515
	idcSizeWE      = 32644
	idcSizeNS      = 32645
	idcSizeAll     = 32646
	idcNo          = 32648
	idcHand        = 32649
	idiApplication = 32512

	dtCenter      = 0x00000001
	dtRight       = 0x00000002
	dtVCenter     = 0x00000004
	dtWordBreak   = 0x00000010
	dtSingleLine  = 0x00000020
	dtCalcRect    = 0x00000400
	dtNoPrefix    = 0x00000800
	dtEditControl = 0x00002000
	dtEndEllipsis = 0x00008000
	dtHidePrefix  = 0x00100000

	transparent = 1

	vkBack      = 0x08
	vkTab       = 0x09
	vkReturn    = 0x0d
	vkShift     = 0x10
	vkControl   = 0x11
	vkMenu      = 0x12
	vkPause     = 0x13
	vkCapital   = 0x14
	vkEscape    = 0x1b
	vkSpace     = 0x20
	vkPrior     = 0x21
	vkNext      = 0x22
	vkEnd       = 0x23
	vkHome      = 0x24
	vkLeft      = 0x25
	vkUp        = 0x26
	vkRight     = 0x27
	vkDown      = 0x28
	vkSnapshot  = 0x2c
	vkInsert    = 0x2d
	vkDelete    = 0x2e
	vkLWin      = 0x5b
	vkRWin      = 0x5c
	vkApps      = 0x5d
	vkNumpad0   = 0x60
	vkNumpad9   = 0x69
	vkMultiply  = 0x6a
	vkAdd       = 0x6b
	vkSubtract  = 0x6d
	vkDecimal   = 0x6e
	vkDivide    = 0x6f
	vkF1        = 0x70
	vkF24       = 0x87
	vkNumLock   = 0x90
	vkScroll    = 0x91
	vkLShift    = 0xa0
	vkRShift    = 0xa1
	vkLControl  = 0xa2
	vkRControl  = 0xa3
	vkLMenu     = 0xa4
	vkRMenu     = 0xa5
	vkOEM1      = 0xba
	vkOEMPlus   = 0xbb
	vkOEMComma  = 0xbc
	vkOEMMinus  = 0xbd
	vkOEMPeriod = 0xbe
	vkOEM2      = 0xbf
	vkOEM3      = 0xc0
	vkOEM4      = 0xdb
	vkOEM5      = 0xdc
	vkOEM6      = 0xdd
	vkOEM7      = 0xde
	vkOEM102    = 0xe2

	mapvkVscToVkEx = 3

	spiGetNonClientMetrics    = 0x0029
	spiGetHighContrast        = 0x0042
	spiGetWheelScrollLines    = 0x0068
	spiGetClientAreaAnimation = 0x1042
	hcfHighContrastOn         = 0x00000001

	monitorDefaultToPrimary = 1
	monitorDefaultToNearest = 2
	enumCurrentSettings     = 0xffffffff
	wpfRestoreToMaximized   = 0x0002
	lwaAlpha                = 0x00000002

	objidMenu   = -3
	objidClient = -4
	childidSelf = 0

	miimString = 0x00000040

	odsSelected = 0x0001
	odsGrayed   = 0x0002
	odsDisabled = 0x0004
	odsHotLight = 0x0040
	odsInactive = 0x0080
	odsNoAccel  = 0x0100

	sbHorz             = 0
	sbVert             = 1
	sbBoth             = 3
	sbLineUp           = 0
	sbLineDown         = 1
	sbPageUp           = 2
	sbPageDown         = 3
	sbThumbPosition    = 4
	sbThumbTrack       = 5
	sbTop              = 6
	sbBottom           = 7
	sifRange           = 0x0001
	sifPage            = 0x0002
	sifPos             = 0x0004
	sifDisableNoScroll = 0x0008
	sifTrackPos        = 0x0010
	sifAll             = sifRange | sifPage | sifPos | sifTrackPos

	uisfHideFocus = 0x1

	bsPushButton     = 0x00000000
	bsDefPushButton  = 0x00000001
	bsAutoCheckBox   = 0x00000003
	bsRadioButton    = 0x00000004
	bsAuto3State     = 0x00000006
	bsTypeMask       = 0x0000000f
	bsLeftText       = 0x00000020
	bsMultiline      = 0x00002000
	bnClicked        = 0
	bmGetCheck       = 0x00f0
	bmSetCheck       = 0x00f1
	bmClick          = 0x00f5
	bstUnchecked     = 0
	bstChecked       = 1
	bstIndeterminate = 2

	ssLeft         = 0x00000000
	ssCenter       = 0x00000001
	ssRight        = 0x00000002
	ssTypeMask     = 0x0000001f
	ssNoPrefix     = 0x00000080
	ssNotify       = 0x00000100
	ssCenterImage  = 0x00000200
	ssEditControl  = 0x00002000
	ssEndEllipsis  = 0x00004000
	ssEllipsisMask = 0x0000c000

	esLeft                = 0x0000
	esCenter              = 0x0001
	esRight               = 0x0002
	esMultiline           = 0x0004
	esAutoVScroll         = 0x0040
	esAutoHScroll         = 0x0080
	esReadOnly            = 0x0800
	esWantReturn          = 0x1000
	enChange              = 0x0300
	emGetSel              = 0x00b0
	emSetSel              = 0x00b1
	emGetRect             = 0x00b2
	emLineScroll          = 0x00b6
	emScrollCaret         = 0x00b7
	emSetLimitText        = 0x00c5
	emGetFirstVisibleLine = 0x00ce
	emSetReadOnly         = 0x00cf
	emSetMargins          = 0x00d3
	ecLeftMargin          = 0x0001
	ecRightMargin         = 0x0002
	emSetCueBanner        = 0x1501

	cbsDropDown       = 0x0002
	cbsDropDownList   = 0x0003
	cbnSelChange      = 1
	cbnEditChange     = 5
	cbnKillFocus      = 4
	cbGetCurSel       = 0x0147
	cbAddString       = 0x0143
	cbGetCount        = 0x0146
	cbGetLBText       = 0x0148
	cbGetLBTextLen    = 0x0149
	cbResetContent    = 0x014b
	cbSetCurSel       = 0x014e
	cbGetItemHeight   = 0x0154
	cbSetDroppedWidth = 0x0160
	cbSetMinVisible   = 0x1701
	cbSetCueBanner    = 0x1703

	tbsHorz        = 0x0000
	tbsAutoTicks   = 0x0001
	tbsVert        = 0x0002
	tbsBoth        = 0x0008
	tbsNoTicks     = 0x0010
	tbEndTrack     = 8
	tbmGetPos      = wmUser
	tbmSetPos      = wmUser + 5
	tbmSetRangeMin = wmUser + 7
	tbmSetRangeMax = wmUser + 8
	tbmClearTics   = wmUser + 9
	tbmSetTicFreq  = wmUser + 20
	tbmSetPageSize = wmUser + 21
	tbmSetLineSize = wmUser + 23

	pbsVertical   = 0x04
	pbsMarquee    = 0x08
	pbmSetPos     = wmUser + 2
	pbmSetRange32 = wmUser + 6
	pbmSetMarquee = wmUser + 10
	pbmSetState   = wmUser + 16
	pbstNormal    = 0x0001
	pbstError     = 0x0002
	pbstPaused    = 0x0003

	lwsTransparent = 0x0001
	lifItemIndex   = 0x00000001
	lifState       = 0x00000002
	lisVisited     = 0x00000004
	lmSetItem      = wmUser + 0x302

	ttsAlwaysTip      = 0x01
	ttsNoPrefix       = 0x02
	ttfIdIsHwnd       = 0x0001
	ttfSubclass       = 0x0010
	ttmAddTool        = wmUser + 50
	ttmDelTool        = wmUser + 51
	ttmSetMaxTipWidth = wmUser + 24

	lvmGetItemRect = 0x1000 + 14
	lvmGetOrigin   = 0x1000 + 41
	lvmScroll      = 0x1000 + 20
	lvirBounds     = 0

	bcmSetImageList            = 0x1602
	buttonImageListAlignLeft   = 0
	buttonImageListAlignRight  = 1
	buttonImageListAlignTop    = 2
	buttonImageListAlignBottom = 3
	buttonImageListAlignCenter = 4

	nmClick         = 0xfffffffe
	nmReturn        = 0xfffffffc
	nmCustomDraw    = 0xfffffff4
	cddsPrePaint    = 0x00000001
	cdrfDoDefault   = 0x00000000
	cdrfSkipDefault = 0x00000004
	cdisSelected    = 0x0001
	cdisFocus       = 0x0010
	cdisHot         = 0x0040

	ilcColor32 = 0x00000020

	iccWin95Classes      = 0x000000ff
	iccDateClasses       = 0x00000100
	iccUserexClasses     = 0x00000200
	iccCoolClasses       = 0x00000400
	iccLinkClass         = 0x00008000
	iccPagescrollerClass = 0x00001000
	iccStandardClasses   = 0x00004000
	iccTabClasses        = 0x00000008
	iccBarClasses        = 0x00000004
	iccProgressClass     = 0x00000020
	iccUpdownClass       = 0x00000010
	iccHotkeyClass       = 0x00000040
	iccListviewClasses   = 0x00000001
	iccTreeviewClasses   = 0x00000002

	wcButton      = "Button"
	wcStatic      = "Static"
	wcEdit        = "Edit"
	wcComboBox    = "ComboBox"
	wcListView    = "SysListView32"
	wcTreeView    = "SysTreeView32"
	wcLink        = "SysLink"
	trackbarClass = "msctls_trackbar32"
	progressClass = "msctls_progress32"
	tooltipsClass = "tooltips_class32"

	actCtxFlagResourceNameValid      = 0x00000008
	actCtxFlagAssemblyDirectoryValid = 0x00000004

	gmemMoveable = 0x0002

	localeIReadingLayout = 0x00000070
	localeReturnNumber   = 0x20000000

	defaultCharset    = 1
	outDefaultPrecis  = 0
	clipDefaultPrecis = 0
	cleartypeQuality  = 5
	defaultPitch      = 0
	biRGB             = 0
	biBitfields       = 3

	rrfRtRegDword  = 0x00000010
	rrfRtRegBinary = 0x00000008

	dwmwaUseImmersiveDarkMode = 20

	bpbfCompatibleBitmap = 0
	epEditBorderNoScroll = 6
	epsnNormal           = 1
	epsnHot              = 2
	epsnFocused          = 3
	epsnDisabled         = 4

	clsctxInprocServer = 0x1
	cfUnicodeText      = 13
	cfHDrop            = 15
	cfDIB              = 8
	dvaspectContent    = 1
	tymedHGlobal       = 1
	datadirGet         = 1
	dropEffectNone     = 0
	dropEffectCopy     = 1
	dropEffectMove     = 2
	dropEffectLink     = 4
	statflagNoName     = 1

	seeMaskNoAsync   = 0x00000100
	seeMaskFlagNoUI  = 0x00000400
	sigdnFileSysPath = 0x80058000

	fosOverwritePrompt  = 0x00000002
	fosPickFolders      = 0x00000020
	fosForceFileSystem  = 0x00000040
	fosAllowMultiSelect = 0x00000200
	fosPathMustExist    = 0x00000800
	fosForceShowHidden  = 0x10000000

	d2d1FactoryTypeSingleThreaded              = 0
	d2d1RenderTargetTypeDefault                = 0
	d2d1RenderTargetUsageGDICompatible         = 0x00000002
	d2d1AlphaModePremultiplied                 = 1
	dxgiFormatB8G8R8A8Unorm                    = 87
	d2d1TextAntialiasModeCleartype             = 1
	d2d1TextAntialiasModeGrayscale             = 2
	d2d1AntialiasModeAliased                   = 1
	d2d1BitmapInterpolationModeNearestNeighbor = 0
	d2d1BitmapInterpolationModeLinear          = 1
	d2d1DrawTextOptionsNone                    = 0
	d2d1DrawTextOptionsClip                    = 0x00000002
	d2d1FigureBeginHollow                      = 1
	d2d1FigureEndOpen                          = 0
	d2d1SweepDirectionClockwise                = 1
	d2d1ArcSizeSmall                           = 0
	d2d1ArcSizeLarge                           = 1
	d2d1CapStyleRound                          = 2

	dwriteFactoryTypeShared            = 0
	dwriteFontStyleNormal              = 0
	dwriteFontStyleItalic              = 2
	dwriteFontStretchNormal            = 5
	dwriteTextAlignmentLeading         = 0
	dwriteTextAlignmentTrailing        = 1
	dwriteTextAlignmentCenter          = 2
	dwriteParagraphAlignmentNear       = 0
	dwriteParagraphAlignmentFar        = 1
	dwriteParagraphAlignmentCenter     = 2
	dwriteWordWrappingWrap             = 0
	dwriteWordWrappingNoWrap           = 1
	dwriteTrimmingGranularityCharacter = 1

	wicDecodeMetadataCacheOnDemand = 0
	wicBitmapEncoderNoCache        = 2
	wicBitmapDitherTypeNone        = 0
	wicBitmapPaletteTypeCustom     = 0
)

var (
	invalidHandleValue                   = ^uintptr(0)
	htTransparent                        = ^uintptr(0)
	hwndTop                              = hwnd(0)
	hwndTopmost                          = hwnd(^uintptr(0))
	hwndNoTopmost                        = hwnd(^uintptr(1))
	hkeyCurrentUser                      = signExtend(0x80000001)
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
)

var (
	iidUnknown                  = newGUID(0x00000000, 0x0000, 0x0000, 0xc000, 0x000000000046)
	iidDataObject               = newGUID(0x0000010e, 0x0000, 0x0000, 0xc000, 0x000000000046)
	iidDropSource               = newGUID(0x00000121, 0x0000, 0x0000, 0xc000, 0x000000000046)
	iidDropTarget               = newGUID(0x00000122, 0x0000, 0x0000, 0xc000, 0x000000000046)
	iidD2D1Factory              = newGUID(0x06152247, 0x6f50, 0x465a, 0x9245, 0x118bfd3b6007)
	iidDWriteFactory            = newGUID(0xb859ee5a, 0xd838, 0x4b5b, 0xa2e8, 0x1adc7d93db48)
	clsidWICImagingFactory      = newGUID(0xcacaf262, 0x9370, 0x4615, 0xa13b, 0x9f5539da4c0a)
	iidWICImagingFactory        = newGUID(0xec5ec8a9, 0xc395, 0x4314, 0x9c77, 0x54d7a935ff70)
	guidContainerFormatPng      = newGUID(0x1b7cfaf4, 0x713f, 0x473c, 0xbbcd, 0x6137425faeaf)
	guidWICPixelFormat32bppRGBA = newGUID(0xf5c7ad2d, 0x6a8d, 0x43dd, 0xa7a8, 0xa29935261ae9)
	guidWICPixelFormat32bppBGRA = newGUID(0x6fddc324, 0x4e03, 0x4bfe, 0xb185, 0x3d77768dc90f)
	clsidFileOpenDialog         = newGUID(0xdc1c5a9c, 0xe88a, 0x4dde, 0xa5a1, 0x60f82a20aef7)
	clsidFileSaveDialog         = newGUID(0xc0b4e2f3, 0xba21, 0x4773, 0x8dba, 0x335ec946eb8b)
	iidFileOpenDialog           = newGUID(0xd57c7288, 0xd4ad, 0x4768, 0xbe02, 0x9d969532d960)
	iidFileSaveDialog           = newGUID(0x84bccd23, 0x5fde, 0x4cdb, 0xaea4, 0xaf64b83d78ab)
	iidShellItem                = newGUID(0x43826d1e, 0xe718, 0x42ee, 0xbc55, 0xa1e261c37bfe)
	clsidAccPropServices        = newGUID(0xb5f8350b, 0x0548, 0x48b1, 0xa6ee, 0x88bd00b4a5e7)
	iidAccPropServices          = newGUID(0x6e26e776, 0x04f0, 0x495d, 0x80e4, 0x3330352e3169)
	propidAccName               = newGUID(0x608d3df8, 0x8128, 0x4aa7, 0xa428, 0xf55e49267291)
	propidAccDescription        = newGUID(0x4d48dfe4, 0xbd3f, 0x491f, 0xa648, 0x492d6f20c588)
	propidAccValue              = newGUID(0x123fe443, 0x211a, 0x4615, 0x9527, 0xc45a7e93717a)
)

// ---- Libraries ----

var (
	user32   = newLibrary("user32.dll")
	kernel32 = newLibrary("kernel32.dll")
	gdi32    = newLibrary("gdi32.dll")
	uxtheme  = newLibrary("uxtheme.dll")
	dwmapi   = newLibrary("dwmapi.dll")
	shcore   = newLibrary("shcore.dll")
	advapi32 = newLibrary("advapi32.dll")
	ole32    = newLibrary("ole32.dll")
	shell32  = newLibrary("shell32.dll")
	d2d1     = newLibrary("d2d1.dll")
	dwrite   = newLibrary("dwrite.dll")
	comctl32 = &library{name: "comctl32.dll", load: loadComctl32}
)

// loadComctl32 loads the common controls in the activation context of version
// 6, because a program that links them loads version 5, which draws them in
// the classic style.
func loadComctl32() uintptr {
	defer win32DeactivateActCtx(0, activateCommonControls())
	return loadLibraryDefault("comctl32.dll")
}

// activateCommonControls activates the activation context of the common
// controls version 6, made from the manifest that shell32.dll carries as
// resource 124, so the backend gets the controls with visual styles without a
// manifest in the program. It returns the cookie that DeactivateActCtx
// takes.
func activateCommonControls() uintptr {
	commonControlsOnce.Do(func() {
		var system [maxPath]uint16
		win32GetSystemDirectory(&system[0], maxPath)
		context := actCtx{
			dwFlags:             actCtxFlagResourceNameValid | actCtxFlagAssemblyDirectoryValid,
			lpSource:            wide("shell32.dll"),
			lpResourceName:      124,
			lpAssemblyDirectory: &system[0],
		}
		context.cbSize = uint32(unsafe.Sizeof(context))
		commonControlsContext = win32CreateActCtx(&context)
	})
	var cookie uintptr
	if commonControlsContext != invalidHandleValue {
		win32ActivateActCtx(commonControlsContext, &cookie)
	}
	return cookie
}

var (
	commonControlsOnce    sync.Once
	commonControlsContext uintptr
)

// ---- user32 ----

var (
	procAdjustWindowRectExForDpi      = user32.proc("AdjustWindowRectExForDpi")
	procBeginPaint                    = user32.proc("BeginPaint")
	procClientToScreen                = user32.proc("ClientToScreen")
	procClipCursor                    = user32.proc("ClipCursor")
	procCreateWindowEx                = user32.proc("CreateWindowExW")
	procDefWindowProc                 = user32.proc("DefWindowProcW")
	procDestroyWindow                 = user32.proc("DestroyWindow")
	procDispatchMessage               = user32.proc("DispatchMessageW")
	procDrawText                      = user32.proc("DrawTextW")
	procEnableNonClientDpiScaling     = user32.proc("EnableNonClientDpiScaling")
	procEnableWindow                  = user32.proc("EnableWindow")
	procEndPaint                      = user32.proc("EndPaint")
	procEnumChildWindows              = user32.proc("EnumChildWindows")
	procEnumDisplayDevices            = user32.proc("EnumDisplayDevicesW")
	procEnumDisplaySettings           = user32.proc("EnumDisplaySettingsW")
	procFillRect                      = user32.proc("FillRect")
	procFrameRect                     = user32.proc("FrameRect")
	procGetActiveWindow               = user32.proc("GetActiveWindow")
	procGetAncestor                   = user32.proc("GetAncestor")
	procGetCapture                    = user32.proc("GetCapture")
	procGetClassName                  = user32.proc("GetClassNameW")
	procGetClientRect                 = user32.proc("GetClientRect")
	procGetComboBoxInfo               = user32.proc("GetComboBoxInfo")
	procGetCursorPos                  = user32.proc("GetCursorPos")
	procGetDC                         = user32.proc("GetDC")
	procGetDpiForSystem               = user32.proc("GetDpiForSystem")
	procGetDpiForWindow               = user32.proc("GetDpiForWindow")
	procGetFocus                      = user32.proc("GetFocus")
	procGetForegroundWindow           = user32.proc("GetForegroundWindow")
	procGetKeyState                   = user32.proc("GetKeyState")
	procGetMenuBarInfo                = user32.proc("GetMenuBarInfo")
	procGetMenuItemInfo               = user32.proc("GetMenuItemInfoW")
	procGetMonitorInfo                = user32.proc("GetMonitorInfoW")
	procGetNextDlgTabItem             = user32.proc("GetNextDlgTabItem")
	procGetParent                     = user32.proc("GetParent")
	procGetProp                       = user32.proc("GetPropW")
	procGetScrollInfo                 = user32.proc("GetScrollInfo")
	procGetSystemMetrics              = user32.proc("GetSystemMetrics")
	procGetWindow                     = user32.proc("GetWindow")
	procGetWindowDC                   = user32.proc("GetWindowDC")
	procGetWindowLongPtr              = user32.proc(pointerSized("GetWindowLongPtrW", "GetWindowLongW"))
	procGetWindowPlacement            = user32.proc("GetWindowPlacement")
	procGetWindowRect                 = user32.proc("GetWindowRect")
	procGetWindowText                 = user32.proc("GetWindowTextW")
	procGetWindowTextLength           = user32.proc("GetWindowTextLengthW")
	procGetWindowThreadProcessID      = user32.proc("GetWindowThreadProcessId")
	procInvalidateRect                = user32.proc("InvalidateRect")
	procIsChild                       = user32.proc("IsChild")
	procIsIconic                      = user32.proc("IsIconic")
	procIsWindow                      = user32.proc("IsWindow")
	procIsWindowEnabled               = user32.proc("IsWindowEnabled")
	procIsWindowVisible               = user32.proc("IsWindowVisible")
	procIsZoomed                      = user32.proc("IsZoomed")
	procKillTimer                     = user32.proc("KillTimer")
	procLoadCursor                    = user32.proc("LoadCursorW")
	procLoadIcon                      = user32.proc("LoadIconW")
	procMapVirtualKey                 = user32.proc("MapVirtualKeyW")
	procMapWindowPoints               = user32.proc("MapWindowPoints")
	procMonitorFromPoint              = user32.proc("MonitorFromPoint")
	procMonitorFromWindow             = user32.proc("MonitorFromWindow")
	procMsgWaitForMultipleObjectsEx   = user32.proc("MsgWaitForMultipleObjectsEx")
	procPeekMessage                   = user32.proc("PeekMessageW")
	procPostMessage                   = user32.proc("PostMessageW")
	procRedrawWindow                  = user32.proc("RedrawWindow")
	procRegisterClassEx               = user32.proc("RegisterClassExW")
	procRegisterClipboardFormat       = user32.proc("RegisterClipboardFormatW")
	procReleaseCapture                = user32.proc("ReleaseCapture")
	procReleaseDC                     = user32.proc("ReleaseDC")
	procRemoveProp                    = user32.proc("RemovePropW")
	procScreenToClient                = user32.proc("ScreenToClient")
	procSendMessage                   = user32.proc("SendMessageW")
	procSetActiveWindow               = user32.proc("SetActiveWindow")
	procSetCapture                    = user32.proc("SetCapture")
	procSetCursor                     = user32.proc("SetCursor")
	procSetCursorPos                  = user32.proc("SetCursorPos")
	procSetFocus                      = user32.proc("SetFocus")
	procSetForegroundWindow           = user32.proc("SetForegroundWindow")
	procSetLayeredWindowAttributes    = user32.proc("SetLayeredWindowAttributes")
	procSetMenu                       = user32.proc("SetMenu")
	procSetParent                     = user32.proc("SetParent")
	procSetProcessDpiAwarenessContext = user32.proc("SetProcessDpiAwarenessContext")
	procSetProp                       = user32.proc("SetPropW")
	procSetScrollInfo                 = user32.proc("SetScrollInfo")
	procSetTimer                      = user32.proc("SetTimer")
	procSetWindowLongPtr              = user32.proc(pointerSized("SetWindowLongPtrW", "SetWindowLongW"))
	procSetWindowPlacement            = user32.proc("SetWindowPlacement")
	procSetWindowPos                  = user32.proc("SetWindowPos")
	procSetWindowRgn                  = user32.proc("SetWindowRgn")
	procSetWindowText                 = user32.proc("SetWindowTextW")
	procShowScrollBar                 = user32.proc("ShowScrollBar")
	procShowWindow                    = user32.proc("ShowWindow")
	procShutdownBlockReasonCreate     = user32.proc("ShutdownBlockReasonCreate")
	procShutdownBlockReasonDestroy    = user32.proc("ShutdownBlockReasonDestroy")
	procSystemParametersInfo          = user32.proc("SystemParametersInfoW")
	procSystemParametersInfoForDpi    = user32.proc("SystemParametersInfoForDpi")
	procTrackMouseEvent               = user32.proc("TrackMouseEvent")
	procTranslateMessage              = user32.proc("TranslateMessage")
	procUpdateWindow                  = user32.proc("UpdateWindow")
	procWindowFromPoint               = user32.proc("WindowFromPoint")
)

// pointerSized returns the name of a function on 64-bit Windows, or the name
// of the function that takes its place on 32-bit Windows.
func pointerSized(name64, name32 string) string {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return name64
	}
	return name32
}

func win32AdjustWindowRectExForDpi(r *rect, style uint32, menu bool, exStyle, dpi uint32) bool {
	return procAdjustWindowRectExForDpi.call(uintptr(unsafe.Pointer(r)), uintptr(style), boolArg(menu),
		uintptr(exStyle), uintptr(dpi)) != 0
}

func win32BeginPaint(h hwnd, paint *paintStruct) hdc {
	return hdc(procBeginPaint.call(uintptr(h), uintptr(unsafe.Pointer(paint))))
}

func win32ClientToScreen(h hwnd, p *point) bool {
	return procClientToScreen.call(uintptr(h), uintptr(unsafe.Pointer(p))) != 0
}

func win32ClipCursor(r *rect) bool {
	return procClipCursor.call(uintptr(unsafe.Pointer(r))) != 0
}

func win32CreateWindowEx(exStyle uint32, class, title string, style uint32, x, y, width, height int32,
	parent hwnd, menu hmenu, instance hinstance, param unsafe.Pointer) hwnd {
	return hwnd(procCreateWindowEx.call(uintptr(exStyle), uintptr(unsafe.Pointer(wide(class))),
		uintptr(unsafe.Pointer(wide(title))), uintptr(style), uintptr(x), uintptr(y), uintptr(width),
		uintptr(height), uintptr(parent), uintptr(menu), uintptr(instance), uintptr(param)))
}

func win32DefWindowProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	return procDefWindowProc.call(uintptr(h), uintptr(message), wparam, lparam)
}

func win32DestroyWindow(h hwnd) bool {
	return procDestroyWindow.call(uintptr(h)) != 0
}

func win32DispatchMessage(message *msg) uintptr {
	return procDispatchMessage.call(uintptr(unsafe.Pointer(message)))
}

func win32DrawText(dc hdc, text string, r *rect, format uint32) int32 {
	return int32(procDrawText.call(uintptr(dc), uintptr(unsafe.Pointer(wide(text))), ^uintptr(0),
		uintptr(unsafe.Pointer(r)), uintptr(format)))
}

func win32EnableNonClientDpiScaling(h hwnd) bool {
	return procEnableNonClientDpiScaling.call(uintptr(h)) != 0
}

func win32EnableWindow(h hwnd, enable bool) bool {
	return procEnableWindow.call(uintptr(h), boolArg(enable)) != 0
}

func win32EndPaint(h hwnd, paint *paintStruct) bool {
	return procEndPaint.call(uintptr(h), uintptr(unsafe.Pointer(paint))) != 0
}

func win32EnumChildWindows(parent hwnd, function, lparam uintptr) bool {
	return procEnumChildWindows.call(uintptr(parent), function, lparam) != 0
}

func win32EnumDisplayDevices(device *uint16, index uint32, info *displayDevice, flags uint32) bool {
	return procEnumDisplayDevices.call(uintptr(unsafe.Pointer(device)), uintptr(index), uintptr(unsafe.Pointer(info)),
		uintptr(flags)) != 0
}

func win32EnumDisplaySettings(device *uint16, mode uint32, settings *devMode) bool {
	return procEnumDisplaySettings.call(uintptr(unsafe.Pointer(device)), uintptr(mode),
		uintptr(unsafe.Pointer(settings))) != 0
}

func win32FillRect(dc hdc, r *rect, brush hbrush) int32 {
	return int32(procFillRect.call(uintptr(dc), uintptr(unsafe.Pointer(r)), uintptr(brush)))
}

func win32FrameRect(dc hdc, r *rect, brush hbrush) int32 {
	return int32(procFrameRect.call(uintptr(dc), uintptr(unsafe.Pointer(r)), uintptr(brush)))
}

func win32GetActiveWindow() hwnd {
	return hwnd(procGetActiveWindow.call())
}

func win32GetAncestor(h hwnd, flags uint32) hwnd {
	return hwnd(procGetAncestor.call(uintptr(h), uintptr(flags)))
}

func win32GetCapture() hwnd {
	return hwnd(procGetCapture.call())
}

func win32GetClassName(h hwnd, name *uint16, size int32) int32 {
	return int32(procGetClassName.call(uintptr(h), uintptr(unsafe.Pointer(name)), uintptr(size)))
}

func win32GetClientRect(h hwnd, r *rect) bool {
	return procGetClientRect.call(uintptr(h), uintptr(unsafe.Pointer(r))) != 0
}

func win32GetComboBoxInfo(h hwnd, info *comboBoxInfo) bool {
	return procGetComboBoxInfo.call(uintptr(h), uintptr(unsafe.Pointer(info))) != 0
}

func win32GetCursorPos(p *point) bool {
	return procGetCursorPos.call(uintptr(unsafe.Pointer(p))) != 0
}

func win32GetDC(h hwnd) hdc {
	return hdc(procGetDC.call(uintptr(h)))
}

func win32GetDpiForSystem() uint32 {
	return uint32(procGetDpiForSystem.call())
}

func win32GetDpiForWindow(h hwnd) uint32 {
	return uint32(procGetDpiForWindow.call(uintptr(h)))
}

func win32GetFocus() hwnd {
	return hwnd(procGetFocus.call())
}

func win32GetForegroundWindow() hwnd {
	return hwnd(procGetForegroundWindow.call())
}

func win32GetKeyState(key int32) uint16 {
	return uint16(procGetKeyState.call(uintptr(key)))
}

func win32GetMenuBarInfo(h hwnd, object, item int32, info *menuBarInfo) bool {
	return procGetMenuBarInfo.call(uintptr(h), uintptr(object), uintptr(item), uintptr(unsafe.Pointer(info))) != 0
}

func win32GetMenuItemInfo(menu hmenu, item uint32, byPosition bool, info *menuItemInfo) bool {
	return procGetMenuItemInfo.call(uintptr(menu), uintptr(item), boolArg(byPosition),
		uintptr(unsafe.Pointer(info))) != 0
}

// win32GetMonitorInfo fills a monitorInfo or a monitorInfoEx.
func win32GetMonitorInfo(monitor uintptr, info unsafe.Pointer) bool {
	return procGetMonitorInfo.call(monitor, uintptr(info)) != 0
}

func win32GetNextDlgTabItem(dialog, control hwnd, previous bool) hwnd {
	return hwnd(procGetNextDlgTabItem.call(uintptr(dialog), uintptr(control), boolArg(previous)))
}

func win32GetParent(h hwnd) hwnd {
	return hwnd(procGetParent.call(uintptr(h)))
}

func win32GetProp(h hwnd, name string) uintptr {
	return procGetProp.call(uintptr(h), uintptr(unsafe.Pointer(propertyName(name))))
}

func win32GetScrollInfo(h hwnd, bar int32, info *scrollInfo) bool {
	return procGetScrollInfo.call(uintptr(h), uintptr(bar), uintptr(unsafe.Pointer(info))) != 0
}

func win32GetSystemMetrics(index int32) int32 {
	return int32(procGetSystemMetrics.call(uintptr(index)))
}

func win32GetWindow(h hwnd, command uint32) hwnd {
	return hwnd(procGetWindow.call(uintptr(h), uintptr(command)))
}

func win32GetWindowDC(h hwnd) hdc {
	return hdc(procGetWindowDC.call(uintptr(h)))
}

func win32GetWindowLongPtr(h hwnd, index int32) uintptr {
	return procGetWindowLongPtr.call(uintptr(h), uintptr(index))
}

func win32GetWindowPlacement(h hwnd, placement *windowPlacement) bool {
	return procGetWindowPlacement.call(uintptr(h), uintptr(unsafe.Pointer(placement))) != 0
}

func win32GetWindowRect(h hwnd, r *rect) bool {
	return procGetWindowRect.call(uintptr(h), uintptr(unsafe.Pointer(r))) != 0
}

func win32GetWindowText(h hwnd, text *uint16, size int) int {
	return int(int32(procGetWindowText.call(uintptr(h), uintptr(unsafe.Pointer(text)), uintptr(size))))
}

func win32GetWindowTextLength(h hwnd) int {
	return int(int32(procGetWindowTextLength.call(uintptr(h))))
}

// win32GetWindowThreadProcessID returns the thread of a window.
func win32GetWindowThreadProcessID(h hwnd, process *uint32) uint32 {
	return uint32(procGetWindowThreadProcessID.call(uintptr(h), uintptr(unsafe.Pointer(process))))
}

func win32InvalidateRect(h hwnd, r *rect, erase bool) bool {
	return procInvalidateRect.call(uintptr(h), uintptr(unsafe.Pointer(r)), boolArg(erase)) != 0
}

func win32IsChild(parent, h hwnd) bool {
	return procIsChild.call(uintptr(parent), uintptr(h)) != 0
}

func win32IsIconic(h hwnd) bool {
	return procIsIconic.call(uintptr(h)) != 0
}

func win32IsWindow(h hwnd) bool {
	return procIsWindow.call(uintptr(h)) != 0
}

func win32IsWindowEnabled(h hwnd) bool {
	return procIsWindowEnabled.call(uintptr(h)) != 0
}

func win32IsWindowVisible(h hwnd) bool {
	return procIsWindowVisible.call(uintptr(h)) != 0
}

func win32IsZoomed(h hwnd) bool {
	return procIsZoomed.call(uintptr(h)) != 0
}

func win32KillTimer(h hwnd, id uintptr) bool {
	return procKillTimer.call(uintptr(h), id) != 0
}

// win32LoadCursor loads a cursor; name is a resource identifier.
func win32LoadCursor(instance hinstance, name uintptr) hcursor {
	return hcursor(procLoadCursor.call(uintptr(instance), name))
}

// win32LoadIcon loads an icon; name is a resource identifier.
func win32LoadIcon(instance hinstance, name uintptr) hicon {
	return hicon(procLoadIcon.call(uintptr(instance), name))
}

func win32MapVirtualKey(code, mapType uint32) uint32 {
	return uint32(procMapVirtualKey.call(uintptr(code), uintptr(mapType)))
}

func win32MapWindowPoints(from, to hwnd, points *point, count uint32) int32 {
	return int32(procMapWindowPoints.call(uintptr(from), uintptr(to), uintptr(unsafe.Pointer(points)), uintptr(count)))
}

func win32MonitorFromPoint(p point, flags uint32) uintptr {
	args := append(pairArgs(uint32(p.x), uint32(p.y)), uintptr(flags))
	return procMonitorFromPoint.call(args...)
}

func win32MonitorFromWindow(h hwnd, flags uint32) uintptr {
	return procMonitorFromWindow.call(uintptr(h), uintptr(flags))
}

func win32MsgWaitForMultipleObjectsEx(count uint32, handles *uintptr, milliseconds, wakeMask, flags uint32) uint32 {
	return uint32(procMsgWaitForMultipleObjectsEx.call(uintptr(count), uintptr(unsafe.Pointer(handles)),
		uintptr(milliseconds), uintptr(wakeMask), uintptr(flags)))
}

func win32PeekMessage(message *msg, h hwnd, first, last, remove uint32) bool {
	return procPeekMessage.call(uintptr(unsafe.Pointer(message)), uintptr(h), uintptr(first), uintptr(last),
		uintptr(remove)) != 0
}

func win32PostMessage(h hwnd, message uint32, wparam, lparam uintptr) bool {
	return procPostMessage.call(uintptr(h), uintptr(message), wparam, lparam) != 0
}

func win32RedrawWindow(h hwnd, update *rect, region hrgn, flags uint32) bool {
	return procRedrawWindow.call(uintptr(h), uintptr(unsafe.Pointer(update)), uintptr(region), uintptr(flags)) != 0
}

func win32RegisterClassEx(class *wndClassEx) uint16 {
	return uint16(procRegisterClassEx.call(uintptr(unsafe.Pointer(class))))
}

func win32RegisterClipboardFormat(name string) uint32 {
	return uint32(procRegisterClipboardFormat.call(uintptr(unsafe.Pointer(wide(name)))))
}

func win32ReleaseCapture() bool {
	return procReleaseCapture.call() != 0
}

func win32ReleaseDC(h hwnd, dc hdc) int32 {
	return int32(procReleaseDC.call(uintptr(h), uintptr(dc)))
}

func win32RemoveProp(h hwnd, name string) uintptr {
	return procRemoveProp.call(uintptr(h), uintptr(unsafe.Pointer(propertyName(name))))
}

func win32ScreenToClient(h hwnd, p *point) bool {
	return procScreenToClient.call(uintptr(h), uintptr(unsafe.Pointer(p))) != 0
}

// win32SendMessage sends a message. A message can call Go code, which can move
// the stack, so a pointer in wparam or lparam moves to the heap.
//
//go:uintptrescapes
func win32SendMessage(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	return procSendMessage.call(uintptr(h), uintptr(message), wparam, lparam)
}

func win32SetActiveWindow(h hwnd) hwnd {
	return hwnd(procSetActiveWindow.call(uintptr(h)))
}

func win32SetCapture(h hwnd) hwnd {
	return hwnd(procSetCapture.call(uintptr(h)))
}

func win32SetCursor(cursor hcursor) hcursor {
	return hcursor(procSetCursor.call(uintptr(cursor)))
}

func win32SetCursorPos(x, y int32) bool {
	return procSetCursorPos.call(uintptr(x), uintptr(y)) != 0
}

func win32SetFocus(h hwnd) hwnd {
	return hwnd(procSetFocus.call(uintptr(h)))
}

func win32SetForegroundWindow(h hwnd) bool {
	return procSetForegroundWindow.call(uintptr(h)) != 0
}

func win32SetLayeredWindowAttributes(h hwnd, key uint32, alpha byte, flags uint32) bool {
	return procSetLayeredWindowAttributes.call(uintptr(h), uintptr(key), uintptr(alpha), uintptr(flags)) != 0
}

func win32SetMenu(h hwnd, menu hmenu) bool {
	return procSetMenu.call(uintptr(h), uintptr(menu)) != 0
}

func win32SetParent(child, parent hwnd) hwnd {
	return hwnd(procSetParent.call(uintptr(child), uintptr(parent)))
}

func win32SetProcessDpiAwarenessContext(context uintptr) bool {
	return procSetProcessDpiAwarenessContext.call(context) != 0
}

func win32SetProp(h hwnd, name string, value uintptr) bool {
	return procSetProp.call(uintptr(h), uintptr(unsafe.Pointer(propertyName(name))), value) != 0
}

func win32SetScrollInfo(h hwnd, bar int32, info *scrollInfo, redraw bool) int32 {
	return int32(procSetScrollInfo.call(uintptr(h), uintptr(bar), uintptr(unsafe.Pointer(info)), boolArg(redraw)))
}

func win32SetTimer(h hwnd, id uintptr, elapse uint32, function uintptr) uintptr {
	return procSetTimer.call(uintptr(h), id, uintptr(elapse), function)
}

func win32SetWindowLongPtr(h hwnd, index int32, value uintptr) uintptr {
	return procSetWindowLongPtr.call(uintptr(h), uintptr(index), value)
}

func win32SetWindowPlacement(h hwnd, placement *windowPlacement) bool {
	return procSetWindowPlacement.call(uintptr(h), uintptr(unsafe.Pointer(placement))) != 0
}

func win32SetWindowPos(h, after hwnd, x, y, width, height int32, flags uint32) bool {
	return procSetWindowPos.call(uintptr(h), uintptr(after), uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		uintptr(flags)) != 0
}

func win32SetWindowRgn(h hwnd, region hrgn, redraw bool) int32 {
	return int32(procSetWindowRgn.call(uintptr(h), uintptr(region), boolArg(redraw)))
}

func win32SetWindowText(h hwnd, text string) bool {
	return procSetWindowText.call(uintptr(h), uintptr(unsafe.Pointer(wide(text)))) != 0
}

func win32ShowScrollBar(h hwnd, bar int32, show bool) bool {
	return procShowScrollBar.call(uintptr(h), uintptr(bar), boolArg(show)) != 0
}

func win32ShowWindow(h hwnd, command int32) bool {
	return procShowWindow.call(uintptr(h), uintptr(command)) != 0
}

func win32ShutdownBlockReasonCreate(h hwnd, reason string) bool {
	return procShutdownBlockReasonCreate.call(uintptr(h), uintptr(unsafe.Pointer(wide(reason)))) != 0
}

func win32ShutdownBlockReasonDestroy(h hwnd) bool {
	return procShutdownBlockReasonDestroy.call(uintptr(h)) != 0
}

func win32SystemParametersInfo(action, param uint32, value unsafe.Pointer, winIni uint32) bool {
	return procSystemParametersInfo.call(uintptr(action), uintptr(param), uintptr(value), uintptr(winIni)) != 0
}

func win32SystemParametersInfoForDpi(action, param uint32, value unsafe.Pointer, winIni, dpi uint32) bool {
	return procSystemParametersInfoForDpi.call(uintptr(action), uintptr(param), uintptr(value), uintptr(winIni),
		uintptr(dpi)) != 0
}

func win32TrackMouseEvent(track *trackMouseEventInfo) bool {
	return procTrackMouseEvent.call(uintptr(unsafe.Pointer(track))) != 0
}

func win32TranslateMessage(message *msg) bool {
	return procTranslateMessage.call(uintptr(unsafe.Pointer(message))) != 0
}

func win32UpdateWindow(h hwnd) bool {
	return procUpdateWindow.call(uintptr(h)) != 0
}

func win32WindowFromPoint(p point) hwnd {
	return hwnd(procWindowFromPoint.call(pairArgs(uint32(p.x), uint32(p.y))...))
}

var (
	propertyNamesMu sync.Mutex
	propertyNames   = map[string]*uint16{}
)

// propertyName returns the UTF-16 name of a window property, which the
// package looks up on every message.
func propertyName(name string) *uint16 {
	propertyNamesMu.Lock()
	defer propertyNamesMu.Unlock()
	value, ok := propertyNames[name]
	if !ok {
		value = wide(name)
		propertyNames[name] = value
	}
	return value
}

// ---- kernel32 ----

var (
	procActivateActCtx           = kernel32.proc("ActivateActCtx")
	procCreateActCtx             = kernel32.proc("CreateActCtxW")
	procDeactivateActCtx         = kernel32.proc("DeactivateActCtx")
	procGetCurrentThreadID       = kernel32.proc("GetCurrentThreadId")
	procGetLocaleInfoEx          = kernel32.proc("GetLocaleInfoEx")
	procGetModuleHandle          = kernel32.proc("GetModuleHandleW")
	procGetSystemDirectory       = kernel32.proc("GetSystemDirectoryW")
	procGetTickCount64           = kernel32.proc("GetTickCount64")
	procGetUserDefaultLocaleName = kernel32.proc("GetUserDefaultLocaleName")
	procGlobalAlloc              = kernel32.proc("GlobalAlloc")
	procGlobalFree               = kernel32.proc("GlobalFree")
	procGlobalLock               = kernel32.proc("GlobalLock")
	procGlobalSize               = kernel32.proc("GlobalSize")
	procGlobalUnlock             = kernel32.proc("GlobalUnlock")
)

func win32ActivateActCtx(context uintptr, cookie *uintptr) bool {
	return procActivateActCtx.call(context, uintptr(unsafe.Pointer(cookie))) != 0
}

func win32CreateActCtx(context *actCtx) uintptr {
	return procCreateActCtx.call(uintptr(unsafe.Pointer(context)))
}

func win32DeactivateActCtx(flags uint32, cookie uintptr) bool {
	if cookie == 0 {
		return false
	}
	return procDeactivateActCtx.call(uintptr(flags), cookie) != 0
}

func win32GetCurrentThreadID() uint32 {
	return uint32(procGetCurrentThreadID.call())
}

func win32GetLocaleInfoEx(locale string, kind uint32, data *uint16, size int32) int32 {
	return int32(procGetLocaleInfoEx.call(uintptr(unsafe.Pointer(wideOrNil(locale))), uintptr(kind),
		uintptr(unsafe.Pointer(data)), uintptr(size)))
}

// win32GetModuleHandle returns a loaded module, or the program for "".
func win32GetModuleHandle(name string) hinstance {
	return hinstance(procGetModuleHandle.call(uintptr(unsafe.Pointer(wideOrNil(name)))))
}

func win32GetSystemDirectory(buffer *uint16, size uint32) uint32 {
	return uint32(procGetSystemDirectory.call(uintptr(unsafe.Pointer(buffer)), uintptr(size)))
}

func win32GetTickCount64() uint64 {
	return procGetTickCount64.call64()
}

func win32GetUserDefaultLocaleName(name *uint16, size int32) int32 {
	return int32(procGetUserDefaultLocaleName.call(uintptr(unsafe.Pointer(name)), uintptr(size)))
}

func win32GlobalAlloc(flags uint32, size uintptr) hglobal {
	return hglobal(procGlobalAlloc.call(uintptr(flags), size))
}

func win32GlobalFree(memory hglobal) hglobal {
	return hglobal(procGlobalFree.call(uintptr(memory)))
}

// win32GlobalLock returns the address of global memory.
func win32GlobalLock(memory hglobal) uintptr {
	return procGlobalLock.call(uintptr(memory))
}

func win32GlobalSize(memory hglobal) uintptr {
	return procGlobalSize.call(uintptr(memory))
}

func win32GlobalUnlock(memory hglobal) bool {
	return procGlobalUnlock.call(uintptr(memory)) != 0
}

// ---- gdi32 ----

var (
	procCreateCompatibleDC = gdi32.proc("CreateCompatibleDC")
	procCreateFont         = gdi32.proc("CreateFontW")
	procCreateRoundRectRgn = gdi32.proc("CreateRoundRectRgn")
	procCreateSolidBrush   = gdi32.proc("CreateSolidBrush")
	procDeleteObject       = gdi32.proc("DeleteObject")
	procSelectObject       = gdi32.proc("SelectObject")
	procSetBkColor         = gdi32.proc("SetBkColor")
	procSetBkMode          = gdi32.proc("SetBkMode")
	procSetTextColor       = gdi32.proc("SetTextColor")
)

func win32CreateCompatibleDC(dc hdc) hdc {
	return hdc(procCreateCompatibleDC.call(uintptr(dc)))
}

func win32CreateFont(height, width, escapement, orientation, weight int32, italic, underline, strikeOut bool,
	charSet, outPrecision, clipPrecision, quality, pitchAndFamily uint32, face string) hfont {
	return hfont(procCreateFont.call(uintptr(height), uintptr(width), uintptr(escapement), uintptr(orientation),
		uintptr(weight), boolArg(italic), boolArg(underline), boolArg(strikeOut), uintptr(charSet),
		uintptr(outPrecision), uintptr(clipPrecision), uintptr(quality), uintptr(pitchAndFamily),
		uintptr(unsafe.Pointer(wide(face)))))
}

func win32CreateRoundRectRgn(x1, y1, x2, y2, width, height int32) hrgn {
	return hrgn(procCreateRoundRectRgn.call(uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2), uintptr(width),
		uintptr(height)))
}

func win32CreateSolidBrush(color uint32) hbrush {
	return hbrush(procCreateSolidBrush.call(uintptr(color)))
}

func win32DeleteObject(object hgdiobj) bool {
	return procDeleteObject.call(uintptr(object)) != 0
}

func win32SelectObject(dc hdc, object hgdiobj) hgdiobj {
	return hgdiobj(procSelectObject.call(uintptr(dc), uintptr(object)))
}

func win32SetBkColor(dc hdc, color uint32) uint32 {
	return uint32(procSetBkColor.call(uintptr(dc), uintptr(color)))
}

func win32SetBkMode(dc hdc, mode int32) int32 {
	return int32(procSetBkMode.call(uintptr(dc), uintptr(mode)))
}

func win32SetTextColor(dc hdc, color uint32) uint32 {
	return uint32(procSetTextColor.call(uintptr(dc), uintptr(color)))
}

// ---- comctl32 ----

var (
	procDefSubclassProc      = comctl32.proc("DefSubclassProc")
	procImageListAdd         = comctl32.proc("ImageList_Add")
	procImageListCreate      = comctl32.proc("ImageList_Create")
	procImageListDestroy     = comctl32.proc("ImageList_Destroy")
	procInitCommonControlsEx = comctl32.proc("InitCommonControlsEx")
	procLoadIconMetric       = comctl32.proc("LoadIconMetric")
	procRemoveWindowSubclass = comctl32.proc("RemoveWindowSubclass")
	procSetWindowSubclass    = comctl32.proc("SetWindowSubclass")
	procTaskDialogIndirect   = comctl32.proc("TaskDialogIndirect")
)

func win32DefSubclassProc(h hwnd, message uint32, wparam, lparam uintptr) uintptr {
	return procDefSubclassProc.call(uintptr(h), uintptr(message), wparam, lparam)
}

func win32ImageListAdd(list himagelist, image, mask hbitmap) int32 {
	return int32(procImageListAdd.call(uintptr(list), uintptr(image), uintptr(mask)))
}

func win32ImageListCreate(width, height int32, flags uint32, initial, grow int32) himagelist {
	return himagelist(procImageListCreate.call(uintptr(width), uintptr(height), uintptr(flags), uintptr(initial),
		uintptr(grow)))
}

func win32ImageListDestroy(list himagelist) bool {
	return procImageListDestroy.call(uintptr(list)) != 0
}

func win32InitCommonControlsEx(controls *initCommonControlsExInfo) bool {
	return procInitCommonControlsEx.call(uintptr(unsafe.Pointer(controls))) != 0
}

func win32LoadIconMetric(instance hinstance, name uintptr, metric int32, icon *hicon) hresult {
	return hresult(procLoadIconMetric.call(uintptr(instance), name, uintptr(metric), uintptr(unsafe.Pointer(icon))))
}

func win32RemoveWindowSubclass(h hwnd, function, id uintptr) bool {
	return procRemoveWindowSubclass.call(uintptr(h), function, id) != 0
}

func win32SetWindowSubclass(h hwnd, function, id, data uintptr) bool {
	return procSetWindowSubclass.call(uintptr(h), function, id, data) != 0
}

// ---- uxtheme ----
//
// The dark mode of the controls of Windows is reached through functions that
// uxtheme.dll exports by ordinal.

var (
	procAllowDarkModeForWindow    = uxtheme.ordinalProc(133)
	procBeginBufferedPaint        = uxtheme.proc("BeginBufferedPaint")
	procBufferedPaintInit         = uxtheme.proc("BufferedPaintInit")
	procCloseThemeData            = uxtheme.proc("CloseThemeData")
	procDrawThemeBackground       = uxtheme.proc("DrawThemeBackground")
	procDrawThemeParentBackground = uxtheme.proc("DrawThemeParentBackground")
	procEndBufferedPaint          = uxtheme.proc("EndBufferedPaint")
	procFlushMenuThemes           = uxtheme.ordinalProc(136)
	procOpenThemeData             = uxtheme.proc("OpenThemeData")
	procSetPreferredAppMode       = uxtheme.ordinalProc(135)
	procSetWindowTheme            = uxtheme.proc("SetWindowTheme")
)

func win32AllowDarkModeForWindow(h hwnd, allow bool) bool {
	return procAllowDarkModeForWindow.call(uintptr(h), boolArg(allow)) != 0
}

// win32BeginBufferedPaint returns a buffered paint handle and its device
// context in target.
func win32BeginBufferedPaint(dc hdc, area *rect, format uint32, params unsafe.Pointer, target *hdc) uintptr {
	return procBeginBufferedPaint.call(uintptr(dc), uintptr(unsafe.Pointer(area)), uintptr(format), uintptr(params),
		uintptr(unsafe.Pointer(target)))
}

func win32BufferedPaintInit() hresult {
	return hresult(procBufferedPaintInit.call())
}

func win32CloseThemeData(theme htheme) hresult {
	return hresult(procCloseThemeData.call(uintptr(theme)))
}

func win32DrawThemeBackground(theme htheme, dc hdc, part, state int32, area, clip *rect) hresult {
	return hresult(procDrawThemeBackground.call(uintptr(theme), uintptr(dc), uintptr(part), uintptr(state),
		uintptr(unsafe.Pointer(area)), uintptr(unsafe.Pointer(clip))))
}

func win32DrawThemeParentBackground(h hwnd, dc hdc, area *rect) hresult {
	return hresult(procDrawThemeParentBackground.call(uintptr(h), uintptr(dc), uintptr(unsafe.Pointer(area))))
}

func win32EndBufferedPaint(buffer uintptr, update bool) hresult {
	return hresult(procEndBufferedPaint.call(buffer, boolArg(update)))
}

func win32FlushMenuThemes() {
	procFlushMenuThemes.call()
}

func win32OpenThemeData(h hwnd, classes string) htheme {
	return htheme(procOpenThemeData.call(uintptr(h), uintptr(unsafe.Pointer(wide(classes)))))
}

func win32SetPreferredAppMode(mode int32) int32 {
	return int32(procSetPreferredAppMode.call(uintptr(mode)))
}

// win32SetWindowTheme sets the visual style of a window; "" passes NULL.
func win32SetWindowTheme(h hwnd, subApp, subIDList string) hresult {
	return hresult(procSetWindowTheme.call(uintptr(h), uintptr(unsafe.Pointer(wideOrNil(subApp))),
		uintptr(unsafe.Pointer(wideOrNil(subIDList)))))
}

// ---- dwmapi, shcore, and advapi32 ----

var (
	procDwmSetWindowAttribute = dwmapi.proc("DwmSetWindowAttribute")
	procGetDpiForMonitor      = shcore.proc("GetDpiForMonitor")
	procRegGetValue           = advapi32.proc("RegGetValueW")
)

func win32DwmSetWindowAttribute(h hwnd, attribute uint32, value unsafe.Pointer, size uint32) hresult {
	return hresult(procDwmSetWindowAttribute.call(uintptr(h), uintptr(attribute), uintptr(value), uintptr(size)))
}

func win32GetDpiForMonitor(monitor uintptr, kind uint32, x, y *uint32) hresult {
	return hresult(procGetDpiForMonitor.call(monitor, uintptr(kind), uintptr(unsafe.Pointer(x)),
		uintptr(unsafe.Pointer(y))))
}

func win32RegGetValue(key uintptr, subKey, value string, flags uint32, kind *uint32, data unsafe.Pointer,
	size *uint32) int32 {
	return int32(procRegGetValue.call(key, uintptr(unsafe.Pointer(wide(subKey))), uintptr(unsafe.Pointer(wide(value))),
		uintptr(flags), uintptr(unsafe.Pointer(kind)), uintptr(data), uintptr(unsafe.Pointer(size))))
}

// ---- ole32 ----

var (
	procCoCreateInstance      = ole32.proc("CoCreateInstance")
	procCoTaskMemFree         = ole32.proc("CoTaskMemFree")
	procCreateStreamOnHGlobal = ole32.proc("CreateStreamOnHGlobal")
	procDoDragDrop            = ole32.proc("DoDragDrop")
	procGetHGlobalFromStream  = ole32.proc("GetHGlobalFromStream")
	procOleInitialize         = ole32.proc("OleInitialize")
	procRegisterDragDrop      = ole32.proc("RegisterDragDrop")
	procReleaseStgMedium      = ole32.proc("ReleaseStgMedium")
	procRevokeDragDrop        = ole32.proc("RevokeDragDrop")
)

func win32CoCreateInstance(clsid *guid, outer comObject, context uint32, iid *guid, result *comObject) hresult {
	return hresult(procCoCreateInstance.call(uintptr(unsafe.Pointer(clsid)), uintptr(outer), uintptr(context),
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(result))))
}

func win32CoTaskMemFree(memory uintptr) {
	procCoTaskMemFree.call(memory)
}

func win32CreateStreamOnHGlobal(memory hglobal, deleteOnRelease bool, stream *comObject) hresult {
	return hresult(procCreateStreamOnHGlobal.call(uintptr(memory), boolArg(deleteOnRelease),
		uintptr(unsafe.Pointer(stream))))
}

func win32DoDragDrop(data, source comObject, allowed uint32, effect *uint32) hresult {
	return hresult(procDoDragDrop.call(uintptr(data), uintptr(source), uintptr(allowed),
		uintptr(unsafe.Pointer(effect))))
}

func win32GetHGlobalFromStream(stream comObject, memory *hglobal) hresult {
	return hresult(procGetHGlobalFromStream.call(uintptr(stream), uintptr(unsafe.Pointer(memory))))
}

func win32OleInitialize(reserved uintptr) hresult {
	return hresult(procOleInitialize.call(reserved))
}

func win32RegisterDragDrop(h hwnd, target comObject) hresult {
	return hresult(procRegisterDragDrop.call(uintptr(h), uintptr(target)))
}

func win32ReleaseStgMedium(medium *stgMedium) {
	procReleaseStgMedium.call(uintptr(unsafe.Pointer(medium)))
}

func win32RevokeDragDrop(h hwnd) hresult {
	return hresult(procRevokeDragDrop.call(uintptr(h)))
}

// IStream methods.

func win32StreamStat(stream comObject, stat *statStg, flags uint32) hresult {
	return comResult(stream, 12, uintptr(unsafe.Pointer(stat)), uintptr(flags))
}

// IDataObject methods.

func win32DataObjectGetData(data comObject, format *formatEtc, medium *stgMedium) hresult {
	return comResult(data, 3, uintptr(unsafe.Pointer(format)), uintptr(unsafe.Pointer(medium)))
}

func win32DataObjectQueryGetData(data comObject, format *formatEtc) hresult {
	return comResult(data, 5, uintptr(unsafe.Pointer(format)))
}

// ---- shell32 ----

var (
	procDragAcceptFiles             = shell32.proc("DragAcceptFiles")
	procDragFinish                  = shell32.proc("DragFinish")
	procDragQueryFile               = shell32.proc("DragQueryFileW")
	procSHCreateItemFromParsingName = shell32.proc("SHCreateItemFromParsingName")
	procSHCreateStdEnumFmtEtc       = shell32.proc("SHCreateStdEnumFmtEtc")
	procShellExecuteEx              = shell32.proc("ShellExecuteExW")
)

func win32DragAcceptFiles(h hwnd, accept bool) {
	procDragAcceptFiles.call(uintptr(h), boolArg(accept))
}

func win32DragFinish(drop uintptr) {
	procDragFinish.call(drop)
}

// win32DragQueryFile returns the count of files for index 0xffffffff, or the
// length of a path; buffer may be nil.
func win32DragQueryFile(drop uintptr, index uint32, buffer []uint16) uint32 {
	var first *uint16
	if len(buffer) > 0 {
		first = &buffer[0]
	}
	return uint32(procDragQueryFile.call(drop, uintptr(index), uintptr(unsafe.Pointer(first)), uintptr(len(buffer))))
}

func win32SHCreateItemFromParsingName(path string, context comObject, iid *guid, item *comObject) hresult {
	return hresult(procSHCreateItemFromParsingName.call(uintptr(unsafe.Pointer(wide(path))), uintptr(context),
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(item))))
}

func win32SHCreateStdEnumFmtEtc(count uint32, formats *formatEtc, result *comObject) hresult {
	return hresult(procSHCreateStdEnumFmtEtc.call(uintptr(count), uintptr(unsafe.Pointer(formats)),
		uintptr(unsafe.Pointer(result))))
}

func win32ShellExecuteEx(info *shellExecuteInfo) bool {
	return procShellExecuteEx.call(uintptr(unsafe.Pointer(info))) != 0
}

// IModalWindow, IFileDialog, and IFileOpenDialog methods.

func win32ModalWindowShow(dialog comObject, owner hwnd) hresult {
	return comResult(dialog, 3, uintptr(owner))
}

func win32FileDialogSetFileTypes(dialog comObject, count uint32, specs *comdlgFilterSpec) hresult {
	return comResult(dialog, 4, uintptr(count), uintptr(unsafe.Pointer(specs)))
}

func win32FileDialogSetFileTypeIndex(dialog comObject, index uint32) hresult {
	return comResult(dialog, 5, uintptr(index))
}

func win32FileDialogSetOptions(dialog comObject, options uint32) hresult {
	return comResult(dialog, 9, uintptr(options))
}

func win32FileDialogGetOptions(dialog comObject, options *uint32) hresult {
	return comResult(dialog, 10, uintptr(unsafe.Pointer(options)))
}

func win32FileDialogSetFolder(dialog, folder comObject) hresult {
	return comResult(dialog, 12, uintptr(folder))
}

func win32FileDialogSetFileName(dialog comObject, name string) hresult {
	return comResult(dialog, 15, uintptr(unsafe.Pointer(wide(name))))
}

func win32FileDialogSetTitle(dialog comObject, title string) hresult {
	return comResult(dialog, 17, uintptr(unsafe.Pointer(wide(title))))
}

func win32FileDialogGetResult(dialog comObject, item *comObject) hresult {
	return comResult(dialog, 20, uintptr(unsafe.Pointer(item)))
}

func win32FileDialogSetDefaultExtension(dialog comObject, extension string) hresult {
	return comResult(dialog, 22, uintptr(unsafe.Pointer(wide(extension))))
}

func win32FileOpenDialogGetResults(dialog comObject, items *comObject) hresult {
	return comResult(dialog, 27, uintptr(unsafe.Pointer(items)))
}

// IShellItem and IShellItemArray methods.

// win32ShellItemGetDisplayName returns a name that the caller frees with
// CoTaskMemFree.
func win32ShellItemGetDisplayName(item comObject, form uint32, name *uintptr) hresult {
	return comResult(item, 5, uintptr(form), uintptr(unsafe.Pointer(name)))
}

func win32ShellItemArrayGetCount(items comObject, count *uint32) hresult {
	return comResult(items, 7, uintptr(unsafe.Pointer(count)))
}

func win32ShellItemArrayGetItemAt(items comObject, index uint32, item *comObject) hresult {
	return comResult(items, 8, uintptr(index), uintptr(unsafe.Pointer(item)))
}

// ---- oleacc ----

// IAccPropServices methods. A MSAAPROPID is a GUID that the methods take by
// value.

func win32AccPropServicesSetHwndPropStr(services comObject, h hwnd, object, child int32, property *guid,
	value string) hresult {
	args := []uintptr{uintptr(h), uintptr(uint32(object)), uintptr(uint32(child))}
	args = append(args, guidArgs(property)...)
	text := wide(value)
	args = append(args, uintptr(unsafe.Pointer(text)))
	result := comResult(services, 7, args...)
	runtime.KeepAlive(text)
	return result
}

func win32AccPropServicesClearHwndProps(services comObject, h hwnd, object, child int32, properties *guid,
	count int32) hresult {
	return comResult(services, 9, uintptr(h), uintptr(uint32(object)), uintptr(uint32(child)),
		uintptr(unsafe.Pointer(properties)), uintptr(count))
}

// ---- Direct2D ----

var procD2D1CreateFactory = d2d1.proc("D2D1CreateFactory")

func win32D2D1CreateFactory(factoryType uint32, iid *guid, options unsafe.Pointer, factory *comObject) hresult {
	return hresult(procD2D1CreateFactory.call(uintptr(factoryType), uintptr(unsafe.Pointer(iid)), uintptr(options),
		uintptr(unsafe.Pointer(factory))))
}

// ID2D1Factory methods.

func win32D2D1FactoryCreatePathGeometry(factory comObject, path *comObject) hresult {
	return comResult(factory, 10, uintptr(unsafe.Pointer(path)))
}

func win32D2D1FactoryCreateStrokeStyle(factory comObject, properties *d2d1StrokeStyleProperties, dashes *float32,
	count uint32, style *comObject) hresult {
	return comResult(factory, 11, uintptr(unsafe.Pointer(properties)), uintptr(unsafe.Pointer(dashes)),
		uintptr(count), uintptr(unsafe.Pointer(style)))
}

func win32D2D1FactoryCreateDCRenderTarget(factory comObject, properties *d2d1RenderTargetProperties,
	target *comObject) hresult {
	return comResult(factory, 16, uintptr(unsafe.Pointer(properties)), uintptr(unsafe.Pointer(target)))
}

// ID2D1PathGeometry, ID2D1SimplifiedGeometrySink, and ID2D1GeometrySink
// methods.

func win32D2D1PathGeometryOpen(path comObject, sink *comObject) hresult {
	return comResult(path, 17, uintptr(unsafe.Pointer(sink)))
}

func win32D2D1SimplifiedGeometrySinkBeginFigure(sink comObject, start d2d1Point2F, begin uint32) {
	call := comFloatCall(sink)
	call.point(start.x, start.y)
	call.arg(uintptr(begin))
	call.call(comMethod(sink, 5))
}

func win32D2D1SimplifiedGeometrySinkEndFigure(sink comObject, end uint32) {
	comCall(sink, 8, uintptr(end))
}

func win32D2D1SimplifiedGeometrySinkClose(sink comObject) hresult {
	return comResult(sink, 9)
}

func win32D2D1GeometrySinkAddArc(sink comObject, arc *d2d1ArcSegment) {
	comCall(sink, 14, uintptr(unsafe.Pointer(arc)))
}

// ID2D1RenderTarget and ID2D1DCRenderTarget methods.

func win32D2D1RenderTargetCreateBitmap(target comObject, width, height uint32, source unsafe.Pointer, pitch uint32,
	properties *d2d1BitmapProperties, bitmap *comObject) hresult {
	call := comFloatCall(target)
	call.pair(width, height)
	call.pointerArg(source)
	call.arg(uintptr(pitch))
	call.pointerArg(unsafe.Pointer(properties))
	call.pointerArg(unsafe.Pointer(bitmap))
	return hresult(uint32(call.call(comMethod(target, 4))))
}

func win32D2D1RenderTargetCreateSolidColorBrush(target comObject, color *d2d1ColorF, properties unsafe.Pointer,
	brush *comObject) hresult {
	return comResult(target, 8, uintptr(unsafe.Pointer(color)), uintptr(properties), uintptr(unsafe.Pointer(brush)))
}

func win32D2D1RenderTargetDrawLine(target comObject, from, to d2d1Point2F, brush comObject, width float32,
	style comObject) {
	call := comFloatCall(target)
	call.point(from.x, from.y)
	call.point(to.x, to.y)
	call.arg(uintptr(brush))
	call.float(width)
	call.arg(uintptr(style))
	call.call(comMethod(target, 15))
}

func win32D2D1RenderTargetFillRectangle(target comObject, area *d2d1RectF, brush comObject) {
	comCall(target, 17, uintptr(unsafe.Pointer(area)), uintptr(brush))
}

func win32D2D1RenderTargetDrawRoundedRectangle(target comObject, rounded *d2d1RoundedRect, brush comObject,
	width float32, style comObject) {
	call := comFloatCall(target)
	call.pointerArg(unsafe.Pointer(rounded))
	call.arg(uintptr(brush))
	call.float(width)
	call.arg(uintptr(style))
	call.call(comMethod(target, 18))
}

func win32D2D1RenderTargetFillRoundedRectangle(target comObject, rounded *d2d1RoundedRect, brush comObject) {
	comCall(target, 19, uintptr(unsafe.Pointer(rounded)), uintptr(brush))
}

func win32D2D1RenderTargetDrawEllipse(target comObject, ellipse *d2d1Ellipse, brush comObject, width float32,
	style comObject) {
	call := comFloatCall(target)
	call.pointerArg(unsafe.Pointer(ellipse))
	call.arg(uintptr(brush))
	call.float(width)
	call.arg(uintptr(style))
	call.call(comMethod(target, 20))
}

func win32D2D1RenderTargetDrawGeometry(target, geometry, brush comObject, width float32, style comObject) {
	call := comFloatCall(target)
	call.arg(uintptr(geometry))
	call.arg(uintptr(brush))
	call.float(width)
	call.arg(uintptr(style))
	call.call(comMethod(target, 22))
}

func win32D2D1RenderTargetDrawBitmap(target, bitmap comObject, destination *d2d1RectF, opacity float32,
	mode uint32, source *d2d1RectF) {
	call := comFloatCall(target)
	call.arg(uintptr(bitmap))
	call.pointerArg(unsafe.Pointer(destination))
	call.float(opacity)
	call.arg(uintptr(mode))
	call.pointerArg(unsafe.Pointer(source))
	call.call(comMethod(target, 26))
}

func win32D2D1RenderTargetDrawTextLayout(target comObject, origin d2d1Point2F, layout, brush comObject,
	options uint32) {
	call := comFloatCall(target)
	call.point(origin.x, origin.y)
	call.arg(uintptr(layout))
	call.arg(uintptr(brush))
	call.arg(uintptr(options))
	call.call(comMethod(target, 28))
}

func win32D2D1RenderTargetSetTextAntialiasMode(target comObject, mode uint32) {
	comCall(target, 34, uintptr(mode))
}

func win32D2D1RenderTargetPushAxisAlignedClip(target comObject, area *d2d1RectF, mode uint32) {
	comCall(target, 45, uintptr(unsafe.Pointer(area)), uintptr(mode))
}

func win32D2D1RenderTargetPopAxisAlignedClip(target comObject) {
	comCall(target, 46)
}

func win32D2D1RenderTargetClear(target comObject, color *d2d1ColorF) {
	comCall(target, 47, uintptr(unsafe.Pointer(color)))
}

func win32D2D1RenderTargetBeginDraw(target comObject) {
	comCall(target, 48)
}

func win32D2D1RenderTargetEndDraw(target comObject, tag1, tag2 *uint64) hresult {
	return comResult(target, 49, uintptr(unsafe.Pointer(tag1)), uintptr(unsafe.Pointer(tag2)))
}

func win32D2D1RenderTargetSetDpi(target comObject, x, y float32) {
	call := comFloatCall(target)
	call.float(x)
	call.float(y)
	call.call(comMethod(target, 51))
}

func win32D2D1DCRenderTargetBindDC(target comObject, dc hdc, area *rect) hresult {
	return comResult(target, 57, uintptr(dc), uintptr(unsafe.Pointer(area)))
}

// ID2D1SolidColorBrush methods.

func win32D2D1SolidColorBrushSetColor(brush comObject, color *d2d1ColorF) {
	comCall(brush, 8, uintptr(unsafe.Pointer(color)))
}

// ---- DirectWrite ----

var procDWriteCreateFactory = dwrite.proc("DWriteCreateFactory")

func win32DWriteCreateFactory(factoryType uint32, iid *guid, factory *comObject) hresult {
	return hresult(procDWriteCreateFactory.call(uintptr(factoryType), uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(factory))))
}

// IDWriteFactory methods.

func win32DWriteFactoryGetSystemFontCollection(factory comObject, collection *comObject, check bool) hresult {
	return comResult(factory, 3, uintptr(unsafe.Pointer(collection)), boolArg(check))
}

func win32DWriteFactoryCreateTextFormat(factory comObject, family string, collection comObject, weight, style,
	stretch uint32, size float32, locale string, format *comObject) hresult {
	familyText, localeText := wide(family), wide(locale)
	call := comFloatCall(factory)
	call.pointerArg(unsafe.Pointer(familyText))
	call.arg(uintptr(collection))
	call.arg(uintptr(weight))
	call.arg(uintptr(style))
	call.arg(uintptr(stretch))
	call.float(size)
	call.pointerArg(unsafe.Pointer(localeText))
	call.pointerArg(unsafe.Pointer(format))
	return hresult(uint32(call.call(comMethod(factory, 15))))
}

// win32DWriteFactoryCreateTextLayout lays out text, which it converts to
// UTF-16.
func win32DWriteFactoryCreateTextLayout(factory comObject, text string, format comObject, width, height float32,
	layout *comObject) hresult {
	units := utf16.Encode([]rune(text))
	units = append(units, 0)
	call := comFloatCall(factory)
	call.pointerArg(unsafe.Pointer(&units[0]))
	call.arg(uintptr(len(units) - 1))
	call.arg(uintptr(format))
	call.float(width)
	call.float(height)
	call.pointerArg(unsafe.Pointer(layout))
	return hresult(uint32(call.call(comMethod(factory, 18))))
}

func win32DWriteFactoryCreateEllipsisTrimmingSign(factory, format comObject, sign *comObject) hresult {
	return comResult(factory, 20, uintptr(format), uintptr(unsafe.Pointer(sign)))
}

// IDWriteFontCollection methods.

func win32DWriteFontCollectionFindFamilyName(collection comObject, name string, index *uint32,
	exists *int32) hresult {
	return comResult(collection, 5, uintptr(unsafe.Pointer(wide(name))), uintptr(unsafe.Pointer(index)),
		uintptr(unsafe.Pointer(exists)))
}

// IDWriteTextFormat methods, which IDWriteTextLayout inherits.

func win32DWriteTextFormatSetTextAlignment(format comObject, alignment uint32) hresult {
	return comResult(format, 3, uintptr(alignment))
}

func win32DWriteTextFormatSetParagraphAlignment(format comObject, alignment uint32) hresult {
	return comResult(format, 4, uintptr(alignment))
}

func win32DWriteTextFormatSetWordWrapping(format comObject, wrapping uint32) hresult {
	return comResult(format, 5, uintptr(wrapping))
}

func win32DWriteTextFormatSetTrimming(format comObject, trimming *dwriteTrimming, sign comObject) hresult {
	return comResult(format, 9, uintptr(unsafe.Pointer(trimming)), uintptr(sign))
}

// IDWriteTextLayout methods.

func win32DWriteTextLayoutGetMetrics(layout comObject, metrics *dwriteTextMetrics) hresult {
	return comResult(layout, 60, uintptr(unsafe.Pointer(metrics)))
}

func win32DWriteTextLayoutGetOverhangMetrics(layout comObject, metrics *dwriteOverhangMetrics) hresult {
	return comResult(layout, 61, uintptr(unsafe.Pointer(metrics)))
}

// ---- Windows Imaging Component ----

// IWICImagingFactory methods.

func win32WICImagingFactoryCreateDecoderFromStream(factory, stream comObject, vendor *guid, options uint32,
	decoder *comObject) hresult {
	return comResult(factory, 4, uintptr(stream), uintptr(unsafe.Pointer(vendor)), uintptr(options),
		uintptr(unsafe.Pointer(decoder)))
}

func win32WICImagingFactoryCreateEncoder(factory comObject, container, vendor *guid, encoder *comObject) hresult {
	return comResult(factory, 8, uintptr(unsafe.Pointer(container)), uintptr(unsafe.Pointer(vendor)),
		uintptr(unsafe.Pointer(encoder)))
}

func win32WICImagingFactoryCreateFormatConverter(factory comObject, converter *comObject) hresult {
	return comResult(factory, 10, uintptr(unsafe.Pointer(converter)))
}

func win32WICImagingFactoryCreateStream(factory comObject, stream *comObject) hresult {
	return comResult(factory, 14, uintptr(unsafe.Pointer(stream)))
}

// IWICStream methods. The stream reads data in place, so the caller keeps it
// alive while the stream is in use.

func win32WICStreamInitializeFromMemory(stream comObject, data []byte) hresult {
	return comResult(stream, 16, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
}

// IWICBitmapDecoder methods.

func win32WICBitmapDecoderGetFrame(decoder comObject, index uint32, frame *comObject) hresult {
	return comResult(decoder, 13, uintptr(index), uintptr(unsafe.Pointer(frame)))
}

// IWICBitmapSource methods, which IWICFormatConverter inherits.

func win32WICBitmapSourceGetSize(source comObject, width, height *uint32) hresult {
	return comResult(source, 3, uintptr(unsafe.Pointer(width)), uintptr(unsafe.Pointer(height)))
}

func win32WICBitmapSourceCopyPixels(source comObject, area unsafe.Pointer, stride uint32, buffer []byte) hresult {
	return comResult(source, 7, uintptr(area), uintptr(stride), uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0])))
}

// IWICFormatConverter methods.

func win32WICFormatConverterInitialize(converter, source comObject, format *guid, dither uint32, palette comObject,
	alphaThreshold float64, translate uint32) hresult {
	call := comFloatCall(converter)
	call.arg(uintptr(source))
	call.pointerArg(unsafe.Pointer(format))
	call.arg(uintptr(dither))
	call.arg(uintptr(palette))
	call.double(alphaThreshold)
	call.arg(uintptr(translate))
	return hresult(uint32(call.call(comMethod(converter, 8))))
}

// IWICBitmapEncoder methods.

func win32WICBitmapEncoderInitialize(encoder, stream comObject, cache uint32) hresult {
	return comResult(encoder, 3, uintptr(stream), uintptr(cache))
}

func win32WICBitmapEncoderCreateNewFrame(encoder comObject, frame, options *comObject) hresult {
	return comResult(encoder, 10, uintptr(unsafe.Pointer(frame)), uintptr(unsafe.Pointer(options)))
}

func win32WICBitmapEncoderCommit(encoder comObject) hresult {
	return comResult(encoder, 11)
}

// IWICBitmapFrameEncode methods.

func win32WICBitmapFrameEncodeInitialize(frame, options comObject) hresult {
	return comResult(frame, 3, uintptr(options))
}

func win32WICBitmapFrameEncodeSetSize(frame comObject, width, height uint32) hresult {
	return comResult(frame, 4, uintptr(width), uintptr(height))
}

func win32WICBitmapFrameEncodeSetPixelFormat(frame comObject, format *guid) hresult {
	return comResult(frame, 6, uintptr(unsafe.Pointer(format)))
}

func win32WICBitmapFrameEncodeWritePixels(frame comObject, lines, stride uint32, pixels []byte) hresult {
	return comResult(frame, 10, uintptr(lines), uintptr(stride), uintptr(len(pixels)), uintptr(unsafe.Pointer(&pixels[0])))
}

func win32WICBitmapFrameEncodeCommit(frame comObject) hresult {
	return comResult(frame, 12)
}

// ---- Menus, bitmaps, icons, and tab controls ----

type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
	bmiColors [1]uint32
}

type bitmapV5Header struct {
	bV5Size          uint32
	bV5Width         int32
	bV5Height        int32
	bV5Planes        uint16
	bV5BitCount      uint16
	bV5Compression   uint32
	bV5SizeImage     uint32
	bV5XPelsPerMeter int32
	bV5YPelsPerMeter int32
	bV5ClrUsed       uint32
	bV5ClrImportant  uint32
	bV5RedMask       uint32
	bV5GreenMask     uint32
	bV5BlueMask      uint32
	bV5AlphaMask     uint32
	bV5CSType        uint32
	bV5Endpoints     [36]byte
	bV5GammaRed      uint32
	bV5GammaGreen    uint32
	bV5GammaBlue     uint32
	bV5Intent        uint32
	bV5ProfileData   uint32
	bV5ProfileSize   uint32
	bV5Reserved      uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  hbitmap
	hbmColor hbitmap
}

// tcItem is TCITEMW.
type tcItem struct {
	mask        uint32
	dwState     uint32
	dwStateMask uint32
	pszText     *uint16
	cchTextMax  int32
	iImage      int32
	lParam      uintptr
}

const (
	dibRGBColors       = 0
	fwNormal           = 400
	antialiasedQuality = 4

	mfString    = 0x00000000
	mfChecked   = 0x00000008
	mfPopup     = 0x00000010
	mfSeparator = 0x00000800
	mftString   = 0x00000000
	mfsEnabled  = 0x00000000
	mfsDisabled = 0x00000003
	mfsChecked  = 0x00000008
	miimState   = 0x00000001
	miimID      = 0x00000002
	miimBitmap  = 0x00000080
	miimFType   = 0x00000100

	tpmRightButton = 0x0002
	tpmRightAlign  = 0x0008
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	wcListBox           = "ListBox"
	wcTabControl        = "SysTabControl32"
	lbsNotify           = 0x0001
	lbsNoIntegralHeight = 0x0100
	lbAddString         = 0x0180
	lbSetCurSel         = 0x0186
	lbGetCurSel         = 0x0188
	lbnSelChange        = 1

	tcsBottom      = 0x0002
	tcifText       = 0x0001
	tcmGetItemRect = 0x130a
	tcmGetCurSel   = 0x130b
	tcmSetCurSel   = 0x130c
	tcmAdjustRect  = 0x1328
	tcmInsertItem  = 0x133e
	tcnSelChange   = 0xfffffdd9
)

var hwndBottom = hwnd(1)

var (
	procAppendMenu         = user32.proc("AppendMenuW")
	procCreateIconIndirect = user32.proc("CreateIconIndirect")
	procCreateMenu         = user32.proc("CreateMenu")
	procCreatePopupMenu    = user32.proc("CreatePopupMenu")
	procDestroyMenu        = user32.proc("DestroyMenu")
	procDrawMenuBar        = user32.proc("DrawMenuBar")
	procGetMenuItemCount   = user32.proc("GetMenuItemCount")
	procInsertMenuItem     = user32.proc("InsertMenuItemW")
	procTrackPopupMenuEx   = user32.proc("TrackPopupMenuEx")
	procCreateBitmap       = gdi32.proc("CreateBitmap")
	procCreateDIBSection   = gdi32.proc("CreateDIBSection")
	procDeleteDC           = gdi32.proc("DeleteDC")
	procGdiFlush           = gdi32.proc("GdiFlush")
)

// win32AppendMenu appends an item; "" passes no text.
func win32AppendMenu(menu hmenu, flags uint32, id uintptr, text string) bool {
	return procAppendMenu.call(uintptr(menu), uintptr(flags), id, uintptr(unsafe.Pointer(wideOrNil(text)))) != 0
}

func win32CreateIconIndirect(info *iconInfo) hicon {
	return hicon(procCreateIconIndirect.call(uintptr(unsafe.Pointer(info))))
}

func win32CreateMenu() hmenu {
	return hmenu(procCreateMenu.call())
}

func win32CreatePopupMenu() hmenu {
	return hmenu(procCreatePopupMenu.call())
}

func win32DestroyMenu(menu hmenu) bool {
	return procDestroyMenu.call(uintptr(menu)) != 0
}

func win32DrawMenuBar(h hwnd) bool {
	return procDrawMenuBar.call(uintptr(h)) != 0
}

func win32GetMenuItemCount(menu hmenu) int32 {
	return int32(procGetMenuItemCount.call(uintptr(menu)))
}

func win32InsertMenuItem(menu hmenu, item uint32, byPosition bool, info *menuItemInfo) bool {
	return procInsertMenuItem.call(uintptr(menu), uintptr(item), boolArg(byPosition), uintptr(unsafe.Pointer(info))) != 0
}

// win32TrackPopupMenuEx shows a pop-up menu; with TPM_RETURNCMD it returns the
// command of the chosen item.
func win32TrackPopupMenuEx(menu hmenu, flags uint32, x, y int32, owner hwnd, params unsafe.Pointer) int32 {
	return int32(procTrackPopupMenuEx.call(uintptr(menu), uintptr(flags), uintptr(x), uintptr(y), uintptr(owner),
		uintptr(params)))
}

func win32CreateBitmap(width, height int32, planes, bitCount uint32, bits unsafe.Pointer) hbitmap {
	return hbitmap(procCreateBitmap.call(uintptr(width), uintptr(height), uintptr(planes), uintptr(bitCount),
		uintptr(bits)))
}

// win32CreateDIBSection creates a DIB section; info is a BITMAPINFO or a
// bitmap header that starts one, and bits receives the address of its pixels.
func win32CreateDIBSection(dc hdc, info unsafe.Pointer, usage uint32, bits *uintptr, section uintptr,
	offset uint32) hbitmap {
	return hbitmap(procCreateDIBSection.call(uintptr(dc), uintptr(info), uintptr(usage), uintptr(unsafe.Pointer(bits)),
		section, uintptr(offset)))
}

func win32DeleteDC(dc hdc) bool {
	return procDeleteDC.call(uintptr(dc)) != 0
}

func win32GdiFlush() bool {
	return procGdiFlush.call() != 0
}

// ---- Containers and controls ----

type textMetric struct {
	tmHeight           int32
	tmAscent           int32
	tmDescent          int32
	tmInternalLeading  int32
	tmExternalLeading  int32
	tmAveCharWidth     int32
	tmMaxCharWidth     int32
	tmWeight           int32
	tmOverhang         int32
	tmDigitizedAspectX int32
	tmDigitizedAspectY int32
	tmFirstChar        uint16
	tmLastChar         uint16
	tmDefaultChar      uint16
	tmBreakChar        uint16
	tmItalic           byte
	tmUnderlined       byte
	tmStruckOut        byte
	tmPitchAndFamily   byte
	tmCharSet          byte
}

// sizeInfo is SIZE, which takes the suffix Info because size names many
// variables.
type sizeInfo struct {
	cx, cy int32
}

type tbButton struct {
	iBitmap   int32
	idCommand int32
	fsState   byte
	fsStyle   byte
	dwData    uintptr
	iString   uintptr
}

type nmUpDown struct {
	hdr    nmhdr
	iPos   int32
	iDelta int32
}

type systemTime struct {
	wYear         uint16
	wMonth        uint16
	wDayOfWeek    uint16
	wDay          uint16
	wHour         uint16
	wMinute       uint16
	wSecond       uint16
	wMilliseconds uint16
}

type nmDateTimeChange struct {
	nmhdr   nmhdr
	dwFlags uint32
	st      systemTime
}

type nmSelChange struct {
	nmhdr      nmhdr
	stSelStart systemTime
	stSelEnd   systemTime
}

type nmDayState struct {
	nmhdr       nmhdr
	stStart     systemTime
	cDayState   int32
	prgDayState *uint32
}

type mcGridInfo struct {
	cbSize    uint32
	dwPart    uint32
	dwFlags   uint32
	iCalendar int32
	iRow      int32
	iCol      int32
	bSelected int32
	stStart   systemTime
	stEnd     systemTime
	rc        rect
	pszName   *uint16
	cchName   uintptr
}

type chooseColor struct {
	lStructSize    uint32
	hwndOwner      hwnd
	hInstance      uintptr
	rgbResult      uint32
	lpCustColors   *uint32
	Flags          uint32
	lCustData      uintptr
	lpfnHook       uintptr
	lpTemplateName *uint16
}

type chooseFont struct {
	lStructSize    uint32
	hwndOwner      hwnd
	hDC            hdc
	lpLogFont      *logFont
	iPointSize     int32
	Flags          uint32
	rgbColors      uint32
	lCustData      uintptr
	lpfnHook       uintptr
	lpTemplateName *uint16
	hInstance      uintptr
	lpszStyle      *uint16
	nFontType      uint16
	nSizeMin       int32
	nSizeMax       int32
}

const (
	wmGetFont      = 0x0031
	wmInitDialog   = 0x0110
	dtLeft         = 0x00000000
	dtTop          = 0x00000000
	bsGroupBox     = 0x00000007
	bsPushLike     = 0x00001000
	gwOwner        = 4
	gwlpHwndParent = -8
	gwlpUserData   = -21
	sFalse         = 1

	cbsAutoHScroll    = 0x0040
	esPassword        = 0x0020
	emSetPasswordChar = 0x00cc

	toolbarClassName   = "ToolbarWindow32"
	tbstyleTooltips    = 0x0100
	tbstyleFlat        = 0x0800
	tbstyleCustomErase = 0x2000
	ccsNoResize        = 0x00000004
	ccsNoParentAlign   = 0x00000008
	ccsNoDivider       = 0x00000040
	tbIsButtonChecked  = wmUser + 10
	tbButtonStructSize = wmUser + 30
	tbSetButtonSize    = wmUser + 31
	tbSetImageList     = wmUser + 48
	tbAddButtons       = wmUser + 68
	tbstateEnabled     = 0x04
	btnsCheck          = 0x0002
	cddsPreErase       = 0x00000003

	acoAutoSuggest        = 0x00000001
	acoUpDownKeyDropsList = 0x00000020

	updownClass   = "msctls_updown32"
	udsArrowKeys  = 0x0020
	udsHotTrack   = 0x0100
	udmSetRange32 = wmUser + 111
	udnDeltaPos   = 0xfffffd2e

	tkpTrack    = 1
	tkpThumb    = 3
	trsNormal   = 1
	tusNormal   = 1
	tusPressed  = 3
	tusFocused  = 4
	tusDisabled = 5
	tbmSetRange = wmUser + 6

	monthCalClass             = "SysMonthCal32"
	dateTimePickClass         = "SysDateTimePick32"
	localeSTimeFormat         = 0x00001003
	dtsUpDown                 = 0x0001
	dtsTimeFormat             = 0x0009
	dtsShortDateCenturyFormat = 0x000c
	mcsDayState               = 0x0001
	mcsMultiSelect            = 0x0002
	mcsNoTodayCircle          = 0x0008
	gdtValid                  = 0
	gdtrMin                   = 0x0001
	gdtrMax                   = 0x0002
	gmrVisible                = 0
	gmrDayState               = 1
	dtmSetSystemTime          = 0x1002
	dtmSetRange               = 0x1004
	dtmSetFormat              = 0x1032
	mcmSetCurSel              = 0x1002
	mcmSetMaxSelCount         = 0x1004
	mcmSetSelRange            = 0x1006
	mcmGetMonthRange          = 0x1007
	mcmSetDayState            = 0x1008
	mcmGetMinReqRect          = 0x1009
	mcmSetRange               = 0x1012
	mcmGetCalendarGridInfo    = 0x1018
	mcgifRect                 = 0x00000002
	mcgipNext                 = 1
	mcgipPrev                 = 2
	dtnDateTimeChange         = 0xfffffd09
	mcnSelChange              = 0xfffffd13
	mcnGetDayState            = 0xfffffd19
	mcnSelect                 = 0xfffffd16

	ccRGBInit    = 0x00000001
	ccFullOpen   = 0x00000002
	ccEnableHook = 0x00000010
	ccAnyColor   = 0x00000100

	cfScreenFonts         = 0x00000001
	cfInitToLogFontStruct = 0x00000040
	cfNoScriptSel         = 0x00800000
	cfNoVertFonts         = 0x01000000
	logPixelsY            = 90
	mapvkVkToChar         = 2
)

var (
	iidEnumString     = newGUID(0x00000101, 0x0000, 0x0000, 0xc000, 0x000000000046)
	clsidAutoComplete = newGUID(0x00bb2763, 0x6a77, 0x11d0, 0xa535, 0x00c04fd7d062)
	iidAutoComplete2  = newGUID(0xeac04bc0, 0x3791, 0x11d2, 0xbb95, 0x0060977b464c)
)

var (
	comdlg32 = newLibrary("comdlg32.dll")

	procDrawFocusRect        = user32.proc("DrawFocusRect")
	procInflateRect          = user32.proc("InflateRect")
	procMapDialogRect        = user32.proc("MapDialogRect")
	procGetLocalTime         = kernel32.proc("GetLocalTime")
	procMulDiv               = kernel32.proc("MulDiv")
	procGetDeviceCaps        = gdi32.proc("GetDeviceCaps")
	procGetTextExtentPoint32 = gdi32.proc("GetTextExtentPoint32W")
	procGetTextMetrics       = gdi32.proc("GetTextMetricsW")
	procCoTaskMemAlloc       = ole32.proc("CoTaskMemAlloc")
	procChooseColor          = comdlg32.proc("ChooseColorW")
	procChooseFont           = comdlg32.proc("ChooseFontW")
)

func win32DrawFocusRect(dc hdc, r *rect) bool {
	return procDrawFocusRect.call(uintptr(dc), uintptr(unsafe.Pointer(r))) != 0
}

func win32InflateRect(r *rect, dx, dy int32) bool {
	return procInflateRect.call(uintptr(unsafe.Pointer(r)), uintptr(dx), uintptr(dy)) != 0
}

func win32MapDialogRect(dialog hwnd, r *rect) bool {
	return procMapDialogRect.call(uintptr(dialog), uintptr(unsafe.Pointer(r))) != 0
}

func win32GetLocalTime(t *systemTime) {
	procGetLocalTime.call(uintptr(unsafe.Pointer(t)))
}

func win32MulDiv(number, numerator, denominator int32) int32 {
	return int32(procMulDiv.call(uintptr(number), uintptr(numerator), uintptr(denominator)))
}

func win32GetDeviceCaps(dc hdc, index int32) int32 {
	return int32(procGetDeviceCaps.call(uintptr(dc), uintptr(index)))
}

func win32GetTextExtentPoint32(dc hdc, text *uint16, count int32, size *sizeInfo) bool {
	return procGetTextExtentPoint32.call(uintptr(dc), uintptr(unsafe.Pointer(text)), uintptr(count),
		uintptr(unsafe.Pointer(size))) != 0
}

func win32GetTextMetrics(dc hdc, metrics *textMetric) bool {
	return procGetTextMetrics.call(uintptr(dc), uintptr(unsafe.Pointer(metrics))) != 0
}

// win32CoTaskMemAlloc returns the address of the memory.
func win32CoTaskMemAlloc(size uintptr) uintptr {
	return procCoTaskMemAlloc.call(size)
}

func win32ChooseColor(choose *chooseColor) bool {
	return procChooseColor.call(uintptr(unsafe.Pointer(choose))) != 0
}

func win32ChooseFont(choose *chooseFont) bool {
	return procChooseFont.call(uintptr(unsafe.Pointer(choose))) != 0
}

// IAutoComplete and IAutoComplete2 methods.

func win32AutoCompleteInit(complete comObject, edit hwnd, source comObject, registryKey, quickComplete string) hresult {
	return comResult(complete, 3, uintptr(edit), uintptr(source), uintptr(unsafe.Pointer(wideOrNil(registryKey))),
		uintptr(unsafe.Pointer(wideOrNil(quickComplete))))
}

func win32AutoComplete2SetOptions(complete comObject, options uint32) hresult {
	return comResult(complete, 5, uintptr(options))
}

// ---- Task dialogs ----

// taskDialogConfig holds the fields of a TASKDIALOGCONFIG that the package
// sets. Windows declares the struct with 1-byte packing, which a Go struct
// cannot express, so win32TaskDialogIndirect packs it.
type taskDialogConfig struct {
	hwndParent             hwnd
	hInstance              hinstance
	dwFlags                uint32
	pszWindowTitle         string
	mainIcon               uintptr
	pszMainInstruction     string
	pszContent             string
	buttons                []taskDialogButton
	nDefaultButton         int32
	pszExpandedInformation string
	pfCallback             uintptr
	lpCallbackData         uintptr
}

// taskDialogButton is a TASKDIALOG_BUTTON, which is also packed.
type taskDialogButton struct {
	nButtonID     int32
	pszButtonText string
}

const (
	idOK                        = 1
	idCancel                    = 2
	tdfEnableHyperlinks         = 0x0001
	tdfUseHIconMain             = 0x0002
	tdfAllowDialogCancellation  = 0x0008
	tdfPositionRelativeToWindow = 0x1000
	tdfSizeToContent            = 0x01000000
	tdnCreated                  = 0
	tdnHyperlinkClicked         = 3
	tdnDestroyed                = 5
	tdmClickButton              = wmUser + 102
	tdWarningIcon               = 0xffff
	tdErrorIcon                 = 0xfffe
	tdInformationIcon           = 0xfffd

	coinitApartmentThreaded = 0x2
	gwlpID                  = -12
	wsExDlgModalFrame       = 0x00000001
	ssIcon                  = 0x00000003
	ssRealSizeControl       = 0x00000040
	stmSetIcon              = 0x0170
	limLarge                = 1
)

// packed builds a struct with 1-byte packing.
type packed struct {
	data []byte
}

func (p *packed) u32(value uint32) {
	p.data = append(p.data, byte(value), byte(value>>8), byte(value>>16), byte(value>>24))
}

func (p *packed) pointerSized(value uintptr) {
	p.u32(uint32(value))
	if unsafe.Sizeof(value) == 8 {
		p.u32(uint32(uint64(value) >> 32))
	}
}

func win32TaskDialogIndirect(config *taskDialogConfig, button *int32) hresult {
	// keep holds the strings and the buttons that the packed struct points
	// at until the dialog closes.
	var keep []any
	text := func(value string) uintptr {
		if value == "" {
			return 0
		}
		units := wideSlice(value)
		keep = append(keep, units)
		return uintptr(unsafe.Pointer(&units[0]))
	}
	var buttons packed
	for _, b := range config.buttons {
		buttons.u32(uint32(b.nButtonID))
		buttons.pointerSized(text(b.pszButtonText))
	}
	var buttonsAddress uintptr
	if len(buttons.data) > 0 {
		keep = append(keep, buttons.data)
		buttonsAddress = uintptr(unsafe.Pointer(&buttons.data[0]))
	}
	var p packed
	p.u32(0) // cbSize, set below
	p.pointerSized(uintptr(config.hwndParent))
	p.pointerSized(uintptr(config.hInstance))
	p.u32(config.dwFlags)
	p.u32(0) // dwCommonButtons
	p.pointerSized(text(config.pszWindowTitle))
	p.pointerSized(config.mainIcon)
	p.pointerSized(text(config.pszMainInstruction))
	p.pointerSized(text(config.pszContent))
	p.u32(uint32(len(config.buttons)))
	p.pointerSized(buttonsAddress)
	p.u32(uint32(config.nDefaultButton))
	p.u32(0)          // cRadioButtons
	p.pointerSized(0) // pRadioButtons
	p.u32(0)          // nDefaultRadioButton
	p.pointerSized(0) // pszVerificationText
	p.pointerSized(text(config.pszExpandedInformation))
	p.pointerSized(0) // pszExpandedControlText
	p.pointerSized(0) // pszCollapsedControlText
	p.pointerSized(0) // hFooterIcon
	p.pointerSized(0) // pszFooter
	p.pointerSized(config.pfCallback)
	p.pointerSized(config.lpCallbackData)
	p.u32(0) // cxWidth
	size := uint32(len(p.data))
	p.data[0], p.data[1], p.data[2], p.data[3] = byte(size), byte(size>>8), byte(size>>16), byte(size>>24)
	result := hresult(procTaskDialogIndirect.call(uintptr(unsafe.Pointer(&p.data[0])), uintptr(unsafe.Pointer(button)),
		0, 0))
	runtime.KeepAlive(keep)
	return result
}

var (
	procDestroyIcon    = user32.proc("DestroyIcon")
	procShellExecute   = shell32.proc("ShellExecuteW")
	procCoInitializeEx = ole32.proc("CoInitializeEx")
	procCoUninitialize = ole32.proc("CoUninitialize")
)

func win32DestroyIcon(icon hicon) bool {
	return procDestroyIcon.call(uintptr(icon)) != 0
}

// win32ShellExecute opens a file or a URL; "" passes NULL.
func win32ShellExecute(h hwnd, verb, file, parameters, directory string, show int32) uintptr {
	return procShellExecute.call(uintptr(h), uintptr(unsafe.Pointer(wideOrNil(verb))),
		uintptr(unsafe.Pointer(wideOrNil(file))), uintptr(unsafe.Pointer(wideOrNil(parameters))),
		uintptr(unsafe.Pointer(wideOrNil(directory))), uintptr(show))
}

func win32CoInitializeEx(reserved uintptr, flags uint32) hresult {
	return hresult(procCoInitializeEx.call(reserved, uintptr(flags)))
}

func win32CoUninitialize() {
	procCoUninitialize.call()
}

// ---- List views, tree views, and headers ----

type hdItem struct {
	mask       uint32
	cxy        int32
	pszText    *uint16
	hbm        hbitmap
	cchTextMax int32
	fmt        int32
	lParam     uintptr
	iImage     int32
	iOrder     int32
	typ        uint32
	pvFilter   uintptr
	state      uint32
}

type lvColumn struct {
	mask       uint32
	fmt        int32
	cx         int32
	pszText    *uint16
	cchTextMax int32
	iSubItem   int32
	iImage     int32
	iOrder     int32
	cxMin      int32
	cxDefault  int32
	cxIdeal    int32
}

type lvItem struct {
	mask       uint32
	iItem      int32
	iSubItem   int32
	state      uint32
	stateMask  uint32
	pszText    *uint16
	cchTextMax int32
	iImage     int32
	lParam     uintptr
	iIndent    int32
	iGroupId   int32
	cColumns   uint32
	puColumns  *uint32
	piColFmt   *int32
	iGroup     int32
}

type nmListView struct {
	hdr       nmhdr
	iItem     int32
	iSubItem  int32
	uNewState uint32
	uOldState uint32
	uChanged  uint32
	ptAction  point
	lParam    uintptr
}

type nmItemActivate struct {
	hdr       nmhdr
	iItem     int32
	iSubItem  int32
	uNewState uint32
	uOldState uint32
	uChanged  uint32
	ptAction  point
	lParam    uintptr
	uKeyFlags uint32
}

type nmLVCustomDraw struct {
	nmcd        nmCustomDrawInfo
	clrText     uint32
	clrTextBk   uint32
	iSubItem    int32
	dwItemType  uint32
	clrFace     uint32
	iIconEffect int32
	iIconPhase  int32
	iPartId     int32
	iStateId    int32
	rcText      rect
	uAlign      uint32
}

type tvItem struct {
	mask           uint32
	hItem          uintptr
	state          uint32
	stateMask      uint32
	pszText        *uint16
	cchTextMax     int32
	iImage         int32
	iSelectedImage int32
	cChildren      int32
	lParam         uintptr
}

// tvItemEx is TVITEMEXW, the largest item of a TVINSERTSTRUCTW.
type tvItemEx struct {
	tvItem
	iIntegral      int32
	uStateEx       uint32
	hwnd           hwnd
	iExpandedImage int32
	iReserved      int32
}

type tvInsertStruct struct {
	hParent      uintptr
	hInsertAfter uintptr
	item         tvItemEx
}

type nmTreeView struct {
	hdr     nmhdr
	action  uint32
	itemOld tvItem
	itemNew tvItem
	ptDrag  point
}

type nmTVCustomDraw struct {
	nmcd      nmCustomDrawInfo
	clrText   uint32
	clrTextBk uint32
	iLevel    int32
}

type nmLink struct {
	hdr  nmhdr
	item lItem
}

const (
	wmSetRedraw = 0x000b
	dtBottom    = 0x00000008
	iImageNone  = -2

	bsAutoRadioButton = 0x00000009
	lwsNoPrefix       = 0x0004

	cdrfNewFont           = 0x00000002
	cdrfNotifyPostPaint   = 0x00000010
	cdrfNotifyItemDraw    = 0x00000020
	cdrfNotifySubItemDraw = 0x00000020
	cddsItemPrePaint      = 0x00010001
	cddsItemPostPaint     = 0x00010002
	cddsSubItem           = 0x00020000

	lvsIcon                     = 0x0000
	lvsReport                   = 0x0001
	lvsSingleSel                = 0x0004
	lvsShowSelAlways            = 0x0008
	lvsAutoArrange              = 0x0100
	lvsNoColumnHeader           = 0x4000
	lvsExHeaderDragDrop         = 0x00000010
	lvsExFullRowSelect          = 0x00000020
	lvsExInfoTip                = 0x00000400
	lvsExDoubleBuffer           = 0x00010000
	lvcfWidth                   = 0x0002
	lvcfText                    = 0x0004
	lvcfSubItem                 = 0x0008
	lvifText                    = 0x00000001
	lvifImage                   = 0x00000002
	lvifState                   = 0x00000008
	lvisFocused                 = 0x0001
	lvisSelected                = 0x0002
	lvniFocused                 = 0x0001
	lvniSelected                = 0x0002
	lvsilNormal                 = 0
	lvsilSmall                  = 1
	lvscwAutoSizeUseHeader      = 0xfffffffe
	lvmSetBkColor               = 0x1001
	lvmSetImageList             = 0x1003
	lvmDeleteAllItems           = 0x1009
	lvmGetNextItem              = 0x100c
	lvmEnsureVisible            = 0x1013
	lvmSetColumnWidth           = 0x101e
	lvmGetHeader                = 0x101f
	lvmSetTextColor             = 0x1024
	lvmSetTextBkColor           = 0x1026
	lvmSetItemState             = 0x102b
	lvmSetIconSpacing           = 0x1035
	lvmSetExtendedListViewStyle = 0x1036
	lvmInsertItem               = 0x104d
	lvmInsertColumn             = 0x1061
	lvmSetItemText              = 0x1074
	lvnItemChanging             = 0xffffff9c
	lvnItemChanged              = 0xffffff9b
	lvnColumnClick              = 0xffffff94
	lvnItemActivate             = 0xffffff8e

	hdiFormat   = 0x0004
	hdfSortDown = 0x0200
	hdfSortUp   = 0x0400
	hdmGetItem  = 0x120b
	hdmSetItem  = 0x120c

	tvsHasButtons          = 0x0001
	tvsLinesAtRoot         = 0x0004
	tvsShowSelAlways       = 0x0020
	tvsTrackSelect         = 0x0200
	tvsFullRowSelect       = 0x1000
	tvsNoHScroll           = 0x8000
	tvsExDoubleBuffer      = 0x0004
	tvsExFadeInOutExpandos = 0x0040
	tvifText               = 0x0001
	tvifImage              = 0x0002
	tvifParam              = 0x0004
	tvifState              = 0x0008
	tvifHandle             = 0x0010
	tvifSelectedImage      = 0x0020
	tvifChildren           = 0x0040
	tvisSelected           = 0x0002
	tvisBold               = 0x0010
	tvisExpanded           = 0x0020
	tveCollapse            = 0x0001
	tveExpand              = 0x0002
	tvgnRoot               = 0x0000
	tvgnNext               = 0x0001
	tvgnParent             = 0x0003
	tvgnChild              = 0x0004
	tvgnFirstVisible       = 0x0005
	tvgnCaret              = 0x0009
	tvsilNormal            = 0
	tvmDeleteItem          = 0x1101
	tvmExpand              = 0x1102
	tvmGetItemRect         = 0x1104
	tvmSetImageList        = 0x1109
	tvmGetNextItem         = 0x110a
	tvmSelectItem          = 0x110b
	tvmEnsureVisible       = 0x1114
	tvmSetItemHeight       = 0x111b
	tvmSetBkColor          = 0x111d
	tvmSetTextColor        = 0x111e
	tvmGetBkColor          = 0x111f
	tvmGetItemState        = 0x1127
	tvmSetExtendedStyle    = 0x112c
	tvmInsertItem          = 0x1132
	tvmGetItem             = 0x113e
	tvmSetItem             = 0x113f
	tvnSelChanging         = 0xfffffe3e
	tvnSelChanged          = 0xfffffe3d
	tvnItemExpanding       = 0xfffffe3a
)

var (
	tviRoot = signExtend(0xffff0000)
	tviLast = signExtend(0xffff0002)
)

// ---- Toolbars and status bars ----

type tbMetrics struct {
	cbSize          uint32
	dwMask          uint32
	cxPad           int32
	cyPad           int32
	cxBarPad        int32
	cyBarPad        int32
	cxButtonSpacing int32
	cyButtonSpacing int32
}

// tbButtonInfo is TBBUTTONINFOW.
type tbButtonInfo struct {
	cbSize    uint32
	dwMask    uint32
	idCommand int32
	iImage    int32
	fsState   byte
	fsStyle   byte
	cx        uint16
	lParam    uintptr
	pszText   *uint16
	cchText   int32
}

const (
	mfGrayed = 0x00000001

	tbstyleList           = 0x1000
	tbstyleExMixedButtons = 0x00000008
	tbstyleExDoubleBuffer = 0x00000080
	tbmfPad               = 0x00000001
	tbmfButtonSpacing     = 0x00000004
	btnsButton            = 0x00
	btnsSep               = 0x01
	btnsAutoSize          = 0x10
	btnsShowText          = 0x40
	tbifState             = 0x00000004
	tbifSize              = 0x00000040
	tbstateHidden         = 0x08
	tbEnableButton        = wmUser + 1
	tbHideButton          = wmUser + 4
	tbDeleteButton        = wmUser + 22
	tbButtonCount         = wmUser + 24
	tbGetItemRect         = wmUser + 29
	tbSetBitmapSize       = wmUser + 32
	tbAutoSize            = wmUser + 33
	tbSetStyle            = wmUser + 56
	tbGetStyle            = wmUser + 57
	tbGetButtonSize       = wmUser + 58
	tbGetButtonInfo       = wmUser + 63
	tbSetButtonInfo       = wmUser + 64
	tbGetMaxSize          = wmUser + 83
	tbSetExtendedStyle    = wmUser + 84
	tbSetPadding          = wmUser + 87
	tbSetMetrics          = wmUser + 102

	statusClassName = "msctls_statusbar32"
	sbarsTooltips   = 0x0800
	sbSetText       = wmUser + 11
	sbtNoBorders    = 0x0100
)

// ---- Feedback ----

// variant is a VARIANT; data is its union, whose first bytes hold a value such
// as the lVal of VT_I4.
type variant struct {
	vt         uint16
	wReserved1 uint16
	wReserved2 uint16
	wReserved3 uint16
	data       [2]uintptr
}

const (
	vtI4                         = 3
	eventObjectLiveRegionChanged = 0x8019
	ttfTrack                     = 0x0020
	ttfAbsolute                  = 0x0080
	ttmTrackActivate             = wmUser + 17
	ttmTrackPosition             = wmUser + 18
	ttmUpdateTipText             = wmUser + 57
	progdlgNoTime                = 0x00000004
	progdlgNoMinimize            = 0x00000008
	progdlgNoCancel              = 0x00000040
)

var (
	liveSettingProperty = newGUID(0xc12bcd8e, 0x2a8e, 0x4950, 0x8ae7, 0x3625111d58eb)
	clsidProgressDialog = newGUID(0xf8383852, 0xfcd3, 0x11d1, 0xa6b9, 0x006097df5bd4)
	iidProgressDialog   = newGUID(0xebbc7c04, 0x315e, 0x11d2, 0xb62f, 0x006097df5bd4)
)

// variantArgs returns the argument slots of a VARIANT passed by value: a
// pointer to it on 64-bit Windows, where it is larger than a register pair,
// and its 16 bytes on 386. v must stay alive during the call.
func variantArgs(v *variant) []uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return []uintptr{heapAddress(uintptr(unsafe.Pointer(v)))}
	}
	words := (*[4]uint32)(unsafe.Pointer(v))
	return []uintptr{uintptr(words[0]), uintptr(words[1]), uintptr(words[2]), uintptr(words[3])}
}

// u64Args returns the argument slots of a 64-bit integer: one on 64-bit
// Windows and two on 386.
func u64Args(value uint64) []uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return []uintptr{uintptr(value)}
	}
	return []uintptr{uintptr(uint32(value)), uintptr(uint32(value >> 32))}
}

var procNotifyWinEvent = user32.proc("NotifyWinEvent")

func win32NotifyWinEvent(event uint32, h hwnd, object, child int32) {
	procNotifyWinEvent.call(uintptr(event), uintptr(h), uintptr(object), uintptr(child))
}

func win32AccPropServicesSetHwndProp(services comObject, h hwnd, object, child int32, property *guid,
	value *variant) hresult {
	args := []uintptr{uintptr(h), uintptr(uint32(object)), uintptr(uint32(child))}
	args = append(args, guidArgs(property)...)
	args = append(args, variantArgs(value)...)
	result := comResult(services, 6, args...)
	runtime.KeepAlive(value)
	return result
}

// IProgressDialog methods.

func win32ProgressDialogStartProgressDialog(dialog comObject, parent hwnd, flags uint32) hresult {
	return comResult(dialog, 3, uintptr(parent), 0, uintptr(flags), 0)
}

func win32ProgressDialogStopProgressDialog(dialog comObject) hresult {
	return comResult(dialog, 4)
}

func win32ProgressDialogSetTitle(dialog comObject, title string) hresult {
	return comResult(dialog, 5, uintptr(unsafe.Pointer(wide(title))))
}

func win32ProgressDialogHasUserCancelled(dialog comObject) bool {
	return comCall(dialog, 7) != 0
}

func win32ProgressDialogSetProgress64(dialog comObject, completed, total uint64) hresult {
	args := append(u64Args(completed), u64Args(total)...)
	return comResult(dialog, 9, args...)
}

func win32ProgressDialogSetLine(dialog comObject, line uint32, text string, compact bool) hresult {
	return comResult(dialog, 10, uintptr(line), uintptr(unsafe.Pointer(wide(text))), boolArg(compact), 0)
}

func win32ProgressDialogSetCancelMsg(dialog comObject, message string) hresult {
	return comResult(dialog, 11, uintptr(unsafe.Pointer(wide(message))), 0)
}

// ---- Content: rich edit, SVG, video, and web views ----

const (
	msftEditClass              = "RichEdit50W"
	esNoHideSel                = 0x0100
	emSetRectNP                = 0x00b4
	emExGetSel                 = wmUser + 52
	emStreamOut                = wmUser + 74
	emGetTextRange             = wmUser + 75
	emSetBkgndColor            = wmUser + 67
	emSetCharFormat            = wmUser + 68
	emSetEventMask             = wmUser + 69
	emSetTextMode              = wmUser + 89
	emAutoURLDetect            = wmUser + 91
	emGetTextEx                = wmUser + 94
	emGetTextLengthEx          = wmUser + 95
	emShowScrollBar            = wmUser + 96
	emSetTextEx                = wmUser + 97
	enLink                     = 0x070b
	enmChange                  = 0x00000001
	enmLink                    = 0x04000000
	scfDefault                 = 0x0000
	scfAll                     = 0x0004
	cfmColor                   = 0x40000000
	tmRichText                 = 2
	tmMultiLevelUndo           = 8
	tmMultiCodePage            = 32
	aurlEnableURL              = 1
	gtlPrecise                 = 2
	gtlNumChars                = 8
	gtDefault                  = 0
	stDefault                  = 0
	sfRTF                      = 0x0002
	cpACP                      = 0
	tbThumbTrack               = 5
	blackBrush                 = 4
	keyRead                    = 0x20019
	keyWow6464Key              = 0x0100
	keyWow6432Key              = 0x0200
	rrfRtRegSz                 = 0x00000002
	loadWithAlteredSearchPath  = 0x00000008
	vtI8                       = 20
	mfpMediaPlayerStatePlaying = 2
)

var hkeyLocalMachine = signExtend(0x80000002)

var (
	guidNull                 = newGUID(0x00000000, 0x0000, 0x0000, 0x0000, 0x000000000000)
	iidD2D1DeviceContext5    = newGUID(0x7836d248, 0x68cc, 0x4df6, 0xb9e8, 0xde991bf62eb7)
	iidWebEnvironmentHandler = newGUID(0x4e8a3389, 0xc9d8, 0x4bd2, 0xb6b5, 0x124fee6cc14d)
	iidWebControllerHandler  = newGUID(0x6c4819f3, 0xc9b7, 0x4260, 0x8127, 0xc9f5bde7f68c)
)

type charRange struct {
	cpMin int32
	cpMax int32
}

type charFormat2 struct {
	cbSize          uint32
	dwMask          uint32
	dwEffects       uint32
	yHeight         int32
	yOffset         int32
	crTextColor     uint32
	bCharSet        uint8
	bPitchAndFamily uint8
	szFaceName      [32]uint16
	wWeight         uint16
	sSpacing        int16
	crBackColor     uint32
	lcid            uint32
	dwReserved      uint32
	sStyle          int16
	wKerning        uint16
	bUnderlineType  uint8
	bAnimation      uint8
	bRevAuthor      uint8
	bUnderlineColor uint8
}

type textRange struct {
	chrg      charRange
	lpstrText *uint16
}

// richedit.h packs its structs to 4 bytes on 64-bit Windows too, so a pointer
// that follows a 32-bit field is two 32-bit words; see packedPointer.

// pointerWords is the number of 32-bit words of a pointer.
const pointerWords = unsafe.Sizeof(uintptr(0)) / 4

// packedPointer returns the words of a pointer in a struct packed to 4 bytes.
func packedPointer(value uintptr) (words [pointerWords]uint32) {
	*(*uintptr)(unsafe.Pointer(&words)) = value
	return words
}

// enLinkInfo is an ENLINK, which Windows writes; nmhdr is its header as words,
// as its size on 64-bit Windows is not a multiple of the alignment of nmhdr.
type enLinkInfo struct {
	nmhdr  [unsafe.Sizeof(nmhdr{}) / 4]uint32
	msg    uint32
	wParam [pointerWords]uint32
	lParam [pointerWords]uint32
	chrg   charRange
}

type getTextLengthEx struct {
	flags    uint32
	codepage uint32
}

// getTextEx is a GETTEXTEX without a default character.
type getTextEx struct {
	cb            uint32
	flags         uint32
	codepage      uint32
	lpDefaultChar [pointerWords]uint32
	lpUsedDefChar [pointerWords]uint32
}

type setTextEx struct {
	flags    uint32
	codepage uint32
}

type editStream struct {
	dwCookie    [pointerWords]uint32
	dwError     uint32
	pfnCallback [pointerWords]uint32
}

type parsedURL struct {
	cbSize      uint32
	pszProtocol *uint16
	cchProtocol uint32
	pszSuffix   *uint16
	cchSuffix   uint32
	nScheme     uint32
}

type d2d1Matrix3x2F struct {
	m11, m12 float32
	m21, m22 float32
	dx, dy   float32
}

// propVariant is a PROPVARIANT; data is its union.
type propVariant struct {
	vt         uint16
	wReserved1 uint16
	wReserved2 uint16
	wReserved3 uint16
	data       [2]uintptr
}

func (v *propVariant) int64Value() int64 {
	return *(*int64)(unsafe.Pointer(&v.data))
}

func (v *propVariant) setInt64(value int64) {
	*(*int64)(unsafe.Pointer(&v.data)) = value
}

// rectArgs returns the argument slots of a RECT passed by value: a pointer to
// it on amd64, its 16 bytes in two registers on arm64, and four slots on 386.
// r must stay alive during the call.
func rectArgs(r *rect) []uintptr {
	switch {
	case arm64ABI:
		words := (*[2]uint64)(unsafe.Pointer(r))
		return []uintptr{uintptr(words[0]), uintptr(words[1])}
	case unsafe.Sizeof(uintptr(0)) == 8:
		return []uintptr{heapAddress(uintptr(unsafe.Pointer(r)))}
	}
	return []uintptr{uintptr(uint32(r.left)), uintptr(uint32(r.top)), uintptr(uint32(r.right)),
		uintptr(uint32(r.bottom))}
}

var (
	shlwapi = newLibrary("shlwapi.dll")
	mfplay  = newLibrary("mfplay.dll")
)

var (
	procGetProcAddressW      = kernel32.proc("GetProcAddress")
	procLoadLibraryExW       = kernel32.proc("LoadLibraryExW")
	procGetStockObject       = gdi32.proc("GetStockObject")
	procRegOpenKeyEx         = advapi32.proc("RegOpenKeyExW")
	procRegCloseKey          = advapi32.proc("RegCloseKey")
	procPropVariantClear     = ole32.proc("PropVariantClear")
	procSHCreateMemStream    = shlwapi.proc("SHCreateMemStream")
	procParseURL             = shlwapi.proc("ParseURLW")
	procMFPCreateMediaPlayer = mfplay.proc("MFPCreateMediaPlayer")
)

func win32GetProcAddress(module uintptr, name string) uintptr {
	text, _ := syscall.BytePtrFromString(name)
	return procGetProcAddressW.call(module, uintptr(unsafe.Pointer(text)))
}

func win32LoadLibraryEx(path string, flags uint32) uintptr {
	return procLoadLibraryExW.call(uintptr(unsafe.Pointer(wide(path))), 0, uintptr(flags))
}

func win32GetStockObject(object int32) uintptr {
	return procGetStockObject.call(uintptr(object))
}

func win32RegOpenKeyEx(key uintptr, subKey string, options, access uint32, result *uintptr) int32 {
	return int32(procRegOpenKeyEx.call(key, uintptr(unsafe.Pointer(wide(subKey))), uintptr(options), uintptr(access),
		uintptr(unsafe.Pointer(result))))
}

func win32RegCloseKey(key uintptr) int32 {
	return int32(procRegCloseKey.call(key))
}

func win32PropVariantClear(value *propVariant) hresult {
	return hresult(procPropVariantClear.call(uintptr(unsafe.Pointer(value))))
}

// win32SHCreateMemStream returns a stream with a copy of data, or 0.
func win32SHCreateMemStream(data []byte) comObject {
	var first *byte
	if len(data) > 0 {
		first = &data[0]
	}
	return comObject(procSHCreateMemStream.call(uintptr(unsafe.Pointer(first)), uintptr(len(data))))
}

func win32ParseURL(url string, parsed *parsedURL) hresult {
	return hresult(procParseURL.call(uintptr(unsafe.Pointer(wide(url))), uintptr(unsafe.Pointer(parsed))))
}

func win32MFPCreateMediaPlayer(url string, start bool, options uint32, callback comObject, window hwnd,
	player *comObject) hresult {
	return hresult(procMFPCreateMediaPlayer.call(uintptr(unsafe.Pointer(wide(url))), boolArg(start), uintptr(options),
		uintptr(callback), uintptr(window), uintptr(unsafe.Pointer(player))))
}

// ID2D1RenderTarget transform and ID2D1DeviceContext5 methods.

func win32D2D1RenderTargetSetTransform(target comObject, transform *d2d1Matrix3x2F) {
	comCall(target, 30, uintptr(unsafe.Pointer(transform)))
}

func win32D2D1RenderTargetGetTransform(target comObject, transform *d2d1Matrix3x2F) {
	comCall(target, 31, uintptr(unsafe.Pointer(transform)))
}

func win32D2D1DeviceContext5CreateSvgDocument(context, stream comObject, viewport d2d1SizeF,
	document *comObject) hresult {
	call := comFloatCall(context)
	call.arg(uintptr(stream))
	call.point(viewport.width, viewport.height)
	call.pointerArg(unsafe.Pointer(document))
	return hresult(uint32(call.call(comMethod(context, 115))))
}

func win32D2D1DeviceContext5DrawSvgDocument(context, document comObject) {
	comCall(context, 116, uintptr(document))
}

// IMFPMediaPlayer methods.

func win32MFPMediaPlayerPlay(player comObject) hresult {
	return comResult(player, 3)
}

func win32MFPMediaPlayerPause(player comObject) hresult {
	return comResult(player, 4)
}

func win32MFPMediaPlayerSetPosition(player comObject, kind *guid, position *propVariant) hresult {
	return comResult(player, 7, uintptr(unsafe.Pointer(kind)), uintptr(unsafe.Pointer(position)))
}

func win32MFPMediaPlayerGetPosition(player comObject, kind *guid, position *propVariant) hresult {
	return comResult(player, 8, uintptr(unsafe.Pointer(kind)), uintptr(unsafe.Pointer(position)))
}

func win32MFPMediaPlayerGetDuration(player comObject, kind *guid, duration *propVariant) hresult {
	return comResult(player, 9, uintptr(unsafe.Pointer(kind)), uintptr(unsafe.Pointer(duration)))
}

func win32MFPMediaPlayerGetState(player comObject, state *uint32) hresult {
	return comResult(player, 13, uintptr(unsafe.Pointer(state)))
}

func win32MFPMediaPlayerSetMute(player comObject, mute bool) hresult {
	return comResult(player, 24, boolArg(mute))
}

func win32MFPMediaPlayerUpdateVideo(player comObject) hresult {
	return comResult(player, 32)
}

func win32MFPMediaPlayerShutdown(player comObject) hresult {
	return comResult(player, 38)
}

// WebView2. The runtime exports CreateWebViewEnvironmentWithOptionsInternal,
// which the loader of the SDK calls.

func win32CreateWebViewEnvironment(create uintptr, checkRunning bool, runtimeType int32, folder string,
	handler comObject) hresult {
	return hresult(callAddress(create, boolArg(checkRunning), uintptr(runtimeType), uintptr(unsafe.Pointer(wideOrNil(folder))),
		0, uintptr(handler)))
}

// ICoreWebView2Environment methods.

func win32WebView2EnvironmentCreateCoreWebView2Controller(environment comObject, parent hwnd,
	handler comObject) hresult {
	return comResult(environment, 3, uintptr(parent), uintptr(handler))
}

// ICoreWebView2Controller methods.

func win32WebView2ControllerPutIsVisible(controller comObject, visible bool) hresult {
	return comResult(controller, 4, boolArg(visible))
}

func win32WebView2ControllerPutBounds(controller comObject, bounds *rect) hresult {
	result := comResult(controller, 6, rectArgs(bounds)...)
	runtime.KeepAlive(bounds)
	return result
}

func win32WebView2ControllerClose(controller comObject) hresult {
	return comResult(controller, 24)
}

func win32WebView2ControllerGetCoreWebView2(controller comObject, core *comObject) hresult {
	return comResult(controller, 25, uintptr(unsafe.Pointer(core)))
}

// ICoreWebView2 methods.

func win32WebView2GetSettings(core comObject, settings *comObject) hresult {
	return comResult(core, 3, uintptr(unsafe.Pointer(settings)))
}

func win32WebView2GetSource(core comObject, source *uintptr) hresult {
	return comResult(core, 4, uintptr(unsafe.Pointer(source)))
}

func win32WebView2Navigate(core comObject, url string) hresult {
	return comResult(core, 5, uintptr(unsafe.Pointer(wide(url))))
}

func win32WebView2NavigateToString(core comObject, html string) hresult {
	return comResult(core, 6, uintptr(unsafe.Pointer(wide(html))))
}

// ICoreWebView2Settings methods.

func win32WebView2SettingsPutIsScriptEnabled(settings comObject, enabled bool) hresult {
	return comResult(settings, 4, boolArg(enabled))
}

// ---- Clipboard, files, and printing ----

const (
	cfText                       = 1
	cfBitmap                     = 2
	cfOEMText                    = 7
	cfDIBV5                      = 17
	lcsSRGB                      = 0x73524742
	fileAttributeDirectory       = 0x00000010
	invalidFileAttributes        = 0xffffffff
	shardPathW                   = 0x00000003
	assocfInitIgnoreUnknown      = 0x00000400
	assocstrFriendlyAppName      = 4
	pdNoSelection                = 0x00000004
	pdNoPageNums                 = 0x00000008
	pdReturnDC                   = 0x00000100
	pdReturnDefault              = 0x00000400
	pdUseDevModeCopiesAndCollate = 0x00040000
	pdNoCurrentPage              = 0x00800000
	pdResultPrint                = 1
	startPageGeneral             = 0xffffffff
	horzRes                      = 8
	vertRes                      = 10
	logPixelsX                   = 88
	halftone                     = 4
	srcCopy                      = 0x00cc0020
	dtNoClip                     = 0x00000100
)

// printDlgEx is a PRINTDLGEXW. commdlg.h packs it to 1 byte on 386, where
// every field is 4 bytes.
type printDlgEx struct {
	lStructSize         uint32
	hwndOwner           hwnd
	hDevMode            hglobal
	hDevNames           hglobal
	hDC                 hdc
	flags               uint32
	flags2              uint32
	exclusionFlags      uint32
	nPageRanges         uint32
	nMaxPageRanges      uint32
	lpPageRanges        uintptr
	nMinPage            uint32
	nMaxPage            uint32
	nCopies             uint32
	hInstance           hinstance
	lpPrintTemplateName *uint16
	lpCallback          comObject
	nPropertyPages      uint32
	lphPropertyPages    uintptr
	nStartPage          uint32
	dwResultAction      uint32
}

type docInfo struct {
	cbSize       int32
	lpszDocName  *uint16
	lpszOutput   *uint16
	lpszDatatype *uint16
	fwType       uint32
}

var (
	procOpenClipboard              = user32.proc("OpenClipboard")
	procCloseClipboard             = user32.proc("CloseClipboard")
	procEmptyClipboard             = user32.proc("EmptyClipboard")
	procSetClipboardData           = user32.proc("SetClipboardData")
	procGetClipboardData           = user32.proc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.proc("IsClipboardFormatAvailable")
	procEnumClipboardFormats       = user32.proc("EnumClipboardFormats")
	procGetClipboardFormatName     = user32.proc("GetClipboardFormatNameW")
	procGetClipboardSequenceNumber = user32.proc("GetClipboardSequenceNumber")
	procGetFileAttributes          = kernel32.proc("GetFileAttributesW")
	procSHParseDisplayName         = shell32.proc("SHParseDisplayName")
	procSHOpenFolderAndSelectItems = shell32.proc("SHOpenFolderAndSelectItems")
	procSHAddToRecentDocs          = shell32.proc("SHAddToRecentDocs")
	procAssocQueryString           = shlwapi.proc("AssocQueryStringW")
	procPrintDlgEx                 = comdlg32.proc("PrintDlgExW")
	procStartDoc                   = gdi32.proc("StartDocW")
	procStartPage                  = gdi32.proc("StartPage")
	procEndPage                    = gdi32.proc("EndPage")
	procEndDoc                     = gdi32.proc("EndDoc")
	procAbortDoc                   = gdi32.proc("AbortDoc")
	procSetStretchBltMode          = gdi32.proc("SetStretchBltMode")
	procStretchDIBits              = gdi32.proc("StretchDIBits")
)

func win32OpenClipboard(owner hwnd) bool {
	return procOpenClipboard.call(uintptr(owner)) != 0
}

func win32CloseClipboard() bool {
	return procCloseClipboard.call() != 0
}

func win32EmptyClipboard() bool {
	return procEmptyClipboard.call() != 0
}

// win32SetClipboardData gives memory to the clipboard, which frees it.
func win32SetClipboardData(format uint32, memory hglobal) uintptr {
	return procSetClipboardData.call(uintptr(format), uintptr(memory))
}

func win32GetClipboardData(format uint32) uintptr {
	return procGetClipboardData.call(uintptr(format))
}

func win32IsClipboardFormatAvailable(format uint32) bool {
	return procIsClipboardFormatAvailable.call(uintptr(format)) != 0
}

func win32EnumClipboardFormats(format uint32) uint32 {
	return uint32(procEnumClipboardFormats.call(uintptr(format)))
}

func win32GetClipboardFormatName(format uint32, name []uint16) int {
	return int(int32(procGetClipboardFormatName.call(uintptr(format), uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)))))
}

func win32GetClipboardSequenceNumber() uint32 {
	return uint32(procGetClipboardSequenceNumber.call())
}

func win32GetFileAttributes(path string) uint32 {
	return uint32(procGetFileAttributes.call(uintptr(unsafe.Pointer(wide(path)))))
}

func win32SHParseDisplayName(name string, context comObject, item *uintptr, query uint32, attributes *uint32) hresult {
	return hresult(procSHParseDisplayName.call(uintptr(unsafe.Pointer(wide(name))), uintptr(context),
		uintptr(unsafe.Pointer(item)), uintptr(query), uintptr(unsafe.Pointer(attributes))))
}

func win32SHOpenFolderAndSelectItems(folder uintptr, count uint32, items *uintptr, flags uint32) hresult {
	return hresult(procSHOpenFolderAndSelectItems.call(folder, uintptr(count), uintptr(unsafe.Pointer(items)),
		uintptr(flags)))
}

func win32SHAddToRecentDocs(flags uint32, path string) {
	procSHAddToRecentDocs.call(uintptr(flags), uintptr(unsafe.Pointer(wide(path))))
}

func win32AssocQueryString(flags, kind uint32, association, extra string, out []uint16, length *uint32) hresult {
	return hresult(procAssocQueryString.call(uintptr(flags), uintptr(kind), uintptr(unsafe.Pointer(wide(association))),
		uintptr(unsafe.Pointer(wideOrNil(extra))), uintptr(unsafe.Pointer(&out[0])), uintptr(unsafe.Pointer(length))))
}

func win32PrintDlgEx(setup *printDlgEx) hresult {
	return hresult(procPrintDlgEx.call(uintptr(unsafe.Pointer(setup))))
}

func win32StartDoc(dc hdc, document *docInfo) int32 {
	return int32(procStartDoc.call(uintptr(dc), uintptr(unsafe.Pointer(document))))
}

func win32StartPage(dc hdc) int32 {
	return int32(procStartPage.call(uintptr(dc)))
}

func win32EndPage(dc hdc) int32 {
	return int32(procEndPage.call(uintptr(dc)))
}

func win32EndDoc(dc hdc) int32 {
	return int32(procEndDoc.call(uintptr(dc)))
}

func win32AbortDoc(dc hdc) int32 {
	return int32(procAbortDoc.call(uintptr(dc)))
}

func win32SetStretchBltMode(dc hdc, mode int32) int32 {
	return int32(procSetStretchBltMode.call(uintptr(dc), uintptr(mode)))
}

func win32StretchDIBits(dc hdc, x, y, width, height, sourceX, sourceY, sourceWidth, sourceHeight int32,
	bits unsafe.Pointer, info *bitmapInfo, usage, operation uint32) int32 {
	return int32(procStretchDIBits.call(uintptr(dc), uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		uintptr(sourceX), uintptr(sourceY), uintptr(sourceWidth), uintptr(sourceHeight), uintptr(bits),
		uintptr(unsafe.Pointer(info)), uintptr(usage), uintptr(operation)))
}

// ---- Notification area ----

const (
	smCxSmIcon         = 49
	fwBold             = 700
	nifMessage         = 0x00000001
	nifIcon            = 0x00000002
	nifTip             = 0x00000004
	nifState           = 0x00000008
	nifShowTip         = 0x00000080
	nisHidden          = 0x00000001
	nimAdd             = 0x00000000
	nimModify          = 0x00000001
	nimDelete          = 0x00000002
	nimSetVersion      = 0x00000004
	notifyIconVersion4 = 4
	ninSelect          = wmUser
	ninKeySelect       = ninSelect | 1
)

// notifyIconData is a NOTIFYICONDATAW; uVersion is the union of uTimeout and
// uVersion.
type notifyIconData struct {
	cbSize           uint32
	hWnd             hwnd
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            hicon
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         guid
	hBalloonIcon     hicon
}

type notifyIconIdentifier struct {
	cbSize   uint32
	hWnd     hwnd
	uID      uint32
	guidItem guid
}

var (
	procShellNotifyIcon        = shell32.proc("Shell_NotifyIconW")
	procShellNotifyIconGetRect = shell32.proc("Shell_NotifyIconGetRect")
	procRegisterWindowMessage  = user32.proc("RegisterWindowMessageW")
)

func win32ShellNotifyIcon(message uint32, data *notifyIconData) bool {
	return procShellNotifyIcon.call(uintptr(message), uintptr(unsafe.Pointer(data))) != 0
}

func win32ShellNotifyIconGetRect(identifier *notifyIconIdentifier, area *rect) hresult {
	return hresult(procShellNotifyIconGetRect.call(uintptr(unsafe.Pointer(identifier)), uintptr(unsafe.Pointer(area))))
}

func win32RegisterWindowMessage(name string) uint32 {
	return uint32(procRegisterWindowMessage.call(uintptr(unsafe.Pointer(wide(name)))))
}

// ---- Windows Runtime ----
//
// The backend reaches the Windows Runtime through its ABI: an interface of the
// Windows Runtime is a COM interface that starts with the methods of
// IInspectable, so its own methods start at index 6.

// hstring is an HSTRING.
type hstring uintptr

// eventRegistrationToken is an EventRegistrationToken.
type eventRegistrationToken int64

const (
	eBounds              = 0x8000000b
	eOutOfMemory         = 0x8007000e
	errorNotFoundResult  = 0x80070490
	asyncStatusStarted   = 0
	asyncStatusCompleted = 1
	baseTrust            = 0
	keySetValue          = 0x0002
	regSz                = 1
)

var (
	iidInspectable = newGUID(0xaf86e2e0, 0xb12d, 0x4c6a, 0x9c5a, 0xd7aa65101e90)
	iidAgileObject = newGUID(0x94ea2b94, 0xe9cc, 0x49e0, 0xc0ff, 0xee64ca8f5b90)
	iidAsyncInfo   = newGUID(0x00000036, 0x0000, 0x0000, 0xc000, 0x000000000046)
)

var combase = newLibrary("combase.dll")

var (
	procRoGetActivationFactory                  = combase.proc("RoGetActivationFactory")
	procRoActivateInstance                      = combase.proc("RoActivateInstance")
	procWindowsCreateString                     = combase.proc("WindowsCreateString")
	procWindowsDeleteString                     = combase.proc("WindowsDeleteString")
	procWindowsGetStringRawBuffer               = combase.proc("WindowsGetStringRawBuffer")
	procGetCurrentProcessExplicitAppUserModelID = shell32.proc("GetCurrentProcessExplicitAppUserModelID")
	procSetCurrentProcessExplicitAppUserModelID = shell32.proc("SetCurrentProcessExplicitAppUserModelID")
	procRegCreateKeyEx                          = advapi32.proc("RegCreateKeyExW")
	procRegSetValueEx                           = advapi32.proc("RegSetValueExW")
)

// win32WindowsCreateString returns a new HSTRING, which the caller deletes, or
// 0 for an empty one.
func win32WindowsCreateString(text string) hstring {
	units := utf16.Encode([]rune(text))
	if len(units) == 0 {
		return 0
	}
	var result hstring
	procWindowsCreateString.call(uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		uintptr(unsafe.Pointer(&result)))
	return result
}

func win32WindowsDeleteString(text hstring) {
	if text != 0 {
		procWindowsDeleteString.call(uintptr(text))
	}
}

func win32WindowsGetStringRawBuffer(text hstring) string {
	if text == 0 {
		return ""
	}
	var length uint32
	buffer := procWindowsGetStringRawBuffer.call(uintptr(text), uintptr(unsafe.Pointer(&length)))
	if buffer == 0 || length == 0 {
		return ""
	}
	return string(utf16.Decode(unsafe.Slice((*uint16)(memoryAt(buffer)), length)))
}

func win32RoGetActivationFactory(class hstring, iid *guid, factory *comObject) hresult {
	return hresult(procRoGetActivationFactory.call(uintptr(class), uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(factory))))
}

func win32RoActivateInstance(class hstring, instance *comObject) hresult {
	return hresult(procRoActivateInstance.call(uintptr(class), uintptr(unsafe.Pointer(instance))))
}

func win32GetCurrentProcessExplicitAppUserModelID(id *uintptr) hresult {
	return hresult(procGetCurrentProcessExplicitAppUserModelID.call(uintptr(unsafe.Pointer(id))))
}

func win32SetCurrentProcessExplicitAppUserModelID(id string) hresult {
	return hresult(procSetCurrentProcessExplicitAppUserModelID.call(uintptr(unsafe.Pointer(wide(id)))))
}

func win32RegCreateKeyEx(key uintptr, subKey string, options, access uint32, result *uintptr) int32 {
	return int32(procRegCreateKeyEx.call(key, uintptr(unsafe.Pointer(wide(subKey))), 0, 0, uintptr(options),
		uintptr(access), 0, uintptr(unsafe.Pointer(result)), 0))
}

func win32RegSetValueEx(key uintptr, name string, kind uint32, data []byte) int32 {
	var first *byte
	if len(data) > 0 {
		first = &data[0]
	}
	return int32(procRegSetValueEx.call(key, uintptr(unsafe.Pointer(wide(name))), 0, uintptr(kind),
		uintptr(unsafe.Pointer(first)), uintptr(len(data))))
}

// IAsyncInfo and IAsyncOperation<T> methods.

func win32AsyncInfoGetStatus(info comObject, status *int32) hresult {
	return comResult(info, 7, uintptr(unsafe.Pointer(status)))
}

func win32AsyncOperationGetResults(operation comObject, result *comObject) hresult {
	return comResult(operation, 8, uintptr(unsafe.Pointer(result)))
}

// ---- Sharing ----

var (
	iidDataTransferManagerInterop       = newGUID(0x3a3dcd6c, 0x3eab, 0x43dc, 0xbcde, 0x45671ce800c8)
	iidDataTransferManager              = newGUID(0xa5caee9b, 0x8708, 0x49d1, 0x8d36, 0x67d25a8da00c)
	iidDataPackage2                     = newGUID(0x041c1fe9, 0x2409, 0x45e1, 0xa538, 0x4c53eeee04a7)
	iidDataRequestedEventArgs           = newGUID(0xcb8ba807, 0x6ac5, 0x43c9, 0x8ac5, 0x9ba232163182)
	iidTargetApplicationChosenEventArgs = newGUID(0xca6fb8ac, 0x2987, 0x4ee3, 0x9c54, 0xd8afbcb86c1d)
	iidUriRuntimeClassFactory           = newGUID(0x44a9796f, 0x723e, 0x4fdf, 0xa218, 0x033e75b0c084)
	iidStorageFileStatics               = newGUID(0x5984c710, 0xdaf2, 0x43c8, 0x8bb4, 0xa4d3eacfd03f)
	iidStorageFolderStatics             = newGUID(0x08f327ff, 0x85d5, 0x48b9, 0xaee9, 0x28511e339f9f)
	iidStorageItem                      = newGUID(0x4207a996, 0xca2f, 0x42f7, 0xbde8, 0x8b10457a7f30)
	iidStorageItems                     = newGUID(0xbb8b8418, 0x65d1, 0x544b, 0xb083, 0x6d172f568c73)
	iidStorageItemIterator              = newGUID(0x05b487c2, 0x3830, 0x5d3c, 0x98da, 0x25fa11542dbd)
	iidDataRequestedHandler             = newGUID(0xec6f9cc8, 0x46d0, 0x5e0e, 0xb4d2, 0x7d7773ae37a0)
	iidTargetApplicationChosenHandler   = newGUID(0xc4ac1ba2, 0x7851, 0x5a44, 0xbc8d, 0x3d7c713f1f41)
)

// IDataTransferManagerInterop methods.

func win32DataTransferManagerInteropGetForWindow(interop comObject, window hwnd, iid *guid,
	manager *comObject) hresult {
	return comResult(interop, 3, uintptr(window), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(manager)))
}

func win32DataTransferManagerInteropShowShareUIForWindow(interop comObject, window hwnd) hresult {
	return comResult(interop, 4, uintptr(window))
}

// IDataTransferManager methods.

func win32DataTransferManagerAddDataRequested(manager, handler comObject,
	token *eventRegistrationToken) hresult {
	return comResult(manager, 6, uintptr(handler), uintptr(unsafe.Pointer(token)))
}

func win32DataTransferManagerRemoveDataRequested(manager comObject, token eventRegistrationToken) hresult {
	return comResult(manager, 7, u64Args(uint64(token))...)
}

func win32DataTransferManagerAddTargetApplicationChosen(manager, handler comObject,
	token *eventRegistrationToken) hresult {
	return comResult(manager, 8, uintptr(handler), uintptr(unsafe.Pointer(token)))
}

func win32DataTransferManagerRemoveTargetApplicationChosen(manager comObject, token eventRegistrationToken) hresult {
	return comResult(manager, 9, u64Args(uint64(token))...)
}

// IDataPackagePropertySet, IDataPackage, and IDataPackage2 methods.

func win32DataPackagePropertySetPutTitle(properties comObject, title hstring) hresult {
	return comResult(properties, 7, uintptr(title))
}

func win32DataPackageGetProperties(data comObject, properties *comObject) hresult {
	return comResult(data, 7, uintptr(unsafe.Pointer(properties)))
}

func win32DataPackageSetText(data comObject, text hstring) hresult {
	return comResult(data, 16, uintptr(text))
}

func win32DataPackageSetStorageItems(data, items comObject, readOnly bool) hresult {
	return comResult(data, 23, uintptr(items), boolArg(readOnly))
}

func win32DataPackage2SetWebLink(data, uri comObject) hresult {
	return comResult(data, 7, uintptr(uri))
}

// IDataRequest, IDataRequestedEventArgs, and
// ITargetApplicationChosenEventArgs methods.

func win32DataRequestGetData(request comObject, data *comObject) hresult {
	return comResult(request, 6, uintptr(unsafe.Pointer(data)))
}

func win32DataRequestedEventArgsGetRequest(args comObject, request *comObject) hresult {
	return comResult(args, 6, uintptr(unsafe.Pointer(request)))
}

func win32TargetApplicationChosenEventArgsGetApplicationName(args comObject, name *hstring) hresult {
	return comResult(args, 6, uintptr(unsafe.Pointer(name)))
}

// IUriRuntimeClassFactory, IStorageFileStatics, and IStorageFolderStatics
// methods.

func win32UriRuntimeClassFactoryCreateUri(factory comObject, text hstring, uri *comObject) hresult {
	return comResult(factory, 6, uintptr(text), uintptr(unsafe.Pointer(uri)))
}

func win32StorageFileStaticsGetFileFromPathAsync(statics comObject, path hstring, operation *comObject) hresult {
	return comResult(statics, 6, uintptr(path), uintptr(unsafe.Pointer(operation)))
}

func win32StorageFolderStaticsGetFolderFromPathAsync(statics comObject, path hstring, operation *comObject) hresult {
	return comResult(statics, 6, uintptr(path), uintptr(unsafe.Pointer(operation)))
}

// ---- Toast notifications and the taskbar ----

var (
	iidToastNotificationManagerStatics  = newGUID(0x50ac103f, 0xd235, 0x4598, 0xbbef, 0x98fe4d1a3ad4)
	iidToastNotificationManagerStatics2 = newGUID(0x7ab93c52, 0x0e48, 0x4750, 0xba9d, 0x1a4113981847)
	iidToastNotificationFactory         = newGUID(0x04124b20, 0x82c6, 0x4229, 0xb109, 0xfd9ed4662b53)
	iidToastNotification2               = newGUID(0x9dfb9fd1, 0x143a, 0x490e, 0x90bf, 0xb9fba7132de7)
	iidToastNotificationHistory2        = newGUID(0x3bc3d253, 0x2f31, 0x4092, 0x9129, 0x8ad5abf067da)
	iidToastActivatedEventArgs          = newGUID(0xe3bf92f3, 0xc197, 0x436f, 0x8265, 0x0625824f8dac)
	iidToastActivatedEventArgs2         = newGUID(0xab7da512, 0xcc61, 0x568e, 0x81be, 0x304ac31038fa)
	iidXmlDocumentIO                    = newGUID(0x6cd0e74e, 0xee65, 0x4489, 0x9ebf, 0xca43e87ba637)
	iidPropertyValue                    = newGUID(0x4bd682dd, 0x7554, 0x40e9, 0x9a9b, 0x82654ede7e62)
	iidStringObjectMap                  = newGUID(0x1b0d3570, 0x0877, 0x5ec2, 0x8a2c, 0x3b9539506aca)
	iidToastActivatedHandler            = newGUID(0xab54de2d, 0x97d9, 0x5528, 0xb6ad, 0x105afe156530)
	clsidTaskbarList                    = newGUID(0x56fdf344, 0xfd6d, 0x11d0, 0x958a, 0x006097c9a090)
	iidTaskbarList3                     = newGUID(0xea1afb91, 0x9e28, 0x4b86, 0x90e9, 0x9e9f8a5eefaf)
)

// IToastNotifier methods.

func win32ToastNotifierShow(notifier, toast comObject) hresult {
	return comResult(notifier, 6, uintptr(toast))
}

func win32ToastNotifierGetSetting(notifier comObject, setting *int32) hresult {
	return comResult(notifier, 8, uintptr(unsafe.Pointer(setting)))
}

// IToastNotificationHistory and IToastNotificationHistory2 methods.

func win32ToastNotificationHistoryRemoveGroupedTagWithId(history comObject, tag, group, app hstring) hresult {
	return comResult(history, 8, uintptr(tag), uintptr(group), uintptr(app))
}

func win32ToastNotificationHistory2GetHistoryWithId(history comObject, app hstring, toasts *comObject) hresult {
	return comResult(history, 7, uintptr(app), uintptr(unsafe.Pointer(toasts)))
}

// IVectorView<ToastNotification> methods.

func win32VectorViewGetAt(view comObject, index uint32, item *comObject) hresult {
	return comResult(view, 6, uintptr(index), uintptr(unsafe.Pointer(item)))
}

func win32VectorViewGetSize(view comObject, size *uint32) hresult {
	return comResult(view, 7, uintptr(unsafe.Pointer(size)))
}

// IToastNotificationManagerStatics and IToastNotificationManagerStatics2
// methods.

func win32ToastNotificationManagerStaticsCreateToastNotifierWithId(manager comObject, app hstring,
	notifier *comObject) hresult {
	return comResult(manager, 7, uintptr(app), uintptr(unsafe.Pointer(notifier)))
}

func win32ToastNotificationManagerStatics2GetHistory(manager comObject, history *comObject) hresult {
	return comResult(manager, 6, uintptr(unsafe.Pointer(history)))
}

// IToastNotificationFactory, IToastNotification, and IToastNotification2
// methods.

func win32ToastNotificationFactoryCreateToastNotification(factory, document comObject, toast *comObject) hresult {
	return comResult(factory, 6, uintptr(document), uintptr(unsafe.Pointer(toast)))
}

func win32ToastNotificationAddActivated(toast, handler comObject, token *eventRegistrationToken) hresult {
	return comResult(toast, 11, uintptr(handler), uintptr(unsafe.Pointer(token)))
}

func win32ToastNotification2PutTag(toast comObject, tag hstring) hresult {
	return comResult(toast, 6, uintptr(tag))
}

func win32ToastNotification2GetTag(toast comObject, tag *hstring) hresult {
	return comResult(toast, 7, uintptr(unsafe.Pointer(tag)))
}

func win32ToastNotification2PutGroup(toast comObject, group hstring) hresult {
	return comResult(toast, 8, uintptr(group))
}

// IToastActivatedEventArgs, IToastActivatedEventArgs2, IMap<String, Object>,
// IPropertyValue, and IXmlDocumentIO methods.

func win32ToastActivatedEventArgsGetArguments(args comObject, arguments *hstring) hresult {
	return comResult(args, 6, uintptr(unsafe.Pointer(arguments)))
}

func win32ToastActivatedEventArgs2GetUserInput(args comObject, input *comObject) hresult {
	return comResult(args, 6, uintptr(unsafe.Pointer(input)))
}

func win32MapLookup(values comObject, key hstring, value *comObject) hresult {
	return comResult(values, 6, uintptr(key), uintptr(unsafe.Pointer(value)))
}

func win32MapHasKey(values comObject, key hstring, found *bool) hresult {
	return comResult(values, 8, uintptr(key), uintptr(unsafe.Pointer(found)))
}

func win32PropertyValueGetString(value comObject, text *hstring) hresult {
	return comResult(value, 19, uintptr(unsafe.Pointer(text)))
}

func win32XmlDocumentIOLoadXml(document comObject, xml hstring) hresult {
	return comResult(document, 6, uintptr(xml))
}

// ITaskbarList3 methods.

func win32TaskbarListHrInit(taskbar comObject) hresult {
	return comResult(taskbar, 3)
}

func win32TaskbarList3SetOverlayIcon(taskbar comObject, window hwnd, icon hicon, description string) hresult {
	return comResult(taskbar, 18, uintptr(window), uintptr(icon), uintptr(unsafe.Pointer(wide(description))))
}
