// Package xreflect provides a stateful builder that materializes runtime
// reflect.Type values from the syntetic/model Node graph. It complements the
// loader/xreflect package (which reads from reflect) by handling the inverse
// direction: model → reflect.
//
// Limitations
//   - New named types cannot be created at runtime; Named nodes are resolved
//     via a Resolver to existing compiled types. When unresolved, the builder
//     falls back to a policy-driven Unknown handler (by default: interface{}).
//     Resolution precedence for model.Named is:
//   1. Resolver/cache hit
//   2. Underlying node materialization (if provided)
//   3. strict-mode error or tolerant-mode unknown fallback
//     Strict behavior is enabled with WithStrictNamedResolution(true).
//     Optional diagnostics callback can be attached with
//     WithUnresolvedNamedReporter.
//   - Anonymous recursive composites (e.g., a dynamic struct that refers to
//     itself) are rejected by default. Tolerant mode can be enabled with
//     WithAllowAnonymousRecursion(true), which resolves recursive edges via
//     unknown fallback and optionally reports synthetic recursion identifiers
//     via WithAnonymousRecursionReporter.
//   - Generics and union constraints have no runtime representation and are
//     represented as unknown fallback in tolerant mode, or rejected in strict
//     mode via WithStrictUnionResolution(true). Diagnostics callback is
//     available via WithUnresolvedUnionReporter.
//   - Alias nodes materialize to their target runtime type. Alias declaration
//     metadata can be observed via WithAliasReporter for codegen/docs layers.
//   - Methods cannot be attached to dynamic structs; InterfaceOf support is
//     limited and policy-gated. Non-empty model.Interface nodes are resolved
//     through an optional InterfaceResolver. WithStrictInterfaceResolution(true)
//     makes unresolved non-empty interfaces fail fast; tolerant mode falls back
//     to interface{}.
package xreflect
