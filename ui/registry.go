package ui

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Config selects the native UI backend that New creates. An empty Backend uses
// the registered default. If no default exists, New uses the only registered
// backend.
type Config struct {
	// Backend is the name that the backend used with RegisterBackend.
	Backend string
}

// BackendFactory creates an independent native UI backend.
type BackendFactory func() Backend

type backendRegistry struct {
	mu sync.RWMutex

	factories   map[string]BackendFactory
	defaultName string
}

var registeredBackends backendRegistry

// RegisterBackend makes a backend available to New. Backend packages normally
// call this from init, so applications only need a blank import. RegisterBackend
// panics if name is empty, factory is nil, or name is already registered.
func RegisterBackend(name string, factory BackendFactory) {
	if err := registeredBackends.register(name, factory); err != nil {
		panic(err)
	}
}

// SetDefaultBackend selects the backend that New uses when Config.Backend is
// empty. Platform-selection packages such as ui/default normally call it.
func SetDefaultBackend(name string) {
	if err := registeredBackends.setDefault(name); err != nil {
		panic(err)
	}
}

// New creates the selected native UI backend. Import a backend package for its
// registration side effect, or import ui/default to select the platform
// default. A nil config is equivalent to an empty Config.
func New(config *Config) (Backend, error) {
	name := ""
	if config != nil {
		name = config.Backend
	}
	return registeredBackends.newBackend(name)
}

// RegisteredBackends returns the registered backend names in sorted order.
func RegisteredBackends() []string {
	return registeredBackends.names()
}

func (r *backendRegistry) register(name string, factory BackendFactory) error {
	if name == "" {
		return errors.New("ui: backend name is empty")
	}
	if factory == nil {
		return fmt.Errorf("ui: backend %q has no factory", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]BackendFactory)
	}
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("ui: backend %q is already registered", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *backendRegistry) setDefault(name string) error {
	if name == "" {
		return errors.New("ui: default backend name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultName = name
	return nil
}

func (r *backendRegistry) newBackend(name string) (Backend, error) {
	factory, selected, err := r.selectFactory(name)
	if err != nil {
		return nil, err
	}

	backend := factory()
	if backend == nil {
		return nil, fmt.Errorf("ui: backend %q factory returned nil", selected)
	}
	return backend, nil
}

func (r *backendRegistry) selectFactory(name string) (BackendFactory, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	selected := name
	if selected == "" {
		selected = r.defaultName
	}
	if selected == "" && len(r.factories) == 1 {
		for registered, factory := range r.factories {
			return factory, registered, nil
		}
	}
	if selected == "" {
		if len(r.factories) == 0 {
			return nil, "", errors.New("ui: no backends are registered; import a backend or ui/default")
		}
		return nil, "", errors.New("ui: multiple backends are registered; select Config.Backend or import ui/default")
	}

	factory, exists := r.factories[selected]
	if !exists {
		return nil, "", fmt.Errorf("ui: backend %q is not registered", selected)
	}
	return factory, selected, nil
}

func (r *backendRegistry) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
