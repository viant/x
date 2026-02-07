package model

import (
	"go/ast"
	"go/token"
)

// ToTypeSpec builds a go/ast.TypeSpec for this type and includes generic
// type parameters (TypeParams) when present. If t.TypeSpec is non-nil it is
// shallow-copied to preserve the original Type node.
func (t *Type) ToTypeSpec(currentPkg string, aliases map[string]string) *ast.TypeSpec {
	if t == nil {
		return nil
	}
	var spec ast.TypeSpec
	if t.TypeSpec != nil {
		spec = *t.TypeSpec
	} else {
		spec = ast.TypeSpec{Name: ast.NewIdent(t.Name)}
	}
	if len(t.TypeParams) > 0 {
		fl := &ast.FieldList{}
		for _, p := range t.TypeParams {
			var field ast.Field
			field.Names = []*ast.Ident{ast.NewIdent(p.Name)}
			if p.Constraint != nil {
				field.Type = nodeToAST(p.Constraint, currentPkg, aliases)
			} else {
				field.Type = ast.NewIdent("any")
			}
			fl.List = append(fl.List, &field)
		}
		spec.TypeParams = fl
	}
	return &spec
}

// ToGenDecl creates a full type declaration (GenDecl) for this Type including
// its type parameters.
func (t *Type) ToGenDecl(currentPkg string, aliases map[string]string) *ast.GenDecl {
	spec := t.ToTypeSpec(currentPkg, aliases)
	if spec == nil {
		return nil
	}
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

// nodeToAST converts a model.Node to a best-effort AST expression suitable for
// constraints or simple type emission.
func nodeToAST(n Node, currentPkg string, aliases map[string]string) ast.Expr {
	switch v := n.(type) {
	case *Basic:
		if v.PkgPath == "" {
			return ast.NewIdent(v.Name)
		}
		return &ast.SelectorExpr{X: ast.NewIdent(aliasFor(v.PkgPath, aliases)), Sel: ast.NewIdent(v.Name)}
	case *Named:
		if v.PkgPath == "" || v.PkgPath == currentPkg {
			return ast.NewIdent(v.Name)
		}
		return &ast.SelectorExpr{X: ast.NewIdent(aliasFor(v.PkgPath, aliases)), Sel: ast.NewIdent(v.Name)}
	case *Alias:
		if v.PkgPath == "" || v.PkgPath == currentPkg {
			return ast.NewIdent(v.Name)
		}
		return &ast.SelectorExpr{X: ast.NewIdent(aliasFor(v.PkgPath, aliases)), Sel: ast.NewIdent(v.Name)}
	case *Pointer:
		return &ast.StarExpr{X: nodeToAST(v.Elem, currentPkg, aliases)}
	case *Slice:
		return &ast.ArrayType{Elt: nodeToAST(v.Elem, currentPkg, aliases)}
	case *Array:
		return &ast.ArrayType{Len: &ast.BasicLit{Kind: token.INT, Value: itoa(v.Len)}, Elt: nodeToAST(v.Elem, currentPkg, aliases)}
	case *Map:
		return &ast.MapType{Key: nodeToAST(v.Key, currentPkg, aliases), Value: nodeToAST(v.Elem, currentPkg, aliases)}
	case *Chan:
		return &ast.ChanType{Dir: ast.ChanDir(v.Dir), Value: nodeToAST(v.Elem, currentPkg, aliases)}
	case *Struct:
		fl := &ast.FieldList{}
		for _, f := range v.Fields {
			var field ast.Field
			if !f.Embedded && f.Name != "" {
				field.Names = []*ast.Ident{ast.NewIdent(f.Name)}
			}
			field.Type = nodeToAST(f.Type, currentPkg, aliases)
			fl.List = append(fl.List, &field)
		}
		return &ast.StructType{Fields: fl}
	case *Interface:
		fl := &ast.FieldList{}
		for _, m := range v.Methods {
			mt := funcToAST(&m.Type, currentPkg, aliases)
			fl.List = append(fl.List, &ast.Field{Names: []*ast.Ident{ast.NewIdent(m.Name)}, Type: mt})
		}
		return &ast.InterfaceType{Methods: fl}
	case *Union:
		// Build a union type-list expression: t1 | t2 | ... with optional '~'.
		// In Go's type-parameter syntax, such a list appears inside an
		// interface type (type set). We materialise this as an InterfaceType
		// with unnamed fields holding the union expression.
		var expr ast.Expr
		for i, term := range v.Terms {
			texpr := nodeToAST(term.Type, currentPkg, aliases)
			if term.Approx {
				texpr = &ast.UnaryExpr{Op: token.TILDE, X: texpr}
			}
			if i == 0 {
				expr = texpr
			} else {
				expr = &ast.BinaryExpr{X: expr, Op: token.OR, Y: texpr}
			}
		}
		// Union constraints are written inside an interface type as a type list.
		it := &ast.InterfaceType{Methods: &ast.FieldList{}}
		if expr != nil {
			it.Methods.List = append(it.Methods.List, &ast.Field{Type: expr})
		}
		return it
	case *Func:
		return funcToAST(v, currentPkg, aliases)
	}
	return ast.NewIdent("any")
}

func funcToAST(fn *Func, currentPkg string, aliases map[string]string) ast.Expr {
	if fn == nil {
		return ast.NewIdent("func")
	}
	params := &ast.FieldList{}
	for i, p := range fn.Params {
		if fn.Variadic && i == len(fn.Params)-1 {
			params.List = append(params.List, &ast.Field{Type: &ast.Ellipsis{Elt: nodeToAST(p.Type, currentPkg, aliases)}})
		} else {
			params.List = append(params.List, &ast.Field{Type: nodeToAST(p.Type, currentPkg, aliases)})
		}
	}
	results := &ast.FieldList{}
	for _, r := range fn.Results {
		results.List = append(results.List, &ast.Field{Type: nodeToAST(r.Type, currentPkg, aliases)})
	}
	return &ast.FuncType{Params: params, Results: results}
}

func lastSegment(p string) string {
	// Avoid importing path module here; a tiny, inline last-segment finder
	// is sufficient for generating selector expressions.
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

func aliasFor(pkgPath string, aliases map[string]string) string {
	if aliases != nil {
		if a, ok := aliases[pkgPath]; ok && a != "" {
			return a
		}
	}
	return lastSegment(pkgPath)
}
