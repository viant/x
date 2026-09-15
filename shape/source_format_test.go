package shape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestSourceParserFormatFileProjectsPositionlessDeclarationDocs(t *testing.T) {
	file := &ast.File{
		Name: ast.NewIdent("write"),
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.TYPE,
				Doc: &ast.CommentGroup{List: []*ast.Comment{{Text: "// OrderLifecycle customizes role Input.Orders."}}},
				Specs: []ast.Spec{&ast.TypeSpec{
					Name: ast.NewIdent("OrderLifecycle"),
					Type: &ast.StructType{Fields: &ast.FieldList{}},
				}},
			},
			&ast.FuncDecl{
				Doc:  &ast.CommentGroup{List: []*ast.Comment{{Text: "// BackfillWindowIfNeeded hydrates omitted group values without marking presence."}}},
				Name: ast.NewIdent("BackfillWindowIfNeeded"),
				Type: &ast.FuncType{Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent("error")}}}},
				Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("nil")}}}},
			},
		},
	}
	source, err := (SourceParser{}).FormatFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "package //") {
		t.Fatalf("declaration docs leaked into package clause:\n%s", text)
	}
	for _, want := range []string{
		"package write\n\n// OrderLifecycle customizes role Input.Orders.\ntype OrderLifecycle struct",
		"// BackfillWindowIfNeeded hydrates omitted group values without marking presence.\nfunc BackfillWindowIfNeeded() error",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing formatted fragment %q in:\n%s", want, text)
		}
	}
	if _, err = parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.ParseComments); err != nil {
		t.Fatalf("formatted source does not parse: %v\n%s", err, text)
	}
}

func TestSourceParserFormatFileDoesNotMutateParserBackedCommentInventory(t *testing.T) {
	const input = "// Package sample documents the package.\npackage sample\n\n// Exported documents the function.\nfunc Exported() {}\n\n// trailing note\n"
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", input, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	before := commentInventory(file)
	source, err := (SourceParser{}).FormatFile(file)
	if err != nil {
		t.Fatal(err)
	}
	after := commentInventory(file)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("caller comment inventory mutated:\nbefore=%v\nafter=%v", before, after)
	}
	text := string(source)
	for _, want := range []string{
		"// Package sample documents the package.",
		"// Exported documents the function.",
		"// trailing note",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted source dropped %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "package //") {
		t.Fatalf("declaration doc leaked into package clause:\n%s", text)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("formatted source does not parse: %v\n%s", err, text)
	}
	if got := commentTexts(parsed); !reflect.DeepEqual(got, []string{
		"Package sample documents the package.\n",
		"Exported documents the function.\n",
		"trailing note\n",
	}) {
		t.Fatalf("formatted comment inventory=%q", got)
	}
}

func TestSourceParserFormatFilePreservesLicenseBuildFreeInlineAndTrailingComments(t *testing.T) {
	const input = `// Copyright 2026
//go:build linux
// +build linux

// Package sample documents the package.
package sample

// free note before import
import "fmt"

// free note before type
// Exported documents the type.
type Exported struct {
	Name string // inline field note
}

// trailing note
`
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", input, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	before := commentInventory(file)
	source, err := (SourceParser{}).FormatFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if after := commentInventory(file); !reflect.DeepEqual(before, after) {
		t.Fatalf("caller comment inventory mutated:\nbefore=%v\nafter=%v", before, after)
	}
	text := string(source)
	for _, want := range []string{
		"// Copyright 2026",
		"//go:build linux",
		"// +build linux",
		"// Package sample documents the package.",
		"// free note before import",
		"// free note before type",
		"// Exported documents the type.",
		"// inline field note",
		"// trailing note",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted source dropped %q:\n%s", want, text)
		}
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("formatted source does not parse: %v\n%s", err, text)
	}
	if doc := typeDoc(parsed, "Exported"); !strings.Contains(doc, "Exported documents the type") {
		t.Fatalf("type doc not bound: %q\n%s", doc, text)
	}
}

func TestSourceParserFormatFileRepeatedConcurrentCallsDoNotMutate(t *testing.T) {
	const input = "// Package sample documents the package.\npackage sample\n\n// Exported documents the function.\nfunc Exported() { println(\"ok\") }\n\n// trailing note\n"
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", input, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	before := commentInventory(file)
	first, err := (SourceParser{}).FormatFile(file)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	const iterations = 25
	var wg sync.WaitGroup
	errs := make(chan string, workers*iterations)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				source, err := (SourceParser{}).FormatFile(file)
				if err != nil {
					errs <- err.Error()
					continue
				}
				if string(source) != string(first) {
					errs <- "formatted source changed between calls"
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if after := commentInventory(file); !reflect.DeepEqual(before, after) {
		t.Fatalf("caller comment inventory mutated:\nbefore=%v\nafter=%v", before, after)
	}
}

func commentInventory(file *ast.File) []string {
	result := make([]string, 0, len(file.Comments))
	for _, group := range file.Comments {
		result = append(result, fmt.Sprintf("%p:%s", group, group.Text()))
	}
	return result
}

func commentTexts(file *ast.File) []string {
	result := make([]string, 0, len(file.Comments))
	for _, group := range file.Comments {
		result = append(result, group.Text())
	}
	return result
}

func typeDoc(file *ast.File, name string) string {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Doc == nil {
			continue
		}
		for _, spec := range general.Specs {
			typed, ok := spec.(*ast.TypeSpec)
			if ok && typed.Name.Name == name {
				return general.Doc.Text()
			}
		}
	}
	return ""
}
