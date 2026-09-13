package shape

import (
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"strconv"
)

// SourceFieldRemoval identifies one previously owned direct field. TypeExpr is
// its canonical package-qualified type, and Tag is its prior unquoted Go tag.
// Removal fails if the existing field has changed or remains in generated source.
type SourceFieldRemoval struct {
	Owner, Field, TypeExpr, Tag string
}

// SourceFieldEdits authorizes exact type changes and exact guarded removals.
// Empty edits retain AppendStructFields behavior.
type SourceFieldEdits struct {
	Types  []SourceFieldTypeUpdate
	Tags   []SourceFieldTagUpdate
	Remove []SourceFieldRemoval
}

// EditStructFields preserves unrelated destination bytes while applying exact
// field authority. Removed fields' comments remain destination-owned. It never
// deletes whole types, methods, or unlisted fields.
func (SourceParser) EditStructFields(existing, generated []byte, edits SourceFieldEdits) ([]byte, error) {
	return (&sourceAppend{existing: existing, generated: generated, updates: edits.Types, removals: edits.Remove, tagUpdates: edits.Tags}).merge()
}

func (m *sourceAppend) removeFields() error {
	if len(m.removals) == 0 {
		return nil
	}
	requests := map[string]SourceFieldRemoval{}
	for _, request := range m.removals {
		key := request.Owner + "." + request.Field
		if !token.IsIdentifier(request.Owner) || !token.IsIdentifier(request.Field) || request.Owner == "_" || request.Field == "_" || request.TypeExpr == "" {
			return fmt.Errorf("invalid exact shape field removal %s", key)
		}
		if _, ok := m.authorized[key]; ok {
			return fmt.Errorf("shape field %s cannot be updated and removed", key)
		}
		if previous, ok := requests[key]; ok && previous != request {
			return fmt.Errorf("conflicting exact shape field removals for %s", key)
		}
		requests[key] = request
	}
	// Both declaration inventories are parser-owned. Old declarations absent
	// from generated source may remain as customized types with their methods.
	for _, file := range []*ast.File{m.newFile, m.oldFile} {
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, item := range group.Specs {
				definition := item.(*ast.TypeSpec)
				structure, ok := definition.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structure.Fields.List {
					for _, name := range m.fieldNames(field) {
						key := definition.Name.Name + "." + name
						request, ok := requests[key]
						if !ok {
							continue
						}
						if file == m.newFile {
							return fmt.Errorf("shape field removal %s remains in generated source", key)
						}
						if definition.Assign.IsValid() || len(field.Names) != 1 {
							return fmt.Errorf("shape field removal %s requires an individually named direct field", key)
						}
						actual, err := m.canonical(field.Type, m.oldImports)
						if err != nil {
							return err
						}
						expected, err := (Resolver{}).Canonical(request.TypeExpr)
						if err != nil {
							return err
						}
						tag := ""
						if field.Tag != nil {
							tag, err = strconv.Unquote(field.Tag.Value)
							if err != nil {
								return err
							}
						}
						if actual != expected || tag != request.Tag {
							return fmt.Errorf("shape field removal %s conflicts with customized type or tag", key)
						}
						start, end := m.oldSet.Position(field.Pos()).Offset, m.oldSet.Position(field.End()).Offset
						m.insertions = append(m.insertions, sourceInsertion{offset: start, end: end})
						// Consume only an explicit field separator; preserve intervening comments.
						suffix := m.existing[end:]
						set := token.NewFileSet()
						scanFile := set.AddFile("suffix.go", -1, len(suffix))
						var lexical scanner.Scanner
						lexical.Init(scanFile, suffix, nil, 0)
						position, kind, literal := lexical.Scan()
						if kind == token.SEMICOLON && literal == ";" {
							offset := end + scanFile.Offset(position)
							m.insertions = append(m.insertions, sourceInsertion{offset: offset, end: offset + 1})
						}
						if m.retiredImports == nil {
							m.retiredImports = map[string]bool{}
						}
						ast.Inspect(field.Type, func(node ast.Node) bool {
							if selector, ok := node.(*ast.SelectorExpr); ok {
								if qualifier, ok := selector.X.(*ast.Ident); ok {
									m.retiredImports[qualifier.Name] = true
								}
							}
							return true
						})
						delete(requests, key)
					}
				}
			}
		}
	}
	// Already absent fields are a no-op, supporting repeatable persistence.
	return nil
}
