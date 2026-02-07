package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// ConstDecl represents a single constant declaration.
// Type and/or Value may be nil; at least one should be provided to
// produce a meaningful declaration.
type ConstDecl struct {
	Name    string
	Type    ast.Expr // optional
	Value   ast.Expr // optional
	Imports map[string]ImportRef
}

// VarDecl represents a single variable declaration.
type VarDecl struct {
	Name    string
	Type    ast.Expr // optional
	Value   ast.Expr // optional
	Imports map[string]ImportRef
}

func renderExpr(e ast.Expr) string {
	if e == nil {
		return ""
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}
