//go:build darwin && cgo

package metal

// residencySet keeps the backend's allocations resident for its command queue.
type residencySet id

func (s residencySet) release() {
	release(s)
}

// enrol adds an allocation and immediately commits and requests its residency.
func (s residencySet) enrol(allocation id) {
	residencySetAddAllocation(s, allocation)
	residencySetCommit(s)
	residencySetRequestResidency(s)
}

// unenrol removes an allocation and immediately commits the change.
func (s residencySet) unenrol(allocation id) {
	residencySetRemoveAllocation(s, allocation)
	residencySetCommit(s)
}
