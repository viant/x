// Package xreflect provides a stateful builder that materializes runtime
// reflect.Type values from the syntetic/model Node graph. It complements the
// loader/xreflect package (which reads from reflect) by handling the inverse
// direction: model → reflect.
//
// Limitations
//   - New named types cannot be created at runtime; Named nodes are resolved
//     via a Resolver to existing compiled types. When unresolved, the builder
//     falls back to a policy-driven Unknown handler (by default: interface{}).
//   - Anonymous recursive composites (e.g., a dynamic struct that refers to
//     itself) are not supported by reflect.StructOf and will be rejected.
//   - Generics and union constraints have no runtime representation and are
//     ignored by the builder.
//   - Methods cannot be attached to dynamic structs; InterfaceOf support is
//     limited and policy-gated.
package xreflect
