package xreflect

import "reflect"
import trf "github.com/viant/x/syntetic/model/transform"
import "github.com/viant/x/syntetic/model"

// Resolver maps a (pkgPath, name) to a compiled reflect.Type when available.
// It is used to resolve Named nodes to existing types since new named types
// cannot be constructed at runtime.
type Resolver interface {
	Resolve(pkgPath, name string) (reflect.Type, bool)
}

// InterfaceResolver resolves model.Interface method sets to compiled interface types.
// Implementations may use method-set fingerprints or registry lookups.
type InterfaceResolver interface {
	ResolveInterface(methods []model.Method) (reflect.Type, bool)
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

// WithStrictNamedResolution controls behavior for unresolved model.Named nodes.
// When enabled, BuildNode returns an error instead of falling back to unknown().
func WithStrictNamedResolution(strict bool) BuildOption {
	return func(b *Builder) { b.strictNamed = strict }
}

// WithUnresolvedNamedReporter installs a callback for unresolved named type diagnostics.
// It is invoked only when strict named resolution is enabled and a name cannot be resolved.
func WithUnresolvedNamedReporter(fn func(pkgPath, name string, candidates []string)) BuildOption {
	return func(b *Builder) { b.onUnresolvedNamed = fn }
}

// WithAllowAnonymousRecursion controls handling of anonymous recursive structs.
// false (default): return error on recursive anonymous struct edge.
// true: materialize recursive edge via unknown() fallback and continue.
func WithAllowAnonymousRecursion(allow bool) BuildOption {
	return func(b *Builder) { b.allowAnonRecursion = allow }
}

// WithAnonymousRecursionReporter installs callback for synthetic recursion names
// emitted when anonymous recursion is tolerated.
func WithAnonymousRecursionReporter(fn func(name string)) BuildOption {
	return func(b *Builder) { b.onAnonymousRecursion = fn }
}

// WithInterfaceResolver installs resolver for non-empty model.Interface nodes.
func WithInterfaceResolver(r InterfaceResolver) BuildOption {
	return func(b *Builder) { b.ifaceResolver = r }
}

// WithStrictInterfaceResolution controls behavior for unresolved non-empty interfaces.
// false (default): fallback to interface{}.
// true: return an error when interface resolver cannot resolve method set.
func WithStrictInterfaceResolution(strict bool) BuildOption {
	return func(b *Builder) { b.strictInterface = strict }
}

// WithUnresolvedInterfaceReporter installs callback for unresolved non-empty
// interfaces. It is invoked when strict interface resolution is enabled and
// method-set resolution fails.
func WithUnresolvedInterfaceReporter(fn func(methods []model.Method)) BuildOption {
	return func(b *Builder) { b.onUnresolvedInterface = fn }
}

// WithStrictUnionResolution controls behavior for model.Union nodes.
// false (default): fallback to unknown().
// true: return an error for union/type-set nodes (no native runtime type).
func WithStrictUnionResolution(strict bool) BuildOption {
	return func(b *Builder) { b.strictUnion = strict }
}

// WithUnresolvedUnionReporter installs callback for union/type-set diagnostics.
// It is invoked when strict union resolution is enabled.
func WithUnresolvedUnionReporter(fn func(terms []model.Term)) BuildOption {
	return func(b *Builder) { b.onUnresolvedUnion = fn }
}

// WithAliasReporter installs callback for alias metadata preservation.
// Alias nodes materialize to their target runtime type, while this callback
// exposes alias declaration metadata to callers.
func WithAliasReporter(fn func(alias *model.Alias, resolved reflect.Type)) BuildOption {
	return func(b *Builder) { b.onAlias = fn }
}
