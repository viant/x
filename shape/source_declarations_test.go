package shape

import (
	"reflect"
	"testing"
)

func TestSourcePackageDeclarations(t *testing.T) {
	parsed, err := (SourceParser{}).Parse([]byte(`package example
import "time"
type Empty struct{}
type Alias = time.Time
var First, Second int
const Value = 1
func Work(){}
func (Empty) Method(){}
func init(){}
var _ = 2
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Empty", "Alias", "First", "Second", "Value", "Work"}
	if !reflect.DeepEqual(parsed.Declarations, want) {
		t.Fatalf("declarations=%v", parsed.Declarations)
	}
}
