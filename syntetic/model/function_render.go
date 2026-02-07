package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// RenderFunctionStub renders a free-standing function declaration signature with an empty body.
func RenderFunctionStub(fd *ast.FuncDecl) string {
	if fd == nil {
		return ""
	}
	clone := *fd
	clone.Body = &ast.BlockStmt{List: nil}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), &clone)
	return buf.String()
}
