// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import "testing"

func TestStringsContain(t *testing.T) {
	for _, c := range []struct {
		slice []string
		str   string
		exp   bool
	}{
		{
			nil,
			"a",
			false,
		},

		{
			[]string{"a", "b", "c"},
			"d",
			false,
		},

		{
			[]string{"a", "b", "c", "d"},
			"c",
			true,
		},

		{
			[]string{"a", "b", "c", "d", "b"},
			"b",
			true,
		},
	} {
		if b := StringsContain(c.slice, c.str); b != c.exp {
			t.Errorf("got unexpected match result, exp=%t got=%t (%s: %s)",
				c.exp, b, c.str, c.slice)
		}
	}
}
