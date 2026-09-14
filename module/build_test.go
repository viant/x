package module

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildWorkspaceUsesGoSelection(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                  "module example.com/root\ngo 1.25.8\n",
		"go.work":                 "go 1.25.8\nuse ./missing\n",
		"base.go":                 "package app\ntype Base struct{}\n",
		"chosen_windows_amd64.go": "//go:build feature\n\npackage app\ntype Chosen struct{}\n",
		"other_linux.go":          "package app\ntype Other struct{}\n",
		"ignored.go":              "//go:build excluded\n\npackage app\ntype Broken ???\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(), "GOWORK=off", "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	selected, err := (BuildWorkspace{BaseDir: root, Tags: "feature", Env: env}).Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Packages) != 1 || !reflect.DeepEqual(selected.Packages[0].GoFiles, []string{"base.go", "chosen_windows_amd64.go"}) {
		t.Fatalf("selection %+v", selected)
	}
	workspace := selected.Workspace()
	var visited []string
	if err = workspace.Walk(context.Background(), []string{"..."}, nil, func(f File) error {
		if filepath.Ext(f.Path) == ".go" {
			visited = append(visited, filepath.Base(f.Path))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(visited, selected.Packages[0].GoFiles) {
		t.Fatalf("walk %v", visited)
	}
	location, err := workspace.Package("example.com/root")
	if err != nil || location == nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(workspace.SourceFS(location.Module), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "ignored.go" || entry.Name() == "other_linux.go" {
			t.Fatal("unselected source reached loader")
		}
	}
	env = append(env, "GOWORK="+filepath.Join(root, "go.work"))
	if _, err = (BuildWorkspace{BaseDir: root, Env: env}).Resolve(context.Background()); err == nil {
		t.Fatal("explicit broken workspace ignored")
	}
}
