//go:build viant_afs

package ast

import (
	"context"
	"io/fs"
	"path"

	afs "github.com/viant/afs"
	"github.com/viant/x/syntetic/adapter"
)

// AFSGOPATHResolver returns a DeepOptions.ResolveExternal-compatible resolver
// that maps importPath to (adapter.NewFS(ctx, svc, baseURL), "src/<importPath>").
// baseURL can be any AFS-supported URL (e.g., mem://localhost/gopath, s3://bucket).
func AFSGOPATHResolver(svc afs.Service, baseURL string) func(ctx context.Context, importPath string) (fs.FS, string, error) {
	return func(ctx context.Context, importPath string) (fs.FS, string, error) {
		fsys := adapter.NewFS(ctx, svc, baseURL)
		return fsys, path.Join("src", importPath), nil
	}
}
