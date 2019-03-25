// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"sort"
)

func StringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i, aa := range a {
		if aa != b[i] {
			return false
		}
	}

	return true
}
