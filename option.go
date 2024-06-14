package x

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
func WithMergeListener(listener MergeListener) RegistryOption {
	return func(r *Registry) {
		r.mergeListener = listener
	}
}
