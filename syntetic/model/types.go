package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"reflect"
)

// ImportRef represents a single import needed by a synthetic type.
// Path is the fully-qualified import path. Alias is optional; when empty,
// callers are expected to rely on the package's default name.
type ImportRef struct {
	Path  string
	Alias string
}

// Key returns a deterministic key combining path and alias used for
// deduplication across collections.
func (r ImportRef) Key() string { return r.Path + " " + r.Alias }

// Type is a synthetic/codegen description of a Go type.
type Type struct {
	Name    string // logical type name
	PkgPath string // logical package path

	// ReflectType is an optional reference to an existing compiled type
	// from a runtime/registry layer.
	ReflectType reflect.Type
	// LinkedinType is deprecated; use ReflectType. Kept for one cycle of
	// backward compatibility.
	LinkedinType reflect.Type

	// TypeSpec is the AST source of truth for the type.
	TypeSpec *ast.TypeSpec

	// Imports lists imports needed for this type's declaration,
	// keyed by alias ("" for default).
	Imports map[string]*ImportRef

	// bodyCache is a cached rendered declaration fragment, e.g.
	// "struct { Bar *other.P }".
	bodyCache string

	// MethodsAST holds parsed method declarations with a value receiver of this type.
	MethodsAST []*ast.FuncDecl
	// PtrMethodsAST holds parsed method declarations with a pointer receiver of this type.
	PtrMethodsAST []*ast.FuncDecl

	// Methods holds parsed method signatures attached to this type. Value
	// contains methods with a value receiver; Pointer contains methods with a
	// pointer receiver. This mirrors the AST lists and provides a model-level
	// view for programmatic access.
	Methods MethodSet

	// TypeParams declares generic type parameters for this type declaration.
	TypeParams []TypeParam
}

// MethodSet groups value and pointer receiver methods for a named type.
type MethodSet struct {
	Value   []Method
	Pointer []Method
}

// Body returns the Go syntax fragment that forms the right-hand side
// of the type declaration. The result is cached internally.
func (t *Type) Body() string {
	if t == nil {
		return ""
	}
	if t.bodyCache != "" {
		return t.bodyCache
	}
	if t.TypeSpec == nil || t.TypeSpec.Type == nil {
		return ""
	}
	fragment := RenderTypeBodyFromSpec(t.TypeSpec)
	t.bodyCache = fragment
	return fragment
}

// RenderTypeBodyFromSpec renders the body of a type specification,
// excluding the leading "type <Name>" prefix.
func RenderTypeBodyFromSpec(spec *ast.TypeSpec) string {
	if spec == nil || spec.Type == nil {
		return ""
	}
	fileSet := token.NewFileSet()
	var buffer bytes.Buffer
	_ = printer.Fprint(&buffer, fileSet, spec.Type) // best-effort; empty on failure
	return buffer.String()
}
