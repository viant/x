package x

import model "github.com/viant/x/syntetic/model"

type (
	//Option represent type option
	Option func(t *Type)

	//RegistryOption represent registry option
	RegistryOption func(r *Registry)
)

// WithPkgPath is an option to set the package path of the type
func WithPkgPath(pkg string) Option {
	return func(t *Type) {
		t.PkgPath = pkg
	}
}

// WithScn is an option to set the scn(sequence change number) of the type
func WithScn(at int) Option {
	return func(t *Type) {
		t.Scn = at
	}
}

// WithName is an option to set the name of the type
func WithName(name string) Option {
	return func(t *Type) {
		t.Name = name
	}
}

// WithForceFlag will force the type to be generated
func WithForceFlag() Option {
	return func(t *Type) {
		t.Force = true
	}
}

// WithSynteticType sets a prebuilt synthetic/model.Type on this runtime Type.
// This allows callers to attach a codegen-ready representation directly.
func WithSynteticType(st *model.Type) Option {
	return func(t *Type) {
		t.SynteticType = st
	}
}

// WithSyntheticType is an alias for WithSynteticType with conventional spelling.
// Kept to improve ergonomics while the package name remains "syntetic".
func WithSyntheticType(st *model.Type) Option { return WithSynteticType(st) }

// WithRegistryScn is an option to set scn time
func WithRegistryScn(scn int) RegistryOption {
	return func(r *Registry) {
		r.scn = scn
	}
}

// WithListener creates a new registry with the specified listener
func WithListener(listener Listener) RegistryOption {
	return func(r *Registry) {
		r.listener = listener
	}
}

// WithMergeListener creates a new registry with the specified listener
//
// Semantics:
//   - The provided factory is invoked at the start of Registry.Merge to
//     obtain a per-merge Listener.
//   - The returned Listener is called once for each merged type.
//   - After all types are processed, the returned Listener is called once
//     with a nil *Type to signal completion. Implementations MUST handle nil.
//   - If no MergeListener is configured, Merge falls back to the registry's
//     general Listener (configured via WithListener) without the nil sentinel.
func WithMergeListener(listener MergeListener) RegistryOption {
	return func(r *Registry) {
		r.mergeListener = listener
	}
}
