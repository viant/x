package ast

import (
	"context"
	"github.com/viant/x/syntetic/model"
	"go/ast"
	"io/fs"
)

// collectEmbeds is a no-op in builds without the viant_afs tag. It exists so
// that the AST loader can compile without importing the afs/embed helper.
func collectEmbeds(ctx context.Context, fsys fs.FS, filename string, file *ast.File, gf *model.GoFile) {
}
