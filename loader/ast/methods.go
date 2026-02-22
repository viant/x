// Package loader: methods.go binds method declarations to their owning types
// and updates both AST and model-level method representations, including
// conversion of signatures to model.Func for downstream code generation.
package ast

import (
	"go/ast"

	"github.com/viant/x/syntetic/model"
)

// bindMethodToType associates an ast.FuncDecl method with its named type in pkg.
func bindMethodToType(pkg *model.Package, fdecl *ast.FuncDecl, aliasIndex map[string]model.ImportRef) {
	if pkg == nil || fdecl == nil || fdecl.Recv == nil || len(fdecl.Recv.List) == 0 {
		return
	}
	recv := fdecl.Recv.List[0]
	ptr := false
	var ident *ast.Ident
	switch rt := recv.Type.(type) {
	case *ast.StarExpr:
		ptr = true
		switch rx := rt.X.(type) {
		case *ast.Ident:
			ident = rx
		case *ast.IndexExpr:
			if id, ok := rx.X.(*ast.Ident); ok {
				ident = id
			}
		case *ast.IndexListExpr:
			if id, ok := rx.X.(*ast.Ident); ok {
				ident = id
			}
		}
	case *ast.Ident:
		ident = rt
	case *ast.IndexExpr:
		if id, ok := rt.X.(*ast.Ident); ok {
			ident = id
		}
	case *ast.IndexListExpr:
		if id, ok := rt.X.(*ast.Ident); ok {
			ident = id
		}
	default:
		return
	}
	if ident == nil || ident.Name == "" {
		return
	}
	var tpe *model.Type
	for _, t := range pkg.Types {
		if t != nil && t.Name == ident.Name {
			tpe = t
			break
		}
	}
	if tpe == nil {
		return
	}
	mf := astFuncTypeToModelFunc(fdecl.Type, pkg.PkgPath, aliasIndex)
	if ptr {
		tpe.PtrMethodsAST = append(tpe.PtrMethodsAST, fdecl)
	} else {
		tpe.MethodsAST = append(tpe.MethodsAST, fdecl)
	}
	m := model.Method{Name: fdecl.Name.Name, Type: mf}
	if ptr {
		tpe.Methods.Pointer = append(tpe.Methods.Pointer, m)
	} else {
		tpe.Methods.Value = append(tpe.Methods.Value, m)
	}
}
