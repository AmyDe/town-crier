// Package offercodes owns the offer-code feature: code format/validation, the
// domain model, the random generator, the Cosmos store, and the authed
// POST /v1/offer-codes/redeem handler (GH#418 iteration 8).
package offercodes

import (
	"strconv"
	"strings"
)

// canonicalLength is the number of characters in a canonical offer code.
const canonicalLength = 12

// alphabet is the Crockford base32 alphabet the codes are drawn from (excludes
// I, L, O, U to avoid ambiguity).
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// InvalidFormatError signals a malformed offer code. Its message is surfaced
// verbatim in the 400 response body.
type InvalidFormatError struct {
	Message string
}

func (e *InvalidFormatError) Error() string { return e.Message }

// Normalize strips separators and whitespace, upper-cases, and validates that
// the remaining characters form a canonical 12-character Crockford code.
// On failure it returns an *InvalidFormatError.
func Normalize(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", &InvalidFormatError{Message: "Offer code is required."}
	}

	var b strings.Builder
	b.Grow(len(input))
	for _, c := range input {
		if c == '-' || isWhitespace(c) {
			continue
		}
		upper := toUpperASCII(c)
		if !strings.ContainsRune(alphabet, upper) {
			return "", &InvalidFormatError{Message: "Offer code contains invalid character '" + string(c) + "'."}
		}
		b.WriteRune(upper)
	}

	normalised := b.String()
	if len(normalised) != canonicalLength {
		return "", &InvalidFormatError{Message: "Offer code must be " + strconv.Itoa(canonicalLength) + " characters (got " + strconv.Itoa(len(normalised)) + ")."}
	}
	return normalised, nil
}

// Format renders a canonical code as the display form XXXX-XXXX-XXXX.
// The input must already be canonical.
func Format(canonical string) string {
	return canonical[:4] + "-" + canonical[4:8] + "-" + canonical[8:]
}

// IsValidCanonical reports whether value is a canonical 12-character code drawn
// entirely from the alphabet.
func IsValidCanonical(value string) bool {
	if len(value) != canonicalLength {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}

// isWhitespace reports whether c is an ASCII whitespace character that can appear
// in user-entered codes (space, tab, newlines).
func isWhitespace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

// toUpperASCII upper-cases an ASCII letter, matching char.ToUpperInvariant for
// the offer-code character set (ASCII letters/digits). Non-letters pass through.
func toUpperASCII(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}
