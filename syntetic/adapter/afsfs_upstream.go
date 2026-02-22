package adapter

import (
	"context"
	afs "github.com/viant/afs"
	afsio "github.com/viant/afs/adapter/io"
)

// FS re-exports the upstream AFS adapter when the viant_afs build tag is set.
type FS = afsio.FS

// NewFS constructs a new adapter filesystem using the upstream adapter.
func NewFS(ctx context.Context, svc afs.Service, root string) *FS { return afsio.NewFS(ctx, svc, root) }
