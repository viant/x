package ast

import (
	"testing"
	"testing/fstest"
)

// mkFS builds an in-memory fs.FS using fstest.MapFS from a map of path->content.
// Paths must use forward slashes and include a module root path prefix, e.g.:
//
//	root/go.mod, root/pkg/types.go
//
// MkFS is exported for reuse by testutil.
func MkFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content), Mode: 0o644}
	}
	return m
}
