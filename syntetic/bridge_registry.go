package syntetic

import (
	"fmt"

	x "github.com/viant/x"
	"github.com/viant/x/loader/xreflect"
	"github.com/viant/x/syntetic/model"
)

// FromRegistry builds a Namespace from all types in the registry.
// It converts reflect.Type to AST and assigns imports per type.
func FromRegistry(reg *x.Registry) (*model.Namespace, error) {
	if reg == nil {
		return nil, fmt.Errorf("syntetic: registry is required")
	}
	keys := reg.Keys()
	if len(keys) == 0 {
		return &model.Namespace{}, nil
	}
	ns := &model.Namespace{}
	for _, key := range keys {
		aType := reg.Lookup(key)
		if aType == nil || aType.Type == nil {
			continue
		}
		// Prefer prebuilt synthetic type if available.
		if aType.SynteticType != nil {
			ns.AddType(aType.SynteticType)
			continue
		}
		st, err := xreflect.BuildType(aType.Type)
		if err != nil || st == nil {
			continue
		}
		ns.AddType(st)
	}
	return ns, nil
}

// translateImports removed: BuildType returns populated imports.

// FromRegistryFile builds a single GoFile containing declarations from the registry.
func FromRegistryFile(reg *x.Registry) (*model.GoFile, error) {
	if reg == nil {
		return nil, fmt.Errorf("syntetic: registry is required")
	}
	keys := reg.Keys()
	file := &model.GoFile{}
	if len(keys) == 0 {
		return file, nil
	}
	for _, key := range keys {
		aType := reg.Lookup(key)
		if aType == nil || aType.Type == nil {
			continue
		}
		st, err := xreflect.BuildType(aType.Type)
		if err != nil || st == nil {
			continue
		}
		file.AddType(st)
	}
	return file, nil
}
