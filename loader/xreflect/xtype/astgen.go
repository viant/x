package xtype

import (
	"errors"
	"go/ast"
	"go/token"
	"strconv"
)

func ToAST(n Node, imps *ImportSet) (ast.Expr, error) {
	if n == nil {
		return nil, errors.New("xtype: nil node")
	}
	if imps == nil {
		return nil, errors.New("xtype: ImportSet is required")
	}
	switch v := n.(type) {
	case *Basic:
		if v.PkgPath == "" {
			return ast.NewIdent(v.Name), nil
		}
		return imps.QualIdent(v.PkgPath, v.Name), nil
	case *Named:
		if v.PkgPath == "" || v.PkgPath == imps.CurrentPkg {
			return ast.NewIdent(v.Name), nil
		}
		return imps.QualIdent(v.PkgPath, v.Name), nil
	case *Alias:
		if v.PkgPath == "" || v.PkgPath == imps.CurrentPkg {
			return ast.NewIdent(v.Name), nil
		}
		return imps.QualIdent(v.PkgPath, v.Name), nil
	case *Pointer:
		elt, err := ToAST(v.Elem, imps)
		if err != nil {
			return nil, err
		}
		return &ast.StarExpr{X: elt}, nil
	case *Slice:
		elt, err := ToAST(v.Elem, imps)
		if err != nil {
			return nil, err
		}
		return &ast.ArrayType{Elt: elt}, nil
	case *Array:
		elt, err := ToAST(v.Elem, imps)
		if err != nil {
			return nil, err
		}
		return &ast.ArrayType{Len: &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(v.Len)}, Elt: elt}, nil
	case *Map:
		k, err := ToAST(v.Key, imps)
		if err != nil {
			return nil, err
		}
		e, err := ToAST(v.Elem, imps)
		if err != nil {
			return nil, err
		}
		return &ast.MapType{Key: k, Value: e}, nil
	case *Chan:
		elt, err := ToAST(v.Elem, imps)
		if err != nil {
			return nil, err
		}
		d := ast.ChanDir(v.Dir)
		return &ast.ChanType{Dir: d, Value: elt}, nil
	case *Func:
		return toFuncType(v, imps)
	case *Interface:
		fl, err := toExprFieldList(v.Methods, imps)
		if err != nil {
			return nil, err
		}
		return &ast.InterfaceType{Methods: fl}, nil
	case *Struct:
		fl, err := toStructFieldList(v.Fields, imps)
		if err != nil {
			return nil, err
		}
		return &ast.StructType{Fields: fl}, nil
	default:
		return nil, errors.New("xtype: unsupported node kind")
	}
}

func toFuncType(fn *Func, imps *ImportSet) (ast.Expr, error) {
	if fn.Variadic && len(fn.Params) == 0 {
		return nil, errors.New("xtype: variadic func without params")
	}
	params, err := toParamResultsFieldList(fn.Params, imps, fn.Variadic)
	if err != nil {
		return nil, err
	}
	results, err := toParamResultsFieldList(fn.Results, imps, false)
	if err != nil {
		return nil, err
	}
	return &ast.FuncType{Params: params, Results: results}, nil
}

func toParamResultsFieldList(fields []Field, imps *ImportSet, variadic bool) (*ast.FieldList, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := make([]*ast.Field, len(fields))
	for i, f := range fields {
		var expr ast.Expr
		var err error
		if variadic && i == len(fields)-1 {
			if s, ok := f.Type.(*Slice); ok {
				elt, err := ToAST(s.Elem, imps)
				if err != nil {
					return nil, err
				}
				expr = &ast.Ellipsis{Elt: elt}
			} else {
				return nil, errors.New("xtype: variadic parameter is not a slice")
			}
		} else {
			expr, err = ToAST(f.Type, imps)
			if err != nil {
				return nil, err
			}
		}
		out[i] = &ast.Field{Type: expr}
	}
	return &ast.FieldList{List: out}, nil
}

func toExprFieldList(methods []Method, imps *ImportSet) (*ast.FieldList, error) {
	if len(methods) == 0 {
		return nil, nil
	}
	out := make([]*ast.Field, len(methods))
	for i, m := range methods {
		expr, err := toFuncType(&m.Type, imps)
		if err != nil {
			return nil, err
		}
		out[i] = &ast.Field{Names: []*ast.Ident{ast.NewIdent(m.Name)}, Type: expr}
	}
	return &ast.FieldList{List: out}, nil
}

func toStructFieldList(fields []Field, imps *ImportSet) (*ast.FieldList, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := make([]*ast.Field, len(fields))
	for i, f := range fields {
		expr, err := ToAST(f.Type, imps)
		if err != nil {
			return nil, err
		}
		var names []*ast.Ident
		if !f.Embedded && f.Name != "" {
			names = []*ast.Ident{ast.NewIdent(f.Name)}
		}
		field := &ast.Field{Names: names, Type: expr}
		if f.Tag != "" {
			field.Tag = &ast.BasicLit{Kind: token.STRING, Value: "`" + f.Tag + "`"}
		}
		out[i] = field
	}
	return &ast.FieldList{List: out}, nil
}
