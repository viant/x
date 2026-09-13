package shape

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

type matchKey struct{ Tenant, ID int }
type matchRow struct {
	Key  matchKey
	Name string
	Has  bool
}
type matchSnapshot struct {
	source   *matchRow
	original matchRow
	err      error
}
type matchAdapter struct{}

func (matchAdapter) Key(s *matchSnapshot) (matchKey, bool, error) {
	return s.original.Key, s.original.Has, s.err
}
func (matchAdapter) CurrentKey(row *matchRow) (matchKey, bool, error) { return row.Key, true, nil }
func (matchAdapter) Source(s *matchSnapshot) *matchRow                { return s.source }
func (matchAdapter) Equal(row *matchRow, s *matchSnapshot) (bool, error) {
	return *row == s.original, nil
}

type matchErrorAdapter struct {
	matchAdapter
	err error
}

type matchKeyErrorAdapter struct {
	matchAdapter
	err error
}

func (a matchKeyErrorAdapter) CurrentKey(*matchRow) (matchKey, bool, error) {
	return matchKey{}, false, a.err
}

func TestCollectionMatcherCurrentKeyError(t *testing.T) {
	cause := errors.New("current key access failed")
	source := &matchRow{Has: true}
	snapshot := &matchSnapshot{source: source, original: *source}
	current := *source
	for _, pointer := range []bool{false, true} {
		m := CollectionMatcher[matchRow, matchKey, *matchSnapshot]{Adapter: matchKeyErrorAdapter{err: cause}}
		result, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{&current}, Original: []*matchSnapshot{snapshot}, PointerIdentity: pointer})
		if !errors.Is(err, cause) || result.Original != nil || result.Changed {
			t.Fatalf("current key result=%+v error=%v", result, err)
		}
	}
}

func (a matchErrorAdapter) Equal(*matchRow, *matchSnapshot) (bool, error) { return false, a.err }

func TestCollectionMatcherPointerEvidenceAndErrors(t *testing.T) {
	row := &matchRow{Name: "source", Has: true}
	snapshot := &matchSnapshot{source: row, original: *row}
	m := CollectionMatcher[matchRow, matchKey, *matchSnapshot]{Adapter: matchAdapter{}}
	result, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{row}, ByPointer: map[*matchRow]*matchSnapshot{row: snapshot}, PointerIdentity: true})
	if err != nil || len(result.Original) != 1 || result.Original[0] != snapshot || !result.Changed {
		t.Fatalf("shared captured association=%+v error=%v", result, err)
	}
	conflict := &matchSnapshot{source: row, original: *row}
	if _, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{row}, Original: []*matchSnapshot{snapshot}, ByPointer: map[*matchRow]*matchSnapshot{row: conflict}, PointerIdentity: true}); err == nil {
		t.Fatal("graph-wide capture replaced local baseline")
	}
	copy := *row
	for _, byPointer := range []map[*matchRow]*matchSnapshot{{&copy: snapshot}, {row: {source: &copy, original: *row}}} {
		current := row
		if _, ok := byPointer[&copy]; ok {
			current = &copy
		}
		if _, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{current}, ByPointer: byPointer, PointerIdentity: true}); err == nil {
			t.Fatal("invalid source association accepted")
		}
	}
	cause := errors.New("equality unavailable")
	m.Adapter = matchErrorAdapter{err: cause}
	result, err = m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{&copy}, Original: []*matchSnapshot{snapshot}, PointerIdentity: true})
	if !errors.Is(err, cause) || result.Original != nil || result.Changed {
		t.Fatalf("equality failure=%+v error=%v", result, err)
	}
	if _, err := (CollectionMatcher[matchRow, matchKey, *matchSnapshot]{}).Match(CollectionMatchInput[matchRow, *matchSnapshot]{}); err == nil {
		t.Fatal("missing adapter accepted")
	}
}

type dynamicMatchAdapter struct{ captured, current any }

func (a dynamicMatchAdapter) Key(*matchSnapshot) (any, bool, error)       { return a.captured, true, nil }
func (a dynamicMatchAdapter) CurrentKey(*matchRow) (any, bool, error)     { return a.current, true, nil }
func (dynamicMatchAdapter) Source(s *matchSnapshot) *matchRow             { return s.source }
func (dynamicMatchAdapter) Equal(*matchRow, *matchSnapshot) (bool, error) { return true, nil }

func TestCollectionMatcherRejectsDynamicNonComparableKeys(t *testing.T) {
	row := &matchRow{}
	snapshot := &matchSnapshot{source: row}
	for _, test := range []struct {
		name              string
		original, current any
		pointer           bool
	}{
		{"captured slice", []int{1}, 1, false}, {"current map", 1, map[string]int{"id": 1}, false}, {"replaced pointer slice", 1, []int{1}, true},
		{"captured NaN", math.NaN(), 1, false}, {"current NaN", 1, math.NaN(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := *row
			m := CollectionMatcher[matchRow, any, *matchSnapshot]{Adapter: dynamicMatchAdapter{captured: test.original, current: test.current}}
			result, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{&copy}, Original: []*matchSnapshot{snapshot}, PointerIdentity: test.pointer})
			if err == nil || result.Original != nil {
				t.Fatalf("invalid key result=%+v error=%v", result, err)
			}
		})
	}
}

func TestCollectionMatcherProcessingAssociation(t *testing.T) {
	for _, test := range []struct {
		name          string
		setup         func(*matchRow, *matchRow, *matchSnapshot, *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot]
		want          []int
		changed, fail bool
	}{
		{"stable pointers changed", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			a.Name = "changed"
			a.Key.ID = 99
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{a, b}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, []int{0, 1}, false, false},
		{"pointer reorder subset", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{b}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, []int{1}, true, false},
		{"value same slot key", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			a.Name = "changed"
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{a, b}, Original: []*matchSnapshot{sa, sb}}
		}, []int{0, 1}, false, false},
		{"value reorder unchanged", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{b, a}, Original: []*matchSnapshot{sa, sb}}
		}, []int{1, 0}, true, false},
		{"value reorder mutated ambiguous", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			b.Name = "changed"
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{b, a}, Original: []*matchSnapshot{sa, sb}}
		}, nil, false, true},
		{"equal replacement pointer", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			copy := *a
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{&copy, b}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, []int{0, 1}, false, false},
		{"unknown pointer", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{{Key: matchKey{9, 9}}}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, nil, false, true},
		{"duplicate captured zero", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			sb.original.Key = sa.original.Key
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: nil, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, nil, false, true},
		{"duplicate current supplied", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{a, a}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, nil, false, true},
		{"distinct unassigned pointers", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			sa.original.Has = false
			sb.original.Has = false
			a.Name = "changed"
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{b, a}, Original: []*matchSnapshot{sa, sb}, PointerIdentity: true}
		}, []int{1, 0}, true, false},
		{"sole unassigned value changed", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			sa.original.Has = false
			a.Name = "changed"
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{a}, Original: []*matchSnapshot{sa}}
		}, []int{0}, false, false},
		{"ambiguous unassigned values", func(a, b *matchRow, sa, sb *matchSnapshot) CollectionMatchInput[matchRow, *matchSnapshot] {
			sa.original.Has = false
			sb.original.Has = false
			return CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{b, a}, Original: []*matchSnapshot{sa, sb}}
		}, nil, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, b := &matchRow{Key: matchKey{}, Name: "a", Has: true}, &matchRow{Key: matchKey{1, 0}, Name: "b", Has: true}
			sa, sb := &matchSnapshot{source: a, original: *a}, &matchSnapshot{source: b, original: *b}
			input := test.setup(a, b, sa, sb)
			beforeA, beforeB := *a, *b
			result, err := (CollectionMatcher[matchRow, matchKey, *matchSnapshot]{Adapter: matchAdapter{}}).Match(input)
			if (err != nil) != test.fail {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			if *a != beforeA || *b != beforeB {
				t.Fatal("matching mutated working values")
			}
			if test.fail {
				if result.Original != nil || result.Changed {
					t.Fatal("failed matching published partial result")
				}
				return
			}
			want := make([]*matchSnapshot, len(test.want))
			for i, index := range test.want {
				want[i] = []*matchSnapshot{sa, sb}[index]
			}
			if !reflect.DeepEqual(result.Original, want) || result.Changed != test.changed {
				t.Fatalf("result=%+v want=%v changed=%v", result, want, test.changed)
			}
		})
	}
}

func TestCollectionMatcherNilAndError(t *testing.T) {
	m := CollectionMatcher[matchRow, matchKey, *matchSnapshot]{Adapter: matchAdapter{}}
	result, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{})
	if err != nil || result.Original != nil || result.Changed {
		t.Fatalf("nil result=%+v error=%v", result, err)
	}
	result, err = m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{}})
	if err != nil || result.Original == nil || !result.Changed {
		t.Fatal("nil/empty distinction lost")
	}
	cause := errors.New("partial key")
	_, err = m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Original: []*matchSnapshot{{err: cause}}})
	if !errors.Is(err, cause) {
		t.Fatalf("key error cause lost: %v", err)
	}
}

func TestCollectionMatcherUnassignedSnapshotUniqueness(t *testing.T) {
	row := &matchRow{Name: "new"}
	snapshot := &matchSnapshot{source: row, original: *row}
	m := CollectionMatcher[matchRow, matchKey, *matchSnapshot]{Adapter: matchAdapter{}}
	for _, input := range []CollectionMatchInput[matchRow, *matchSnapshot]{
		{Original: []*matchSnapshot{snapshot, snapshot}},
		{Current: []*matchRow{row, row}, Original: []*matchSnapshot{snapshot}, PointerIdentity: true},
		{Current: []*matchRow{row, row}, ByPointer: map[*matchRow]*matchSnapshot{row: snapshot}, PointerIdentity: true},
	} {
		result, err := m.Match(input)
		if err == nil || result.Original != nil || result.Changed {
			t.Fatalf("duplicate snapshot result=%+v error=%v", result, err)
		}
	}
	for i := 0; i < 2; i++ {
		result, err := m.Match(CollectionMatchInput[matchRow, *matchSnapshot]{Current: []*matchRow{row}, Original: []*matchSnapshot{snapshot}, PointerIdentity: true})
		if err != nil || result.Original[0] != snapshot || result.Changed {
			t.Fatalf("shared independent collection=%+v error=%v", result, err)
		}
	}
}
