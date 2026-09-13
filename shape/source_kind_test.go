package shape

import "testing"

func TestSourceClassify(t *testing.T) {
	for _, test := range []struct {
		source string
		kind   SourceKind
	}{
		{"package p; type Row struct{}", SourceShape},
		{"package p; type Row struct{};func(*Row)Init(){}", SourceShape},
		{"package p; func New(){}", SourceProgram},
		{"package p; var Component=1", SourceProgram},
		{"package p; type Row struct{};func New(){}", SourceUnknown},
		{"package p", SourceUnknown},
	} {
		kind, err := (SourceParser{}).Classify([]byte(test.source))
		if err != nil || kind != test.kind {
			t.Fatalf("%s: %s %v", test.source, kind, err)
		}
	}
}
