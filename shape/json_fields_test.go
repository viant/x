//go:build go1.24

package shape

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type JSONCommon struct {
	Value  string
	Unique int
}
type JSONLeft struct{ JSONCommon }
type JSONRight struct{ JSONCommon }
type JSONTagged struct {
	Other string `json:"Value"`
}
type JSONPointer struct{ Name string }
type jsonPrivate struct {
	Exported string
	hidden   string
}

func TestJSONFieldsDominanceMatchesStandardEncoder(t *testing.T) {
	duplicate, err := (Runtime{}).Struct([]RuntimeField{{Name: "A", Type: reflect.TypeOf(""), Tag: `json:"same"`}, {Name: "B", Type: reflect.TypeOf(0), Tag: `json:"same"`}})
	require.NoError(t, err)

	tests := []struct {
		name  string
		value any
		names []string
	}{

		{"repeated same type", struct {
			JSONLeft
			JSONRight
		}{JSONLeft{JSONCommon{"L", 1}}, JSONRight{JSONCommon{"R", 2}}}, nil},
		{"tagged wins same depth", struct {
			JSONCommon
			JSONTagged
		}{JSONCommon{"plain", 1}, JSONTagged{"tagged"}}, []string{"Unique", "Value"}},
		{"shallow untagged wins", struct {
			JSONTagged
			Value string
		}{JSONTagged{"deep"}, "shallow"}, []string{"Value"}},
		{"named anonymous holder", struct {
			JSONCommon `json:"holder"`
		}{JSONCommon{"V", 2}}, []string{"holder"}},
		{"unexported anonymous struct", struct{ jsonPrivate }{jsonPrivate{"visible", "hidden"}}, []string{"Exported"}},
		{"ignore json dash only", struct {
			Dash   string `json:"-,omitempty"`
			Hidden string `json:"-"`
		}{"value", "secret"}, []string{"-"}},
		{"invalid tag falls back", struct {
			F string `json:"bad\\name"`
		}{"V"}, []string{"F"}},
		{"tag containing equals", struct {
			F string `json:"a=b"`
		}{"V"}, []string{"a=b"}},
	}
	tests = append(tests, struct {
		name  string
		value any
		names []string
	}{"equal priority annihilation", reflect.New(duplicate).Elem().Interface(), nil})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := Linked(reflect.TypeOf(tt.value)).JSONFields()
			require.NoError(t, err)
			var names []string
			for _, field := range fields {
				names = append(names, field.Name)
			}
			require.Equal(t, tt.names, names)
			encoded, err := json.Marshal(tt.value)
			require.NoError(t, err)
			var object map[string]any
			require.NoError(t, json.Unmarshal(encoded, &object))
			require.Len(t, object, len(fields))
			for _, field := range fields {
				require.Contains(t, object, field.Name)
			}
		})
	}
}

func TestJSONFieldsNilHolderAndOmission(t *testing.T) {
	type sample struct {
		*JSONPointer
		Fixed  [2]int              `json:",omitempty"`
		Empty  [0]int              `json:",omitempty"`
		Struct struct{ Value int } `json:",omitempty"`
		Ptr    *int                `json:",omitempty"`
		Zero   struct{ Value int } `json:",omitzero"`
	}
	fields, err := Linked(reflect.TypeFor[sample]()).JSONFields()
	require.NoError(t, err)
	expected := map[string]bool{"Name": true, "Fixed": false, "Empty": true, "Struct": false, "Ptr": true, "Zero": true}
	for _, field := range fields {
		require.Equal(t, expected[field.Name], field.MayOmit(), field.Name)
	}
	raw, err := json.Marshal(sample{})
	require.NoError(t, err)
	require.JSONEq(t, `{"Fixed":[0,0],"Struct":{"Value":0}}`, string(raw))
	fields[0].Field.Index[0] = 99
	again, err := Linked(reflect.TypeFor[sample]()).JSONFields()
	require.NoError(t, err)
	require.NotEqual(t, 99, again[0].Field.Index[0])
}

func TestJSONFieldsStringOptionEligibility(t *testing.T) {
	type namedPointer *int
	type sample struct {
		I     int             `json:",string"`
		P     *int            `json:",string"`
		PP    **int           `json:",string"`
		Named namedPointer    `json:",string"`
		S     string          `json:",string"`
		B     bool            `json:",string"`
		F     float64         `json:",string"`
		A     []int           `json:",string"`
		O     struct{ I int } `json:",string"`
	}
	fields, err := Linked(reflect.TypeFor[sample]()).JSONFields()
	require.NoError(t, err)
	quoted := map[string]bool{"I": true, "P": true, "S": true, "B": true, "F": true}
	for _, field := range fields {
		require.Equal(t, quoted[field.Name], field.Quoted, field.Name)
	}
}
