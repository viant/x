// Package loader: convert.go contains helpers that convert parsed Go AST
// expressions and function types into the model's intermediate Node and
// Func representations, including generic type parameters and alias
// resolution using import alias maps.
package ast

import (
	"go/ast"
	"go/token"
	"path"
	"strconv"

	"github.com/viant/x/syntetic/model"
)

// astFuncTypeToModelFunc converts an *ast.FuncType to a model.Func using alias
// resolution for package-qualified identifiers and best-effort conversion for
// nested types. It also captures generic type parameters on the function.
func astFuncTypeToModelFunc(ft *ast.FuncType, currentPkg string, aliasIndex map[string]model.ImportRef) model.Func {
	var out model.Func
	if ft == nil {
		return out
	}
	if ft.TypeParams != nil {
		out.TypeParams = astTypeParamsToModel(ft.TypeParams, currentPkg, aliasIndex)
	}
	if ft.Params != nil {
		for i, f := range ft.Params.List {
			variadic := false
			if i == len(ft.Params.List)-1 {
				if _, ok := f.Type.(*ast.Ellipsis); ok {
					variadic = true
				}
			}
			n := astExprToModelNode(f.Type, currentPkg, aliasIndex)
			out.Params = append(out.Params, model.Field{Type: n})
			if variadic {
				out.Variadic = true
			}
		}
	}
	if ft.Results != nil {
		for _, f := range ft.Results.List {
			n := astExprToModelNode(f.Type, currentPkg, aliasIndex)
			out.Results = append(out.Results, model.Field{Type: n})
		}
	}
	return out
}

// astExprToModelNode converts a Go AST expression to a model.Node. It handles
// builtins, named/qualified identifiers, composite types (ptr/slice/array/map/
// chan), function types (recursively), minimal interfaces (method sets), and
// struct literals. For interface type sets (union constraints) it produces a
// model.Union node.
func astExprToModelNode(e ast.Expr, currentPkg string, aliasIndex map[string]model.ImportRef) model.Node {
	switch v := e.(type) {
	case *ast.Ident:
		switch v.Name {
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "string", "bool", "float32", "float64", "complex64", "complex128", "byte", "rune", "error":
			return &model.Basic{Name: v.Name}
		default:
			return &model.Named{PkgPath: currentPkg, Name: v.Name}
		}
	case *ast.StarExpr:
		return &model.Pointer{Elem: astExprToModelNode(v.X, currentPkg, aliasIndex)}
	case *ast.ArrayType:
		if v.Len == nil {
			return &model.Slice{Elem: astExprToModelNode(v.Elt, currentPkg, aliasIndex)}
		}
		ln := 0
		if bl, ok := v.Len.(*ast.BasicLit); ok && bl.Kind == token.INT {
			if n, err := strconv.Atoi(bl.Value); err == nil {
				ln = n
			}
		}
		return &model.Array{Len: ln, Elem: astExprToModelNode(v.Elt, currentPkg, aliasIndex)}
	case *ast.MapType:
		return &model.Map{Key: astExprToModelNode(v.Key, currentPkg, aliasIndex), Elem: astExprToModelNode(v.Value, currentPkg, aliasIndex)}
	case *ast.ChanType:
		return &model.Chan{Dir: int(v.Dir), Elem: astExprToModelNode(v.Value, currentPkg, aliasIndex)}
	case *ast.SelectorExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			if ref, ok := aliasIndex[id.Name]; ok {
				return &model.Named{PkgPath: ref.Path, Name: v.Sel.Name}
			}
			return &model.Named{PkgPath: "", Name: v.Sel.Name}
		}
		return &model.Named{Name: v.Sel.Name}
	case *ast.Ellipsis:
		return &model.Slice{Elem: astExprToModelNode(v.Elt, currentPkg, aliasIndex)}
	case *ast.FuncType:
		mf := astFuncTypeToModelFunc(v, currentPkg, aliasIndex)
		return &mf
	case *ast.InterfaceType:
		// Detect whether this interface node represents a type-set constraint
		// (i.e. a union of types used in generics) or a traditional interface
		// with named methods. In type-set form, its FieldList contains unnamed
		// entries that are type expressions, possibly combined with '|'.
		if v.Methods != nil && len(v.Methods.List) > 0 {
			// If fields have no names, treat as type elements; attempt to parse union.
			allUnnamed := true
			for _, fld := range v.Methods.List {
				if len(fld.Names) > 0 {
					allUnnamed = false
					break
				}
			}
			if allUnnamed {
				// Attempt to flatten a union expression across unnamed fields.
				// The Go syntax allows union terms with optional '~' (approximation)
				// which we represent as model.Term{Approx:true}.
				var terms []model.Term
				for _, fld := range v.Methods.List {
					terms = append(terms, flattenUnionTerms(fld.Type, currentPkg, aliasIndex)...)
				}
				if len(terms) > 0 {
					return &model.Union{Terms: terms}
				}
			}
			// Named methods interface
			it := &model.Interface{}
			for _, f := range v.Methods.List {
				if len(f.Names) == 0 {
					continue
				}
				mname := f.Names[0].Name
				if ft, ok := f.Type.(*ast.FuncType); ok {
					mf := astFuncTypeToModelFunc(ft, currentPkg, aliasIndex)
					it.Methods = append(it.Methods, model.Method{Name: mname, Type: mf})
				}
			}
			return it
		}
		return &model.Interface{}
	case *ast.StructType:
		st := &model.Struct{}
		if v.Fields != nil {
			for _, f := range v.Fields.List {
				n := astExprToModelNode(f.Type, currentPkg, aliasIndex)
				if len(f.Names) == 0 {
					st.Fields = append(st.Fields, model.Field{Embedded: true, Type: n})
					continue
				}
				for _, nm := range f.Names {
					st.Fields = append(st.Fields, model.Field{Name: nm.Name, Type: n})
				}
			}
		}
		return st
	}
	return &model.Basic{Name: ""}
}

// flattenUnionTerms flattens a union expression (e.g. ~int | string) into
// model.Terms. It recurses on OR nodes and detects '~' via UnaryExpr(TILDE).
func flattenUnionTerms(e ast.Expr, currentPkg string, aliasIndex map[string]model.ImportRef) []model.Term {
	switch v := e.(type) {
	case *ast.BinaryExpr:
		if v.Op == token.OR {
			left := flattenUnionTerms(v.X, currentPkg, aliasIndex)
			right := flattenUnionTerms(v.Y, currentPkg, aliasIndex)
			return append(left, right...)
		}
	case *ast.UnaryExpr:
		if v.Op == token.TILDE {
			return []model.Term{{Type: astExprToModelNode(v.X, currentPkg, aliasIndex), Approx: true}}
		}
	case *ast.ParenExpr:
		return flattenUnionTerms(v.X, currentPkg, aliasIndex)
	default:
		return []model.Term{{Type: astExprToModelNode(e, currentPkg, aliasIndex)}}
	}
	return nil
}

// astTypeParamsToModel converts an ast.FieldList of type parameters into model.TypeParam slice.
func astTypeParamsToModel(fl *ast.FieldList, currentPkg string, aliasIndex map[string]model.ImportRef) []model.TypeParam {
	if fl == nil || len(fl.List) == 0 {
		return nil
	}
	out := make([]model.TypeParam, 0, len(fl.List))
	for _, f := range fl.List {
		var cons model.Node
		if f.Type != nil {
			cons = astExprToModelNode(f.Type, currentPkg, aliasIndex)
		}
		for _, nm := range f.Names {
			out = append(out, model.TypeParam{Name: nm.Name, Constraint: cons})
		}
	}
	return out
}

// buildAliasIndex returns a map from effective alias (as used in code) to ImportRef.
// For unaliased imports, the effective alias is path.Base(importPath).
func buildAliasIndex(gf *model.GoFile) map[string]model.ImportRef {
	out := map[string]model.ImportRef{}
	if gf == nil || len(gf.Imports) == 0 {
		return out
	}
	for _, r := range gf.Imports {
		alias := r.Alias
		if alias == "" {
			alias = path.Base(r.Path)
		} // default alias from import path segment
		if alias == "." || alias == "_" {
			continue
		} // dot/blank imports are not resolvable
		out[alias] = r
	}
	return out
}

// collectExprAliases traverses provided expressions and returns a set of
// import aliases referenced via selector expressions (alias.Ident).
func collectExprAliases(exprs ...ast.Expr) map[string]struct{} {
	used := map[string]struct{}{}
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch v := e.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok {
				if id.Name != "" {
					used[id.Name] = struct{}{}
				}
			} else {
				walk(v.X)
			}
		case *ast.StarExpr:
			walk(v.X)
		case *ast.ArrayType:
			walk(v.Elt)
		case *ast.MapType:
			walk(v.Key)
			walk(v.Value)
		case *ast.ChanType:
			walk(v.Value)
		case *ast.Ellipsis:
			walk(v.Elt)
		case *ast.FuncType:
			if v.Params != nil {
				for _, f := range v.Params.List {
					walk(f.Type)
				}
			}
			if v.Results != nil {
				for _, f := range v.Results.List {
					walk(f.Type)
				}
			}
		case *ast.CompositeLit:
			walk(v.Type)
			for _, el := range v.Elts {
				walk(el)
			}
		case *ast.KeyValueExpr:
			walk(v.Key)
			walk(v.Value)
		case *ast.CallExpr:
			walk(v.Fun)
			for _, a := range v.Args {
				walk(a)
			}
		case *ast.ParenExpr:
			walk(v.X)
		}
	}
	for _, e := range exprs {
		walk(e)
	}
	return used
}
