package xreflect

import "reflect"
import trf "github.com/viant/x/syntetic/model/transform"

// Resolver maps a (pkgPath, name) to a compiled reflect.Type when available.
// It is used to resolve Named nodes to existing types since new named types
// cannot be constructed at runtime.
type Resolver interface {
	Resolve(pkgPath, name string) (reflect.Type, bool)
}

// BuildOption configures the Builder.
type BuildOption func(*Builder)

// WithResolver installs a Named type resolver.
func WithResolver(r Resolver) BuildOption { return func(b *Builder) { b.resolver = r } }

// WithUnknownFallback sets the function used when a node cannot be
// materialized (e.g., unresolved Named). Default returns interface{}.
func WithUnknownFallback(fn func() reflect.Type) BuildOption {
	return func(b *Builder) { b.unknown = fn }
}

// WithAllowInterface enables attempting to construct interface types via
// reflect.InterfaceOf when method sets are present. By default the builder
// falls back to interface{} for non-empty interfaces to avoid portability
// issues.
func WithAllowInterface(allow bool) BuildOption { return func(b *Builder) { b.allowIface = allow } }

// WithCache provides an external cache for named type resolutions to improve
// reuse across builds and break cycles. Keys are "pkgPath.name".
func WithCache(cache map[string]reflect.Type) BuildOption {
	return func(b *Builder) { b.namedCache = cache }
}

// WithTransforms configures a transformer applied to nodes before materializing
// reflect.Type. This allows tag-driven rewrites or custom node adjustments to
// be applied uniformly.
func WithTransforms(t trf.Transformer) BuildOption { return func(b *Builder) { b.tr = t } }
