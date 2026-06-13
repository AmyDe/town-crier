// Package geocoding owns the postcode-geocoding feature: UK postcode validation,
// the postcodes.io outbound client, and GET /v1/geocode/{postcode}. It mirrors
// the .NET TownCrier.{Domain,Application,Infrastructure}.Geocoding slices
// (GH#418 iteration 7).
package geocoding

import (
	"regexp"
	"strings"
)

// ukPostcodeRegex matches a normalised (trimmed, upper-cased) UK postcode. It is
// the exact pattern the .NET Postcode value object uses.
var ukPostcodeRegex = regexp.MustCompile(`^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$`)

// normalisePostcode trims and upper-cases a raw postcode and validates its
// format, mirroring .NET Postcode.Create. The boolean reports validity; an
// invalid (blank or malformed) postcode yields a 400 at the HTTP boundary.
func normalisePostcode(raw string) (string, bool) {
	normalised := strings.ToUpper(strings.TrimSpace(raw))
	if normalised == "" || !ukPostcodeRegex.MatchString(normalised) {
		return "", false
	}
	return normalised, true
}
