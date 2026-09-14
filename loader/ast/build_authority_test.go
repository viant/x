package ast_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/viant/x"
	loader "github.com/viant/x/loader/ast"
	"github.com/viant/x/module"
	"github.com/viant/x/shape"
)

func TestBuildSelectedDependencyImportAuthority(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                        "module example.com/app\ngo 1.25\nrequire example.com/contracts/v2 v2.0.0\nreplace example.com/contracts/v2 => ./dependency\n",
		"app.go":                        "package app\nimport \"example.com/contracts/v2\"\ntype Input struct{Value *contracts.Claims}\n",
		"dependency/go.mod":             "module example.com/contracts/v2\ngo 1.25\n",
		"dependency/claims.go":          "package contracts\nimport \"example.com/contracts/v2/models\"\ntype Claims struct{records.Registered; Next *Claims}\ntype Alias = records.Registered\n",
		"dependency/explicit.go":        "package contracts\nimport other \"example.com/contracts/v2/models\"\ntype Explicit struct{Value map[string][]*other.Registered}\n",
		"dependency/models/selected.go": "//go:build selected\n\npackage records\nimport \"time\"\ntype Registered struct{At time.Time}\n",
		"dependency/models/excluded.go": "//go:build !selected\n\npackage records\ntype Registered struct{Wrong int}\n",
	}
	for name, data := range files {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	selected, err := (module.BuildWorkspace{BaseDir: root, Patterns: []string{"."}, Tags: "selected", Env: append(os.Environ(), "GOWORK=off")}).Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	workspace := selected.Workspace()
	var roots []string
	if err = workspace.Walk(ctx, []string{"..."}, nil, func(f module.File) error { roots = append(roots, f.ImportPath); return nil }); err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		if root != "example.com/app" {
			t.Fatalf("dependency discovered as root: %s", root)
		}
	}
	set, err := (loader.LocalPackageLoader{Workspace: workspace}).Load(ctx, "example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Roots) != 1 || len(set.Packages) != 3 {
		t.Fatalf("roots %d packages %d", len(set.Roots), len(set.Packages))
	}
	checks := map[string]map[string][]string{
		"example.com/app": {"Input": {"example.com/contracts/v2.Claims"}},
		"example.com/contracts/v2": {
			"Claims":   {"example.com/contracts/v2.Claims", "example.com/contracts/v2/models.Registered"},
			"Alias":    {"example.com/contracts/v2/models.Registered"},
			"Explicit": {"example.com/contracts/v2/models.Registered"},
		},
		"example.com/contracts/v2/models": {"Registered": {"time.Time"}},
	}
	for path, types := range checks {
		pkg := set.Packages[path]
		if pkg == nil {
			t.Fatalf("missing %s", path)
		}
		for _, typ := range pkg.Types {
			want, ok := types[typ.Name]
			if !ok {
				continue
			}
			refs, err := shape.New(&x.Type{PkgPath: path, Name: typ.Name, SynteticType: typ}, nil).References()
			if err != nil || !reflect.DeepEqual(want, refs) {
				t.Errorf("%s.%s references %v, want %v: %v", path, typ.Name, refs, want, err)
			}
			delete(types, typ.Name)
		}
		if len(types) > 0 {
			t.Fatalf("missing declarations: %v", types)
		}
	}
	// The model retains authored imports while descriptor lookup uses Go's names.
	for _, file := range set.Packages["example.com/contracts/v2"].Files {
		for _, imp := range file.Imports {
			if file.Name == "claims.go" && imp.Alias != "" {
				t.Fatalf("authored alias changed: %+v", imp)
			}
			if file.Name == "explicit.go" && imp.Alias != "other" {
				t.Fatalf("explicit alias changed: %+v", imp)
			}
		}
	}
}
