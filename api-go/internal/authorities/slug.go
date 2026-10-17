package authorities

import "strings"

// apostropheStripper removes ASCII ' and U+2019 so "King's Lynn" slugs to
// "kings-lynn"; it must run before the non-alphanumeric collapse in Slugify.
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
// Step 4 matches the JS regex /[^a-z0-9]+/g over runes, so every non-ASCII rune
// is a separator and a multibyte character is never split.
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
