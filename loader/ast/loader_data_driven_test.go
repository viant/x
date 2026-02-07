//go:build viant_afs

package ast_test

import (
	"testing"

	tu "github.com/viant/x/loader/ast/testutil"
)

func TestLoader_DataDriven(t *testing.T) {
	cases := []tu.Case{
		{
			Name: "struct_basic",
			Files: map[string]string{
				"root/go.mod":     "module example.com/dd\n\n",
				"root/p/types.go": "package p\n\n type S struct{ ID int; Name string }\n",
			},
			Dir: "root/p",
			Expect: &tu.Expect{
				PkgPath: "example.com/dd/p",
				Types:   []string{"S"},
				Body:    map[string]string{"S": "struct{ ID int; Name string }"},
			},
		},
		{
			Name: "imports_globals",
			Files: map[string]string{
				"root/go.mod": "module example.com/dd\n\n",
				"root/p/a.go": "package p\n\nimport tm \"time\"\nconst A = tm.Second\nvar B = tm.Now()\n",
			},
			Dir:    "root/p",
			Expect: &tu.Expect{Consts: []string{"A"}, Vars: []string{"B"}},
		},
		{
			Name: "embed_single",
			Files: map[string]string{
				"root/go.mod":     "module example.com/dd\n\n",
				"root/p/data.txt": "hello",
				"root/p/e.go":     "package p\n\nimport \"embed\"\n//go:embed data.txt\nvar FS embed.FS\n",
			},
			Dir:    "root/p",
			Expect: &tu.Expect{Embeds: map[string][]string{"e.go": {"FS"}}},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, c.Run)
	}
}
