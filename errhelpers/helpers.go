package errhelpers

import (
	"errors"
		"fmt"
		)

// Augment takes an error and returns a new one with additional context.
func Augment(message string, err error) error {
			return errors.New(fmt.Sprintf(message+": %v", err))
}
