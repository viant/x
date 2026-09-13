package shape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
)

type SourceImport struct {
	Path, Alias string
	Explicit    bool
}
type SourceField struct {
	Owner         string
	Names         []string
	TypeExpr, Tag string
}
type Source struct {
	Package string
	Imports map[string]SourceImport
	Fields  []SourceField
}
type SourceParser struct{}

func (SourceParser) ParseFile(filename string) (*Source, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	result := &Source{Package: file.Name.Name, Imports: map[string]SourceImport{}}
	for _, imported := range file.Imports {
		location, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, err
		}
		alias := path.Base(location)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		result.Imports[alias] = SourceImport{Path: location, Alias: alias, Explicit: imported.Name != nil}
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, item := range general.Specs {
			definition := item.(*ast.TypeSpec)
			structure, ok := definition.Type.(*ast.StructType)
			if !ok || structure.Fields == nil {
				continue
			}
			for _, field := range structure.Fields.List {
				value := SourceField{Owner: definition.Name.Name, TypeExpr: rendered(field.Type)}
				for _, name := range field.Names {
					value.Names = append(value.Names, name.Name)
				}
				if field.Tag != nil {
					value.Tag, err = strconv.Unquote(field.Tag.Value)
					if err != nil {
						return nil, err
					}
				}
				result.Fields = append(result.Fields, value)
			}
		}
	}
	return result, nil
}
