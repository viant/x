package shape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
)

// SourceFieldTypeUpdate authorizes only the named, direct struct field's exact
// generated type. TypeExpr uses the generated source's import context. Existing
// tags remain destination-owned, including when the type is already identical.
type SourceFieldTypeUpdate struct {
	Owner, Field, TypeExpr string
}

// UpdateStructFields appends fields and applies exact, opt-in field type changes.
// Unlisted fields retain AppendStructFields conflict protection. It preserves
// destination bytes outside changed type expressions and necessary import edits;
// it does not replace declarations, tags, comments, methods or field order.
func (SourceParser) UpdateStructFields(existing, generated []byte, updates []SourceFieldTypeUpdate) ([]byte, error) {
	return (SourceParser{}).EditStructFields(existing, generated, SourceFieldEdits{Types: updates})
}

func (m *sourceAppend) prepareUpdates() error {
	if len(m.updates) == 0 {
		return nil
	}
	m.authorized = map[string]string{}
	m.retiredImports = map[string]bool{}
	for _, update := range m.updates {
		key := update.Owner + "." + update.Field
		if !token.IsIdentifier(update.Owner) || !token.IsIdentifier(update.Field) || update.Owner == "_" || update.Field == "_" {
			return fmt.Errorf("invalid exact shape field update %s", key)
		}
		expression, err := parser.ParseExpr(update.TypeExpr)
		if err != nil {
			return fmt.Errorf("shape field update %s: %w", key, err)
		}
		expected, err := m.canonical(expression, m.newImports)
		if err != nil {
			return err
		}
		if previous, ok := m.authorized[key]; ok && previous != expected {
			return fmt.Errorf("conflicting exact shape field updates for %s", key)
		}
		m.authorized[key] = expected
	}
	matched := map[string]bool{}
	for _, declaration := range m.newFile.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, item := range group.Specs {
			definition := item.(*ast.TypeSpec)
			structure, ok := definition.Type.(*ast.StructType)
			if !ok || definition.Assign.IsValid() {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					key := definition.Name.Name + "." + name.Name
					expected, ok := m.authorized[key]
					if !ok {
						continue
					}
					actual, err := m.canonical(field.Type, m.newImports)
					if err != nil {
						return err
					}
					if actual != expected {
						return fmt.Errorf("shape field update %s does not match exact generated type", key)
					}
					matched[key] = true
				}
			}
		}
	}
	for key := range m.authorized {
		if !matched[key] {
			return fmt.Errorf("shape field update %s has no direct generated field", key)
		}
	}
	return nil
}

func (m *sourceAppend) updateField(owner, name string, previous, generated *ast.Field) error {
	if len(previous.Names) != 1 || len(generated.Names) != 1 {
		return fmt.Errorf("shape field update %s.%s requires an individually named field; grouped or embedded fields require explicit migration", owner, name)
	}
	text, err := m.newText(generated.Type, generated.Type.Pos(), generated.Type.End())
	if err != nil {
		return err
	}
	m.insertions = append(m.insertions, sourceInsertion{
		offset: m.oldSet.Position(previous.Type.Pos()).Offset,
		end:    m.oldSet.Position(previous.Type.End()).Offset, text: text,
	})
	ast.Inspect(previous.Type, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok {
			if qualifier, ok := selector.X.(*ast.Ident); ok {
				m.retiredImports[qualifier.Name] = true
			}
		}
		return true
	})
	return nil
}

// Remove only imports made unused by an authorized type change. Existing
// unrelated imports and comments remain byte-for-byte destination-owned.
func (m *sourceAppend) pruneRetiredImports(content []byte) ([]byte, error) {
	if len(m.retiredImports) == 0 {
		return content, nil
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "updated.go", content, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok {
			if qualifier, ok := selector.X.(*ast.Ident); ok && qualifier.Obj == nil {
				used[qualifier.Name] = true
			}
		}
		return true
	})
	var removals []sourceInsertion
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.IMPORT {
			continue
		}
		for _, item := range group.Specs {
			imported := item.(*ast.ImportSpec)
			alias := ""
			for name, location := range m.imports(&ast.File{Imports: []*ast.ImportSpec{imported}}) {
				if m.oldImports[name] == location {
					alias = name
				}
			}
			if !m.retiredImports[alias] || used[alias] {
				continue
			}
			start := imported.Pos()
			if !group.Lparen.IsValid() {
				start = group.Pos()
				// An explicit import semicolon belongs to the declaration;
				// preserve intervening comments while removing that token too.
				end := set.Position(imported.End()).Offset
				suffix := content[end:]
				scanSet := token.NewFileSet()
				scanFile := scanSet.AddFile("suffix.go", -1, len(suffix))
				var lexical scanner.Scanner
				lexical.Init(scanFile, suffix, nil, 0)
				position, kind, literal := lexical.Scan()
				if kind == token.SEMICOLON && literal == ";" {
					offset := end + scanFile.Offset(position)
					removals = append(removals, sourceInsertion{offset: offset, end: offset + 1})
				}
			}
			removals = append(removals, sourceInsertion{offset: set.Position(start).Offset, end: set.Position(imported.End()).Offset})
		}
	}
	sort.Slice(removals, func(i, j int) bool { return removals[i].offset > removals[j].offset })
	for _, removal := range removals {
		content = append(content[:removal.offset], content[removal.end:]...)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "updated.go", content, parser.SkipObjectResolution); err != nil {
		return nil, fmt.Errorf("updated shape imports: %w", err)
	}
	return content, nil
}
