package uk.towncrierapp.domain.watchzones

/**
 * A validated UK postcode, normalised to upper case with surrounding
 * whitespace trimmed. Validity is established once, at [parse] time - every
 * [Postcode] in hand is already known well-formed, so nothing downstream
 * re-checks the format. Port of iOS `Postcode`.
 */
@JvmInline
public value class Postcode private constructor(
    public val value: String,
) {
    public companion object {
        // The internal space is optional ("SW1A1AA" and "SW1A 1AA" both
        // match); the trailing digit+2-letter "inward code" is not, which is
        // why a truncated postcode like "SW1A1" (no inward code at all)
        // fails rather than just matching the outward part.
        private val FORMAT_REGEX = Regex("^[A-Z]{1,2}\\d[A-Z\\d]?\\s?\\d[A-Z]{2}$")

        /**
         * Parses [raw] into a [Postcode], upper-casing and trimming first.
         * Returns `null` when [raw] doesn't match the UK postcode format -
         * an expected user-input outcome, not a programmer error.
         */
        public fun parse(raw: String): Postcode? {
            val normalised = raw.trim().uppercase()
            return if (FORMAT_REGEX.matches(normalised)) Postcode(normalised) else null
        }
    }
}
