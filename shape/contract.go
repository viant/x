package shape

import (
	"fmt"
	"path"
	"reflect"
	"strings"
)

type Packages struct {
	Default string
	Imports map[string]string
}
type Contract struct{ Packages Packages }

func (c Contract) Equivalent(left, right string) (bool, error) {
	resolver := Resolver{Package: c.Packages.Default, Imports: c.Packages.Imports}
	a, err := resolver.Canonical(left)
	if err != nil {
		return false, err
	}
	b, err := resolver.Canonical(right)
	if err != nil {
		return false, err
	}
	return strings.ReplaceAll(a, "interface{}", "any") == strings.ReplaceAll(b, "interface{}", "any"), nil
}

func (c Contract) ValidateLinked(expression string, typeOf reflect.Type) error {
	actual, err := (Resolver{Qualifier: func(pkg, _ string) string { return pkg }}).Expression(typeOf)
	if err != nil {
		return err
	}
	imports := map[string]string{}
	for alias, pkg := range c.Packages.Imports {
		imports[alias] = pkg
	}
	if c.Packages.Default != "" {
		imports[path.Base(c.Packages.Default)] = c.Packages.Default
	}
	declared, err := (Resolver{Package: c.Packages.Default, Imports: imports}).Canonical(expression)
	if err != nil {
		return err
	}
	linked, err := (Resolver{}).Canonical(actual)
	if err != nil {
		return err
	}
	if strings.ReplaceAll(declared, "interface{}", "any") != strings.ReplaceAll(linked, "interface{}", "any") {
		return fmt.Errorf("declared %s differs from runtime %s", declared, linked)
	}
	return nil
}
