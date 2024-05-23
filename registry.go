package x

import (
	"sync"
)

type (
	//Registry represents an extension type
	Registry struct {
		mux           sync.RWMutex
		scn           int //scn is the serial number of the registry
		types         map[string]*Type
		listener      Listener
		mergeListener MergeListener
	}
)

// Scn returns registry sequence change number
func (r *Registry) Scn() int {
	return r.scn
}

// Merge merges registry
func (r *Registry) Merge(registry *Registry) {
	listener := r.listener
	if r.mergeListener != nil {
		listener = r.mergeListener()
	}
	for _, aType := range registry.types {
		r.register(aType, listener)
	}
	if r.mergeListener != nil {
		listener(nil)
	}
}

// Register registers a type
func (r *Registry) Register(aType *Type) {
	r.register(aType, r.listener)
}

func (r *Registry) register(aType *Type, listener Listener) {
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
		if listener != nil {
			listener(aType)
		}
		return
	}
	if prev.Scn >= aType.Scn {
		return
	}
	r.mux.Lock()
	r.types[key] = aType
	r.mux.Unlock()
	if listener != nil {
		listener(aType)
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
