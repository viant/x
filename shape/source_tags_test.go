package shape

import (
	"bytes"
	"strings"
	"testing"
)

func TestSourceExactTagUpdate(t *testing.T) {
	old := []byte("package p;type Row struct{Value int `codec:\"old\"` /*keep*/; Other int}")
	next := []byte("package p;type Row struct{Value int `codec:\"new\"`; Other int}")
	edits := SourceFieldEdits{Tags: []SourceFieldTagUpdate{{"Row", "Value", `codec:"old"`, `codec:"new"`}}}
	actual, err := (SourceParser{}).EditStructFields(old, next, edits)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != strings.Replace(string(old), `codec:"old"`, `codec:"new"`, 1) {
		t.Fatalf("unrelated bytes changed: %s", actual)
	}
	again, err := (SourceParser{}).EditStructFields(actual, next, edits)
	if err != nil || !bytes.Equal(actual, again) {
		t.Fatal("tag update not idempotent", err)
	}
	if _, err = (SourceParser{}).UpdateStructFields(old, next, nil); err == nil {
		t.Fatal("default tag protection lost")
	}
	custom := []byte(strings.Replace(string(old), `codec:"old"`, `codec:"custom"`, 1))
	if _, err = (SourceParser{}).EditStructFields(custom, next, edits); err == nil || !strings.Contains(err.Error(), "customized tag") {
		t.Fatalf("custom tag error %v", err)
	}
	wrong := edits
	wrong.Tags = []SourceFieldTagUpdate{{"Row", "Value", `codec:"old"`, `codec:"wrong"`}}
	if _, err = (SourceParser{}).EditStructFields(old, next, wrong); err == nil {
		t.Fatal("wrong exact tag accepted")
	}
}
