package shape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/viant/x"
	model "github.com/viant/x/syntetic/model"
)

const promotedMethodSource = `package hooks
type Leaf struct{}
func(Leaf) Read()string{return ""}
func(*Leaf) Write(int){}
type Left struct{Leaf}
type Right struct{Leaf}
type Diamond struct{Left;Right}
type Near struct{Left;Leaf}
type Shadow struct{Leaf;Read int}
type Pointer struct{*Leaf}
type Cycle struct{*Cycle;Leaf}
type Override struct{Leaf}
func(*Override)Read()string{return ""}
type Empty struct{}
type State[T,P any]struct{}
type Hook[T,P any]struct{}
func(*Hook[A,B]) Init(current *A,state State[A,B])error{return nil}
func(*Hook[T,P]) Validate(current *T,state State[T,P])error{return nil}
type GenericParent struct{Hook[int,string]}
`

func TestSyntheticMethodPromotionMatchesGo(t *testing.T) {
	registry, file, set := methodFixture(t, promotedMethodSource)
	pkg, err := (&types.Config{}).Check("example.com/hooks", set, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (*x.Type, error) { return registry[name], nil }
	for _, name := range []string{"Leaf", "Left", "Diamond", "Near", "Shadow", "Pointer", "Cycle", "Override", "GenericParent"} {
		for _, pointer := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/value", true: "/pointer"}[pointer], func(t *testing.T) {
				actual, err := New(registry["example.com/hooks."+name], lookup).Methods(pointer)
				if err != nil {
					t.Fatal(err)
				}
				typ := pkg.Scope().Lookup(name).Type()
				if pointer {
					typ = types.NewPointer(typ)
				}
				want := checkedMethods(types.NewMethodSet(typ))
				if !reflect.DeepEqual(actual, want) {
					t.Fatalf("methods = %#v, compiler = %#v", actual, want)
				}
			})
		}
	}
}

func TestSyntheticGenericMethodSpecializationMatchesGo(t *testing.T) {
	registry, file, set := methodFixture(t, promotedMethodSource)
	pkg, err := (&types.Config{}).Check("example.com/hooks", set, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{Package: "example.com/hooks", Lookup: func(name string) (*x.Type, error) { return registry[name], nil }}
	for _, test := range []struct {
		expression string
		args       []types.Type
	}{{"Hook[int,string]", []types.Type{types.Typ[types.Int], types.Typ[types.String]}}, {"Hook[string,int]", []types.Type{types.Typ[types.String], types.Typ[types.Int]}}} {
		t.Run(test.expression, func(t *testing.T) {
			resolved, err := resolver.Resolve(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := New(resolved.Descriptor, resolver.Lookup).Methods(true)
			if err != nil {
				t.Fatal(err)
			}
			instance, err := types.Instantiate(nil, pkg.Scope().Lookup("Hook").Type(), test.args, true)
			if err != nil {
				t.Fatal(err)
			}
			want := checkedMethods(types.NewMethodSet(types.NewPointer(instance)))
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("methods = %#v, compiler = %#v", actual, want)
			}
		})
	}
	if registry["example.com/hooks.Hook"].SynteticType.TypeSpec.TypeParams == nil {
		t.Fatal("specialization mutated original generic type")
	}
}

func methodFixture(t *testing.T, source string) (map[string]*x.Type, *ast.File, *token.FileSet) {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "hooks.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]*x.Type{}
	for _, decl := range file.Decls {
		if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.TYPE {
			for _, spec := range group.Specs {
				typ := spec.(*ast.TypeSpec)
				result["example.com/hooks."+typ.Name.Name] = &x.Type{Name: typ.Name.Name, PkgPath: "example.com/hooks", SynteticType: &model.Type{Name: typ.Name.Name, PkgPath: "example.com/hooks", TypeSpec: typ}}
			}
		}
	}
	for _, decl := range file.Decls {
		if method, ok := decl.(*ast.FuncDecl); ok && method.Recv != nil {
			reference, err := (Resolver{}).Reference(rendered(method.Recv.List[0].Type))
			if err != nil {
				t.Fatal(err)
			}
			typ := result["example.com/hooks."+reference.BaseName].SynteticType
			if len(reference.Wrappers) > 0 {
				typ.PtrMethodsAST = append(typ.PtrMethodsAST, method)
			} else {
				typ.MethodsAST = append(typ.MethodsAST, method)
			}
		}
	}
	return result, file, set
}

func checkedMethods(set *types.MethodSet) []Method {
	result := []Method{}
	for i := 0; i < set.Len(); i++ {
		object := set.At(i).Obj()
		if !object.Exported() {
			continue
		}
		signature := object.Type().(*types.Signature)
		method := Method{Name: object.Name(), Variadic: signature.Variadic()}
		for j := 0; j < signature.Params().Len(); j++ {
			method.Parameters = append(method.Parameters, types.TypeString(signature.Params().At(j).Type(), func(p *types.Package) string { return p.Path() }))
		}
		for j := 0; j < signature.Results().Len(); j++ {
			method.Results = append(method.Results, types.TypeString(signature.Results().At(j).Type(), func(p *types.Package) string { return p.Path() }))
		}
		for j, value := range method.Parameters {
			canonical, _ := (Resolver{}).Canonical(value)
			method.Parameters[j] = canonical
		}
		result = append(result, method)
	}
	return result
}

func TestGeneratedGenericHookPackage(t *testing.T) {
	directory := t.TempDir()
	verification := `package hooks
import "testing"
type contract[T,P any]interface{Init(*T,State[T,P])error;Validate(*T,State[T,P])error}
var _ contract[int,string]=(*Hook[int,string])(nil)
var _ contract[int,string]=(*GenericParent)(nil)
func TestGeneratedMethods(t *testing.T){value:=&GenericParent{};if err:=value.Init(nil,State[int,string]{});err!=nil{t.Fatal(err)};if err:=value.Validate(nil,State[int,string]{});err!=nil{t.Fatal(err)}}
`
	for name, content := range map[string]string{"go.mod": "module example.com/hooks\n\ngo 1.24\n", "hooks.go": promotedMethodSource, "generated_test.go": verification} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", "-race", "./...")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated generic hook package: %v\n%s", err, output)
	}
}
