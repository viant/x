package module

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestModuleIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		invalid            bool
	}{
		{"plain", "module example.com/app\ngo 1.21\n", "example.com/app", false},
		{"quoted", "// comment\nmodule \"example.com/app\" // module identity\n", "example.com/app", false},
		{"missing", "go 1.21\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePath("go.mod", []byte(tc.source))
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("path=%s error=%v", got, err)
			}
		})
	}
	fsys := fstest.MapFS{"go.mod": {Data: []byte("module example.com/app")}, "sub/file.go": {Data: []byte("package sub")}}
	info, err := LocateFS(fsys, "sub/file.go")
	if err != nil || info.Dir != "." || info.Path != "example.com/app" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	if _, err := LocateFS(fsys, "../escape"); err == nil {
		t.Fatal("accepted escaping filesystem path")
	}
}

func TestLocalDiscoveryMatrix(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{"go.mod": "module example.com/app", "root.go": "package app", "svc/a.go": "package svc", "svc/inner/b.go": "package inner", "svc2/c.go": "package svc2", "vendor/v.go": "package vendor", "testdata/x.go": "package testdata", "nested/go.mod": "module example.com/nested", "nested/x.go": "package nested", ".hidden/x.go": "package hidden"} {
		location := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(location), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(location, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name                   string
		include, exclude, want []string
	}{
		{"exact", []string{"example.com/app/svc"}, nil, []string{"svc/a.go"}},
		{"recursive boundary", []string{"example.com/app/svc/..."}, nil, []string{"svc/a.go", "svc/inner/b.go"}},
		{"exclude", []string{"example.com/app/..."}, []string{"example.com/app/svc/..."}, []string{"go.mod", "root.go", "svc2/c.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			err := (LocalDiscovery{BaseDir: root, Include: tc.include, Exclude: tc.exclude}).Walk(context.Background(), func(f File) error {
				rel, _ := filepath.Rel(root, f.Path)
				got = append(got, filepath.ToSlash(rel))
				return nil
			})
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("files=%v want=%v err=%v", got, tc.want, err)
			}
		})
	}
	info, err := LocateLocal(filepath.Join(root, "future", "package"))
	if err != nil || info.Dir != root {
		t.Fatalf("location=%+v err=%v", info, err)
	}
	if _, err := ImportPathLocal(root, "example.com/app", filepath.Dir(root)); err == nil {
		t.Fatal("allowed package outside module")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (LocalDiscovery{BaseDir: root, Include: []string{"..."}}).Walk(ctx, func(File) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
