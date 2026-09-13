package shape

import (
	"bytes"
	"strings"
	"testing"
)

func TestSourceRemoveExactStructFields(t *testing.T) {
	for _, test := range []struct {
		name, old, next, want, failure string
		remove                         []SourceFieldRemoval
	}{
		{name: "preserve authored bytes", old: "package p\ntype Row struct {\n ID int\n // retain note\n Extra int `json:\"extra\"` // retain comment\n Authored string\n}\nfunc(r Row) Note() string {return r.Authored}\n", next: "package p;type Row struct{ ID int }", remove: []SourceFieldRemoval{{"Row", "Extra", "int", `json:"extra"`}}, want: "package p\ntype Row struct {\n ID int\n // retain note\n  // retain comment\n Authored string\n}\nfunc(r Row) Note() string {return r.Authored}\n"},
		{name: "inline separator", old: "package p;type Row struct{ID int; Extra int; Authored string}", next: "package p;type Row struct{ID int}", remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}, want: "package p;type Row struct{ID int;  Authored string}"},
		{name: "presence mirror", old: "package p;type Row struct{ID int; Extra *int; Has *RowHas};type RowHas struct{ID bool; Extra bool}", next: "package p;type Row struct{ID int;Has *RowHas};type RowHas struct{ID bool}", remove: []SourceFieldRemoval{{"Row", "Extra", "*int", ""}, {"RowHas", "Extra", "bool", ""}}},
		{name: "remove retired helper field", old: "package p;type Row struct{ID int};type Helper struct{Extra int}", next: "package p;type Row struct{ID int}", remove: []SourceFieldRemoval{{"Helper", "Extra", "int", ""}}, want: "package p;type Row struct{ID int};type Helper struct{}"},
		{name: "retired import", old: "package p;import clock \"time\";type Row struct{ID int;Extra clock.Time}", next: "package p;type Row struct{ID int}", remove: []SourceFieldRemoval{{"Row", "Extra", "time.Time", ""}}, want: "package p;type Row struct{ID int;}"},
		{name: "custom type conflicts", old: "package p;type Row struct{Extra string}", next: "package p;type Row struct{}", remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}, failure: "customized type or tag"},
		{name: "custom tag conflicts", old: "package p;type Row struct{Extra int `custom:\"keep\"`}", next: "package p;type Row struct{}", remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}, failure: "customized type or tag"},
		{name: "still generated conflicts", old: "package p;type Row struct{Extra int}", next: "package p;type Row struct{Extra int}", remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}, failure: "remains in generated source"},
		{name: "grouped fails closed", old: "package p;type Row struct{Extra,Authored int}", next: "package p;type Row struct{}", remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}, failure: "individually named"},
		{name: "default retains omitted fields", old: "package p;type Row struct{ID int;Extra int}", next: "package p;type Row struct{ID int}", want: "package p;type Row struct{ID int;Extra int}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			old, next := []byte(test.old), []byte(test.next)
			actual, err := (SourceParser{}).EditStructFields(old, next, SourceFieldEdits{Remove: test.remove})
			if string(old) != test.old || string(next) != test.next {
				t.Fatal("source mutated")
			}
			if test.failure != "" {
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.want != "" && string(actual) != test.want {
				t.Fatalf("wanted %q got %q", test.want, actual)
			}
			again, err := (SourceParser{}).EditStructFields(actual, next, SourceFieldEdits{Remove: test.remove})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, again) {
				t.Fatalf("not idempotent: %s", again)
			}
		})
	}
}

func TestSourceEditChangesTypeAndRemovesDifferentField(t *testing.T) {
	actual, err := (SourceParser{}).EditStructFields([]byte("package p;type Row struct{ID int;Extra int;Authored string}"), []byte("package p;type Row struct{ID *int}"), SourceFieldEdits{Types: []SourceFieldTypeUpdate{{"Row", "ID", "*int"}}, Remove: []SourceFieldRemoval{{"Row", "Extra", "int", ""}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != "package p;type Row struct{ID *int;Authored string}" {
		t.Fatalf("source %s", actual)
	}
}
