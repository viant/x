package ast

import (
	"context"
	"path"
	"reflect"
	"strings"
	"testing"

	xr "github.com/viant/x/builder/xreflect"
	mdl "github.com/viant/x/syntetic/model"
)

type datlyInterfaceResolver struct {
	t reflect.Type
}

func (r datlyInterfaceResolver) ResolveInterface(methods []mdl.Method) (reflect.Type, bool) {
	if len(methods) == 1 && methods[0].Name == "Init" {
		return r.t, true
	}
	return nil, false
}

func TestDatlyConformanceFixtures(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/datlyfx\n\n",
		"root/shared/types.go": `package shared

type User struct {
	ID int
}
`,
		"root/contracts/contracts.go": `package contracts

import "example.com/datlyfx/shared"

type Input struct {
	User shared.User
}

type Initializer interface {
	Init() error
}

type Number interface {
	~int | ~int64
}

type IDs = []int
`,
	})

	ctx := context.Background()
	contracts, err := LoadPackageDeepFS(ctx, fsys, "root/contracts")
	if err != nil {
		t.Fatalf("LoadPackageDeepFS contracts error: %v", err)
	}
	sharedPkg, err := LoadPackageFS(ctx, fsys, "root/shared")
	if err != nil {
		t.Fatalf("LoadPackageFS shared error: %v", err)
	}

	sharedUser := findTypeByName(sharedPkg, "User")
	if sharedUser == nil {
		t.Fatalf("missing fixture type shared.User")
	}
	sharedUserNode := toModelNode(sharedPkg.PkgPath, sharedUser)
	sharedUserRT, err := xr.New().BuildNode(sharedUserNode)
	if err != nil {
		t.Fatalf("shared.User materialization error: %v", err)
	}

	t.Run("cross_package_named_strict_with_resolver", func(t *testing.T) {
		inputType := findTypeByName(contracts, "Input")
		if inputType == nil {
			t.Fatalf("missing fixture type contracts.Input")
		}
		node := toModelNode(contracts.PkgPath, inputType)

		cache := map[string]reflect.Type{
			sharedPkg.PkgPath + ".User": sharedUserRT,
		}
		b := xr.New(
			xr.WithCache(cache),
			xr.WithStrictNamedResolution(true),
		)
		rt, err := b.BuildNode(node)
		if err != nil {
			t.Fatalf("BuildNode Input error: %v", err)
		}
		if rt.Kind() != reflect.Struct || rt.NumField() != 1 {
			t.Fatalf("unexpected Input runtime shape: %v", rt)
		}
		field := rt.Field(0)
		if field.Name != "User" || field.Type.Kind() != reflect.Struct {
			t.Fatalf("unexpected Input.User field: %s %v", field.Name, field.Type)
		}
		if field.Type.NumField() != 1 || field.Type.Field(0).Name != "ID" || field.Type.Field(0).Type.Kind() != reflect.Int {
			t.Fatalf("unexpected shared.User materialization: %v", field.Type)
		}
	})

	t.Run("interface_strict_with_resolver", func(t *testing.T) {
		type initializer interface {
			Init() error
		}

		initType := findTypeByName(contracts, "Initializer")
		if initType == nil {
			t.Fatalf("missing fixture type contracts.Initializer")
		}
		node := toModelNode(contracts.PkgPath, initType)

		want := reflect.TypeOf((*initializer)(nil)).Elem()
		b := xr.New(
			xr.WithStrictInterfaceResolution(true),
			xr.WithInterfaceResolver(datlyInterfaceResolver{t: want}),
		)
		rt, err := b.BuildNode(node)
		if err != nil {
			t.Fatalf("BuildNode Initializer error: %v", err)
		}
		if rt != want {
			t.Fatalf("unexpected interface runtime type: got=%v want=%v", rt, want)
		}
	})

	t.Run("union_strict_reports_normalized_terms", func(t *testing.T) {
		numberType := findTypeByName(contracts, "Number")
		if numberType == nil {
			t.Fatalf("missing fixture type contracts.Number")
		}
		node := toModelNode(contracts.PkgPath, numberType)
		unionNode, ok := node.(*mdl.Union)
		if !ok {
			t.Fatalf("expected union/type-set node, got %T", node)
		}

		var captured []mdl.Term
		b := xr.New(
			xr.WithStrictUnionResolution(true),
			xr.WithUnresolvedUnionReporter(func(terms []mdl.Term) {
				captured = terms
			}),
		)
		_, err := b.BuildNode(unionNode)
		if err == nil {
			t.Fatalf("expected strict union error")
		}
		if !strings.Contains(err.Error(), "~int | ~int64") {
			t.Fatalf("unexpected union error: %v", err)
		}
		if len(captured) != 2 {
			t.Fatalf("expected 2 union terms, got: %d", len(captured))
		}
		if got := xr.JoinUnionTerms(captured); got != "~int | ~int64" {
			t.Fatalf("unexpected normalized union terms: %q", got)
		}
	})

	t.Run("alias_reports_metadata_and_target_type", func(t *testing.T) {
		idsType := findTypeByName(contracts, "IDs")
		if idsType == nil {
			t.Fatalf("missing fixture type contracts.IDs")
		}
		node := toModelNode(contracts.PkgPath, idsType)
		aliasNode, ok := node.(*mdl.Alias)
		if !ok {
			t.Fatalf("expected alias node, got %T", node)
		}

		called := false
		b := xr.New(
			xr.WithAliasReporter(func(a *mdl.Alias, resolved reflect.Type) {
				called = true
				if a.Name != "IDs" || a.PkgPath != contracts.PkgPath {
					t.Fatalf("unexpected alias metadata: %+v", a)
				}
				if resolved.Kind() != reflect.Slice || resolved.Elem().Kind() != reflect.Int {
					t.Fatalf("unexpected alias resolved type: %v", resolved)
				}
			}),
		)
		rt, err := b.BuildNode(aliasNode)
		if err != nil {
			t.Fatalf("BuildNode IDs alias error: %v", err)
		}
		if !called {
			t.Fatalf("expected alias reporter callback")
		}
		if rt.Kind() != reflect.Slice || rt.Elem().Kind() != reflect.Int {
			t.Fatalf("unexpected alias runtime type: %v", rt)
		}
	})
}

func findTypeByName(p *mdl.Package, name string) *mdl.Type {
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func toModelNode(currentPkg string, t *mdl.Type) mdl.Node {
	if t == nil || t.TypeSpec == nil || t.TypeSpec.Type == nil {
		return nil
	}
	target := astExprToModelNode(t.TypeSpec.Type, currentPkg, aliasIndexFromType(t))
	if t.TypeSpec.Assign.IsValid() {
		return &mdl.Alias{
			PkgPath: currentPkg,
			Name:    t.Name,
			Target:  target,
		}
	}
	return target
}

func aliasIndexFromType(t *mdl.Type) map[string]mdl.ImportRef {
	out := map[string]mdl.ImportRef{}
	if t == nil || len(t.Imports) == 0 {
		return out
	}
	for _, imp := range t.Imports {
		if imp == nil || imp.Path == "" {
			continue
		}
		alias := imp.Alias
		if alias == "" {
			alias = path.Base(imp.Path)
		}
		if alias == "_" || alias == "." {
			continue
		}
		out[alias] = *imp
	}
	return out
}
