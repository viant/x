package x

import (
	"fmt"
	"sort"
)

// RegisterFunctions atomically registers compiled exports in this registry.
// Duplicate full keys are errors, including repeated registrations of the same
// function: closures cannot be safely compared by their instruction address.
// Registration never invokes functions or type listeners.
func (r *Registry) RegisterFunctions(functions ...*Function) error {
	if r == nil {
		return fmt.Errorf("registry is required")
	}
	batch := make(map[string]*Function, len(functions))
	for _, function := range functions {
		if function == nil || function.Type() == nil {
			return fmt.Errorf("compiled function is required")
		}
		key := function.Key()
		if _, ok := batch[key]; ok {
			return fmt.Errorf("function %q is duplicated", key)
		}
		copy := *function
		batch[key] = &copy
	}
	r.mux.Lock()
	defer r.mux.Unlock()
	for key := range batch {
		if _, ok := r.functions[key]; ok {
			return fmt.Errorf("function %q is already registered", key)
		}
	}
	if r.functions == nil {
		r.functions = map[string]*Function{}
	}
	for key, function := range batch {
		r.functions[key] = function
	}
	return nil
}

// LookupFunction requires an exact full package-qualified key. There are no
// short-name aliases, package-prefix fallbacks or implicit global registrations.
func (r *Registry) LookupFunction(key string) (*Function, bool) {
	if r == nil {
		return nil, false
	}
	r.mux.RLock()
	defer r.mux.RUnlock()
	function, ok := r.functions[key]
	if !ok {
		return nil, false
	}
	copy := *function
	return &copy, true
}

// Functions returns a deterministic detached export table snapshot. Functions
// themselves remain the same compiled closures; capture state is caller-owned.
func (r *Registry) Functions() []*Function {
	if r == nil {
		return nil
	}
	r.mux.RLock()
	defer r.mux.RUnlock()
	keys := make([]string, 0, len(r.functions))
	for key := range r.functions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*Function, 0, len(keys))
	for _, key := range keys {
		copy := *r.functions[key]
		result = append(result, &copy)
	}
	return result
}
