// Color.kt is the bead-mandated file name for the whole token file (epic
// #770): TcPalette below is a supporting type alongside the Light/Dark/
// OledPalette vals that follow it, not the file's sole declaration in spirit.
@file:Suppress("MatchingDeclarationName")

package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.graphics.Color

/**
 * One resolved set of Town Crier color tokens — exact hex values from epic
 * #770 / the design-language skill's token table. [TcPalette] never appears
 * in feature code directly: [TownCrierTheme] maps it onto a Material 3
 * `ColorScheme` (see [colorScheme]) and the extended [TownCrierColors]
 * CompositionLocal (see [extendedColors]) for tokens with no Material role.
 */
internal data class TcPalette(
    val amber: Color,
    val amberMuted: Color,
    val amberHover: Color,
    val background: Color,
    val surface: Color,
    val surfaceElevated: Color,
    val textPrimary: Color,
    val textSecondary: Color,
    val textTertiary: Color,
    val textOnAccent: Color,
    val statusPermitted: Color,
    val statusConditions: Color,
    val statusRejected: Color,
    val statusPending: Color,
    val statusWithdrawn: Color,
    val statusAppealed: Color,
    val border: Color,
    val borderFocused: Color,
    val overlay: Color,
)

private fun muted(amber: Color) = amber.copy(alpha = 0.15f)

internal val LightPalette =
    TcPalette(
        amber = Color(0xFFD4910A),
        amberMuted = muted(Color(0xFFD4910A)),
        amberHover = Color(0xFFB87A08),
        background = Color(0xFFFAF8F5),
        surface = Color(0xFFFFFFFF),
        surfaceElevated = Color(0xFFFFFFFF),
        textPrimary = Color(0xFF1C1917),
        textSecondary = Color(0xFF6B6560),
        textTertiary = Color(0xFFA39E98),
        textOnAccent = Color(0xFFFFFFFF),
        statusPermitted = Color(0xFF1A7D37),
        statusConditions = Color(0xFFB85C00),
        statusRejected = Color(0xFFC42B2B),
        statusPending = Color(0xFFC27A0A),
        statusWithdrawn = Color(0xFF7A7570),
        statusAppealed = Color(0xFF7C3AED),
        border = Color(0xFFE8E4DF),
        borderFocused = Color(0xFFD4910A),
        overlay = Color.Black.copy(alpha = 0.40f),
    )

internal val DarkPalette =
    TcPalette(
        amber = Color(0xFFE9A620),
        amberMuted = muted(Color(0xFFE9A620)),
        amberHover = Color(0xFFF0B83A),
        background = Color(0xFF1A1A1E),
        surface = Color(0xFF242428),
        surfaceElevated = Color(0xFF2E2E33),
        textPrimary = Color(0xFFF1EFE9),
        textSecondary = Color(0xFF9B9590),
        textTertiary = Color(0xFF5C5852),
        textOnAccent = Color(0xFF1C1917),
        statusPermitted = Color(0xFF34C759),
        statusConditions = Color(0xFFFF9F0A),
        statusRejected = Color(0xFFFF453A),
        statusPending = Color(0xFFFFB340),
        statusWithdrawn = Color(0xFF8E8A85),
        statusAppealed = Color(0xFFA78BFA),
        border = Color(0xFF3A3A3F),
        borderFocused = Color(0xFFE9A620),
        overlay = Color.Black.copy(alpha = 0.50f),
    )

// OLED is a dark sub-variant, not a fourth palette: every token matches Dark
// except the three surfaces (and border, which steps darker with them) that
// go true-black. See design-language skill / epic #770.
internal val OledPalette =
    DarkPalette.copy(
        background = Color(0xFF000000),
        surface = Color(0xFF0A0A0A),
        surfaceElevated = Color(0xFF161616),
        border = Color(0xFF1E1E22),
    )
