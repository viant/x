package shape

import "fmt"

// FieldIndex returns a detached native struct-field index. Collection-indexed
// accessors require runtime collection positions and have no single index.
func (a *Accessor) FieldIndex() ([]int, error) {
	if a == nil {
		return nil, fmt.Errorf("field accessor is required")
	}
	if len(a.steps) > 0 {
		var result []int
		for _, step := range a.steps {
			if step.collections > 0 {
				return nil, fmt.Errorf("collection-indexed accessor has no single field index")
			}
			result = append(result, step.index...)
		}
		return result, nil
	}
	return append([]int(nil), a.field.Index...), nil
}
