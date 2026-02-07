//go:build viant_afs

// Package loader: embeds.go extracts //go:embed directives from parsed files
// and materialises embedded filesystem instances using afs/embed Holder,
// attaching them to the file model for later rendering or inspection.
package ast

import (
	"context"
	"io/fs"
	stdpath "path"
	"path/filepath"
	"strings"

	"go/ast"
	"go/token"

	afsembed "github.com/viant/afs/embed"
	"github.com/viant/x/syntetic/model"
)

// collectEmbeds finds //go:embed directives preceding var declarations in file,
// and for each variable of type embed.FS it builds an embedded FS instance.
func collectEmbeds(ctx context.Context, fsys fs.FS, filename string, file *ast.File, gf *model.GoFile) {
	if file == nil || gf == nil {
		return
	}
	dir := stdpath.Dir(filename)
	add := func(varName string, holder *afsembed.Holder) {
		if holder == nil || varName == "" {
			return
		}
		gf.AddEmbed(varName, holder.EmbedFs())
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		patterns := gatherEmbedPatterns(gen.Doc)
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			p := patterns
			if cg := vs.Doc; cg != nil {
				p = append(p, gatherEmbedPatterns(cg)...)
			}
			if len(p) == 0 || !isEmbedFSType(vs.Type) {
				continue
			}
			holder := afsembed.NewHolder()
			for _, pat := range p {
				addMatchesToHolder(ctx, fsys, dir, pat, holder)
			}
			for _, name := range vs.Names {
				add(name.Name, holder)
			}
		}
	}
}

func gatherEmbedPatterns(cg *ast.CommentGroup) []string {
	if cg == nil {
		return nil
	}
	var out []string
	for _, c := range cg.List {
		txt := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(txt, "go:embed ") {
			rest := strings.TrimSpace(strings.TrimPrefix(txt, "go:embed "))
			out = append(out, strings.Fields(rest)...)
		}
	}
	return out
}

func isEmbedFSType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.X == nil {
		return false
	}
	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name == "embed" && sel.Sel.Name == "FS"
	}
	return false
}

func addMatchesToHolder(ctx context.Context, fsys fs.FS, baseDir, pattern string, holder *afsembed.Holder) {
	if holder == nil || pattern == "" {
		return
	}
	pat := stdpath.Clean(pattern)
	root := baseDir
	if strings.HasPrefix(pat, "/") {
		pat = strings.TrimPrefix(pat, "/")
	}
	if !strings.ContainsAny(pat, "*?[") {
		target := stdpath.Join(root, pat)
		if data, err := fs.ReadFile(fsys, target); err == nil {
			holder.Add(pat, string(data))
		}
		return
	}
	_ = fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if ok, _ := stdpath.Match(pat, rel); ok {
			if data, rerr := fs.ReadFile(fsys, p); rerr == nil {
				holder.Add(rel, string(data))
			}
		}
		return nil
	})
}
