// Package validation holds generic, field-agnostic rules. It knows nothing
// about HTTP; callers decide how to report the errors.
package validation

import (
	"errors"
	"unicode/utf8"
)

var (
	ErrRequired  = errors.New("required")
	ErrMinLength = errors.New("min length")
	ErrMaxLength = errors.New("max length")
)

// String checks s against required, min and max length, in that order, and
// returns the first rule that fails. Length is counted in characters (runes),
// not bytes, so "日本語" has length 3.
func String(s string, min, max int) error {
	n := utf8.RuneCountInString(s)

	switch {
	case n == 0:
		return ErrRequired
	case n < min:
		return ErrMinLength
	case n > max:
		return ErrMaxLength
	}
	return nil
}
