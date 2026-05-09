package utils

import (
	"errors"
	"regexp"
	"strings"
)

// pkPhonePattern matches Pakistani mobile numbers in any of the common forms:
//
//	+923xxxxxxxxx
//	923xxxxxxxxx
//	03xxxxxxxxx
//	3xxxxxxxxx (rare, locally typed)
//
// The mobile prefix is always "3" (e.g. 0300, 0321, 0345 ...). After
// normalization the number is stored as +923xxxxxxxxx.
var pkPhonePattern = regexp.MustCompile(`^(?:\+?92|0)?3\d{9}$`)

// NormalizePKPhone validates and normalizes a Pakistani phone number.
// On success it returns the canonical "+923xxxxxxxxx" form.
func NormalizePKPhone(input string) (string, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(input), " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")

	if cleaned == "" {
		return "", errors.New("phone number is required")
	}

	if !pkPhonePattern.MatchString(cleaned) {
		return "", errors.New("invalid Pakistani phone number; use +923xxxxxxxxx or 03xxxxxxxxx")
	}

	// Strip all leading prefixes down to the bare 10-digit subscriber number
	// starting with "3", then prepend the country code.
	digits := cleaned
	digits = strings.TrimPrefix(digits, "+")
	digits = strings.TrimPrefix(digits, "92")
	digits = strings.TrimPrefix(digits, "0")

	return "+92" + digits, nil
}
