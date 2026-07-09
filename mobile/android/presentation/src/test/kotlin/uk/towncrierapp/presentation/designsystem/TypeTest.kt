package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

/**
 * Public Notice (epic #848 R5) moves the display/headline M3 roles to
 * Fraunces — the two bundled static instances (Regular/SemiBold, reused
 * byte-for-byte from the iOS bundle) — while body/caption stay Inter
 * unchanged. Mirrors iOS `TCTypography`'s uniform semibold treatment across
 * all three Fraunces roles (only two static weights are bundled, so every
 * Fraunces role settles on SemiBold rather than mixing in a synthesized
 * Bold). These are pure-JVM assertions against the `TextStyle`/`FontFamily`
 * values — no Robolectric, no device (android-coding-standards skill,
 * testing.md).
 */
class TypeTest {
    @Test
    fun `headlineLarge, titleLarge and titleMedium move to Fraunces SemiBold`() {
        assertEquals(FrauncesFontFamily, TownCrierTypography.headlineLarge.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.headlineLarge.fontWeight)

        assertEquals(FrauncesFontFamily, TownCrierTypography.titleLarge.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.titleLarge.fontWeight)

        assertEquals(FrauncesFontFamily, TownCrierTypography.titleMedium.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.titleMedium.fontWeight)
    }

    @Test
    fun `body and caption roles stay Inter, unaffected by the Fraunces move`() {
        assertEquals(InterFontFamily, TownCrierTypography.bodyLarge.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.bodyMedium.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.bodySmall.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.labelMedium.fontFamily)
        assertNotEquals(FrauncesFontFamily, TownCrierTypography.bodyLarge.fontFamily)
    }

    @Test
    fun `mono text style uses the system monospace family with tabular figures`() {
        assertEquals(FontFamily.Monospace, monoTextStyle.fontFamily)
        assertEquals("tnum", monoTextStyle.fontFeatureSettings)
    }
}
