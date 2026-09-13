package shape

import (
	"go/ast"
	"go/parser"
	"go/token"
)

type SourceKind string

const (
	SourceUnknown SourceKind = ""
	SourceShape   SourceKind = "shape"
	SourceProgram SourceKind = "program"
)

// Classify distinguishes type declarations (including their methods) from
// implementation files. Mixed shape and package-level execution declarations
// remain unknown so callers cannot delete possible user-owned type content.
func (SourceParser) Classify(source []byte) (SourceKind, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", source, parser.SkipObjectResolution)
	if err != nil {
		return SourceUnknown, err
	}
	shape, program := false, false
	for _, declaration := range file.Decls {
		switch actual := declaration.(type) {
		case *ast.GenDecl:
			switch actual.Tok {
			case token.TYPE:
				shape = true
			case token.VAR, token.CONST:
				program = true
			}
		case *ast.FuncDecl:
			if actual.Recv == nil {
				program = true
			}
		}
	}
	if shape && !program {
		return SourceShape, nil
	}
	if program && !shape {
		return SourceProgram, nil
	}
	return SourceUnknown, nil
}
