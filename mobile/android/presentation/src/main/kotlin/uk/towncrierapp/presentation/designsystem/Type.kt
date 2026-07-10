package uk.towncrierapp.presentation.designsystem

import androidx.compose.material3.Typography
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import uk.towncrierapp.presentation.R

/**
 * Inter — the same upstream release (v4.001, git-66647c0bb) the web app
 * self-hosts at `web/public/fonts/inter-latin*.woff2`, bundled here as
 * static-weight TTFs (Regular/Medium/SemiBold/Bold — the four weights the
 * type scale below uses). See `presentation/licenses/Inter-OFL.txt`.
 */
internal val InterFontFamily =
    FontFamily(
        Font(R.font.inter_regular, FontWeight.Normal),
        Font(R.font.inter_medium, FontWeight.Medium),
        Font(R.font.inter_semibold, FontWeight.SemiBold),
        Font(R.font.inter_bold, FontWeight.Bold),
    )

private val baseTypography = Typography()

/**
 * Town Crier's type scale, mapped tc token → Material 3 role (design-language
 * skill). Sizes/line-heights stay at Material 3's defaults for each role —
 * only the font family and weight are Town Crier's; text always renders in
 * sp, never a raw dp, so it respects the user's font-size setting.
 *
 * Sans standardisation (issue #912 Phase 5): all roles use Inter, including
 * the three display/headline roles that previously used a bundled display
 * serif — dropped everywhere per the 2026-07-10 owner-approved decision.
 * They stay at SemiBold (mirrors iOS `TCTypography`'s unchanged weight
 * treatment for those roles).
 *
 * | tc token             | M3 role      | Family   | Weight   |
 * |-----------------------|--------------|----------|----------|
 * | tcDisplayLarge         | headlineLarge| Inter    | SemiBold |
 * | tcDisplaySmall         | titleLarge   | Inter    | SemiBold |
 * | tcHeadline             | titleMedium  | Inter    | SemiBold |
 * | tcBody                 | bodyLarge    | Inter    | Regular  |
 * | tcCaption               | bodySmall    | Inter    | Regular  |
 * | tcCaptionEmphasis       | labelMedium  | Inter    | Medium   |
 *
 * tcBodyEmphasis has no dedicated M3 slot (it shares bodyLarge's metrics with
 * a heavier weight) — see [bodyEmphasisTextStyle], exposed as
 * `TownCrierTheme.bodyEmphasis`. There is likewise no M3 slot for the new
 * mono metadata role (references, dates, distances) — see [monoTextStyle],
 * exposed as `TownCrierTheme.mono`.
 */
internal val TownCrierTypography =
    Typography(
        headlineLarge =
            baseTypography.headlineLarge.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.SemiBold),
        titleLarge = baseTypography.titleLarge.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.SemiBold),
        titleMedium =
            baseTypography.titleMedium.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.SemiBold),
        bodyLarge = baseTypography.bodyLarge.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.Normal),
        bodyMedium = baseTypography.bodyMedium.copy(fontFamily = InterFontFamily),
        bodySmall = baseTypography.bodySmall.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.Normal),
        labelLarge = baseTypography.labelLarge.copy(fontFamily = InterFontFamily),
        labelMedium = baseTypography.labelMedium.copy(fontFamily = InterFontFamily, fontWeight = FontWeight.Medium),
        labelSmall = baseTypography.labelSmall.copy(fontFamily = InterFontFamily),
        displayLarge = baseTypography.displayLarge.copy(fontFamily = InterFontFamily),
        displayMedium = baseTypography.displayMedium.copy(fontFamily = InterFontFamily),
        displaySmall = baseTypography.displaySmall.copy(fontFamily = InterFontFamily),
        headlineMedium = baseTypography.headlineMedium.copy(fontFamily = InterFontFamily),
        headlineSmall = baseTypography.headlineSmall.copy(fontFamily = InterFontFamily),
        titleSmall = baseTypography.titleSmall.copy(fontFamily = InterFontFamily),
    )

/** tcBodyEmphasis: bodyLarge's metrics at SemiBold — see [TownCrierTypography] doc. */
internal val bodyEmphasisTextStyle = TownCrierTypography.bodyLarge.copy(fontWeight = FontWeight.SemiBold)

/**
 * The new mono metadata role (epic #848 R5): planning references, dates, and
 * distances — the system monospace family with tabular figures
 * (`fontFeatureSettings = "tnum"`) so columns of digits align. Built off
 * [TownCrierTypography.bodySmall]'s metrics (the same base [bodyEmphasisTextStyle]
 * uses) rather than a new M3 role, since there is no dedicated slot for it.
 * Exposed as `TownCrierTheme.mono`.
 */
internal val monoTextStyle =
    TownCrierTypography.bodySmall.copy(fontFamily = FontFamily.Monospace, fontFeatureSettings = "tnum")
