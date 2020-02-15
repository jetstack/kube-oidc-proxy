// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"errors"
)

// JoinErrors will join a slice of errors into a single error, pretty
// printed.
func JoinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}

	errS := errs[0].Error()
	for _, err := range errs[1:] {
		errS += ", "
		errS += err.Error()
	}

	return errors.New(errS)
}
