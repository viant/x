package model

import "go/ast"

// Function describes a free-standing function in a package.
type Function struct {
	Name string
	Type Func
	Decl *ast.FuncDecl
	File string // optional source filename (base)
}
