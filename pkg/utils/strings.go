// Copyright Jetstack Ltd. See LICENSE for details.
package utils

func StringsContain(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}

	return false
}
