//go:build darwin && cgo

package metal

// mtlResidencySet keeps the backend's allocations resident for its command queue.
type mtlResidencySet id

func (s mtlResidencySet) release() {
	release(s)
}

func (s mtlResidencySet) addAllocation(allocation id) {
	residencySetAddAllocation(s, allocation)
}

func (s mtlResidencySet) removeAllocation(allocation id) {
	residencySetRemoveAllocation(s, allocation)
}

func (s mtlResidencySet) commit() {
	residencySetCommit(s)
}

func (s mtlResidencySet) requestResidency() {
	residencySetRequestResidency(s)
}

// enrol adds an allocation and immediately commits and requests its residency.
func (s mtlResidencySet) enrol(allocation id) {
	s.addAllocation(allocation)
	s.commit()
	s.requestResidency()
}

// unenrol removes an allocation and immediately commits the change.
func (s mtlResidencySet) unenrol(allocation id) {
	s.removeAllocation(allocation)
	s.commit()
}
