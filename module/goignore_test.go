package module

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalDiscoveryGoIgnore(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		location := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(location), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(location, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/app\n")
	for _, name := range []string{"keep.go", "logs/a.go", "logs/nested/b.go", "logs2/c.go", "single.go", "vendor/v.go", "testdata/t.go"} {
		write(name, "package test\n")
	}

	walk := func(include ...string) []string {
		t.Helper()
		var got []string
		err := (LocalDiscovery{BaseDir: root, Include: include}).Walk(context.Background(), func(file File) error {
			rel, err := filepath.Rel(root, file.Path)
			if err != nil {
				return err
			}
			got = append(got, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	without := walk("...")
	exactWithout := walk("example.com/app/logs")
	wildcardWithout := walk("example.com/app/logs/...")
	if err := os.WriteFile(filepath.Join(root, ".goignore"), []byte("# comment\r\n  \r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if withEmpty := walk("..."); !reflect.DeepEqual(withEmpty, without) {
		t.Fatalf("comment-only .goignore changed selection: before=%v after=%v", without, withEmpty)
	}
	if got := walk("example.com/app/logs"); !reflect.DeepEqual(got, exactWithout) {
		t.Fatalf("comment-only .goignore changed exact selection: before=%v after=%v", exactWithout, got)
	}
	if got := walk("example.com/app/logs/..."); !reflect.DeepEqual(got, wildcardWithout) {
		t.Fatalf("comment-only .goignore changed wildcard selection: before=%v after=%v", wildcardWithout, got)
	}
	if err := os.WriteFile(filepath.Join(root, ".goignore"), []byte("# literal paths\r\n logs/ \r\nsingle.go\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	want := []string{"go.mod", "keep.go", "logs2/c.go"}
	if got := walk("..."); !reflect.DeepEqual(got, want) {
		t.Fatalf("files=%v want=%v", got, want)
	}
}

func TestLocalDiscoveryGoIgnoreFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":                  "module example.com/app\n",
		".goignore":               "private\n",
		"private/child/secret.go": "package private\n",
		"visible/keep.go":         "package visible\n",
	} {
		location := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(location), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(location, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	err := (LocalDiscovery{BaseDir: filepath.Join(root, "private"), Include: []string{"..."}}).Walk(context.Background(), func(file File) error {
		got = append(got, filepath.Base(file.Path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("ignored BaseDir was traversed: %v", got)
	}
}

func TestReadGoIgnoreRejectsEscapingPaths(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"/absolute", "../parent", "nested/../../escape", ".", "./"} {
		t.Run(strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, ".goignore"), []byte(value+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := readGoIgnore(root); err == nil || !strings.Contains(err.Error(), ":1:") {
				t.Fatalf("expected line-numbered rejection for %q, got %v", value, err)
			}
		})
	}
}

func TestLocalDiscoveryGoIgnorePrunesUnreadableDirectory(t *testing.T) {
	root := t.TempDir()
	ignored := filepath.Join(root, "ignored")
	if err := os.MkdirAll(ignored, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(ignored, 0700)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goignore"), []byte("ignored\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(ignored); err == nil {
		t.Skip("current user can read mode-000 directories")
	}
	err := (LocalDiscovery{BaseDir: root, Include: []string{"..."}}).Walk(context.Background(), func(File) error { return nil })
	if err != nil {
		t.Fatalf("ignored directory was still traversed: %v", err)
	}
}
