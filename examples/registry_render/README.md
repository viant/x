# Registry → File Rendering Example

This example shows how to:

- Define some Go types
- Register them in `x.Registry`
- Convert the registry to a single `syntetic/model.GoFile`
- Render the file to Go source

Files:

- `main.go` – builds a registry from local types, bridges to a file with `syntetic.FromRegistryFile`, and prints the rendered source.

Run:

```bash
go run ./examples/registry_render
```

You should see a formatted Go source file printed to stdout with the generated type declarations under package `example`.
