package module

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func workspaceFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		name = filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocalWorkspaceReplacementAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, app, work string
		relative        bool
	}{
		{"unused_missing", "module corp.example/app\nreplace corp.example/private => ../private\nreplace corp.example/unused => ../absent\n", "", false},
		{"relative_base", "module corp.example/app\nreplace corp.example/private => ../private\n", "", true},
		{"work_override", "module corp.example/app\nreplace corp.example/private => ../absent\n", "go 1.21\nuse ./app\nreplace corp.example/private => ./private\n", false},
		{"required_version", "module corp.example/app\nrequire corp.example/private v1.2.3\nreplace corp.example/private => ../absent\nreplace corp.example/private v1.2.3 => ../private\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			workspaceFiles(t, root, map[string]string{"app/go.mod": tc.app, "app/api/a.go": "package api", "private/go.mod": "module corp.example/private\nreplace corp.example/peer => ../absent\n", "private/model/p.go": "package model"})
			if tc.work != "" {
				workspaceFiles(t, root, map[string]string{"go.work": tc.work})
			}
			base := filepath.Join(root, "app")
			if tc.relative {
				current, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				base, err = filepath.Rel(current, base)
				if err != nil {
					t.Fatal(err)
				}
			}
			workspace, err := (LocalWorkspace{BaseDir: base}).Resolve(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			location, err := workspace.Package("corp.example/private/model")
			if err != nil || location == nil {
				t.Fatalf("private package=%v,error=%v", location, err)
			}
			if location, err := workspace.Package("corp.example/peer/model"); err != nil || location != nil {
				t.Fatalf("dependency replacement incorrectly inherited: %v,%v", location, err)
			}
			location.Module.Path = "mutated"
			if again, err := workspace.Package("corp.example/private/model"); err != nil || again == nil || again.Module.Path != "corp.example/private" {
				t.Fatalf("workspace authority mutated: %v,%v", again, err)
			}
		})
	}
}

func TestLocalWorkspaceSelection(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"app/go.mod":         "module corp.example/app\ngo 1.21\nreplace corp.example/private => ../private\n",
		"app/api/a.go":       "package api",
		"private/go.mod":     "module corp.example/private\ngo 1.21\n",
		"private/model/m.go": "package model",
		"other/go.mod":       "module corp.example/other\ngo 1.21\n",
		"other/api/o.go":     "package api",
		"go.work":            "go 1.21\nuse (\n ./app\n ./other\n)\n",
	} {
		name = filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := (LocalWorkspace{BaseDir: filepath.Join(root, "app")}).Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path  string
		found bool
	}{{"corp.example/app/api", true}, {"corp.example/private/model", true}, {"corp.example/other/api", true}, {"fmt", false}} {
		location, err := workspace.Package(test.path)
		if err != nil || (location != nil) != test.found {
			t.Fatalf("package=%s,location=%v,error=%v", test.path, location, err)
		}
	}
	if _, err := workspace.Package("corp.example/app/../private/model"); err == nil {
		t.Fatal("escaped module accepted")
	}
	var privateFiles int
	if err := workspace.Walk(context.Background(), []string{"..."}, nil, func(file File) error {
		if file.ImportPath == "corp.example/private/model" {
			privateFiles++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if privateFiles != 0 {
		t.Fatal("dependency module became a selected root")
	}
}

func TestLocalWorkspaceRejectsDuplicateModule(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		directory := filepath.Join(root, name)
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module corp.example/same\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dirs := range [][]string{{"a", "b"}, {"b", "a"}} {
		if _, err := (LocalWorkspace{BaseDir: root, ModuleDirs: dirs}).Resolve(context.Background()); err == nil {
			t.Fatal("ambiguous module accepted")
		}
	}
}
