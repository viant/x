package shape

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

// SourceFieldTagUpdate authorizes one exact tag transition from a previously
// emitted tag. Customized tags fail closed; the field and surrounding bytes
// stay in place. Previous and Tag are unquoted Go struct tag values.
type SourceFieldTagUpdate struct{ Owner, Field, Previous, Tag string }

func (m *sourceAppend) prepareTagUpdates() error {
	if len(m.tagUpdates) == 0 {
		return nil
	}
	m.authorizedTags = map[string]SourceFieldTagUpdate{}
	for _, update := range m.tagUpdates {
		key := update.Owner + "." + update.Field
		if !token.IsIdentifier(update.Owner) || !token.IsIdentifier(update.Field) || update.Owner == "_" || update.Field == "_" {
			return fmt.Errorf("invalid exact shape tag update %s", key)
		}
		if previous, ok := m.authorizedTags[key]; ok && previous != update {
			return fmt.Errorf("conflicting shape tag updates %s", key)
		}
		m.authorizedTags[key] = update
	}
	matched := map[string]bool{}
	for _, declaration := range m.newFile.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, item := range group.Specs {
			definition := item.(*ast.TypeSpec)
			structure, ok := definition.Type.(*ast.StructType)
			if !ok || definition.Assign.IsValid() {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					key := definition.Name.Name + "." + name.Name
					update, ok := m.authorizedTags[key]
					if !ok {
						continue
					}
					tag := ""
					if field.Tag != nil {
						var err error
						tag, err = strconv.Unquote(field.Tag.Value)
						if err != nil {
							return err
						}
					}
					if tag != update.Tag {
						return fmt.Errorf("shape tag update %s does not match exact generated tag", key)
					}
					matched[key] = true
				}
			}
		}
	}
	for key := range m.authorizedTags {
		if !matched[key] {
			return fmt.Errorf("shape tag update %s has no direct generated field", key)
		}
	}
	return nil
}

func (m *sourceAppend) updateFieldTag(key string, previous, generated *ast.Field) (bool, error) {
	update, ok := m.authorizedTags[key]
	if !ok {
		return false, nil
	}
	oldTag, newTag := "", ""
	if previous.Tag != nil {
		oldTag, _ = strconv.Unquote(previous.Tag.Value)
	}
	if generated.Tag != nil {
		newTag, _ = strconv.Unquote(generated.Tag.Value)
	}
	if oldTag == newTag {
		return true, nil
	}
	if oldTag != update.Previous {
		return false, fmt.Errorf("shape tag update %s conflicts with customized tag", key)
	}
	if len(previous.Names) != 1 || len(generated.Names) != 1 {
		return false, fmt.Errorf("shape tag update %s requires an individually named field", key)
	}
	edit := sourceInsertion{offset: m.oldSet.Position(previous.End()).Offset}
	if previous.Tag != nil {
		edit.offset = m.oldSet.Position(previous.Tag.Pos()).Offset
		edit.end = m.oldSet.Position(previous.Tag.End()).Offset
	}
	if generated.Tag != nil {
		edit.text = generated.Tag.Value
		if previous.Tag == nil {
			edit.text = " " + edit.text
		}
	}
	m.insertions = append(m.insertions, edit)
	return true, nil
}
