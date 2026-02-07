package xreflect

import (
	"fmt"

	smodel "github.com/viant/x/syntetic/model"
)

// equalNodes performs a structural comparison of two syntetic/model.Node
// graphs. It is intended for golden-ish tests where we care about the
// semantic shape rather than pointer identity.
func equalNodes(got, want smodel.Node) (bool, string) {
	if got == nil || want == nil {
		if got == nil && want == nil {
			return true, ""
		}
		return false, fmt.Sprintf("nil mismatch: got=%T, want=%T", got, want)
	}
	if got.Kind() != want.Kind() {
		return false, fmt.Sprintf("kind mismatch: got=%v, want=%v", got.Kind(), want.Kind())
	}

	switch g := got.(type) {
	case *smodel.Basic:
		w := want.(*smodel.Basic)
		if g.Name != w.Name || g.PkgPath != w.PkgPath {
			return false, fmt.Sprintf("Basic mismatch: got=%+v, want=%+v", g, w)
		}
		return true, ""

	case *smodel.Named:
		w := want.(*smodel.Named)
		if g.Name != w.Name || g.PkgPath != w.PkgPath {
			return false, fmt.Sprintf("Named mismatch: got=%+v, want=%+v", g, w)
		}
		// Do not compare Underlying in reflect-path tests, as the
		// reflection bridge does not provide reliable underlying shapes.
		return true, ""

	case *smodel.Pointer:
		w := want.(*smodel.Pointer)
		return equalNodes(g.Elem, w.Elem)

	case *smodel.Slice:
		w := want.(*smodel.Slice)
		return equalNodes(g.Elem, w.Elem)

	case *smodel.Array:
		w := want.(*smodel.Array)
		if g.Len != w.Len {
			return false, fmt.Sprintf("Array length mismatch: got=%d, want=%d", g.Len, w.Len)
		}
		return equalNodes(g.Elem, w.Elem)

	case *smodel.Map:
		w := want.(*smodel.Map)
		if ok, msg := equalNodes(g.Key, w.Key); !ok {
			return false, "map key: " + msg
		}
		return equalNodes(g.Elem, w.Elem)

	case *smodel.Chan:
		w := want.(*smodel.Chan)
		if g.Dir != w.Dir {
			return false, fmt.Sprintf("Chan dir mismatch: got=%d, want=%d", g.Dir, w.Dir)
		}
		return equalNodes(g.Elem, w.Elem)

	case *smodel.Func:
		return equalFuncs(g, want.(*smodel.Func))

	case *smodel.Interface:
		return equalInterfaces(g, want.(*smodel.Interface))

	case *smodel.Struct:
		return equalStructs(g, want.(*smodel.Struct))
	}

	return false, fmt.Sprintf("unsupported node kind in comparator: %T", got)
}

func equalFuncs(got, want *smodel.Func) (bool, string) {
	if got.Variadic != want.Variadic {
		return false, fmt.Sprintf("Func variadic mismatch: got=%v, want=%v", got.Variadic, want.Variadic)
	}
	if len(got.Params) != len(want.Params) {
		return false, fmt.Sprintf("Func params len mismatch: got=%d, want=%d", len(got.Params), len(want.Params))
	}
	if len(got.Results) != len(want.Results) {
		return false, fmt.Sprintf("Func results len mismatch: got=%d, want=%d", len(got.Results), len(want.Results))
	}
	for i := range got.Params {
		if ok, msg := equalField(got.Params[i], want.Params[i]); !ok {
			return false, fmt.Sprintf("param %d: %s", i, msg)
		}
	}
	for i := range got.Results {
		if ok, msg := equalField(got.Results[i], want.Results[i]); !ok {
			return false, fmt.Sprintf("result %d: %s", i, msg)
		}
	}
	return true, ""
}

func equalInterfaces(got, want *smodel.Interface) (bool, string) {
	if len(got.Methods) != len(want.Methods) {
		return false, fmt.Sprintf("Interface methods len mismatch: got=%d, want=%d", len(got.Methods), len(want.Methods))
	}
	for i := range got.Methods {
		gm, wm := got.Methods[i], want.Methods[i]
		if gm.Name != wm.Name {
			return false, fmt.Sprintf("Interface method[%d] name mismatch: got=%s, want=%s", i, gm.Name, wm.Name)
		}
		if ok, msg := equalFuncs(&gm.Type, &wm.Type); !ok {
			return false, fmt.Sprintf("Interface method[%d] type: %s", i, msg)
		}
	}
	return true, ""
}

func equalStructs(got, want *smodel.Struct) (bool, string) {
	if len(got.Fields) != len(want.Fields) {
		return false, fmt.Sprintf("Struct fields len mismatch: got=%d, want=%d", len(got.Fields), len(want.Fields))
	}
	for i := range got.Fields {
		if ok, msg := equalField(got.Fields[i], want.Fields[i]); !ok {
			return false, fmt.Sprintf("field %d: %s", i, msg)
		}
	}
	return true, ""
}

func equalField(got, want smodel.Field) (bool, string) {
	if got.Name != want.Name || got.Tag != want.Tag || got.Embedded != want.Embedded {
		return false, fmt.Sprintf("Field meta mismatch: got=%+v, want=%+v", got, want)
	}
	return equalNodes(got.Type, want.Type)
}
