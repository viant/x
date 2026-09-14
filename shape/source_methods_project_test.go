package shape

import (
	"strings"
	"testing"
)

func TestProjectReceiverMethods(t *testing.T) {
	source := []byte(`package component
import ("fmt";clock "time";_ "database/sql")
type Root struct{}
type Child struct{}
type Other struct{}
// Move preserves this method comment.
func(r *Root) Move(value *Child) *Child {temporary:=Child{};_=temporary;fmt.Println("moved");return value}
func(r *Root) New()*Child{return new(Child)}
func(r *Root) Shadow()int {Child:=func()int{return 7};return Child()}
func(r *Root) Local(){type Child struct{};var v Child;_=v;_=Child{}}
func(r *Other) Keep()clock.Time{return clock.Now()}
func Free(){fmt.Println("kept")}
`)
	imports := map[string]string{"child": "example.com/child"}
	rewriter := Resolver{Rewriter: func(name string) (string, error) {
		if name == "Child" {
			return "child.Child", nil
		}
		return name, nil
	}}
	projected, err := (SourceParser{}).ProjectMethods(source, MethodProjection{Receivers: []string{"Root"}, Package: "root", Resolver: rewriter, Imports: imports})
	if err != nil {
		t.Fatal(err)
	}
	selected, remaining := string(projected.Selected), string(projected.Remaining)
	for _, part := range []string{"package root", "value *child.Child", "new(child.Child)", "temporary := child.Child{}", "return Child()", "var v Child", "_ = Child{}", "Move preserves this method comment"} {
		if !strings.Contains(selected, part) {
			t.Fatalf("selected missing %q:\n%s", part, selected)
		}
	}
	if strings.Contains(selected, "func Free") || strings.Contains(selected, "clock") || strings.Contains(selected, `"database/sql"`) {
		t.Fatal(selected)
	}
	for _, part := range []string{"package component", "Keep() clock.Time", "func Free", `_ "database/sql"`} {
		if !strings.Contains(remaining, part) {
			t.Fatalf("remaining missing %q:\n%s", part, remaining)
		}
	}
	if strings.Contains(remaining, "func (r *Root)") || strings.Contains(remaining, `"example.com/child"`) {
		t.Fatal(remaining)
	}
	if !strings.Contains(string(source), "temporary:=Child{}") {
		t.Fatal("caller source changed")
	}
}

func TestProjectMethodsRejectsUnresolvedDotImport(t *testing.T) {
	_, err := (SourceParser{}).ProjectMethods([]byte(`package p;import . "time";func(r *Root) Now()Time{return Now()}`), MethodProjection{Receivers: []string{"Root"}, Package: "r"})
	if err == nil || !strings.Contains(err.Error(), "dot import") {
		t.Fatal(err)
	}
}

func TestProjectMethodsRejectsComponentPrivateDependency(t *testing.T) {
	_, err := (SourceParser{}).ProjectMethods([]byte(`package component
func privateHelper(){}
func(r *Root) Apply(){privateHelper()}
`), MethodProjection{Receivers: []string{"Root"}, Package: "entities"})
	if err == nil || !strings.Contains(err.Error(), "source-package declaration privateHelper") {
		t.Fatal(err)
	}
}
