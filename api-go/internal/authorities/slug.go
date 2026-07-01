package authorities

import "strings"

// apostropheStripper removes the ASCII apostrophe (') and the Unicode right
// single quotation mark (U+2019) so possessive names slug cleanly:
// "King's Lynn" -> "kings-lynn", not "king-s-lynn". This must run BEFORE the
// non-alphanumeric collapse in Slugify, otherwise the apostrophe would itself
// become a hyphen.
var apostropheStripper = strings.NewReplacer("'", "", "’", "")

// Slugify converts an authority name into a lowercase-hyphenated URL slug. It is
// a byte-equal port of web/scripts/lib/slug.mjs slugify(), guarded against drift
// by a shared test-vector fixture (see TestSlugify_MatchesSharedFixture). The
// steps run in this exact order:
//
//  1. lowercase;
//  2. expand every "&" to " and " (space, a, n, d, space);
//  3. strip apostrophes (ASCII ' and U+2019) — removed, not hyphenated;
//  4. collapse each maximal run of non-[a-z0-9] runes into a single "-";
//  5. trim leading and trailing "-".
//
// Step 4 is Unicode-aware, matching the JS regex /[^a-z0-9]+/g: it iterates over
// runes and keeps a rune only when it is an ASCII a-z or 0-9 (after lowercasing);
// every other rune — accented letters, CJK, punctuation, whitespace — is a
// separator. Iterating runes (not bytes) avoids splitting a multibyte character.
func Slugify(name string) string {
	lowered := strings.ToLower(name)
	expanded := strings.ReplaceAll(lowered, "&", " and ")
	stripped := apostropheStripper.Replace(expanded)

	var b strings.Builder
	b.Grow(len(stripped))

	// pendingSeparator records that at least one separator rune has been seen
	// since the last kept rune. A single "-" is emitted only when the next kept
	// rune arrives AND some output already exists, which collapses runs and drops
	// leading/trailing separators in one pass (equivalent to collapse-then-trim).
	pendingSeparator := false
	for _, r := range stripped {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingSeparator && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingSeparator = false
			b.WriteRune(r)
			continue
		}
		pendingSeparator = true
	}
	return b.String()
}
