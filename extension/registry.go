package extension

import (
	"sync"
)

type (
	//Registry represents an extension type
	Registry struct {
		mux      sync.RWMutex
		scn      int //scn is the serial number of the registry
		types    map[string]*Type
		listener Listener
	}
)

// Scn returns registry sequence change number
func (r *Registry) Scn() int {
	return r.scn
}

// Merge merges registry
func (r *Registry) Merge(registry *Registry) {
	for _, aType := range registry.types {
		r.Register(aType)
	}
}

// Register registers a type
func (r *Registry) Register(aType *Type) {
	if aType.Scn == 0 {
		aType.Scn = r.scn
	}
	r.mux.RLock()
	key := aType.Key()
	prev, ok := r.types[key]
	r.mux.RUnlock()
	if !ok || aType.Force {
		r.mux.Lock()
		r.types[key] = aType
		r.mux.Unlock()
		if r.listener != nil {
			r.listener(aType)
		}
		return
	}
	if prev.Scn >= aType.Scn {
		return
	}
	r.mux.Lock()
	r.types[key] = aType
	r.mux.Unlock()
	if r.listener != nil {
		r.listener(aType)
	}
}

// Lookup returns a type by name
func (r *Registry) Lookup(name string) *Type {
	r.mux.RLock()
	aType, _ := r.types[name]
	r.mux.RUnlock()
	return aType
}

// NewRegistry creates a registry
func NewRegistry(options ...RegistryOption) *Registry {
	ret := &Registry{types: make(map[string]*Type)}
	for _, opt := range options {
		opt(ret)
	}
	return ret
}
