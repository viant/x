# Builder: AST and Reflect

This directory contains small, focused builders that operate on the syntetic/model types:

- `builder/ast`: façade for AST emission and rendering (delegates to `model`).
- `builder/xreflect`: materializes runtime `reflect.Type` shapes from `model.Node`.

Both builders accept optional transforms so you can modify the model graph (e.g., via tags) just before emission/building without mutating the canonical model.

## Common Transform: FromTags

The `model/transform` package provides a tag‑driven transformer that reads directives from a configurable tag key (default `x`) and applies rewrites to struct fields:

- `type=pkg/import/Path.Name`: override field type to a named type.
- `rename=New`: rename field for emission/materialization.
- `omit`: drop the field.
- `flat` or `inline`: mark embedded/flatten.
- `tag=raw:"tag"`: override (replace) the field tag.

Example (reflect builder):

```
n := &model.Struct{Fields: []model.Field{
  {Name: "A", Tag: `x:"omit"`, Type: &model.Basic{Name: "int"}},
  {Name: "B", Tag: `x:"rename=R,tag=json:"r",type=ex/p.Alias"`, Type: &model.Basic{Name: "string"}},
}}
res := myResolver // implements Resolve(pkgPath, name) -> reflect.Type
b := xreflect.New(xreflect.WithResolver(res), xreflect.WithTransforms(transform.FromTags()))
rt, _ := b.BuildNode(n)
// rt now has field: R uint16 `json:"r"` (assuming resolver mapped ex/p.Alias -> uint16)
```

## builder/xreflect (model → reflect)

- State: holds a `Resolver`, cache, unknown fallback, and policies.
- Options:
  - `WithResolver(Resolver)`: map `Named(PkgPath, Name)` to compiled `reflect.Type`.
  - `WithCache(map[string]reflect.Type)`: reuse named resolutions across builds.
  - `WithUnknownFallback(func() reflect.Type)`: defines fallback type (default `interface{}`).
  - `WithAllowInterface(bool)`: policy for interface method-set handling.
  - `WithTransforms(transform.Transformer)`: apply node/field/tag rewrites before build.
- Limits:
  - Cannot create new named defined types; must resolve existing ones.
  - Cannot attach methods to dynamic structs.
  - Anonymous recursive composites are not supported by `reflect.StructOf`.
  - No runtime generics/union constraints.

## builder/ast (model → AST)

- Façade over `model` emission helpers:
  - `TypeSpec/GenDecl`: build AST declarations.
  - `RenderFile` / `RenderPackage` / `WritePackage`: render sources.
- Options:
  - `WithTransforms(transform.Transformer)`: applies transforms to the package before rendering, and consults `DeclNameOverride` when building `TypeSpec/GenDecl`.
- Name override (printed declaration only):

```
b := ast.New(ast.WithTransforms(transform.DeclNameOverride(func(t *model.Type) (string, bool) {
  if t.Name == "Orig" { return "NewName", true }
  return "", false
})))
spec := b.TypeSpec(t, nil) // printed name is NewName; t.Name stays "Orig"
```

## Design notes

- Keep `model` pure; do not bake codegen semantics into types. Use transforms.
- Use builders to apply transforms late, just before emission/materialization.
- Prefer `ReflectType` on `model.Type`; `LinkedinType` remains only for backward compatibility.
- Use `RenderOptions.ImportAliases` to preserve/override import aliases in emitted code.

## Where to add new rules

- Tag and structural rewrites: `model/transform` (e.g., add a new transformer).
- Emission policies (ordering/format): `model` and `builder/ast`.
- Runtime type policies (resolver, interface handling): `builder/xreflect`.
