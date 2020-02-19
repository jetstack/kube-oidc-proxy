// Copyright Jetstack Ltd. See LICENSE for details.
package flags

import (
	"reflect"
	"testing"
)

func TestStringToStringSliceSet(t *testing.T) {
	tests := map[string]struct {
		val       string
		expError  bool
		expValues map[string][]string
	}{
		"if empty string set, no values": {
			val:       "",
			expError:  false,
			expValues: make(map[string][]string),
		},
		"if only key then error": {
			val:       "key1",
			expError:  true,
			expValues: make(map[string][]string),
		},
		"if single key value return": {
			val:      "key1=foo",
			expError: false,
			expValues: map[string][]string{
				"key1": []string{"foo"},
			},
		},
		"if two keys with two values return": {
			val:      "key1=foo,key2=bar",
			expError: false,
			expValues: map[string][]string{
				"key1": []string{"foo"},
				"key2": []string{"bar"},
			},
		},
		"if 3 keys with 5 values return": {
			val:      "key1=foo,key2=bar,key1=a,key2=c,key3=c",
			expError: false,
			expValues: map[string][]string{
				"key1": []string{"foo", "a"},
				"key2": []string{"bar", "c"},
				"key3": []string{"c"},
			},
		},
		"if key with no value error": {
			val:       "key1=foo,key2=bar,key1=a,key2=c,key3",
			expError:  true,
			expValues: make(map[string][]string),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := new(stringToStringSliceValue)

			err := s.Set(test.val)
			if test.expError != (err != nil) {
				t.Errorf("got unexpected error: %v", err)
				t.FailNow()
			}

			match := true
			if s.values == nil {
				if test.expValues != nil {
					match = false
				}
			} else if !reflect.DeepEqual(test.expValues, *s.values) {
				match = false
			}

			if !match {
				t.Errorf("unexpected values, exp=%v got=%v", test.expValues, *s.values)
			}
		})
	}
}
