package ui

// IsolateRegistry replaces the backend registry with an empty one until the
// returned function restores it. Tests use it to observe registry behavior
// without the backends that their test binary imports.
func IsolateRegistry() (restore func()) {
	registeredBackends.mu.Lock()
	saved := registeredBackends.factories
	savedDefault := registeredBackends.defaultName
	registeredBackends.factories = nil
	registeredBackends.defaultName = ""
	registeredBackends.mu.Unlock()
	return func() {
		registeredBackends.mu.Lock()
		registeredBackends.factories = saved
		registeredBackends.defaultName = savedDefault
		registeredBackends.mu.Unlock()
	}
}
