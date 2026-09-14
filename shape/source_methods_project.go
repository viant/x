package shape

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
)

// MethodProjection selects receiver declarations from a source file and rewrites
// their type references under a destination package's import authority. Imports
// may be populated by Resolver.Rewriter as it allocates qualified type names.
// It never copies free functions or invocation state into the receiver package.
type MethodProjection struct {
	Receivers []string
	Package   string
	Resolver  Resolver
	Imports   map[string]string
}

type MethodSources struct{ Selected, Remaining []byte }

func (p SourceParser) ProjectMethods(source []byte, request MethodProjection) (*MethodSources, error) {
	if !token.IsIdentifier(request.Package) || request.Package == "_" {
		return nil, fmt.Errorf("method destination package is invalid")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	comments := ast.NewCommentMap(fset, file, file.Comments)
	receivers := map[string]bool{}
	for _, name := range request.Receivers {
		receivers[name] = true
	}
	selected := &ast.File{Name: ast.NewIdent(request.Package)}
	imports := map[string]string{}
	for _, item := range file.Imports {
		location, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return nil, err
		}
		alias := path.Base(location)
		if item.Name != nil {
			alias = item.Name.Name
		}
		if alias == "." {
			return nil, fmt.Errorf("method projection cannot resolve dot import %s", location)
		}
		imports[alias] = location
	}
	remaining := make([]ast.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		method, ok := decl.(*ast.FuncDecl)
		if !ok || method.Recv == nil || len(method.Recv.List) != 1 {
			remaining = append(remaining, decl)
			continue
		}
		receiver, err := (Resolver{}).Reference(rendered(method.Recv.List[0].Type))
		if err != nil {
			return nil, err
		}
		if !receivers[receiver.BaseName] {
			remaining = append(remaining, decl)
			continue
		}
		if err = request.Resolver.rewriteMethodTypes(method); err != nil {
			return nil, fmt.Errorf("relocate %s.%s: %w", receiver.Name, method.Name.Name, err)
		}
		// A receiver package cannot reach the source package's free invocation
		// helpers or globals without a dependency supplied by its owner.
		var dependency string
		ast.Inspect(method, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if ok && id.Obj != nil && file.Scope != nil && file.Scope.Lookup(id.Name) == id.Obj && id.Obj.Kind != ast.Typ {
				dependency = id.Name
				return false
			}
			return true
		})
		if dependency != "" {
			return nil, fmt.Errorf("method %s.%s depends on source-package declaration %s", receiver.Name, method.Name.Name, dependency)
		}
		selected.Decls = append(selected.Decls, method)
	}
	file.Decls = remaining
	for alias, location := range request.Imports {
		if previous, ok := imports[alias]; ok && previous != location {
			return nil, fmt.Errorf("method import alias %s conflicts between %s and %s", alias, previous, location)
		}
		imports[alias] = location
	}
	selected.Comments = comments.Filter(selected).Comments()
	file.Comments = comments.Filter(file).Comments()
	p.projectImports(selected, imports)
	// Remaining source retains its own original import scope.
	original := map[string]string{}
	for _, item := range file.Imports {
		location, _ := strconv.Unquote(item.Path.Value)
		alias := path.Base(location)
		if item.Name != nil {
			alias = item.Name.Name
		}
		original[alias] = location
	}
	p.projectImports(file, original)
	result := &MethodSources{}
	for _, output := range []struct {
		file *ast.File
		dst  *[]byte
	}{{selected, &result.Selected}, {file, &result.Remaining}} {
		var content bytes.Buffer
		if err = format.Node(&content, fset, output.file); err != nil {
			return nil, err
		}
		*output.dst, err = format.Source(content.Bytes())
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// rewriteMethodTypes operates on syntactic type positions. Parser object
// bindings keep local expressions that shadow a type name out of conversions
// and generic arguments; selector member names and struct field keys are not
// type references. Reference parsing and rewriting stay on Resolver.
func (r Resolver) rewriteMethodTypes(method *ast.FuncDecl) error {
	var failure error
	rewrite := func(expression *ast.Expr) {
		if failure != nil || *expression == nil {
			return
		}
		local := map[string]bool{}
		ast.Inspect(*expression, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && id.Obj != nil {
				if decl, ok := id.Obj.Decl.(*ast.TypeSpec); ok && method.Body != nil && decl.Pos() >= method.Body.Pos() && decl.End() <= method.Body.End() {
					local[id.Name] = true
				}
			}
			return true
		})
		resolver := r
		resolver.Rewriter = func(name string) (string, error) {
			if local[name] || r.Rewriter == nil {
				return name, nil
			}
			return r.Rewriter(name)
		}
		text, err := resolver.Rewrite(rendered(*expression))
		if err != nil {
			failure = err
			return
		}
		*expression, failure = r.parse(text)
	}
	unbound := func(expression ast.Expr) bool {
		if id, ok := expression.(*ast.Ident); ok {
			if id.Obj == nil {
				return true
			}
			decl, ok := id.Obj.Decl.(*ast.TypeSpec)
			return ok && (method.Body == nil || decl.Pos() < method.Body.Pos() || decl.End() > method.Body.End())
		}
		selected, ok := expression.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		id, ok := selected.X.(*ast.Ident)
		return ok && id.Obj == nil
	}
	typeCall := func(expression ast.Expr) bool {
		for {
			switch node := expression.(type) {
			case *ast.ParenExpr:
				expression = node.X
			case *ast.StarExpr:
				expression = node.X
			case *ast.ArrayType, *ast.MapType, *ast.StructType, *ast.FuncType, *ast.ChanType:
				return true
			default:
				return unbound(expression)
			}
		}
	}
	ast.Inspect(method, func(node ast.Node) bool {
		if failure != nil {
			return false
		}
		switch n := node.(type) {
		case *ast.Field:
			rewrite(&n.Type)
		case *ast.CompositeLit:
			rewrite(&n.Type)
		case *ast.TypeAssertExpr:
			rewrite(&n.Type)
		case *ast.TypeSpec:
			rewrite(&n.Type)
		case *ast.ValueSpec:
			rewrite(&n.Type)
		case *ast.CallExpr:
			if builtin, ok := n.Fun.(*ast.Ident); ok && builtin.Obj == nil && (builtin.Name == "new" || builtin.Name == "make") && len(n.Args) > 0 {
				rewrite(&n.Args[0])
			}
			if typeCall(n.Fun) {
				rewrite(&n.Fun)
			}
			for i, arg := range n.Args {
				switch arg.(type) {
				case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.StructType, *ast.FuncType:
					rewrite(&n.Args[i])
				}
			}
		case *ast.IndexExpr:
			if unbound(n.X) && unbound(n.Index) {
				rewrite(&n.Index)
			}
		case *ast.IndexListExpr:
			if unbound(n.X) {
				for i := range n.Indices {
					rewrite(&n.Indices[i])
				}
			}
		}
		return true
	})
	return failure
}

func (SourceParser) projectImports(file *ast.File, imports map[string]string) {
	used := map[string]bool{}
	for _, item := range file.Imports {
		if item.Name != nil && item.Name.Name == "_" {
			used["_"] = true
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if selected, ok := node.(*ast.SelectorExpr); ok {
			if id, ok := selected.X.(*ast.Ident); ok && id.Obj == nil {
				used[id.Name] = true
			}
		}
		return true
	})
	declarations := file.Decls[:0]
	for _, decl := range file.Decls {
		if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.IMPORT {
			continue
		}
		declarations = append(declarations, decl)
	}
	file.Decls = declarations
	file.Imports = nil
	aliases := make([]string, 0, len(imports))
	for alias := range imports {
		if used[alias] {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	group := &ast.GenDecl{Tok: token.IMPORT, Lparen: 1}
	for _, alias := range aliases {
		item := &ast.ImportSpec{Name: ast.NewIdent(alias), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(imports[alias])}}
		group.Specs = append(group.Specs, item)
		file.Imports = append(file.Imports, item)
	}
	if len(group.Specs) > 0 {
		file.Decls = append([]ast.Decl{group}, file.Decls...)
	}
}
