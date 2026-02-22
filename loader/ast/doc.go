// Package loader provides utilities to parse Go packages and modules into
// an intermediate model suitable for code generation. It discovers modules,
// loads package files, extracts types, globals, methods, free functions, and
// embedded resources, and converts Go AST into the model's Node graph.
// Methods are captured both as AST (for stub rendering) and as model-level
// signatures under Type.Methods (value/pointer receivers) for programmatic use.
//
// Primary entrypoints
//   - LoadPackageDeepFS: recursively loads in-module dependencies from any fs.FS
//     and optionally resolves external imports via DeepOptions.ResolveExternal.
//   - LoadExternalModule: start from an import path and a resolver that maps
//     import paths to (fs.FS, dir); use this to deep-load external modules.
//
// Thin helpers
//   - LoadPackageDeepOS: convenience wrapper around the OS filesystem.
//   - LoadPackageDeepGOPATH: GOPATH-based deep loader (directory-based; deprecated in favour of LoadExternalModuleGOPATH when starting from an import path).
//   - LoadPackageDeepAFS (viant_afs build): deep loader backed by an AFS service.
//   - LoadExternalModuleGOPATH: start from import path using GOPATHResolver.
//   - LoadExternalModuleAFSGOPATH (viant_afs build): start from import path using AFSGOPATHResolver.
//
// Resolver helpers
//   - GOPATHResolver(gopath): maps import paths to os.DirFS("/") + $GOPATH/src/<import>.
//   - AFSGOPATHResolver (viant_afs build): maps import paths under an AFS baseURL
//     (e.g., mem://localhost/gopath/src/<import>).
//
// Examples demonstrate both shallow (LoadPackageFS) and deep loading. In most
// cases, prefer LoadPackageDeepFS with a ResolveExternal function (via
// DeepOptions) or use LoadExternalModule to start from an import path: this
// keeps the API minimal and backend-agnostic. GOPATH and AFS wrappers are
// provided as convenience.
package ast
