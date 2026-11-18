package account_types

import (
	"reflect"
	"sort"
	"testing"
)

func TestDiffPermissions(t *testing.T) {
	cases := []struct {
		name             string
		have, want       []string
		wantAdd, wantRem []string
	}{
		{"no change", []string{"a", "b"}, []string{"b", "a"}, nil, nil},
		{"add only", []string{"a"}, []string{"a", "b", "c"}, []string{"b", "c"}, nil},
		{"remove only", []string{"a", "b", "c"}, []string{"a"}, nil, []string{"b", "c"}},
		{"add and remove", []string{"a", "b"}, []string{"b", "c"}, []string{"c"}, []string{"a"}},
		{"from empty", nil, []string{"a"}, []string{"a"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			add, rem := diffPermissions(c.have, c.want)
			sort.Strings(add)
			sort.Strings(rem)
			if !reflect.DeepEqual(add, c.wantAdd) || !reflect.DeepEqual(rem, c.wantRem) {
				t.Fatalf("got add=%v remove=%v, want add=%v remove=%v", add, rem, c.wantAdd, c.wantRem)
			}
		})
	}
}
