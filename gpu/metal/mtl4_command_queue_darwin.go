//go:build darwin && cgo

package metal

// mtl4CommandQueue is a Metal command queue. Its methods dispatch through the native binding
// functions in objc_darwin.go.
type mtl4CommandQueue id

func (q mtl4CommandQueue) release() {
	release(q)
}

func (q mtl4CommandQueue) addResidencySet(set mtlResidencySet) {
	commandQueueAddResidencySet(q, set)
}

func (q mtl4CommandQueue) commit(buffers *mtl4CommandBuffer, count uint, options id) {
	commandQueueCommit(q, buffers, count, options)
}

func (q mtl4CommandQueue) signalDrawable(drawable mtlDrawable) {
	commandQueueSignalDrawable(q, drawable)
}

func (q mtl4CommandQueue) signalEvent(event mtlSharedEvent, value uint64) {
	commandQueueSignalEvent(q, event, value)
}
