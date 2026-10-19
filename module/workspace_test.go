package module

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
			t.Setenv("GOENV", "off")
			t.Setenv("GOWORK", "off")
			if tc.work != "" {
				t.Setenv("GOWORK", "")
			}
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
	t.Setenv("GOENV", "off")
	t.Setenv("GOWORK", "")
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

func TestLocalWorkspaceWalkUsesEachModulesGoIgnore(t *testing.T) {
	t.Setenv("GOENV", "off")
	t.Setenv("GOWORK", "")
	root := t.TempDir()
	workspaceFiles(t, root, map[string]string{
		"go.work":                 "go 1.21\nuse (\n ./app\n ./other\n)\n",
		"app/go.mod":              "module corp.example/app\n",
		"app/.goignore":           "api/app-only.go\n",
		"app/api/app-only.go":     "package api\n",
		"app/api/other-only.go":   "package api\n",
		"app/api/keep.go":         "package api\n",
		"other/go.mod":            "module corp.example/other\n",
		"other/.goignore":         "api/other-only.go\n",
		"other/api/app-only.go":   "package api\n",
		"other/api/other-only.go": "package api\n",
		"other/api/keep.go":       "package api\n",
	})
	workspace, err := (LocalWorkspace{BaseDir: filepath.Join(root, "app")}).Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	err = workspace.Walk(context.Background(), []string{"..."}, nil, func(file File) error {
		relative, err := filepath.Rel(resolvedRoot, file.Path)
		if err != nil {
			return err
		}
		selected[filepath.ToSlash(relative)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"app/api/app-only.go":     false,
		"app/api/other-only.go":   true,
		"app/api/keep.go":         true,
		"other/api/app-only.go":   true,
		"other/api/other-only.go": false,
		"other/api/keep.go":       true,
	} {
		if got := selected[name]; got != want {
			t.Errorf("selected[%q]=%v want %v", name, got, want)
		}
	}
}

func TestLocalWorkspaceRejectsDuplicateModule(t *testing.T) {
	t.Setenv("GOWORK", "off")
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

func TestLocalWorkspaceGoWorkEnvironment(t *testing.T) {
	for _, mode := range []string{"off", "explicit", "unset", "empty", "persisted"} {
		t.Run(mode, func(t *testing.T) {
			root, external := t.TempDir(), t.TempDir()
			workspaceFiles(t, root, map[string]string{
				"app/go.mod":         "module corp.example/app\ngo 1.21\nreplace corp.example/private => ../private\n",
				"app/api/a.go":       "package api",
				"extra/go.mod":       "module corp.example/extra\ngo 1.21\nreplace corp.example/peer => ../peer\n",
				"extra/api/e.go":     "package api",
				"private/go.mod":     "module corp.example/private\ngo 1.21\n",
				"private/model/p.go": "package model",
				"peer/go.mod":        "module corp.example/peer\ngo 1.21\n",
				"peer/model/p.go":    "package model",
				"parent/go.mod":      "module corp.example/parent\ngo 1.21\n",
				"parent/api/p.go":    "package api",
				"go.work":            "go 1.21\nuse ./parent\n",
			})
			workspaceFiles(t, external, map[string]string{
				"go.work":            "go 1.21\nuse ./selected\nreplace corp.example/private => ./private\n",
				"selected/go.mod":    "module corp.example/selected\ngo 1.21\n",
				"selected/api/s.go":  "package api",
				"private/go.mod":     "module corp.example/private\ngo 1.21\n",
				"private/model/p.go": "package model",
			})
			t.Setenv("GOENV", "off")
			t.Setenv("GOWORK", "")
			switch mode {
			case "off":
				t.Setenv("GOWORK", "off")
				// The parent workspace is valid syntax but contains a removed module.
				workspaceFiles(t, root, map[string]string{"go.work": "go 1.21\nuse ./removed\n"})
			case "explicit":
				t.Setenv("GOWORK", filepath.Join(external, "go.work"))
			case "unset":
				if err := os.Unsetenv("GOWORK"); err != nil {
					t.Fatal(err)
				}
			case "persisted":
				workspaceFiles(t, external, map[string]string{"goenv": "GOWORK=" + filepath.Join(external, "go.work") + "\n"})
				t.Setenv("GOENV", filepath.Join(external, "goenv"))
				if err := os.Unsetenv("GOWORK"); err != nil {
					t.Fatal(err)
				}
			}
			workspace, err := (LocalWorkspace{BaseDir: filepath.Join(root, "app"), ModuleDirs: []string{"../extra"}}).Resolve(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for path, found := range map[string]bool{
				"corp.example/app/api":       true,
				"corp.example/extra/api":     true,
				"corp.example/private/model": true,
				"corp.example/peer/model":    true,
				"corp.example/parent/api":    mode == "unset" || mode == "empty",
				"corp.example/selected/api":  mode == "explicit" || mode == "persisted",
			} {
				location, err := workspace.Package(path)
				if err != nil || (location != nil) != found {
					t.Fatalf("package %s=%v, error=%v, want found=%v", path, location, err, found)
				}
			}
			location, err := workspace.Package("corp.example/private/model")
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(root, "private", "model")
			if mode == "explicit" || mode == "persisted" {
				want = filepath.Join(external, "private", "model")
			}
			want, err = filepath.EvalSymlinks(want)
			if err != nil {
				t.Fatal(err)
			}
			if location.Dir != want {
				t.Fatalf("replacement directory=%s, want %s", location.Dir, want)
			}
			var roots []string
			if err := workspace.Walk(context.Background(), []string{"..."}, nil, func(file File) error {
				if !strings.HasSuffix(file.Path, ".go") {
					return nil
				}
				roots = append(roots, file.ImportPath)
				if file.ImportPath == "corp.example/private/model" || file.ImportPath == "corp.example/peer/model" {
					t.Fatalf("replacement dependency became a selected root: %s", file.ImportPath)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			wantRoots := 3
			if mode == "off" {
				wantRoots = 2
			}
			if len(roots) != wantRoots {
				t.Fatalf("selected roots=%v, want %d files", roots, wantRoots)
			}
		})
	}
}
