// Package loader: package_load.go contains the high-level package loading
// workflow. It enumerates Go files, derives the package import path, and
// orchestrates per-file parsing and model population via loadPackageFile.
package ast

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/viant/x/syntetic/model"
)

// LoadPackageFS parses all non-test .go files under dir within fsys and
// produces a model.Package with files, types, functions, and imports.
func LoadPackageFS(ctx context.Context, fsys fs.FS, dir string) (*model.Package, error) {
	files, err := listGoFiles(fsys, dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("loader: no go files in %s", dir)
	}

	pkgPath, err := packagePathForDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	pkg := &model.Package{PkgPath: pkgPath}
	for _, filename := range files {
		gf, err := loadPackageFile(ctx, fsys, filename, pkg)
		if err != nil {
			return nil, err
		}
		if pkg.Name == "" && gf != nil {
			pkg.Name = gf.PkgName
		}
		pkg.AddFile(gf)
	}
	return pkg, nil
}

// listGoFiles returns non-test Go files in dir.
func listGoFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, path.Join(dir, name))
	}
	return files, nil
}

// packagePathForDir derives the package import path for dir using its module.
func packagePathForDir(fsys fs.FS, dir string) (string, error) {
	moduleRoot, modulePath, err := discoverModuleFS(fsys, dir)
	if err != nil {
		return "", err
	}
	relDir, _ := filepath.Rel(moduleRoot, dir)
	relDir = filepath.ToSlash(relDir)
	if relDir == "." || relDir == "" {
		return modulePath, nil
	}
	return strings.TrimSuffix(modulePath, "/") + "/" + relDir, nil
}

// loadPackageFile parses a single Go source file and updates the package model.
// It collects imports, //go:embed directives, type declarations (including
// generics), consts and vars (tracking referenced imports), and binds both
// methods (receiver functions) and free-standing functions.
func loadPackageFile(ctx context.Context, fsys fs.FS, filename string, pkg *model.Package) (*model.GoFile, error) {
	src, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return nil, fmt.Errorf("loader: read %s: %w", filename, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("loader: parse %s: %w", filename, err)
	}
	pkgName := file.Name.Name
	gf := &model.GoFile{Name: path.Base(filename), PkgName: pkgName}
	// imports
	for _, spec := range file.Imports {
		pathVal := strings.Trim(spec.Path.Value, "\"")
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		ref := model.ImportRef{Path: pathVal, Alias: alias}
		gf.AddImport(ref)
		pkg.AddImport(ref)
	}
	// embeds
	collectEmbeds(ctx, fsys, filename, file, gf)
	// decls
	aliasIndex := buildAliasIndex(gf)
	for _, decl := range file.Decls {
		if fdecl, ok := decl.(*ast.FuncDecl); ok {
			if fdecl.Recv != nil { // method
				bindMethodToType(pkg, fdecl, aliasIndex)
			} else { // free-standing function
				fn := &model.Function{Name: fdecl.Name.Name, Type: astFuncTypeToModelFunc(fdecl.Type, pkg.PkgPath, aliasIndex), Decl: fdecl, File: path.Base(filename)}
				pkg.Funcs = append(pkg.Funcs, fn)
			}
			continue
		}
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gen.Tok {
		case token.TYPE:
			processTypes(gen, gf, pkg, aliasIndex)
		case token.CONST:
			processConsts(gen, gf, pkg, aliasIndex)
		case token.VAR:
			processVars(gen, gf, pkg, aliasIndex)
		}
	}
	return gf, nil
}

// processTypes handles type specs: captures per-file imports, generic type
// parameters, and adds the type to file and package collections.
func processTypes(gen *ast.GenDecl, gf *model.GoFile, pkg *model.Package, aliasIndex map[string]model.ImportRef) {
	for _, s := range gen.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			continue
		}
		t := &model.Type{Name: ts.Name.Name, PkgPath: pkg.PkgPath, TypeSpec: ts, Imports: map[string]*model.ImportRef{}}
		for _, r := range gf.Imports {
			r := r
			t.Imports[r.Alias] = &r
		}
		if ts.TypeParams != nil {
			// Capture type parameters declared on this type (e.g. type Box[T any]).
			t.TypeParams = astTypeParamsToModel(ts.TypeParams, pkg.PkgPath, aliasIndex)
		}
		gf.AddType(t)
		pkg.AddType(t)
	}
}

// processConsts handles const declarations and records used imports derived
// from referenced selector expressions in type and value AST nodes.
func processConsts(gen *ast.GenDecl, gf *model.GoFile, pkg *model.Package, aliasIndex map[string]model.ImportRef) {
	for _, s := range gen.Specs {
		vs, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			var val ast.Expr
			if i < len(vs.Values) {
				val = vs.Values[i]
			} else if len(vs.Values) > 0 {
				val = vs.Values[0]
			}
			used := collectExprAliases(vs.Type, val)
			imports := make(map[string]model.ImportRef)
			for a := range used {
				if ref, ok := aliasIndex[a]; ok {
					imports[ref.Key()] = ref
				}
			}
			cd := model.ConstDecl{Name: name.Name, Type: vs.Type, Value: val, Imports: imports}
			gf.AddConst(cd)
			pkg.AddConst(cd)
		}
	}
}

// processVars handles var declarations and records used imports derived from
// referenced selector expressions in type and value AST nodes.
func processVars(gen *ast.GenDecl, gf *model.GoFile, pkg *model.Package, aliasIndex map[string]model.ImportRef) {
	for _, s := range gen.Specs {
		vs, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			var val ast.Expr
			if i < len(vs.Values) {
				val = vs.Values[i]
			} else if len(vs.Values) > 0 {
				val = vs.Values[0]
			}
			used := collectExprAliases(vs.Type, val)
			imports := make(map[string]model.ImportRef)
			for a := range used {
				if ref, ok := aliasIndex[a]; ok {
					imports[ref.Key()] = ref
				}
			}
			vd := model.VarDecl{Name: name.Name, Type: vs.Type, Value: val, Imports: imports}
			gf.AddVar(vd)
			pkg.AddVar(vd)
		}
	}
}
