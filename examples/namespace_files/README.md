# Namespace → Per-Package Files Example

This example shows how to:

- Register types from multiple packages
- Build a `syntetic/model.Namespace` from the registry
- Generate a `GoFile` per package with `Namespace.BuildFiles`
- Write the rendered files under `./gen/<pkg-alias>/types_gen.go`

Run:

```bash
go run ./examples/namespace_files
```

You should see two files written:

- `./gen/customer/types_gen.go` (package `customer`)
- `./gen/orders/types_gen.go` (package `orders`)

These files contain type declarations for the registered types, with imports
assigned deterministically.

