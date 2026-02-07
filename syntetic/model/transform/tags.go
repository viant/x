package transform

import (
	"reflect"
	"strings"

	mdl "github.com/viant/x/syntetic/model"
)

// TagOption configures the TagTransformer.
type TagOption func(*TagTransformer)

// WithTagKey sets the struct tag key to read directives from (default: "x").
func WithTagKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.tagKey = key
		}
	}
}

// WithRenameKey sets the directive name for field rename (default: "rename").
func WithRenameKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.renameKey = key
		}
	}
}

// WithTypeKey sets the directive name for type override (default: "type").
func WithTypeKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.typeKey = key
		}
	}
}

// WithOmitKey sets the directive name for omitting a field (default: "omit").
func WithOmitKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.omitKey = key
		}
	}
}

// WithFlatKey sets the directive name for embedding/flattening (default: "flat").
func WithFlatKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.flatKey = key
		}
	}
}

// WithInlineKey sets an additional directive alias for embedding/flattening (default: "inline").
func WithInlineKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.inlineKey = key
		}
	}
}

// WithTagOverrideKey sets the directive name used to override the field tag (default: "tag").
func WithTagOverrideKey(key string) TagOption {
	return func(t *TagTransformer) {
		if key != "" {
			t.tagOverrideKey = key
		}
	}
}

// TagTransformer applies struct-field level rewrites based on a configurable tag
// namespace (default key: x). Supported directives:
//   - type: fully-qualified type path "pkg/import/path.Name" to override field type
//   - rename: new field name for emission
//   - omit: drop this field from emission
//   - flat / inline: mark field as embedded (flatten)
//   - tag: override field tag for emission (raw literal without backticks)
//
// The transformer operates on model.Nodes. For *model.Struct it evaluates and
// applies directives to each field; it recurses into nested nodes.
type TagTransformer struct {
	tagKey         string
	renameKey      string
	typeKey        string
	omitKey        string
	flatKey        string
	inlineKey      string
	tagOverrideKey string
}

// FromTags returns a transformer that applies tag-driven rewrites.
func FromTags(opts ...TagOption) Transformer {
	t := &TagTransformer{
		tagKey:         "x",
		renameKey:      "rename",
		typeKey:        "type",
		omitKey:        "omit",
		flatKey:        "flat",
		inlineKey:      "inline",
		tagOverrideKey: "tag",
	}
	for _, o := range opts {
		if o != nil {
			o(t)
		}
	}
	return t
}

func (t *TagTransformer) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (t *TagTransformer) ApplyType(tp *mdl.Type) *mdl.Type         { return tp }

func (t *TagTransformer) ApplyNode(n mdl.Node) mdl.Node {
	switch v := n.(type) {
	case *mdl.Struct:
		if v == nil || len(v.Fields) == 0 {
			return n
		}
		out := &mdl.Struct{Fields: make([]mdl.Field, 0, len(v.Fields))}
		for _, f := range v.Fields {
			// Apply to nested type first
			if f.Type != nil {
				f.Type = t.ApplyNode(f.Type)
			}
			// Parse directives from tag
			omit, rename, flat, tagovr, typeref := t.parseDirectives(f.Tag)
			if omit {
				continue
			}
			if rename != "" {
				f.Name = rename
			}
			if flat {
				f.Embedded = true
			}
			if tagovr != "" {
				f.Tag = tagovr
			}
			if typeref != "" {
				if pkg, name := splitPkgAndName(typeref); name != "" {
					f.Type = &mdl.Named{PkgPath: pkg, Name: name}
				}
			}
			out.Fields = append(out.Fields, f)
		}
		return out
	case *mdl.Pointer:
		v.Elem = t.ApplyNode(v.Elem)
		return v
	case *mdl.Slice:
		v.Elem = t.ApplyNode(v.Elem)
		return v
	case *mdl.Array:
		v.Elem = t.ApplyNode(v.Elem)
		return v
	case *mdl.Map:
		v.Key = t.ApplyNode(v.Key)
		v.Elem = t.ApplyNode(v.Elem)
		return v
	case *mdl.Chan:
		v.Elem = t.ApplyNode(v.Elem)
		return v
	case *mdl.Interface:
		// No tag-based changes; return as-is.
		return v
	case *mdl.Func:
		// No tag-based changes; return as-is.
		return v
	}
	return n
}

func (t *TagTransformer) parseDirectives(tag string) (omit bool, rename string, flat bool, tagOverride string, typeRef string) {
	if tag == "" || t.tagKey == "" {
		return
	}
	raw := reflect.StructTag(tag).Get(t.tagKey)
	if raw == "" {
		return
	}
	// Split by comma; support k=v or lone flags
	parts := splitCSV(raw)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == t.omitKey {
			omit = true
			continue
		}
		if p == t.flatKey || p == t.inlineKey {
			flat = true
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		key := kv[0]
		val := ""
		if len(kv) == 2 {
			val = kv[1]
		}
		switch key {
		case t.renameKey:
			rename = val
		case t.tagOverrideKey:
			tagOverride = val
		case t.typeKey:
			typeRef = val
		}
	}
	return
}

func splitPkgAndName(s string) (pkgPath, name string) {
	if s == "" {
		return "", ""
	}
	// Expect "a/b/c.Name"; find last dot
	if i := strings.LastIndexByte(s, '.'); i > 0 && i+1 < len(s) {
		return s[:i], s[i+1:]
	}
	// If no dot, treat entire as name
	return "", s
}

func splitCSV(s string) []string {
	// Simple comma split; values are not quoted by reflect.StructTag.Get
	// so we can treat commas as separators safely.
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
