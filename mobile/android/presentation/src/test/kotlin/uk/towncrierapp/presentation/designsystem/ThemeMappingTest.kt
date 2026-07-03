package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.graphics.Color
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Exact hex assertions for the light/dark/OLED token tables (epic #770,
 * design-language skill). These are pure-JVM tests against
 * [androidx.compose.ui.graphics.Color] value equality — no Robolectric, no
 * device needed (android-coding-standards skill, testing.md).
 */
class ThemeMappingTest {
    @Nested
    inner class LightMapping {
        private val scheme = colorScheme(LightPalette, isDark = false)
        private val extended = extendedColors(LightPalette)

        @Test
        fun `light Material role mapping matches the epic token table`() {
            assertEquals(Color(0xFFD4910A), scheme.primary) // tcAmber
            assertEquals(Color(0xFFFFFFFF), scheme.onPrimary) // tcTextOnAccent
            assertEquals(Color(0xFF1C1917), scheme.onPrimaryContainer) // tcTextPrimary
            assertEquals(Color(0xFFFAF8F5), scheme.background) // tcBackground
            assertEquals(Color(0xFF1C1917), scheme.onBackground) // tcTextPrimary
            assertEquals(Color(0xFFFFFFFF), scheme.surface) // tcSurface
            assertEquals(Color(0xFF1C1917), scheme.onSurface) // tcTextPrimary
            assertEquals(Color(0xFFFFFFFF), scheme.surfaceContainerHigh) // tcSurfaceElevated
            assertEquals(Color(0xFF6B6560), scheme.onSurfaceVariant) // tcTextSecondary
            assertEquals(Color(0xFFE8E4DF), scheme.outline) // tcBorder
            assertEquals(Color(0xFFE8E4DF), scheme.outlineVariant) // tcBorder
            assertEquals(Color(0xFFC42B2B), scheme.error) // tcStatusRejected
            assertEquals(Color(0xFFFFFFFF), scheme.onError) // tcTextOnAccent
            assertEquals(Color.Black.copy(alpha = 0.40f), scheme.scrim) // tcOverlay @40%
        }

        @Test
        fun `light extended tokens match the epic token table`() {
            assertEquals(Color(0xFF1A7D37), extended.statusPermitted)
            assertEquals(Color(0xFFB85C00), extended.statusConditions)
            assertEquals(Color(0xFFC42B2B), extended.statusRejected)
            assertEquals(Color(0xFFC27A0A), extended.statusPending)
            assertEquals(Color(0xFF7A7570), extended.statusWithdrawn)
            assertEquals(Color(0xFF7C3AED), extended.statusAppealed)
            assertEquals(Color(0xFFD4910A).copy(alpha = 0.15f), extended.amberMuted)
            assertEquals(Color.Black.copy(alpha = 0.40f), extended.overlay)
        }
    }

    @Nested
    inner class DarkMapping {
        private val scheme = colorScheme(DarkPalette, isDark = true)
        private val extended = extendedColors(DarkPalette)

        @Test
        fun `dark Material role mapping matches the epic token table`() {
            assertEquals(Color(0xFFE9A620), scheme.primary) // tcAmber
            assertEquals(Color(0xFF1C1917), scheme.onPrimary) // tcTextOnAccent
            assertEquals(Color(0xFFF1EFE9), scheme.onPrimaryContainer) // tcTextPrimary
            assertEquals(Color(0xFF1A1A1E), scheme.background) // tcBackground
            assertEquals(Color(0xFFF1EFE9), scheme.onBackground) // tcTextPrimary
            assertEquals(Color(0xFF242428), scheme.surface) // tcSurface
            assertEquals(Color(0xFFF1EFE9), scheme.onSurface) // tcTextPrimary
            assertEquals(Color(0xFF2E2E33), scheme.surfaceContainerHigh) // tcSurfaceElevated
            assertEquals(Color(0xFF9B9590), scheme.onSurfaceVariant) // tcTextSecondary
            assertEquals(Color(0xFF3A3A3F), scheme.outline) // tcBorder
            assertEquals(Color(0xFF3A3A3F), scheme.outlineVariant) // tcBorder
            assertEquals(Color(0xFFFF453A), scheme.error) // tcStatusRejected
            assertEquals(Color(0xFF1C1917), scheme.onError) // tcTextOnAccent
            assertEquals(Color.Black.copy(alpha = 0.50f), scheme.scrim) // tcOverlay @50%
        }

        @Test
        fun `dark extended tokens match the epic token table`() {
            assertEquals(Color(0xFF34C759), extended.statusPermitted)
            assertEquals(Color(0xFFFF9F0A), extended.statusConditions)
            assertEquals(Color(0xFFFF453A), extended.statusRejected)
            assertEquals(Color(0xFFFFB340), extended.statusPending)
            assertEquals(Color(0xFF8E8A85), extended.statusWithdrawn)
            assertEquals(Color(0xFFA78BFA), extended.statusAppealed)
            assertEquals(Color(0xFFE9A620).copy(alpha = 0.15f), extended.amberMuted)
            assertEquals(Color.Black.copy(alpha = 0.50f), extended.overlay)
        }
    }

    @Nested
    inner class OledMapping {
        private val scheme = colorScheme(OledPalette, isDark = true)
        private val extended = extendedColors(OledPalette)

        @Test
        fun `OLED Material role mapping matches the epic token table`() {
            assertEquals(Color(0xFFE9A620), scheme.primary) // tcAmber (same as dark)
            assertEquals(Color(0xFF1C1917), scheme.onPrimary) // tcTextOnAccent
            assertEquals(Color(0xFF000000), scheme.background) // tcBackground — true black
            assertEquals(Color(0xFFF1EFE9), scheme.onBackground) // tcTextPrimary
            assertEquals(Color(0xFF0A0A0A), scheme.surface) // tcSurface
            assertEquals(Color(0xFFF1EFE9), scheme.onSurface) // tcTextPrimary
            assertEquals(Color(0xFF161616), scheme.surfaceContainerHigh) // tcSurfaceElevated
            assertEquals(Color(0xFF9B9590), scheme.onSurfaceVariant) // tcTextSecondary
            assertEquals(Color(0xFF1E1E22), scheme.outline) // tcBorder
            assertEquals(Color(0xFF1E1E22), scheme.outlineVariant) // tcBorder
            assertEquals(Color(0xFFFF453A), scheme.error) // tcStatusRejected (same as dark)
            assertEquals(Color.Black.copy(alpha = 0.50f), scheme.scrim) // tcOverlay @50% (same as dark)
        }

        @Test
        fun `OLED extended tokens match dark except for surfaces`() {
            assertEquals(Color(0xFF34C759), extended.statusPermitted)
            assertEquals(Color(0xFFA78BFA), extended.statusAppealed)
            assertEquals(Color(0xFFE9A620).copy(alpha = 0.15f), extended.amberMuted)
            assertEquals(Color.Black.copy(alpha = 0.50f), extended.overlay)
        }
    }

    @Nested
    inner class AppearanceResolution {
        @Test
        fun `System appearance follows the system dark flag`() {
            assertEquals(false, resolveIsDark(Appearance.System, systemInDarkTheme = false))
            assertEquals(true, resolveIsDark(Appearance.System, systemInDarkTheme = true))
        }

        @Test
        fun `Light appearance is never dark regardless of the system flag`() {
            assertEquals(false, resolveIsDark(Appearance.Light, systemInDarkTheme = true))
        }

        @Test
        fun `Dark and OledDark appearance are always dark`() {
            assertEquals(true, resolveIsDark(Appearance.Dark, systemInDarkTheme = false))
            assertEquals(true, resolveIsDark(Appearance.OledDark, systemInDarkTheme = false))
        }

        @Test
        fun `OledDark appearance resolves OLED regardless of the oled flag`() {
            assertEquals(true, resolveIsOled(Appearance.OledDark, oled = false, isDark = true))
            assertEquals(true, resolveIsOled(Appearance.OledDark, oled = null, isDark = true))
        }

        @Test
        fun `Dark appearance resolves OLED only when the oled flag is explicitly set`() {
            assertEquals(false, resolveIsOled(Appearance.Dark, oled = null, isDark = true))
            assertEquals(false, resolveIsOled(Appearance.Dark, oled = false, isDark = true))
            assertEquals(true, resolveIsOled(Appearance.Dark, oled = true, isDark = true))
        }

        @Test
        fun `OLED never applies when not dark, regardless of the oled flag`() {
            assertEquals(false, resolveIsOled(Appearance.Light, oled = true, isDark = false))
        }

        @Test
        fun `resolvePalette picks light, dark or OLED per the isDark-then-isOled formula`() {
            assertEquals(LightPalette, resolvePalette(isDark = false, isOled = false))
            assertEquals(LightPalette, resolvePalette(isDark = false, isOled = true)) // OLED is a dark sub-variant
            assertEquals(DarkPalette, resolvePalette(isDark = true, isOled = false))
            assertEquals(OledPalette, resolvePalette(isDark = true, isOled = true))
        }
    }
}
