// Package model defines an intermediate, reflect-agnostic representation of
// Go types used by the loader and code generation pipeline. It provides
// Nodes (basic, named, pointer, slice, map, func, interface, struct, union),
// type parameters for generics, containers for packages/files, and helpers to
// render go/ast declarations and formatted source.
//
// Key types
//   - Type: single declaration with AST TypeSpec, optional ReflectType link,
//     TypeParams for generics, MethodSet for programmatic access to value and
//     pointer receiver methods, and AST-captured method stubs for exact stub
//     rendering.
//   - GoFile/Package: containers with import merging and rendering helpers.
//   - Namespace: minimal aggregator for file-level rendering across types.
package model
