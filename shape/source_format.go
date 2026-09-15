package shape

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// FormatFile formats a Go file AST while preserving top-level declaration
// comments that were created without parser positions.
func (p SourceParser) FormatFile(file *ast.File) ([]byte, error) {
	if file == nil || file.Name == nil {
		return nil, fmt.Errorf("Go source file is required")
	}
	if !hasTopLevelDeclarationDoc(file) {
		var result bytes.Buffer
		if err := format.Node(&result, token.NewFileSet(), file); err != nil {
			return nil, err
		}
		return format.Source(result.Bytes())
	}
	docs, leading, detached := detachedTopLevelDeclarationDocs(file)
	var rendered bytes.Buffer
	if err := format.Node(&rendered, token.NewFileSet(), detached); err != nil {
		return nil, err
	}
	bare, err := format.Source(rendered.Bytes())
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	positioned, err := parser.ParseFile(fset, "source.go", bare, 0)
	if err != nil {
		return nil, err
	}
	if len(positioned.Decls) != len(file.Decls) {
		return nil, fmt.Errorf("formatted Go declaration count changed from %d to %d", len(file.Decls), len(positioned.Decls))
	}
	insertions := make([]commentInsertion, 0, len(docs)+len(leading))
	for _, group := range leading {
		insertions = append(insertions, commentInsertion{group: group})
	}
	for index, declaration := range positioned.Decls {
		if group := docs[index]; group != nil {
			offset, err := sourceOffset(fset, declaration.Pos())
			if err != nil {
				return nil, err
			}
			insertions = append(insertions, commentInsertion{offset: offset, group: group})
		}
	}
	source := sourceWithCommentInsertions(bare, insertions)
	formatted, err := format.Source(source.Bytes())
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

type commentInsertion struct {
	offset int
	group  *ast.CommentGroup
}

func sourceWithCommentInsertions(source []byte, insertions []commentInsertion) *bytes.Buffer {
	var result bytes.Buffer
	last := 0
	for _, insertion := range insertions {
		if insertion.offset < last {
			continue
		}
		result.Write(source[last:insertion.offset])
		if insertion.offset > 0 && source[insertion.offset-1] != '\n' {
			result.WriteByte('\n')
		}
		writeCommentGroup(&result, insertion.group)
		last = insertion.offset
	}
	result.Write(source[last:])
	return &result
}

func sourceOffset(fset *token.FileSet, position token.Pos) (int, error) {
	location := fset.Position(position)
	if !location.IsValid() {
		return 0, fmt.Errorf("invalid Go source position")
	}
	return location.Offset, nil
}

func hasTopLevelDeclarationDoc(file *ast.File) bool {
	for _, declaration := range file.Decls {
		if declarationDoc(declaration) != nil {
			return true
		}
	}
	return false
}

func detachedTopLevelDeclarationDocs(file *ast.File) ([]*ast.CommentGroup, []*ast.CommentGroup, *ast.File) {
	docs := make([]*ast.CommentGroup, len(file.Decls))
	detached := *file
	detached.Doc = nil
	detached.Decls = append([]ast.Decl(nil), file.Decls...)
	detached.Comments = append([]*ast.CommentGroup(nil), file.Comments...)
	leading := leadingCommentGroups(file)
	for index, declaration := range file.Decls {
		switch actual := declaration.(type) {
		case *ast.FuncDecl:
			docs[index] = actual.Doc
			copy := *actual
			copy.Doc = nil
			detached.Decls[index] = &copy
		case *ast.GenDecl:
			docs[index] = actual.Doc
			copy := *actual
			copy.Doc = nil
			detached.Decls[index] = &copy
		}
	}
	if len(detached.Comments) != 0 {
		detachedDocs := map[*ast.CommentGroup]bool{}
		for _, group := range docs {
			if group != nil {
				detachedDocs[group] = true
			}
		}
		for _, group := range leading {
			detachedDocs[group] = true
		}
		comments := make([]*ast.CommentGroup, 0, len(file.Comments))
		for _, group := range file.Comments {
			if !detachedDocs[group] {
				comments = append(comments, group)
			}
		}
		detached.Comments = comments
	}
	return docs, leading, &detached
}

func leadingCommentGroups(file *ast.File) []*ast.CommentGroup {
	seen := map[*ast.CommentGroup]bool{}
	var result []*ast.CommentGroup
	for _, group := range file.Comments {
		if group != nil && group.End() <= file.Package {
			result = append(result, group)
			seen[group] = true
		}
	}
	if file.Doc != nil && !seen[file.Doc] {
		result = append(result, file.Doc)
	}
	return result
}

func declarationDoc(declaration ast.Decl) *ast.CommentGroup {
	switch actual := declaration.(type) {
	case *ast.FuncDecl:
		return actual.Doc
	case *ast.GenDecl:
		return actual.Doc
	}
	return nil
}

func writeCommentGroup(buffer *bytes.Buffer, group *ast.CommentGroup) {
	for _, comment := range group.List {
		text := strings.TrimRight(comment.Text, " \t")
		if text == "" {
			continue
		}
		buffer.WriteString(text)
		buffer.WriteByte('\n')
	}
}
