package x

import (
	"sort"
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

// Merge merges all types from the supplied registry into r.
//
// Listener behaviour:
//   - If a MergeListener is configured, Merge calls it once to obtain a
//     per-merge Listener and uses that for all per-type notifications.
//     After processing all types, the per-merge Listener is called once with
//     a nil *Type to signal completion. Implementations MUST tolerate nil.
//   - If no MergeListener is configured, Merge uses the registry's general
//     Listener (if any) for per-type notifications and does not emit a final
//     nil sentinel call.
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

// Keys returns a sorted snapshot of registry keys.
//
// The returned slice is detached from the underlying map so callers are
// free to modify it. Ordering is deterministic for a given registry
// state but concurrent mutation while iterating is not supported.
func (r *Registry) Keys() []string {
	r.mux.RLock()
	keys := make([]string, 0, len(r.types))
	for k := range r.types {
		keys = append(keys, k)
	}
	r.mux.RUnlock()
	sort.Strings(keys)
	return keys
}

// ForEach invokes fn for each registered type using a sorted snapshot of
// Keys. Iteration stops early when fn returns false. Callbacks are invoked
// without holding internal locks.
func (r *Registry) ForEach(fn func(key string, t *Type) bool) {
	if fn == nil {
		return
	}
	for _, k := range r.Keys() {
		if !fn(k, r.Lookup(k)) {
			return
		}
	}
}

// NewRegistry creates a registry
func NewRegistry(options ...RegistryOption) *Registry {
	ret := &Registry{types: make(map[string]*Type)}
	for _, opt := range options {
		opt(ret)
	}
	return ret
}
