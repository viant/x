package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// RenderMethodStub renders a method declaration signature with an empty body.
// It expects a parsed *ast.FuncDecl.
func RenderMethodStub(fd *ast.FuncDecl) string {
	if fd == nil {
		return ""
	}
	// Build a shallow copy with empty body.
	clone := *fd
	clone.Body = &ast.BlockStmt{List: nil}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), &clone)
	return buf.String()
}

// MethodStubs returns rendered stubs for value and pointer receiver methods.
func (t *Type) MethodStubs() []string {
	var out []string
	for _, m := range t.MethodsAST {
		if s := RenderMethodStub(m); s != "" {
			out = append(out, s)
		}
	}
	for _, m := range t.PtrMethodsAST {
		if s := RenderMethodStub(m); s != "" {
			out = append(out, s)
		}
	}
	return out
}
