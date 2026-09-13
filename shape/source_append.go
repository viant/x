package shape

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// AppendStructFields updates a generated shape without replacing existing
// declarations. Existing bytes, field order, tags and comments remain owned by
// the destination; incompatible definitions require an explicit migration.
func (SourceParser) AppendStructFields(existing, generated []byte) ([]byte, error) {
	m := &sourceAppend{existing: existing, generated: generated}
	return m.merge()
}

type sourceInsertion struct {
	offset int
	text   string
}
type sourceAppend struct {
	existing, generated    []byte
	oldSet, newSet         *token.FileSet
	oldFile, newFile       *ast.File
	oldImports, newImports map[string]string
	insertions             []sourceInsertion
	needed                 map[string]bool
}

func (m *sourceAppend) merge() ([]byte, error) {
	var err error
	m.oldSet, m.newSet = token.NewFileSet(), token.NewFileSet()
	m.oldFile, err = parser.ParseFile(m.oldSet, "existing.go", m.existing, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	m.newFile, err = parser.ParseFile(m.newSet, "generated.go", m.generated, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if m.oldFile.Name.Name != m.newFile.Name.Name {
		return nil, fmt.Errorf("shape package changed from %s to %s", m.oldFile.Name.Name, m.newFile.Name.Name)
	}
	m.oldImports, m.newImports = m.imports(m.oldFile), m.imports(m.newFile)
	m.needed = map[string]bool{}
	existingTypes := map[string]*ast.TypeSpec{}
	reserved := map[string]bool{}
	for _, declaration := range m.oldFile.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil {
			reserved[function.Name.Name] = true
		}
		if group, ok := declaration.(*ast.GenDecl); ok && (group.Tok == token.VAR || group.Tok == token.CONST) {
			for _, item := range group.Specs {
				for _, name := range item.(*ast.ValueSpec).Names {
					reserved[name.Name] = true
				}
			}
		}
		if group, ok := declaration.(*ast.GenDecl); ok && group.Tok == token.TYPE {
			for _, item := range group.Specs {
				definition := item.(*ast.TypeSpec)
				if existingTypes[definition.Name.Name] != nil {
					return nil, fmt.Errorf("duplicate existing shape type %s", definition.Name.Name)
				}
				existingTypes[definition.Name.Name] = definition
			}
		}
	}
	generatedTypes := map[string]bool{}
	for _, declaration := range m.newFile.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			return nil, fmt.Errorf("generated shape contains non-type declaration")
		}
		if group.Tok == token.IMPORT {
			continue
		}
		if group.Tok != token.TYPE {
			return nil, fmt.Errorf("generated shape contains non-type declaration")
		}
		for _, item := range group.Specs {
			definition := item.(*ast.TypeSpec)
			if generatedTypes[definition.Name.Name] {
				return nil, fmt.Errorf("duplicate generated shape type %s", definition.Name.Name)
			}
			generatedTypes[definition.Name.Name] = true
			previous := existingTypes[definition.Name.Name]
			if reserved[definition.Name.Name] {
				return nil, fmt.Errorf("shape type %s conflicts with existing package declaration", definition.Name.Name)
			}
			if previous == nil {
				text, err := m.newText(definition, definition.Pos(), definition.End())
				if err != nil {
					return nil, err
				}
				m.insertions = append(m.insertions, sourceInsertion{len(m.existing), "\n\ntype " + text + "\n"})
				continue
			}
			oldStruct, oldOK := previous.Type.(*ast.StructType)
			newStruct, newOK := definition.Type.(*ast.StructType)
			if !oldOK || !newOK || previous.Assign.IsValid() != definition.Assign.IsValid() {
				oldType, e1 := (Resolver{Imports: m.oldImports}).Canonical(previous.Type)
				newType, e2 := (Resolver{Imports: m.newImports}).Canonical(definition.Type)
				if e1 != nil || e2 != nil || oldType != newType || previous.Assign.IsValid() != definition.Assign.IsValid() {
					return nil, fmt.Errorf("shape type %s changed; explicit migration required", definition.Name.Name)
				}
				continue
			}
			oldParameters, err := m.parameters(previous.TypeParams, m.oldImports)
			if err != nil {
				return nil, err
			}
			newParameters, err := m.parameters(definition.TypeParams, m.newImports)
			if err != nil {
				return nil, err
			}
			if oldParameters != newParameters {
				return nil, fmt.Errorf("shape type parameters changed for %s", definition.Name.Name)
			}
			if err = m.fields(definition.Name.Name, oldStruct, newStruct); err != nil {
				return nil, err
			}
		}
	}
	if err = m.addImports(); err != nil {
		return nil, err
	}
	if len(m.insertions) == 0 {
		return append([]byte(nil), m.existing...), nil
	}
	sort.SliceStable(m.insertions, func(i, j int) bool { return m.insertions[i].offset < m.insertions[j].offset })
	var result bytes.Buffer
	position := 0
	for _, insertion := range m.insertions {
		result.Write(m.existing[position:insertion.offset])
		result.WriteString(insertion.text)
		position = insertion.offset
	}
	result.Write(m.existing[position:])
	if _, err = parser.ParseFile(token.NewFileSet(), "merged.go", result.Bytes(), parser.SkipObjectResolution); err != nil {
		return nil, fmt.Errorf("merged shape: %w", err)
	}
	return result.Bytes(), nil
}

func (m *sourceAppend) parameters(fields *ast.FieldList, imports map[string]string) (string, error) {
	if fields == nil {
		return "", nil
	}
	var result strings.Builder
	for _, field := range fields.List {
		for _, name := range field.Names {
			result.WriteString(name.Name)
			result.WriteByte(',')
		}
		canonical, err := (Resolver{Imports: imports}).Canonical(field.Type)
		if err != nil {
			return "", err
		}
		result.WriteString(canonical)
		result.WriteByte(';')
	}
	return result.String(), nil
}

func (m *sourceAppend) imports(file *ast.File) map[string]string {
	result := map[string]string{}
	for _, item := range file.Imports {
		location, _ := strconv.Unquote(item.Path.Value)
		alias := path.Base(location)
		if item.Name != nil {
			alias = item.Name.Name
		}
		result[alias] = location
	}
	return result
}

func (m *sourceAppend) fields(owner string, previous, generated *ast.StructType) error {
	fields := map[string]*ast.Field{}
	for _, field := range previous.Fields.List {
		for _, name := range m.fieldNames(field) {
			if fields[name] != nil {
				return fmt.Errorf("duplicate existing shape field %s.%s", owner, name)
			}
			fields[name] = field
		}
	}
	var additions strings.Builder
	generatedNames := map[string]bool{}
	for _, field := range generated.Fields.List {
		missing := 0
		for _, name := range m.fieldNames(field) {
			if generatedNames[name] {
				return fmt.Errorf("duplicate generated shape field %s.%s", owner, name)
			}
			generatedNames[name] = true
			old := fields[name]
			if old == nil {
				missing++
				continue
			}
			oldType, err := (Resolver{Imports: m.oldImports}).Canonical(old.Type)
			if err != nil {
				return err
			}
			newType, err := (Resolver{Imports: m.newImports}).Canonical(field.Type)
			if err != nil {
				return err
			}
			oldTag, newTag := "", ""
			if old.Tag != nil {
				oldTag, _ = strconv.Unquote(old.Tag.Value)
			}
			if field.Tag != nil {
				newTag, _ = strconv.Unquote(field.Tag.Value)
			}
			if oldType != newType || oldTag != newTag {
				return fmt.Errorf("shape field %s.%s has conflicting type or tag; explicit migration required", owner, name)
			}
		}
		if missing == 0 {
			continue
		}
		if missing != len(m.fieldNames(field)) {
			return fmt.Errorf("shape field group in %s partially overlaps existing fields", owner)
		}
		start, end := field.Pos(), field.End()
		if field.Doc != nil {
			start = field.Doc.Pos()
		}
		if field.Comment != nil {
			end = field.Comment.End()
		}
		text, err := m.newText(field, start, end)
		if err != nil {
			return err
		}
		additions.WriteString("\n\t")
		additions.WriteString(text)
		additions.WriteString("\n")
	}
	if additions.Len() != 0 {
		m.insertions = append(m.insertions, sourceInsertion{m.oldSet.Position(previous.Fields.Closing).Offset, additions.String()})
	}
	return nil
}

func (m *sourceAppend) fieldNames(field *ast.Field) []string {
	if len(field.Names) == 0 {
		reference, _ := (Resolver{}).Reference(rendered(field.Type))
		return []string{reference.BaseName}
	}
	result := make([]string, len(field.Names))
	for i, name := range field.Names {
		result[i] = name.Name
	}
	return result
}

func (m *sourceAppend) newText(node ast.Node, start, end token.Pos) (string, error) {
	begin, finish := m.newSet.Position(start).Offset, m.newSet.Position(end).Offset
	var edits []struct {
		start, end int
		text       string
	}
	var failure error
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		location, ok := m.newImports[qualifier.Name]
		if !ok {
			failure = fmt.Errorf("cannot resolve generated type qualifier %s: use an explicit import alias matching its package identifier", qualifier.Name)
			return false
		}
		alias := qualifier.Name
		for existing, imported := range m.oldImports {
			if imported == location && existing != "_" && existing != "." {
				alias = existing
				break
			}
		}
		if imported, found := m.oldImports[alias]; found && imported != location {
			failure = fmt.Errorf("shape import alias %s conflicts with existing import", alias)
			return false
		}
		m.needed[qualifier.Name] = true
		if alias != qualifier.Name {
			edits = append(edits, struct {
				start, end int
				text       string
			}{m.newSet.Position(qualifier.Pos()).Offset, m.newSet.Position(qualifier.End()).Offset, alias})
		}
		return true
	})
	if failure != nil {
		return "", failure
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var result strings.Builder
	position := begin
	for _, edit := range edits {
		result.Write(m.generated[position:edit.start])
		result.WriteString(edit.text)
		position = edit.end
	}
	result.Write(m.generated[position:finish])
	return result.String(), nil
}

func (m *sourceAppend) addImports() error {
	var lines []string
	for alias := range m.needed {
		location := m.newImports[alias]
		found := false
		for existing, imported := range m.oldImports {
			if imported == location {
				if existing == "_" || existing == "." {
					return fmt.Errorf("shape import %s already has unsupported alias %s", location, existing)
				}
				found = true
				break
			}
		}
		if found {
			continue
		}
		lines = append(lines, "import "+alias+" "+strconv.Quote(location)+"\n")
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Strings(lines)
	// The Go scanner owns explicit/implicit semicolons and package comments.
	// A package and declaration may legally share one line.
	set := token.NewFileSet()
	file := set.AddFile("existing.go", -1, len(m.existing))
	var lexical scanner.Scanner
	lexical.Init(file, m.existing, nil, 0)
	offset := len(m.existing)
	for {
		position, kind, _ := lexical.Scan()
		if kind == token.SEMICOLON {
			offset = file.Offset(position)
			if offset < len(m.existing) {
				offset++
			}
			break
		}
		if kind == token.EOF {
			break
		}
	}
	m.insertions = append(m.insertions, sourceInsertion{offset, "\n" + strings.Join(lines, "") + "\n"})
	return nil
}
