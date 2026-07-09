package uk.towncrierapp.presentation.designsystem

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle

/**
 * Town Crier's appearance preference — mirrors the iOS `AppearanceMode` enum
 * as implemented (epic #770 pre-resolved decision). The picker UI lands in a
 * later issue; this is the theme-resolution API only.
 */
public enum class Appearance {
    System,
    Light,
    Dark,
    OledDark,
}

/**
 * Extended Town Crier tokens with no Material 3 role: the six status colors,
 * the muted amber tint, and the modal scrim. Read via
 * `TownCrierTheme.colors.*` — never construct this directly in feature code.
 */
public data class TownCrierColors(
    val statusPermitted: Color,
    val statusConditions: Color,
    val statusRejected: Color,
    val statusPending: Color,
    val statusWithdrawn: Color,
    val statusAppealed: Color,
    val amberMuted: Color,
    val overlay: Color,
    val textTertiary: Color,
)

private val LocalTownCrierColors = staticCompositionLocalOf { extendedColors(LightPalette) }

/**
 * OLED is a dark sub-variant, not a fourth appearance in its own right: an
 * explicit [Appearance.OledDark] choice always resolves OLED, and a plain
 * [Appearance.Dark] resolves OLED only when the separate `oled` preference
 * (e.g. a "True Black" settings toggle) is set. Never true when not dark.
 */
internal fun resolveIsDark(
    appearance: Appearance,
    systemInDarkTheme: Boolean,
): Boolean =
    when (appearance) {
        Appearance.System -> systemInDarkTheme
        Appearance.Light -> false
        Appearance.Dark, Appearance.OledDark -> true
    }

internal fun resolveIsOled(
    appearance: Appearance,
    oled: Boolean?,
    isDark: Boolean,
): Boolean = isDark && (appearance == Appearance.OledDark || oled == true)

/** The formula the epic specifies: `isDark ? (isOled ? oled : dark) : light`. */
internal fun resolvePalette(
    isDark: Boolean,
    isOled: Boolean,
): TcPalette =
    if (isDark) {
        if (isOled) OledPalette else DarkPalette
    } else {
        LightPalette
    }

internal fun colorScheme(
    palette: TcPalette,
    isDark: Boolean,
): ColorScheme {
    val base = if (isDark) darkColorScheme() else lightColorScheme()
    return base.copy(
        primary = palette.amber,
        onPrimary = palette.textOnAccent,
        primaryContainer = palette.amberMuted,
        onPrimaryContainer = palette.textPrimary,
        background = palette.background,
        onBackground = palette.textPrimary,
        surface = palette.surface,
        onSurface = palette.textPrimary,
        surfaceContainerHigh = palette.surfaceElevated,
        onSurfaceVariant = palette.textSecondary,
        outline = palette.border,
        outlineVariant = palette.border,
        error = palette.statusRejected,
        onError = palette.textOnAccent,
        scrim = palette.overlay,
    )
}

internal fun extendedColors(palette: TcPalette): TownCrierColors =
    TownCrierColors(
        statusPermitted = palette.statusPermitted,
        statusConditions = palette.statusConditions,
        statusRejected = palette.statusRejected,
        statusPending = palette.statusPending,
        statusWithdrawn = palette.statusWithdrawn,
        statusAppealed = palette.statusAppealed,
        amberMuted = palette.amberMuted,
        overlay = palette.overlay,
        textTertiary = palette.textTertiary,
    )

/**
 * Town Crier's Material 3 theme. Maps the epic's color tokens onto a
 * [ColorScheme] (see [colorScheme]) and exposes the extended [TownCrierColors]
 * plus the Inter [TownCrierTypography] and [TownCrierShapes]. Dynamic color is
 * off by design — brand colors are pinned, never derived from wallpaper.
 */
@Composable
public fun TownCrierTheme(
    appearance: Appearance = Appearance.System,
    oled: Boolean? = null,
    content: @Composable () -> Unit,
) {
    val isDark = resolveIsDark(appearance, isSystemInDarkTheme())
    val isOled = resolveIsOled(appearance, oled, isDark)
    val palette = resolvePalette(isDark, isOled)

    CompositionLocalProvider(LocalTownCrierColors provides extendedColors(palette)) {
        MaterialTheme(
            colorScheme = colorScheme(palette, isDark),
            typography = TownCrierTypography,
            shapes = TownCrierShapes,
            content = content,
        )
    }
}

/**
 * Mirrors `MaterialTheme.colorScheme`/`.typography` for tokens Material 3 has
 * no slot for: `TownCrierTheme.colors.*` (status colors, amberMuted, overlay),
 * `TownCrierTheme.bodyEmphasis` (tcBodyEmphasis — bodyLarge at SemiBold), and
 * `TownCrierTheme.mono` (the Public Notice mono metadata role — references,
 * dates, distances — epic #848 R5).
 */
public object TownCrierTheme {
    public val colors: TownCrierColors
        @Composable
        get() = LocalTownCrierColors.current

    public val bodyEmphasis: TextStyle
        get() = bodyEmphasisTextStyle

    public val mono: TextStyle
        get() = monoTextStyle
}
