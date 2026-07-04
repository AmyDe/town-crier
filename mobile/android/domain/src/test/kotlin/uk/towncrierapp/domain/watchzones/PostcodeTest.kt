package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * `Postcode.parse` validates against
 * `^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$` after uppercasing and trimming the
 * raw input (tc-7ttz, port of iOS `Postcode`).
 */
class PostcodeTest {
    @Test
    fun `a garbage string is rejected`() {
        assertNull(Postcode.parse("NOTAPOSTCODE"))
    }

    @Test
    fun `a postcode missing its inward code is rejected`() {
        // "SW1A1" has no space AND no room for the mandatory digit+2-letter
        // inward code - the regex requires it, so this is genuinely invalid,
        // not just missing the optional space.
        assertNull(Postcode.parse("SW1A1"))
    }

    @Test
    fun `a valid postcode without a space is accepted`() {
        // The regex's internal space is optional - "SW1A1AA" is well-formed.
        assertEquals("SW1A1AA", Postcode.parse("SW1A1AA")?.value)
    }

    @Test
    fun `a valid postcode with a space is accepted`() {
        assertEquals("SW1A 1AA", Postcode.parse("SW1A 1AA")?.value)
    }

    @Test
    fun `lower case input is normalised to upper case`() {
        assertEquals("SW1A1AA", Postcode.parse("sw1a1aa")?.value)
    }

    @Test
    fun `mixed case input with a space is normalised and accepted`() {
        assertEquals("Cb1 2ad".uppercase(), Postcode.parse("Cb1 2ad")?.value)
    }

    @Test
    fun `surrounding whitespace is trimmed before matching`() {
        assertEquals("SW1A1AA", Postcode.parse("  sw1a1aa  ")?.value)
    }

    @Test
    fun `blank input is rejected`() {
        assertNull(Postcode.parse("   "))
    }
}
