package shape

import (
	"fmt"
	"reflect"
)

// CollectionMatchAdapter supplies captured association evidence. Key must use
// captured identity presence; CurrentKey reads values, never working markers.
// Equal compares the selected processing baseline, not a previous database row.
// Adapter methods must not mutate entities or snapshots.
type CollectionMatchAdapter[T any, K comparable, S comparable] interface {
	Key(S) (K, bool, error)
	CurrentKey(*T) (K, bool, error)
	Source(S) *T
	Equal(*T, S) (bool, error)
}

type CollectionMatchInput[T any, S comparable] struct {
	Current  []*T
	Original []S
	// ByPointer is trusted capture-time authority from the SAME generation. It
	// may include records outside this local Original sequence (for example a
	// record moved between parent collections). It cannot override a different
	// local snapshot for the same source. Callers must never supply working or
	// foreign-generation snapshots here; this utility cannot establish provenance.
	ByPointer map[*T]S
	// PointerIdentity permits captured source addresses as identity. It must be
	// false for addresses of value-slice slots, which may change meaning on reorder.
	PointerIdentity bool
}

type CollectionMatchResult[S comparable] struct {
	// Original is aligned with Current; S's zero value represents a nil item.
	Original []S
	// Changed reports membership/order/nil changes, not changed business values.
	Changed bool
}

// CollectionMatcher associates an initialized graph with captured processing
// snapshots. It does not match sparse requests to database rows or apply marks.
// Replaced pointers and relocated value records require both captured supplied
// identity and equal baseline evidence. Unknown/ambiguous records fail closed.
// Each nonzero snapshot may occur once per collection, even without an assigned
// key. Sharing it across independent collections or self-reference edges is valid.
type CollectionMatcher[T any, K comparable, S comparable] struct {
	Adapter CollectionMatchAdapter[T, K, S]
}

func (m CollectionMatcher[T, K, S]) Match(input CollectionMatchInput[T, S]) (CollectionMatchResult[S], error) {
	zeroResult := CollectionMatchResult[S]{}
	if (Runtime{}).IsNil(m.Adapter) {
		return zeroResult, fmt.Errorf("collection match adapter is required")
	}
	var zero S
	keys := make(map[K]S)
	originals := make(map[S]bool)
	sources := make(map[*T]S)
	originalCount, currentCount := 0, 0
	var sole S
	for index, snapshot := range input.Original {
		if value := reflect.ValueOf(snapshot); value.IsValid() && !value.Comparable() {
			return zeroResult, fmt.Errorf("original item %d has non-comparable snapshot identity", index)
		}
		if snapshot != snapshot {
			return zeroResult, fmt.Errorf("original item %d has non-reflexive snapshot identity", index)
		}
		if snapshot == zero {
			continue
		}
		if originals[snapshot] {
			return zeroResult, fmt.Errorf("duplicate captured snapshot at item %d", index)
		}
		originals[snapshot] = true
		originalCount++
		sole = snapshot
		key, supplied, err := m.Adapter.Key(snapshot)
		if err != nil {
			return zeroResult, fmt.Errorf("original item %d: %w", index, err)
		}
		if supplied {
			if value := reflect.ValueOf(key); value.IsValid() && !value.Comparable() {
				return zeroResult, fmt.Errorf("original item %d has non-comparable key", index)
			}
			if key != key {
				return zeroResult, fmt.Errorf("original item %d has non-reflexive key", index)
			}
			if _, exists := keys[key]; exists {
				return zeroResult, fmt.Errorf("duplicate captured identity at item %d", index)
			}
			keys[key] = snapshot
		}
		if input.PointerIdentity {
			if source := m.Adapter.Source(snapshot); source != nil {
				if previous, exists := sources[source]; exists && previous != snapshot {
					return zeroResult, fmt.Errorf("ambiguous captured source at item %d", index)
				}
				sources[source] = snapshot
			}
		}
	}
	for _, current := range input.Current {
		if current != nil {
			currentCount++
		}
	}
	result := CollectionMatchResult[S]{Original: make([]S, len(input.Current)), Changed: len(input.Current) != len(input.Original) || (input.Current == nil) != (input.Original == nil)}
	if input.Current == nil {
		result.Original = nil
	}
	used := make(map[K]bool)
	usedSnapshots := make(map[S]bool)
	for index, current := range input.Current {
		var matched S
		if current != nil {
			if input.PointerIdentity {
				matched = input.ByPointer[current]
				if value := reflect.ValueOf(matched); value.IsValid() && !value.Comparable() {
					return zeroResult, fmt.Errorf("current item %d has non-comparable snapshot identity", index)
				}
				if matched != matched {
					return zeroResult, fmt.Errorf("current item %d has non-reflexive snapshot identity", index)
				}
				if matched != zero && m.Adapter.Source(matched) != current {
					return zeroResult, fmt.Errorf("invalid captured pointer association at item %d", index)
				}
				if local, exists := sources[current]; matched != zero && exists && local != matched {
					return zeroResult, fmt.Errorf("conflicting local captured association at item %d", index)
				}
				if matched == zero {
					matched = sources[current]
				}
			}
			if matched == zero && !input.PointerIdentity && index < len(input.Original) {
				candidate := input.Original[index]
				if candidate != zero {
					key, supplied, err := m.Adapter.Key(candidate)
					if err != nil {
						return zeroResult, err
					}
					actual, valid, err := m.Adapter.CurrentKey(current)
					if err != nil {
						return zeroResult, fmt.Errorf("current item %d: %w", index, err)
					}
					if value := reflect.ValueOf(actual); valid && value.IsValid() && !value.Comparable() {
						return zeroResult, fmt.Errorf("current item %d has non-comparable key", index)
					}
					if valid && actual != actual {
						return zeroResult, fmt.Errorf("current item %d has non-reflexive key", index)
					}
					if supplied && valid && actual == key {
						matched = candidate
					} else if !supplied {
						equal, err := m.Adapter.Equal(current, candidate)
						if err != nil {
							return zeroResult, err
						}
						if equal {
							matched = candidate
						}
					}
				}
			}
			if matched == zero {
				key, valid, err := m.Adapter.CurrentKey(current)
				if err != nil {
					return zeroResult, fmt.Errorf("current item %d: %w", index, err)
				}
				if value := reflect.ValueOf(key); valid && value.IsValid() && !value.Comparable() {
					return zeroResult, fmt.Errorf("current item %d has non-comparable key", index)
				}
				if valid && key != key {
					return zeroResult, fmt.Errorf("current item %d has non-reflexive key", index)
				}
				if valid {
					if candidate, exists := keys[key]; exists {
						equal, err := m.Adapter.Equal(current, candidate)
						if err != nil {
							return zeroResult, err
						}
						if equal {
							matched = candidate
						}
					}
				}
			}
			if matched == zero && !input.PointerIdentity && originalCount == 1 && currentCount == 1 {
				_, supplied, err := m.Adapter.Key(sole)
				if err != nil {
					return zeroResult, err
				}
				if !supplied {
					matched = sole
				}
			}
			if matched == zero {
				return zeroResult, fmt.Errorf("current item %d has no unambiguous captured association", index)
			}
			if usedSnapshots[matched] {
				return zeroResult, fmt.Errorf("duplicate matched snapshot at item %d", index)
			}
			usedSnapshots[matched] = true
			key, supplied, err := m.Adapter.Key(matched)
			if err != nil {
				return zeroResult, err
			}
			if supplied {
				if value := reflect.ValueOf(key); value.IsValid() && !value.Comparable() {
					return zeroResult, fmt.Errorf("matched item %d has non-comparable key", index)
				}
				if key != key {
					return zeroResult, fmt.Errorf("matched item %d has non-reflexive key", index)
				}
				if used[key] {
					return zeroResult, fmt.Errorf("duplicate matched identity at item %d", index)
				}
				used[key] = true
			}
		}
		result.Original[index] = matched
		if index >= len(input.Original) || input.Original[index] != matched {
			result.Changed = true
		}
	}
	return result, nil
}
