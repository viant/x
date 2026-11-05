# Linked type rewriting

`CompileRewrite(original)` prepares immutable **direct-field** traversal metadata.
Cache the plan per original type; `Rewrite(callback)` is safe for concurrent calls
when callbacks obey the read-only original metadata contract.

```go
plan, err := shape.CompileRewrite(reflect.TypeOf(Output{}))
effective, err := plan.Rewrite(func(parentPath string, parent reflect.Type,
    field *shape.FieldMetadata) (*shape.FieldMetadata, error) {
    if field.Name != "Secret" {
        return nil, nil
    }
    return &shape.FieldMetadata{Name: field.Name, Tag: `json:"-"`}, nil
})
```

Nil and equivalent replacements retain the original field. Every unchanged node
retains its exact linked `reflect.Type`; only changed ancestors are rebuilt.
Nil callbacks return the root immediately. Paths use original Go member names,
`[]` for slice/array elements and `{}` for map values. Embedded holders are explicit
in paths even when JSON promotes their members. Map keys, private non-embedded
storage and runtime interface implementations are not traversed.

Field storage types, order, offsets, package authority and Anonymous flags are
preserved. Renames must be exported valid identifiers without duplicate fields;
embedded holders cannot be renamed. No fields are removed. A retagged embedded
struct remains anonymous and pointer embeddings retain their nil behavior.

The effective type describes representation; it is **not** a replacement for the
application's named type or its method set. Consumers retain the original authority
and map values through their native codecs. No unsafe casting is performed here.
Changes to recursive type cycles and custom JSON/text-encoded nodes require explicit
codec projections and fail. Unchanged recursive/custom nodes remain linked. Other
runtime reflection restrictions produce errors, not partial rewritten contracts.

The callback owns tag policy and validation. This package does not infer permissions,
parse enums, decide requiredness, bind dependencies or cache caller variants.
