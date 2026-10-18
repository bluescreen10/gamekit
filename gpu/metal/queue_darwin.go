//go:build darwin && cgo

package metal

// queue is a Metal command queue. Its methods dispatch through the native binding
// functions in objc_darwin.go.
type queue id

func (q queue) addResidencySet(set residencySet) {
	commandQueueAddResidencySet(q, set)
}

func (q queue) commit(buffers *commandBuffer, count uint, options id) {
	commandQueueCommit(q, buffers, count, options)
}

func (q queue) signalDrawable(drawable id) {
	commandQueueSignalDrawable(q, drawable)
}

func (q queue) signalEvent(event id, value uint64) {
	commandQueueSignalEvent(q, event, value)
}
