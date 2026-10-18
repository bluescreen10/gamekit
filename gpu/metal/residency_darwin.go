//go:build darwin && cgo

package metal

// residencySet keeps the backend's allocations resident for its command queue.
type residencySet id

func newResidencySet(device id) (residencySet, error) {
	object, err := newWithDescriptor(device, deviceNewResidencySetWithDescriptor, "MTLResidencySetDescriptor",
		func(descriptor id) {
			setLabel(descriptor, "gamekit.residency")
		}, "cannot create residency set")
	return residencySet(object), err
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
