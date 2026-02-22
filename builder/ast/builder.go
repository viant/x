package ast

import (
	"go/ast"
	"go/token"

	"github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

// Builder is a minimal façade that delegates to the syntetic/model package for
// AST generation and rendering. It exists to provide a uniform "builder"
// surface alongside builder/xreflect.
type Builder struct {
	tr trf.Transformer
}

// New returns a new AST builder. The builder is stateless; all options are
// provided per call via parameters.
func New(opts ...Option) *Builder {
	b := &Builder{}
	for _, o := range opts {
		if o != nil {
			o(b)
		}
	}
	return b
}

// Option configures the AST builder.
type Option func(*Builder)

// WithTransforms configures the builder to consult the provided transformer
// for declaration-name overrides and, when used via helper methods, to apply
// transforms to packages prior to rendering.
func WithTransforms(t trf.Transformer) Option { return func(b *Builder) { b.tr = t } }

// TypeSpec builds a go/ast.TypeSpec for t using its TypeParams and PkgPath. The
// aliases map optionally overrides import aliases when t references external
// packages (as used by TypeParams constraints); pass nil to use defaults.
func (b *Builder) TypeSpec(t *model.Type, aliases map[string]string) *ast.TypeSpec {
	if t == nil {
		return nil
	}
	spec := t.ToTypeSpec(t.PkgPath, aliases)
	if b.tr != nil && spec != nil {
		if name, ok := trf.LookupDeclName(b.tr, t); ok && name != "" {
			spec.Name = ast.NewIdent(name)
		}
	}
	return spec
}

// GenDecl builds a go/ast.GenDecl (type declaration) for t.
func (b *Builder) GenDecl(t *model.Type, aliases map[string]string) *ast.GenDecl {
	if t == nil {
		return nil
	}
	spec := b.TypeSpec(t, aliases)
	if spec == nil {
		return nil
	}
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

// RenderFile renders a single GoFile with the provided options.
func (b *Builder) RenderFile(f *model.GoFile, opts model.RenderOptions) (string, error) {
	if f == nil {
		return "", nil
	}
	return f.RenderWithOptions(opts)
}

// RenderPackage renders all files for p with the provided options. When
// includeFreeFunctions is true, free-standing function stubs are appended per
// file.
func (b *Builder) RenderPackage(p *model.Package, opts model.RenderOptions, includeFreeFunctions bool) (map[string]string, error) {
	if p == nil {
		return nil, nil
	}
	pkg := p
	if b.tr != nil {
		pkg = trf.ApplyPackage(p, b.tr)
	}
	return pkg.RenderFilesWithOptions(opts, includeFreeFunctions)
}

// WritePackage writes all files for p into dir using the provided options.
func (b *Builder) WritePackage(p *model.Package, dir string, opts model.RenderOptions, includeFreeFunctions bool) error {
	if p == nil {
		return nil
	}
	pkg := p
	if b.tr != nil {
		pkg = trf.ApplyPackage(p, b.tr)
	}
	return pkg.WriteFilesWithOptions(dir, opts, includeFreeFunctions)
}

// no extra helpers needed
