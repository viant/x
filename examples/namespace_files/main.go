package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	x "github.com/viant/x"
	"github.com/viant/x/examples/namespace_files/customer"
	"github.com/viant/x/examples/namespace_files/orders"
	"github.com/viant/x/loader/xreflect"
	"github.com/viant/x/syntetic"
)

func main() {
	reg := x.NewRegistry()
	// Attach prebuilt synthetic types for both packages.
	if st, err := xreflect.BuildType(reflect.TypeOf(customer.Customer{})); err == nil {
		reg.Register(x.NewType(reflect.TypeOf(customer.Customer{}), x.WithSynteticType(st)))
	} else {
		log.Fatalf("customer: %v", err)
	}
	if st, err := xreflect.BuildType(reflect.TypeOf(orders.Order{})); err == nil {
		reg.Register(x.NewType(reflect.TypeOf(orders.Order{}), x.WithSynteticType(st)))
	} else {
		log.Fatalf("orders: %v", err)
	}

	// Build a namespace and generate per-package files.
	ns, err := syntetic.FromRegistry(reg)
	if err != nil {
		log.Fatalf("bridge: %v", err)
	}

	files, err := ns.BuildFiles(syntetic.RenderOptions{})
	if err != nil {
		log.Fatalf("build files: %v", err)
	}

	// Write each package’s file under ./gen/<pkg-alias>/types_gen.go
	outRoot := filepath.Join(".", "gen")
	for pkgPath, gf := range files {
		src, err := gf.Render()
		if err != nil {
			log.Fatalf("render %s: %v", pkgPath, err)
		}
		alias := lastSegment(pkgPath)
		dir := filepath.Join(outRoot, alias)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir: %v", err)
		}
		out := filepath.Join(dir, "types_gen.go")
		if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
			log.Fatalf("write: %v", err)
		}
		fmt.Printf("wrote %s (package %s)\n", out, gf.PkgName)
	}
}

func lastSegment(p string) string {
	if p == "" {
		return ""
	}
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return p
	}
	return p[i+1:]
}
