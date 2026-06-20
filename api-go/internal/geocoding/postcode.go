// Package geocoding owns the postcode-geocoding feature: UK postcode validation,
// the postcodes.io outbound client, and GET /v1/geocode/{postcode} (GH#418).
package geocoding

import (
	"regexp"
	"strings"
)

// ukPostcodeRegex matches a normalised (trimmed, upper-cased) UK postcode.
var ukPostcodeRegex = regexp.MustCompile(`^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$`)

// normalisePostcode trims and upper-cases a raw postcode and validates its
// format. The boolean reports validity; an invalid (blank or malformed) postcode
// yields a 400 at the HTTP boundary.
func normalisePostcode(raw string) (string, bool) {
	normalised := strings.ToUpper(strings.TrimSpace(raw))
	if normalised == "" || !ukPostcodeRegex.MatchString(normalised) {
		return "", false
	}
	return normalised, true
}
