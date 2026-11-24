package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/** Display/headline M3 roles use the same Inter family as body/caption (no serif). */
class TypeTest {
    @Test
    fun `headlineLarge, titleLarge and titleMedium use Inter SemiBold`() {
        assertEquals(InterFontFamily, TownCrierTypography.headlineLarge.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.headlineLarge.fontWeight)

        assertEquals(InterFontFamily, TownCrierTypography.titleLarge.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.titleLarge.fontWeight)

        assertEquals(InterFontFamily, TownCrierTypography.titleMedium.fontFamily)
        assertEquals(FontWeight.SemiBold, TownCrierTypography.titleMedium.fontWeight)
    }

    @Test
    fun `body and caption roles stay Inter, unaffected by the sans standardisation`() {
        assertEquals(InterFontFamily, TownCrierTypography.bodyLarge.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.bodyMedium.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.bodySmall.fontFamily)
        assertEquals(InterFontFamily, TownCrierTypography.labelMedium.fontFamily)
    }

    @Test
    fun `mono text style uses the system monospace family with tabular figures`() {
        assertEquals(FontFamily.Monospace, monoTextStyle.fontFamily)
        assertEquals("tnum", monoTextStyle.fontFeatureSettings)
    }
}
