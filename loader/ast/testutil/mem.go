//go:build viant_afs

package testutil

import (
	"context"
	"io/fs"
	"testing"

	"github.com/viant/afs/mem"
	"github.com/viant/x/syntetic/adapter"
	"github.com/viant/x/syntetic/loader"
)

// NewMemModuleFS creates a unique in-memory module and seeds it with files.
// files maps a relative path (e.g., "go.mod", "pkg/types.go") to file content.
// It returns the root URI and an io/fs filesystem to pass to loader helpers.
func NewMemModuleFS(t *testing.T, caseName string, files map[string]string) (string, fs.FS) {
	t.Helper()
	ctx := context.Background()
	svc := mem.New()
	root := loader.NewMemCaseURI(caseName)

	bin := map[string][]byte{}
	for k, v := range files {
		bin[k] = []byte(v)
	}
	if err := loader.SeedMemModule(ctx, svc, root, bin); err != nil {
		t.Fatalf("SeedMemModule error: %v", err)
	}
	fsys := adapter.NewFS(ctx, svc, root)
	return root, fsys
}
