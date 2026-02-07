package xreflect

import "reflect"

// LoadOption configures LoadPackage behaviour.
type LoadOption func(*cfg)

type cfg struct {
	pkgPath    string
	pkgName    string
	types      []reflect.Type
	allowCross bool
	onUnnamed  func(reflect.Type) error
	namePolicy func(reflect.Type) (string, bool)
}

// WithPackagePath overrides the derived package path.
func WithPackagePath(p string) LoadOption { return func(c *cfg) { c.pkgPath = p } }

// WithPackageName overrides the derived package name.
func WithPackageName(n string) LoadOption { return func(c *cfg) { c.pkgName = n } }

// WithTypes adds additional reflect.Type values to include.
func WithTypes(ts ...reflect.Type) LoadOption {
	return func(c *cfg) { c.types = append(c.types, ts...) }
}

// WithValues adds values whose reflect.Type should be included.
func WithValues(vs ...any) LoadOption {
	return func(c *cfg) {
		for _, v := range vs {
			if v != nil {
				c.types = append(c.types, reflect.TypeOf(v))
			}
		}
	}
}

// WithTypesFrom registers a provider that returns additional types to include.
func WithTypesFrom(provider func() []reflect.Type) LoadOption {
	return func(c *cfg) {
		if provider != nil {
			c.types = append(c.types, provider()...)
		}
	}
}

// WithAllowCrossPackage disables errors on types from other packages. Such
// types are skipped by default; with this option they are still skipped but no
// error is returned. This keeps the output package consistent.
func WithAllowCrossPackage() LoadOption { return func(c *cfg) { c.allowCross = true } }

// WithOnUnnamedRecursion installs a callback to handle unnamed recursive
// composites reported by the xtype reflection bridge.
func WithOnUnnamedRecursion(fn func(reflect.Type) error) LoadOption {
	return func(c *cfg) { c.onUnnamed = fn }
}

// WithNamePolicy customises the declared name for a reflect.Type. The boolean
// return value is reserved for future alias handling and is currently ignored.
func WithNamePolicy(fn func(reflect.Type) (string, bool)) LoadOption {
	return func(c *cfg) { c.namePolicy = fn }
}
