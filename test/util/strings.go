package util

import "sort"

func StringSlicesEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	s12, s22 := make([]string, len(s1)), make([]string, len(s2))

	copy(s12, s1)
	copy(s22, s2)

	sort.Strings(s12)
	sort.Strings(s22)

	for i, s := range s12 {
		if s != s22[i] {
			return false
		}
	}

	return true
}
