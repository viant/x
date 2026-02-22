//go:build viant_afs

package ast

import (
	"context"
	"fmt"
	"time"

	afs "github.com/viant/afs"
	"github.com/viant/afs/url"
	"github.com/viant/x/syntetic/adapter"
)

// NewMemCaseURI builds a unique mem://localhost/test/<ts>/<caseName> root URI
// using a high-resolution timestamp to avoid collisions.
func NewMemCaseURI(caseName string) string {
	ts := time.Now().UTC().UnixNano()
	return fmt.Sprintf("mem://localhost/test/%d/%s", ts, caseName)
}

// SeedMemModule uploads the provided files map (relative path -> content)
// into the in-memory module rooted at root. Paths are joined with url.Join
// so you can pass entries like "go.mod" or "pkg/types.go".
func SeedMemModule(ctx context.Context, svc afs.Service, root string, files map[string][]byte) error {
	for rel, data := range files {
		if err := svc.Upload(ctx, url.Join(root, rel), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// NewMemFS returns an io/fs adapter for the in-memory module so it can be
// consumed by LoadModuleFS/LoadPackageFS.
func NewMemFS(ctx context.Context, svc afs.Service, root string) any { // returns fs.FS, typed as any to avoid import cycles here
	return adapter.NewFS(ctx, svc, root)
}
