# x — Types Registry + AST/Reflect Reconciliation

This project provides a small set of building blocks to reconcile Go types
across three views of the world:

- Runtime: a lightweight types registry (`package x`) holding reflect types and metadata
- Source/AST: a synthetic model (`syntetic/model`) used to render compilable Go
- Reflection: a reflect→AST bridge (`loader/xreflect`) for runtime‑discovered types

The intent is to make code generation and component composition ergonomic whether
types originate from source (AST) or from runtime registries. You can attach
synthetic model types directly to registry entries or build them from reflect
as needed. A thin bridge then aggregates types and emits Go files with
deterministic imports.

Note: An early “placeholder module” workflow (for Go’s plugin story) remains
possible via the `transient` submodule, but the primary focus is the registry +
AST/reflect reconciliation.

## Quick Start

Register types and attach a prebuilt synthetic type for codegen:

```go
package main

import (
    "reflect"
    x "github.com/viant/x"
    "github.com/viant/x/loader/xreflect"
    "github.com/viant/x/syntetic"
)

type Person struct{ ID int; Name string }

func main() {
    reg := x.NewRegistry()
    st, _ := xreflect.BuildType(reflect.TypeOf(Person{}))
    reg.Register(x.NewType(reflect.TypeOf(Person{}), x.WithSynteticType(st)))

    // Aggregate into a single file and render
    file, _ := syntetic.FromRegistryFile(reg)
    file.PkgName = "example"
    src, _ := file.Render()
    _ = src // write to disk or compile
}
```

See `examples/registry_render` for a complete runnable example.

## Components

- `package x` (runtime)
  - `Registry` with listener hooks and merge support
  - `Type` wrapper with `SynteticType *model.Type` for attached synthetic types
  - Options: `WithListener`, `WithMergeListener`, `WithSynteticType`
- `syntetic/model` (source model + rendering)
  - Go type model, `GoFile`/`Package`/`Module`, deterministic imports
  - Renders valid Go via `Render/RenderWithOptions`
- `loader/ast` (source loader)
  - Parses module/packages from fs.FS; discovers types, funcs, consts, vars, embeds
- `loader/xreflect` (reflect loader)
  - `BuildType` (single reflect.Type → model.Type)
  - `LoadPackage` (group reflect.Types → model.Package)
- `syntetic` (bridge)
  - `FromRegistry`/`FromRegistryFile` collect registry types into a Namespace/GoFile
  - Uses `SynteticType` when present, otherwise falls back to `xreflect.BuildType`

## Why two loaders?

- Use `loader/ast` for full fidelity from source (generics, aliases, methods,
  top‑level funcs/consts/vars, embeds).
- Use `loader/xreflect` when you only have runtime types and need to produce
  declarations; it models structural shapes and exported method sets.

## Transient module (optional)

You can still wire a transient module into your workspace to make extension
types loadable with current Go tooling. This remains optional and may become
less relevant once Go improves plugin support.

## CI and Local Development

- CI: The canonical `go.mod` references published module versions (no local replaces).
- Local dev against sibling modules (e.g., a local checkout of `github.com/viant/afs`):
  - Preferred: use Go workspaces (`go work init . ../afs`) to link local copies.
  - Alternative: temporarily add `go mod edit -replace github.com/viant/afs=../afs` and drop it before committing, or use a separate `-modfile`.
- See `DEVELOPMENT.md` for detailed workflows and examples (including `go.work.example`).

## Merge Listener Semantics (example)

`WithMergeListener` installs a factory for a per-merge listener. During `Merge`, the returned listener is invoked once per merged type and once more with `nil` to signal completion.

```go
package main

import (
    "fmt"
    "reflect"
    x "github.com/viant/x"
)

func example() {
    // Source registry with a couple of types.
    from := x.NewRegistry()
    from.Register(x.NewType(reflect.TypeOf(struct{ A int }{})))
    from.Register(x.NewType(reflect.TypeOf(struct{ B string }{})))

    // Destination registry with a per-merge listener.
    to := x.NewRegistry()
    var keys []string
    done := false
    x.WithMergeListener(func() x.Listener {
        return func(t *x.Type) {
            if t == nil {
                // End-of-merge sentinel.
                done = true
                return
            }
            keys = append(keys, t.Key())
        }
    })(to) // apply option to existing registry

    to.Merge(from)
    fmt.Println("merged keys:", keys)
    fmt.Println("completed:", done)
}
```

## Notes on fidelity and transforms

- AST loader (`loader/ast`) preserves source intent (generics, type sets,
  aliases vs definitions, functions/consts/vars, embeds) and is best for
  end‑to‑end codegen.
- Reflect loader (`loader/xreflect`) focuses on structural shapes of types
  and exported method sets; it’s perfect for runtime registries and quick
  declarations but does not capture everything available in source.
- The `syntetic/model/transform` package can adapt or rewrite the model prior
  to rendering (e.g., tags, renames, type substitutions).
