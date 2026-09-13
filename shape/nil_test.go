package shape

import (
	"testing"
	"unsafe"
)

func TestRuntimeIsNil(t *testing.T) {
	for _, test := range []struct {
		value any
		want  bool
	}{
		{nil, true}, {(*int)(nil), true}, {map[string]int(nil), true}, {[]int(nil), true}, {(func())(nil), true}, {(chan int)(nil), true}, {unsafe.Pointer(nil), true},
		{0, false}, {false, false}, {"", false}, {[]int{}, false}, {map[string]int{}, false}, {new(int), false},
	} {
		if got := (Runtime{}).IsNil(test.value); got != test.want {
			t.Fatalf("IsNil(%T)=%v want=%v", test.value, got, test.want)
		}
	}
}
