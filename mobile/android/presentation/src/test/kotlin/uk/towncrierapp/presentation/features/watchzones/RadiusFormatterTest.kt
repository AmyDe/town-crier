package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Byte-exact port of iOS `RadiusFormatter` — metric only, no locale
 * `NumberFormat` grouping (tc-z95t). The branching, not the presentation
 * layer's usual locale-aware formatting, is the contract under test.
 */
class RadiusFormatterTest {
    @Test
    fun `under 1000 metres formats as whole metres`() {
        assertEquals("500 m", RadiusFormatter.format(500.0))
        assertEquals("100 m", RadiusFormatter.format(100.0))
        assertEquals("999 m", RadiusFormatter.format(999.0))
    }

    @Test
    fun `a whole number of kilometres has no decimal`() {
        assertEquals("2 km", RadiusFormatter.format(2_000.0))
        assertEquals("10 km", RadiusFormatter.format(10_000.0))
        assertEquals("1 km", RadiusFormatter.format(1_000.0))
    }

    @Test
    fun `a fractional number of kilometres shows one decimal place`() {
        assertEquals("1.5 km", RadiusFormatter.format(1_500.0))
        assertEquals("2.1 km", RadiusFormatter.format(2_100.0))
    }
}
